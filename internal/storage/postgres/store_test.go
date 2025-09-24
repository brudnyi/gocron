package postgres

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testPool  *pgxpool.Pool
	testStore Storer
)

func TestMain(m *testing.M) {
	// Setup
	testDbUrl := os.Getenv("TEST_DATABASE_URL")
	if testDbUrl == "" {
		testDbUrl = "postgres://user:password@localhost:5432/cron?sslmode=disable"
		log.Println("TEST_DATABASE_URL not set, using default database. PLEASE BE CAREFUL.")
	}

	var err error
	testPool, err = pgxpool.New(context.Background(), testDbUrl)
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}
	defer testPool.Close()

	// Run migrations
	migrations1, err := os.ReadFile("../../../migrations/000001_create_jobs_table.up.sql")
	if err != nil {
		log.Fatalf("could not read migration 1: %v", err)
	}
	_, err = testPool.Exec(context.Background(), string(migrations1))
	if err != nil {
		log.Fatalf("could not run migration 1: %v", err)
	}

	migrations2, err := os.ReadFile("../../../migrations/000002_create_job_logs_table.up.sql")
	if err != nil {
		log.Fatalf("could not read migration 2: %v", err)
	}
	_, err = testPool.Exec(context.Background(), string(migrations2))
	if err != nil {
		log.Fatalf("could not run migration 2: %v", err)
	}

	testStore = NewStore(testPool)

	// Run tests
	code := m.Run()

	os.Exit(code)
}

func cleanup(t *testing.T) {
	// Truncate tables to ensure a clean state for each test
	_, err := testPool.Exec(context.Background(), "TRUNCATE TABLE job_logs, jobs RESTART IDENTITY")
	require.NoError(t, err)
}

func TestNewStore(t *testing.T) {
	store := NewStore(testPool)
	assert.NotNil(t, store)
	assert.Implements(t, (*Storer)(nil), store)
}

func TestCreateJob(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	t.Run("create job with custom ID", func(t *testing.T) {
		webhook := map[string]interface{}{
			"url":    "https://example.com/webhook",
			"method": "POST",
		}
		webhookBytes, err := json.Marshal(webhook)
		require.NoError(t, err)

		params := CreateJobParams{
			CustomID:   pgtype.Text{String: "test-job-123", Valid: true},
			Delay:      10,
			Repeat:     3,
			Webhook:    webhookBytes,
			DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(10 * time.Second), Valid: true},
		}

		job, err := testStore.CreateJob(ctx, params)
		require.NoError(t, err)

		assert.Equal(t, int64(1), job.ID)
		assert.True(t, job.CustomID.Valid)
		assert.Equal(t, "test-job-123", job.CustomID.String)
		assert.Equal(t, int32(10), job.Delay)
		assert.Equal(t, int32(3), job.Repeat)
		assert.Equal(t, webhookBytes, job.Webhook)
		assert.Equal(t, JobStatusACTIVE, job.Status)
		assert.Equal(t, int32(0), job.Executions)
		assert.True(t, job.CreatedAt.Valid)
		assert.True(t, job.UpdatedAt.Valid)
		assert.True(t, job.DeadlineAt.Valid)
		assert.False(t, job.CompletedAt.Valid)
	})

	t.Run("create job without custom ID", func(t *testing.T) {
		webhook := map[string]interface{}{
			"url":    "https://example.com/webhook2",
			"method": "GET",
		}
		webhookBytes, err := json.Marshal(webhook)
		require.NoError(t, err)

		params := CreateJobParams{
			CustomID:   pgtype.Text{Valid: false},
			Delay:      0,
			Repeat:     1,
			Webhook:    webhookBytes,
			DeadlineAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}

		job, err := testStore.CreateJob(ctx, params)
		require.NoError(t, err)

		assert.Equal(t, int64(2), job.ID)
		assert.False(t, job.CustomID.Valid)
		assert.Equal(t, int32(0), job.Delay)
		assert.Equal(t, int32(1), job.Repeat)
		assert.Equal(t, JobStatusACTIVE, job.Status)
	})
}

