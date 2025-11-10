package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"gitlab.uis.dev/service/gocron/internal/config"
	"gitlab.uis.dev/service/gocron/internal/models"
	"gitlab.uis.dev/service/gocron/internal/storage/postgres"
	"gitlab.uis.dev/service/gocron/internal/worker"
)

// Scheduler handles the core business logic of scheduling and running jobs.
type Scheduler struct {
	log    *slog.Logger
	cfg    config.SchedulerConfig
	store interface{ ExecTx(ctx context.Context, fn func(*postgres.Queries) error) error }
	worker ManagerInterface
	client *http.Client
}

// New creates a new Scheduler.
func New(log *slog.Logger, cfg config.Config, store interface{ ExecTx(ctx context.Context, fn func(*postgres.Queries) error) error }) (*Scheduler, error) {
	s := &Scheduler{
		log:    log,
		cfg:    cfg.Scheduler,
		store:  store,
		client: &http.Client{Timeout: cfg.Scheduler.WebhookTimeout},
	}

	workerManager, err := worker.NewManager(log, cfg.RabbitMQ, cfg.Scheduler, s.processJob)
	if err != nil {
		return nil, fmt.Errorf("failed to create worker manager: %w", err)
	}
	s.worker = workerManager

	return s, nil
}

// Start starts the scheduler's worker manager.
func (s *Scheduler) Start(ctx context.Context) {
	s.worker.Start(ctx)
	s.log.Info("scheduler started")
}

// Stop gracefully stops the scheduler.
func (s *Scheduler) Stop() {
	s.worker.Stop()
	s.log.Info("scheduler stopped")
}

// CreateJob creates a new job, stores it in the database, and schedules it.
func (s *Scheduler) CreateJob(ctx context.Context, req models.CreateJobRequest) (*models.Job, error) {
	webhookJSON, err := json.Marshal(req.Webhook)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal webhook: %w", err)
	}

	delayDuration := time.Duration(req.Delay) * time.Second
	params := postgres.CreateJobParams{
		CustomID:   pgtype.Text{String: *req.CustomID, Valid: req.CustomID != nil},
		Delay:      int32(req.Delay),
		Repeat:     int32(req.Repeat),
		Webhook:    webhookJSON,
		DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(delayDuration), Valid: true},
	}

	var dbJob postgres.Job
	err = s.store.ExecTx(ctx, func(q *postgres.Queries) error {
		var txErr error
		dbJob, txErr = q.CreateJob(ctx, params)
		if txErr != nil {
			return fmt.Errorf("failed to create job in db: %w", txErr)
		}
		return nil
	})

	if err != nil {
		return nil, err // The error is already wrapped
	}

	job, err := dbJobToModel(dbJob)
	if err != nil {
		return nil, err
	}

	// Publish to worker only after the DB transaction is successful
	if err := s.worker.Publish(ctx, job.ID, delayDuration); err != nil {
		// This is a critical failure. The job is in the DB but not in the queue.
		// A background reconciliation process would be needed for a robust system.
		s.log.Error("CRITICAL: failed to publish job after DB transaction", "job_id", job.ID, "error", err)
		return nil, fmt.Errorf("failed to publish job: %w", err)
	}

	s.log.Info("job created and scheduled", "job_id", job.ID, "custom_id", job.CustomID)
	return job, nil
}

