package scheduler

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"gitlab.uis.dev/service/gocron/internal/config"
	"gitlab.uis.dev/service/gocron/internal/models"
	"gitlab.uis.dev/service/gocron/internal/storage/postgres"
)

// MockWorkerManager is a mock implementation of the ManagerInterface for testing.
type MockWorkerManager struct {
	PublishedJobs map[int64]time.Duration
}

// NewMockWorkerManager creates a new mock worker manager.
func NewMockWorkerManager() *MockWorkerManager {
	return &MockWorkerManager{
		PublishedJobs: make(map[int64]time.Duration),
	}
}

// Publish records the job and its delay.
func (m *MockWorkerManager) Publish(ctx context.Context, jobID int64, delay time.Duration) error {
	m.PublishedJobs[jobID] = delay
	return nil
}

// Start is a no-op for the mock.
func (m *MockWorkerManager) Start(ctx context.Context) {}

// Stop is a no-op for the mock.
func (m *MockWorkerManager) Stop() {}

var (
	testScheduler *Scheduler
	testStore     Storer
	mockWorker    *MockWorkerManager
	testPool      *pgxpool.Pool
)

func TestMain(m *testing.M) {
	// Setup
	cfg, err := config.LoadConfig("../..") // Load config from root
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	// Use a different database for testing if specified, otherwise use the default one
	// It's highly recommended to use a dedicated test database
	testDbUrl := os.Getenv("TEST_DATABASE_URL")
	if testDbUrl == "" {
		testDbUrl = cfg.Postgres.URL
		log.Println("TEST_DATABASE_URL not set, using default database. PLEASE BE CAREFUL.")
	}

	testPool, err = pgxpool.New(context.Background(), testDbUrl)
	if err != nil {
		log.Fatalf("could not connect to database: %v", err)
	}
	defer testPool.Close()

	// Run migrations
	// This is a simplified approach. In a real project, you'd use a migration tool.
	migrations, err := os.ReadFile("../../migrations/000001_create_jobs_table.up.sql")
	if err != nil {
		log.Fatalf("could not read migration 1: %v", err)
	}
	_, err = testPool.Exec(context.Background(), string(migrations))
	if err != nil {
		log.Fatalf("could not run migration 1: %v", err)
	}
	migrations, err = os.ReadFile("../../migrations/000002_create_job_logs_table.up.sql")
	if err != nil {
		log.Fatalf("could not read migration 2: %v", err)
	}
	_, err = testPool.Exec(context.Background(), string(migrations))
	if err != nil {
		log.Fatalf("could not run migration 2: %v", err)
	}

	testStore = postgres.NewStore(testPool)
	mockWorker = NewMockWorkerManager()

	testScheduler = &Scheduler{
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		cfg:    cfg.Scheduler,
		store:  testStore,
		worker: mockWorker,
		client: nil, // HTTP client is not needed for these tests
	}

	// Run tests
	code := m.Run()

	// Teardown
	// You might want to drop the tables or the whole database here
	// For simplicity, we'll just close the pool, which is already deferred.

	os.Exit(code)
}

func cleanup(t *testing.T) {
	// Truncate tables to ensure a clean state for each test
	_, err := testPool.Exec(context.Background(), "TRUNCATE TABLE job_logs, jobs RESTART IDENTITY")
	require.NoError(t, err)
	// Reset the mock worker
	mockWorker.PublishedJobs = make(map[int64]time.Duration)
}

func TestCreateJob(t *testing.T) {
	cleanup(t)
	ctx := context.Background()
	customID := "test-job-123"
	delay := 10

	req := models.CreateJobRequest{
		CustomID: &customID,
		Delay:    delay,
		Repeat:   1,
		Webhook: models.Webhook{
			Method: "POST",
			URL:    "http://example.com/webhook",
		},
	}

	job, err := testScheduler.CreateJob(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, int64(1), job.ID)
	require.Equal(t, *req.CustomID, *job.CustomID)
	require.Equal(t, models.StatusActive, job.Status)

	// Check that the job was "published"
	publishedDelay, ok := mockWorker.PublishedJobs[job.ID]
	require.True(t, ok, "job should have been published")
	require.Equal(t, time.Duration(delay)*time.Second, publishedDelay)

	// Verify in DB
	dbJob, err := testStore.GetJob(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, job.ID, dbJob.ID)
	require.Equal(t, *req.CustomID, dbJob.CustomID.String)
}

func TestProcessJob_Complete(t *testing.T) {
	cleanup(t)
	ctx := context.Background()

	// 1. Create a job that runs once
	req := models.CreateJobRequest{
		Delay:  0,
		Repeat: 1, // Run only once
		Webhook: models.Webhook{
			Method: "GET",
			URL:    "http://example.com",
		},
	}
	job, err := testScheduler.CreateJob(ctx, req)
	require.NoError(t, err)

	// 2. Process the job
	// We need to manually set the HTTP client for the scheduler for this test
	// to simulate the webhook execution.
	testScheduler.client = &http.Client{}
	err = testScheduler.processJob(ctx, job.ID)
	require.NoError(t, err)

	// 3. Verify the job is marked as COMPLETED
	dbJob, err := testStore.GetJob(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, postgres.JobStatusCOMPLETED, dbJob.Status)
	require.True(t, dbJob.CompletedAt.Valid)
	require.Equal(t, int32(1), dbJob.Executions)

	// 4. Verify a log was created
	logs, err := testStore.GetJobLogs(ctx, postgres.GetJobLogsParams{
		JobID: job.ID,
		Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, job.ID, logs[0].JobID)
	require.True(t, logs[0].CompletedAt.Valid)

	// 5. Verify it was not rescheduled
	_, ok := mockWorker.PublishedJobs[job.ID]
	require.False(t, ok, "job should not be rescheduled")
}

func TestProcessJob_Reschedule(t *testing.T) {
	cleanup(t)
	ctx := context.Background()
	delay := 5

	// 1. Create a job that runs twice
	req := models.CreateJobRequest{
		Delay:  delay,
		Repeat: 2, // Run twice
		Webhook: models.Webhook{
			Method: "GET",
			URL:    "http://example.com",
		},
	}
	job, err := testScheduler.CreateJob(ctx, req)
	require.NoError(t, err)

	// Clear the mock worker's state after creation
	mockWorker.PublishedJobs = make(map[int64]time.Duration)

	// 2. Process the job for the first time
	testScheduler.client = &http.Client{}
	err = testScheduler.processJob(ctx, job.ID)
	require.NoError(t, err)

	// 3. Verify the job is still ACTIVE and rescheduled
	dbJob, err := testStore.GetJob(ctx, job.ID)
	require.NoError(t, err)
	require.Equal(t, postgres.JobStatusACTIVE, dbJob.Status)
	require.False(t, dbJob.CompletedAt.Valid)
	require.Equal(t, int32(1), dbJob.Executions)

	// 4. Verify it was rescheduled with the correct delay
	rescheduledDelay, ok := mockWorker.PublishedJobs[job.ID]
	require.True(t, ok, "job should have been rescheduled")
	require.Equal(t, time.Duration(delay)*time.Second, rescheduledDelay)
}