func TestGetJob(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	// Create a test job
	webhook := map[string]interface{}{
		"url":    "https://example.com/test",
		"method": "POST",
	}
	webhookBytes, err := json.Marshal(webhook)
	require.NoError(t, err)

	params := CreateJobParams{
		CustomID:   pgtype.Text{String: "get-test-job", Valid: true},
		Delay:      5,
		Repeat:     2,
		Webhook:    webhookBytes,
		DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Second), Valid: true},
	}

	createdJob, err := testStore.CreateJob(ctx, params)
	require.NoError(t, err)

	t.Run("get existing job", func(t *testing.T) {
		job, err := testStore.GetJob(ctx, createdJob.ID)
		require.NoError(t, err)

		assert.Equal(t, createdJob.ID, job.ID)
		assert.Equal(t, createdJob.CustomID, job.CustomID)
		assert.Equal(t, createdJob.Delay, job.Delay)
		assert.Equal(t, createdJob.Repeat, job.Repeat)
		assert.Equal(t, createdJob.Webhook, job.Webhook)
		assert.Equal(t, createdJob.Status, job.Status)
	})

	t.Run("get non-existent job", func(t *testing.T) {
		_, err := testStore.GetJob(ctx, 999)
		assert.Error(t, err)
	})
}

func TestGetJobByCustomID(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	// Create a test job
	webhook := map[string]interface{}{
		"url":    "https://example.com/custom",
		"method": "POST",
	}
	webhookBytes, err := json.Marshal(webhook)
	require.NoError(t, err)

	customID := "custom-id-test"
	params := CreateJobParams{
		CustomID:   pgtype.Text{String: customID, Valid: true},
		Delay:      15,
		Repeat:     1,
		Webhook:    webhookBytes,
		DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(15 * time.Second), Valid: true},
	}

	createdJob, err := testStore.CreateJob(ctx, params)
	require.NoError(t, err)

	t.Run("get by existing custom ID", func(t *testing.T) {
		job, err := testStore.GetJobByCustomID(ctx, pgtype.Text{String: customID, Valid: true})
		require.NoError(t, err)

		assert.Equal(t, createdJob.ID, job.ID)
		assert.Equal(t, customID, job.CustomID.String)
	})

	t.Run("get by non-existent custom ID", func(t *testing.T) {
		_, err := testStore.GetJobByCustomID(ctx, pgtype.Text{String: "non-existent", Valid: true})
		assert.Error(t, err)
	})
}