// processJob is the core logic for handling a job received from a worker.
func (s *Scheduler) processJob(ctx context.Context, jobID int64) error {
	var jobToReschedule *models.Job
	var rescheduleDelay time.Duration

	err := s.store.ExecTx(ctx, func(q *postgres.Queries) error {
		// 1. Atomically lock the job in the DB
		lockedJob, err := q.ProcessJob(ctx, jobID)
		if err != nil {
			// If no rows are returned, it means another worker got it. This is not an error.
			if errors.Is(err, pgx.ErrNoRows) {
				s.log.Info("job already processed by another worker", "job_id", jobID)
				return nil // Return nil to commit the (empty) transaction
			}
			return fmt.Errorf("failed to process job in db: %w", err)
		}

		s.log.Info("job locked for processing", "job_id", lockedJob.ID)

		job, err := dbJobToModel(lockedJob)
		if err != nil {
			return err // Will cause a rollback
		}

		// 2. Execute the webhook
		log := s.executeWebhook(ctx, job)

		// 3. Create the log entry
		_, err = q.CreateJobLog(ctx, postgres.CreateJobLogParams{
			JobID:       job.ID,
			StartedAt:   pgtype.Timestamptz{Time: log.StartedAt, Valid: true},
			CompletedAt: pgtype.Timestamptz{Time: log.CompletedAt, Valid: true},
			StatusCode:  pgtype.Int4{Int32: log.StatusCode, Valid: true},
			Reason:      pgtype.Text{String: log.Reason, Valid: log.Reason != ""},
			Payload:     pgtype.Text{String: log.Payload, Valid: log.Payload != ""},
			Error:       pgtype.Text{String: log.Error, Valid: log.Error != ""},
			ErrorType:   pgtype.Text{String: log.ErrorType, Valid: log.ErrorType != ""},
		})
		if err != nil {
			return fmt.Errorf("failed to create job log: %w", err) // Will cause a rollback
		}

		// 4. Reschedule or complete the job
		if job.Executions+1 >= job.Repeat {
			// Mark as completed
			_, err = q.UpdateJobStatus(ctx, postgres.UpdateJobStatusParams{
				ID:          job.ID,
				Status:      postgres.JobStatusCOMPLETED,
				CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
			})
			s.log.Info("job completed", "job_id", job.ID)
		} else {
			// Reschedule
			delay := time.Duration(job.Delay) * time.Second
			deadline := time.Now().Add(delay)
			_, err = q.UpdateJobAfterExecution(ctx, postgres.UpdateJobAfterExecutionParams{
				ID:         job.ID,
				Status:     postgres.JobStatusACTIVE,
				UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
				DeadlineAt: pgtype.Timestamptz{Time: deadline, Valid: true},
			})
			if err == nil {
				jobToReschedule = job
				rescheduleDelay = delay
			}
		}
		return err // Return the last error to be handled by ExecTx
	})

	if err != nil {
		return err
	}

	// Publish to worker only after the DB transaction is successful
	if jobToReschedule != nil {
		if pubErr := s.worker.Publish(ctx, jobToReschedule.ID, rescheduleDelay); pubErr != nil {
			s.log.Error("CRITICAL: failed to republish job after DB transaction", "job_id", jobToReschedule.ID, "error", pubErr)
			// The job is in the DB as 'ACTIVE' but not in the queue.
			// A background reconciliation process is needed.
			return fmt.Errorf("failed to republish job %d: %w", jobToReschedule.ID, pubErr)
		}
		s.log.Info("job rescheduled", "job_id", jobToReschedule.ID, "next_run", time.Now().Add(rescheduleDelay))
	}

	return nil
}

func (s *Scheduler) executeWebhook(ctx context.Context, job *models.Job) models.JobLog {
	startedAt := time.Now()
	log := models.JobLog{
		JobID:     job.ID,
		StartedAt: startedAt,
	}

	var reqBody io.Reader
	if job.Webhook.JSON != nil {
		jsonBytes, err := json.Marshal(job.Webhook.JSON)
		if err != nil {
			log.Error = err.Error()
			log.ErrorType = "RequestMarshalError"
			log.CompletedAt = time.Now()
			return log
		}
		reqBody = bytes.NewBuffer(jsonBytes)
	} else if job.Webhook.Data != "" {
		reqBody = bytes.NewBufferString(job.Webhook.Data)
	}

	req, err := http.NewRequestWithContext(ctx, job.Webhook.Method, job.Webhook.URL, reqBody)
	if err != nil {
		log.Error = err.Error()
		log.ErrorType = "RequestCreationError"
		log.CompletedAt = time.Now()
		return log
	}

	for k, v := range job.Webhook.Headers {
		req.Header.Set(k, v)
	}
	if job.Webhook.JSON != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.client.Do(req)
	if err != nil {
		log.Error = err.Error()
		log.ErrorType = "RequestError"
		log.CompletedAt = time.Now()
		return log
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		log.Error = readErr.Error()
		log.ErrorType = "ResponseReadError"
	}

	log.StatusCode = int32(resp.StatusCode)
	log.Reason = resp.Status
	log.Payload = string(body)
	log.CompletedAt = time.Now()

	s.log.Info("webhook executed", "job_id", job.ID, "status", resp.StatusCode)
	return log
}

// dbJobToModel converts a database job object to a domain model object.
func dbJobToModel(dbJob postgres.Job) (*models.Job, error) {
	var webhook models.Webhook
	if err := json.Unmarshal(dbJob.Webhook, &webhook); err != nil {
		return nil, fmt.Errorf("failed to unmarshal webhook: %w", err)
	}

	var customID *string
	if dbJob.CustomID.Valid {
		customID = &dbJob.CustomID.String
	}

	var completedAt *time.Time
	if dbJob.CompletedAt.Valid {
		completedAt = &dbJob.CompletedAt.Time
	}

	return &models.Job{
		ID:          dbJob.ID,
		CustomID:    customID,
		CreatedAt:   dbJob.CreatedAt.Time,
		UpdatedAt:   dbJob.UpdatedAt.Time,
		Delay:       int(dbJob.Delay),
		Repeat:      int(dbJob.Repeat),
		Webhook:     webhook,
		Status:      models.StatusEnum(dbJob.Status),
		Executions:  int(dbJob.Executions),
		DeadlineAt:  dbJob.DeadlineAt.Time,
		CompletedAt: completedAt,
	}, nil
}