func TestGetActiveJobs(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	webhook := map[string]interface{}{
		"url":    "https://example.com/active",
		"method": "POST",
	}
	webhookBytes, err := json.Marshal(webhook)
	require.NoError(t, err)

	// Create multiple jobs
	for i := 0; i < 3; i++ {
		params := CreateJobParams{
			CustomID:   pgtype.Text{String: "active-job-" + string(rune(i)), Valid: true},
			Delay:      int32(i * 10),
			Repeat:     1,
			Webhook:    webhookBytes,
			DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(time.Duration(i*10) * time.Second), Valid: true},
		}
		_, err := testStore.CreateJob(ctx, params)
		require.NoError(t, err)
	}

	t.Run("get all active jobs", func(t *testing.T) {
		jobs, err := testStore.GetActiveJobs(ctx)
		require.NoError(t, err)

		assert.Len(t, jobs, 3)
		for _, job := range jobs {
			assert.Equal(t, JobStatusACTIVE, job.Status)
		}
	})

	// Mark one job as completed
	_, err = testStore.UpdateJobStatus(ctx, UpdateJobStatusParams{
		ID:          1,
		Status:      JobStatusCOMPLETED,
		CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	require.NoError(t, err)

	t.Run("get active jobs after completion", func(t *testing.T) {
		jobs, err := testStore.GetActiveJobs(ctx)
		require.NoError(t, err)

		assert.Len(t, jobs, 2)
		for _, job := range jobs {
			assert.Equal(t, JobStatusACTIVE, job.Status)
		}
	})
}

func TestProcessJob(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	webhook := map[string]interface{}{
		"url":    "https://example.com/process",
		"method": "POST",
	}
	webhookBytes, err := json.Marshal(webhook)
	require.NoError(t, err)

	params := CreateJobParams{
		CustomID:   pgtype.Text{String: "process-job", Valid: true},
		Delay:      5,
		Repeat:     2,
		Webhook:    webhookBytes,
		DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Second), Valid: true},
	}

	createdJob, err := testStore.CreateJob(ctx, params)
	require.NoError(t, err)

	t.Run("process active job", func(t *testing.T) {
		job, err := testStore.ProcessJob(ctx, createdJob.ID)
		require.NoError(t, err)

		assert.Equal(t, createdJob.ID, job.ID)
		assert.Equal(t, JobStatusPROCESSING, job.Status)
		assert.True(t, job.UpdatedAt.Time.After(createdJob.UpdatedAt.Time))
	})

	t.Run("process already processing job", func(t *testing.T) {
		_, err := testStore.ProcessJob(ctx, createdJob.ID)
		assert.Error(t, err) // Should fail because job is already PROCESSING
	})

	t.Run("process non-existent job", func(t *testing.T) {
		_, err := testStore.ProcessJob(ctx, 999)
		assert.Error(t, err)
	})
}

func TestUpdateJobAfterExecution(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	webhook := map[string]interface{}{
		"url":    "https://example.com/update",
		"method": "POST",
	}
	webhookBytes, err := json.Marshal(webhook)
	require.NoError(t, err)

	params := CreateJobParams{
		CustomID:   pgtype.Text{String: "update-job", Valid: true},
		Delay:      10,
		Repeat:     3,
		Webhook:    webhookBytes,
		DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(10 * time.Second), Valid: true},
	}

	createdJob, err := testStore.CreateJob(ctx, params)
	require.NoError(t, err)

	t.Run("update job after execution", func(t *testing.T) {
		newDeadline := time.Now().Add(20 * time.Second)
		updateParams := UpdateJobAfterExecutionParams{
			ID:         createdJob.ID,
			Status:     JobStatusACTIVE,
			UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
			DeadlineAt: pgtype.Timestamptz{Time: newDeadline, Valid: true},
		}

		job, err := testStore.UpdateJobAfterExecution(ctx, updateParams)
		require.NoError(t, err)

		assert.Equal(t, createdJob.ID, job.ID)
		assert.Equal(t, JobStatusACTIVE, job.Status)
		assert.Equal(t, int32(1), job.Executions) // Should increment
		assert.True(t, job.UpdatedAt.Time.After(createdJob.UpdatedAt.Time))
		assert.WithinDuration(t, newDeadline, job.DeadlineAt.Time, time.Second)
	})
}

func TestUpdateJobStatus(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	webhook := map[string]interface{}{
		"url":    "https://example.com/status",
		"method": "POST",
	}
	webhookBytes, err := json.Marshal(webhook)
	require.NoError(t, err)

	params := CreateJobParams{
		CustomID:   pgtype.Text{String: "status-job", Valid: true},
		Delay:      5,
		Repeat:     1,
		Webhook:    webhookBytes,
		DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Second), Valid: true},
	}

	createdJob, err := testStore.CreateJob(ctx, params)
	require.NoError(t, err)

	t.Run("update job status to completed", func(t *testing.T) {
		completedAt := time.Now()
		updateParams := UpdateJobStatusParams{
			ID:          createdJob.ID,
			Status:      JobStatusCOMPLETED,
			CompletedAt: pgtype.Timestamptz{Time: completedAt, Valid: true},
		}

		job, err := testStore.UpdateJobStatus(ctx, updateParams)
		require.NoError(t, err)

		assert.Equal(t, createdJob.ID, job.ID)
		assert.Equal(t, JobStatusCOMPLETED, job.Status)
		assert.True(t, job.CompletedAt.Valid)
		assert.WithinDuration(t, completedAt, job.CompletedAt.Time, time.Second)
	})

	t.Run("update job status to cancelled", func(t *testing.T) {
		// Create another job
		params2 := CreateJobParams{
			CustomID:   pgtype.Text{String: "cancel-job", Valid: true},
			Delay:      5,
			Repeat:     1,
			Webhook:    webhookBytes,
			DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Second), Valid: true},
		}

		createdJob2, err := testStore.CreateJob(ctx, params2)
		require.NoError(t, err)

		updateParams := UpdateJobStatusParams{
			ID:          createdJob2.ID,
			Status:      JobStatusCANCELLED,
			CompletedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
		}

		job, err := testStore.UpdateJobStatus(ctx, updateParams)
		require.NoError(t, err)

		assert.Equal(t, createdJob2.ID, job.ID)
		assert.Equal(t, JobStatusCANCELLED, job.Status)
		assert.True(t, job.CompletedAt.Valid)
	})
}

func TestCreateJobLog(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	// First create a job
	webhook := map[string]interface{}{
		"url":    "https://example.com/log",
		"method": "POST",
	}
	webhookBytes, err := json.Marshal(webhook)
	require.NoError(t, err)

	jobParams := CreateJobParams{
		CustomID:   pgtype.Text{String: "log-job", Valid: true},
		Delay:      5,
		Repeat:     1,
		Webhook:    webhookBytes,
		DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Second), Valid: true},
	}

	job, err := testStore.CreateJob(ctx, jobParams)
	require.NoError(t, err)

	t.Run("create successful job log", func(t *testing.T) {
		startedAt := time.Now()
		completedAt := startedAt.Add(2 * time.Second)

		logParams := CreateJobLogParams{
			JobID:       job.ID,
			StartedAt:   pgtype.Timestamptz{Time: startedAt, Valid: true},
			CompletedAt: pgtype.Timestamptz{Time: completedAt, Valid: true},
			StatusCode:  pgtype.Int4{Int32: 200, Valid: true},
			Reason:      pgtype.Text{String: "OK", Valid: true},
			Payload:     pgtype.Text{String: "Success response", Valid: true},
			Error:       pgtype.Text{Valid: false},
			ErrorType:   pgtype.Text{Valid: false},
		}

		log, err := testStore.CreateJobLog(ctx, logParams)
		require.NoError(t, err)

		assert.Equal(t, int64(1), log.ID)
		assert.Equal(t, job.ID, log.JobID)
		assert.True(t, log.StartedAt.Valid)
		assert.True(t, log.CompletedAt.Valid)
		assert.True(t, log.StatusCode.Valid)
		assert.Equal(t, int32(200), log.StatusCode.Int32)
		assert.True(t, log.Reason.Valid)
		assert.Equal(t, "OK", log.Reason.String)
		assert.True(t, log.Payload.Valid)
		assert.Equal(t, "Success response", log.Payload.String)
		assert.False(t, log.Error.Valid)
		assert.False(t, log.ErrorType.Valid)
	})

	t.Run("create error job log", func(t *testing.T) {
		startedAt := time.Now()
		completedAt := startedAt.Add(1 * time.Second)

		logParams := CreateJobLogParams{
			JobID:       job.ID,
			StartedAt:   pgtype.Timestamptz{Time: startedAt, Valid: true},
			CompletedAt: pgtype.Timestamptz{Time: completedAt, Valid: true},
			StatusCode:  pgtype.Int4{Valid: false},
			Reason:      pgtype.Text{Valid: false},
			Payload:     pgtype.Text{Valid: false},
			Error:       pgtype.Text{String: "Connection timeout", Valid: true},
			ErrorType:   pgtype.Text{String: "RequestError", Valid: true},
		}

		log, err := testStore.CreateJobLog(ctx, logParams)
		require.NoError(t, err)

		assert.Equal(t, int64(2), log.ID)
		assert.Equal(t, job.ID, log.JobID)
		assert.False(t, log.StatusCode.Valid)
		assert.False(t, log.Reason.Valid)
		assert.False(t, log.Payload.Valid)
		assert.True(t, log.Error.Valid)
		assert.Equal(t, "Connection timeout", log.Error.String)
		assert.True(t, log.ErrorType.Valid)
		assert.Equal(t, "RequestError", log.ErrorType.String)
	})
}

func TestGetJobLogs(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	// First create a job
	webhook := map[string]interface{}{
		"url":    "https://example.com/getlogs",
		"method": "POST",
	}
	webhookBytes, err := json.Marshal(webhook)
	require.NoError(t, err)

	jobParams := CreateJobParams{
		CustomID:   pgtype.Text{String: "getlogs-job", Valid: true},
		Delay:      5,
		Repeat:     3,
		Webhook:    webhookBytes,
		DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Second), Valid: true},
	}

	job, err := testStore.CreateJob(ctx, jobParams)
	require.NoError(t, err)

	// Create multiple logs
	for i := 0; i < 5; i++ {
		logParams := CreateJobLogParams{
			JobID:       job.ID,
			StartedAt:   pgtype.Timestamptz{Time: time.Now().Add(time.Duration(i) * time.Second), Valid: true},
			CompletedAt: pgtype.Timestamptz{Time: time.Now().Add(time.Duration(i+1) * time.Second), Valid: true},
			StatusCode:  pgtype.Int4{Int32: int32(200 + i), Valid: true},
			Reason:      pgtype.Text{String: "Test " + string(rune(i)), Valid: true},
			Payload:     pgtype.Text{String: "Payload " + string(rune(i)), Valid: true},
			Error:       pgtype.Text{Valid: false},
			ErrorType:   pgtype.Text{Valid: false},
		}
		_, err := testStore.CreateJobLog(ctx, logParams)
		require.NoError(t, err)
	}

	t.Run("get all logs", func(t *testing.T) {
		logs, err := testStore.GetJobLogs(ctx, GetJobLogsParams{
			JobID:  job.ID,
			Limit:  10,
			Offset: 0,
		})
		require.NoError(t, err)

		assert.Len(t, logs, 5)
		for i, log := range logs {
			assert.Equal(t, job.ID, log.JobID)
			assert.Equal(t, int32(200+i), log.StatusCode.Int32)
		}
	})

	t.Run("get logs with limit", func(t *testing.T) {
		logs, err := testStore.GetJobLogs(ctx, GetJobLogsParams{
			JobID:  job.ID,
			Limit:  3,
			Offset: 0,
		})
		require.NoError(t, err)

		assert.Len(t, logs, 3)
	})

	t.Run("get logs with offset", func(t *testing.T) {
		logs, err := testStore.GetJobLogs(ctx, GetJobLogsParams{
			JobID:  job.ID,
			Limit:  10,
			Offset: 2,
		})
		require.NoError(t, err)

		assert.Len(t, logs, 3) // 5 total - 2 offset = 3
		assert.Equal(t, int32(202), logs[0].StatusCode.Int32) // Should start from 3rd log
	})

	t.Run("get logs for non-existent job", func(t *testing.T) {
		logs, err := testStore.GetJobLogs(ctx, GetJobLogsParams{
			JobID:  999,
			Limit:  10,
			Offset: 0,
		})
		require.NoError(t, err)

		assert.Len(t, logs, 0)
	})
}

func TestDeleteJob(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	webhook := map[string]interface{}{
		"url":    "https://example.com/delete",
		"method": "POST",
	}
	webhookBytes, err := json.Marshal(webhook)
	require.NoError(t, err)

	params := CreateJobParams{
		CustomID:   pgtype.Text{String: "delete-job", Valid: true},
		Delay:      5,
		Repeat:     1,
		Webhook:    webhookBytes,
		DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Second), Valid: true},
	}

	job, err := testStore.CreateJob(ctx, params)
	require.NoError(t, err)

	t.Run("delete existing job", func(t *testing.T) {
		err := testStore.DeleteJob(ctx, job.ID)
		assert.NoError(t, err)

		// Verify job is deleted
		_, err = testStore.GetJob(ctx, job.ID)
		assert.Error(t, err)
	})

	t.Run("delete non-existent job", func(t *testing.T) {
		err := testStore.DeleteJob(ctx, 999)
		assert.NoError(t, err) // DELETE should not error for non-existent rows
	})
}

func TestExecTx(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	webhook := map[string]interface{}{
		"url":    "https://example.com/tx",
		"method": "POST",
	}
	webhookBytes, err := json.Marshal(webhook)
	require.NoError(t, err)

	t.Run("successful transaction", func(t *testing.T) {
		var createdJob Job
		var createdLog JobLog

		err := testStore.ExecTx(ctx, func(q *Queries) error {
			// Create job
			jobParams := CreateJobParams{
				CustomID:   pgtype.Text{String: "tx-job", Valid: true},
				Delay:      5,
				Repeat:     1,
				Webhook:    webhookBytes,
				DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Second), Valid: true},
			}

			job, err := q.CreateJob(ctx, jobParams)
			if err != nil {
				return err
			}
			createdJob = job

			// Create log
			logParams := CreateJobLogParams{
				JobID:       job.ID,
				StartedAt:   pgtype.Timestamptz{Time: time.Now(), Valid: true},
				CompletedAt: pgtype.Timestamptz{Time: time.Now().Add(time.Second), Valid: true},
				StatusCode:  pgtype.Int4{Int32: 200, Valid: true},
				Reason:      pgtype.Text{String: "OK", Valid: true},
				Payload:     pgtype.Text{String: "Success", Valid: true},
				Error:       pgtype.Text{Valid: false},
				ErrorType:   pgtype.Text{Valid: false},
			}

			log, err := q.CreateJobLog(ctx, logParams)
			if err != nil {
				return err
			}
			createdLog = log

			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, int64(1), createdJob.ID)
		assert.Equal(t, int64(1), createdLog.ID)
		assert.Equal(t, createdJob.ID, createdLog.JobID)

		// Verify both records exist
		job, err := testStore.GetJob(ctx, createdJob.ID)
		require.NoError(t, err)
		assert.Equal(t, createdJob.ID, job.ID)

		logs, err := testStore.GetJobLogs(ctx, GetJobLogsParams{
			JobID:  createdJob.ID,
			Limit:  10,
			Offset: 0,
		})
		require.NoError(t, err)
		assert.Len(t, logs, 1)
		assert.Equal(t, createdLog.ID, logs[0].ID)
	})

	t.Run("failed transaction rollback", func(t *testing.T) {
		err := testStore.ExecTx(ctx, func(q *Queries) error {
			// Create job
			jobParams := CreateJobParams{
				CustomID:   pgtype.Text{String: "rollback-job", Valid: true},
				Delay:      5,
				Repeat:     1,
				Webhook:    webhookBytes,
				DeadlineAt: pgtype.Timestamptz{Time: time.Now().Add(5 * time.Second), Valid: true},
			}

			_, err := q.CreateJob(ctx, jobParams)
			if err != nil {
				return err
			}

			// Return an error to trigger rollback
			return assert.AnError
		})

		assert.Error(t, err)
		assert.Equal(t, assert.AnError, err)

		// Verify job was not created due to rollback
		_, err = testStore.GetJobByCustomID(ctx, pgtype.Text{String: "rollback-job", Valid: true})
		assert.Error(t, err)
	})
}