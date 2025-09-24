package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.uis.dev/service/gocron/internal/api"
	"gitlab.uis.dev/service/gocron/internal/config"
	"gitlab.uis.dev/service/gocron/internal/models"
	"gitlab.uis.dev/service/gocron/internal/scheduler"
	"gitlab.uis.dev/service/gocron/internal/storage/postgres"
)

// IntegrationTestSuite holds the test environment
type IntegrationTestSuite struct {
	pool      *pgxpool.Pool
	store     postgres.Storer
	scheduler *scheduler.Scheduler
	server    *api.Server
	testSrv   *httptest.Server
}

func setupIntegrationTest(t *testing.T) *IntegrationTestSuite {
	// Setup test database
	testDbUrl := os.Getenv("TEST_DATABASE_URL")
	if testDbUrl == "" {
		testDbUrl = "postgres://user:password@localhost:5432/cron_test?sslmode=disable"
		log.Println("TEST_DATABASE_URL not set, using default test database")
	}

	pool, err := pgxpool.New(context.Background(), testDbUrl)
	require.NoError(t, err)

	// Run migrations
	migrations1, err := os.ReadFile("migrations/000001_create_jobs_table.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(migrations1))
	require.NoError(t, err)

	migrations2, err := os.ReadFile("migrations/000002_create_job_logs_table.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(migrations2))
	require.NoError(t, err)

	// Setup components
	store := postgres.NewStore(pool)
	
	// Create a mock scheduler for integration tests
	mockScheduler := &MockSchedulerForIntegration{
		store:       store,
		createdJobs: make(map[int64]*models.Job),
	}

	server := api.NewServer(nil, mockScheduler)
	testSrv := httptest.NewServer(server)

	return &IntegrationTestSuite{
		pool:      pool,
		store:     store,
		scheduler: nil, // We use mock for integration tests
		server:    server,
		testSrv:   testSrv,
	}
}

func (suite *IntegrationTestSuite) cleanup(t *testing.T) {
	// Clean up database
	_, err := suite.pool.Exec(context.Background(), "TRUNCATE TABLE job_logs, jobs RESTART IDENTITY")
	require.NoError(t, err)
}

func (suite *IntegrationTestSuite) teardown() {
	if suite.testSrv != nil {
		suite.testSrv.Close()
	}
	if suite.pool != nil {
		suite.pool.Close()
	}
}

// MockSchedulerForIntegration implements scheduler interface for integration tests
type MockSchedulerForIntegration struct {
	store       postgres.Storer
	createdJobs map[int64]*models.Job
	nextID      int64
}

func (m *MockSchedulerForIntegration) CreateJob(ctx context.Context, req models.CreateJobRequest) (*models.Job, error) {
	m.nextID++
	
	job := &models.Job{
		ID:         m.nextID,
		CustomID:   req.CustomID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Delay:      req.Delay,
		Repeat:     req.Repeat,
		Webhook:    req.Webhook,
		Status:     models.StatusActive,
		Executions: 0,
		DeadlineAt: time.Now().Add(time.Duration(req.Delay) * time.Second),
	}

	m.createdJobs[m.nextID] = job
	return job, nil
}

func (m *MockSchedulerForIntegration) Start(ctx context.Context) {}
func (m *MockSchedulerForIntegration) Stop()                   {}

func TestIntegrationFullWorkflow(t *testing.T) {
	suite := setupIntegrationTest(t)
	defer suite.teardown()
	suite.cleanup(t)

	t.Run("create and retrieve job via API", func(t *testing.T) {
		// Create job request
		customID := "integration-test-job"
		reqBody := models.CreateJobRequest{
			CustomID: &customID,
			Delay:    10,
			Repeat:   2,
			Webhook: models.Webhook{
				URL:    "https://httpbin.org/post",
				Method: "POST",
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				JSON: map[string]interface{}{
					"test": "integration",
					"timestamp": time.Now().Unix(),
				},
			},
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		// Make HTTP request to create job
		resp, err := http.Post(suite.testSrv.URL+"/jobs", "application/json", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		// Parse response
		var createdJob models.Job
		err = json.NewDecoder(resp.Body).Decode(&createdJob)
		require.NoError(t, err)

		// Verify job details
		assert.Equal(t, int64(1), createdJob.ID)
		assert.Equal(t, customID, *createdJob.CustomID)
		assert.Equal(t, 10, createdJob.Delay)
		assert.Equal(t, 2, createdJob.Repeat)
		assert.Equal(t, models.StatusActive, createdJob.Status)
		assert.Equal(t, "https://httpbin.org/post", createdJob.Webhook.URL)
		assert.Equal(t, "POST", createdJob.Webhook.Method)
		assert.Equal(t, "integration", createdJob.Webhook.JSON["test"])
	})
}

func TestIntegrationAPIEndpoints(t *testing.T) {
	suite := setupIntegrationTest(t)
	defer suite.teardown()
	suite.cleanup(t)

	t.Run("health check endpoint", func(t *testing.T) {
		resp, err := http.Get(suite.testSrv.URL + "/")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var response map[string]string
		err = json.NewDecoder(resp.Body).Decode(&response)
		require.NoError(t, err)
		assert.Equal(t, "ok", response["status"])
	})

	t.Run("create job with minimal data", func(t *testing.T) {
		reqBody := models.CreateJobRequest{
			Delay:  0,
			Repeat: 1,
			Webhook: models.Webhook{
				URL:    "https://example.com/webhook",
				Method: "GET",
			},
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		resp, err := http.Post(suite.testSrv.URL+"/jobs", "application/json", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		var createdJob models.Job
		err = json.NewDecoder(resp.Body).Decode(&createdJob)
		require.NoError(t, err)

		assert.Greater(t, createdJob.ID, int64(0))
		assert.Nil(t, createdJob.CustomID)
		assert.Equal(t, 0, createdJob.Delay)
		assert.Equal(t, 1, createdJob.Repeat)
	})

	t.Run("create job with invalid JSON", func(t *testing.T) {
		resp, err := http.Post(suite.testSrv.URL+"/jobs", "application/json", bytes.NewBufferString("invalid json"))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResponse map[string]string
		err = json.NewDecoder(resp.Body).Decode(&errorResponse)
		require.NoError(t, err)
		assert.Equal(t, "Invalid request payload", errorResponse["error"])
	})

	t.Run("unsupported HTTP method", func(t *testing.T) {
		req, err := http.NewRequest("DELETE", suite.testSrv.URL+"/jobs", nil)
		require.NoError(t, err)

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("non-existent endpoint", func(t *testing.T) {
		resp, err := http.Get(suite.testSrv.URL + "/nonexistent")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

func TestIntegrationJobCreationVariations(t *testing.T) {
	suite := setupIntegrationTest(t)
	defer suite.teardown()
	suite.cleanup(t)

	testCases := []struct {
		name        string
		request     models.CreateJobRequest
		expectError bool
	}{
		{
			name: "job with data payload",
			request: models.CreateJobRequest{
				Delay:  5,
				Repeat: 1,
				Webhook: models.Webhook{
					URL:    "https://example.com/data",
					Method: "POST",
					Data:   "raw data payload",
				},
			},
			expectError: false,
		},
		{
			name: "job with JSON payload",
			request: models.CreateJobRequest{
				Delay:  15,
				Repeat: 3,
				Webhook: models.Webhook{
					URL:    "https://example.com/json",
					Method: "PUT",
					JSON: map[string]interface{}{
						"action": "update",
						"id":     12345,
						"data": map[string]interface{}{
							"name":  "test",
							"value": 42.5,
						},
					},
				},
			},
			expectError: false,
		},
		{
			name: "job with custom headers",
			request: models.CreateJobRequest{
				Delay:  30,
				Repeat: 1,
				Webhook: models.Webhook{
					URL:    "https://api.example.com/endpoint",
					Method: "POST",
					Headers: map[string]string{
						"Authorization": "Bearer token123",
						"User-Agent":    "GoCron/1.0",
						"X-Custom":      "custom-value",
					},
					Data: "authenticated request",
				},
			},
			expectError: false,
		},
		{
			name: "job with long delay",
			request: models.CreateJobRequest{
				Delay:  86400, // 24 hours
				Repeat: 1,
				Webhook: models.Webhook{
					URL:    "https://example.com/delayed",
					Method: "GET",
				},
			},
			expectError: false,
		},
		{
			name: "job with many repeats",
			request: models.CreateJobRequest{
				Delay:  1,
				Repeat: 100,
				Webhook: models.Webhook{
					URL:    "https://example.com/repeat",
					Method: "POST",
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonBody, err := json.Marshal(tc.request)
			require.NoError(t, err)

			resp, err := http.Post(suite.testSrv.URL+"/jobs", "application/json", bytes.NewBuffer(jsonBody))
			require.NoError(t, err)
			defer resp.Body.Close()

			if tc.expectError {
				assert.NotEqual(t, http.StatusCreated, resp.StatusCode)
			} else {
				assert.Equal(t, http.StatusCreated, resp.StatusCode)

				var createdJob models.Job
				err = json.NewDecoder(resp.Body).Decode(&createdJob)
				require.NoError(t, err)

				assert.Greater(t, createdJob.ID, int64(0))
				assert.Equal(t, tc.request.Delay, createdJob.Delay)
				assert.Equal(t, tc.request.Repeat, createdJob.Repeat)
				assert.Equal(t, tc.request.Webhook.URL, createdJob.Webhook.URL)
				assert.Equal(t, tc.request.Webhook.Method, createdJob.Webhook.Method)
				assert.Equal(t, models.StatusActive, createdJob.Status)
			}
		})
	}
}

func TestIntegrationConcurrentRequests(t *testing.T) {
	suite := setupIntegrationTest(t)
	defer suite.teardown()
	suite.cleanup(t)

	t.Run("concurrent job creation", func(t *testing.T) {
		const numJobs = 10
		results := make(chan models.Job, numJobs)
		errors := make(chan error, numJobs)

		// Create jobs concurrently
		for i := 0; i < numJobs; i++ {
			go func(jobIndex int) {
				customID := fmt.Sprintf("concurrent-job-%d", jobIndex)
				reqBody := models.CreateJobRequest{
					CustomID: &customID,
					Delay:    jobIndex,
					Repeat:   1,
					Webhook: models.Webhook{
						URL:    fmt.Sprintf("https://example.com/job-%d", jobIndex),
						Method: "POST",
						JSON: map[string]interface{}{
							"index": jobIndex,
						},
					},
				}

				jsonBody, err := json.Marshal(reqBody)
				if err != nil {
					errors <- err
					return
				}

				resp, err := http.Post(suite.testSrv.URL+"/jobs", "application/json", bytes.NewBuffer(jsonBody))
				if err != nil {
					errors <- err
					return
				}
				defer resp.Body.Close()

				if resp.StatusCode != http.StatusCreated {
					errors <- fmt.Errorf("unexpected status code: %d", resp.StatusCode)
					return
				}

				var job models.Job
				err = json.NewDecoder(resp.Body).Decode(&job)
				if err != nil {
					errors <- err
					return
				}

				results <- job
			}(i)
		}

		// Collect results
		var jobs []models.Job
		var errs []error

		for i := 0; i < numJobs; i++ {
			select {
			case job := <-results:
				jobs = append(jobs, job)
			case err := <-errors:
				errs = append(errs, err)
			case <-time.After(10 * time.Second):
				t.Fatal("timeout waiting for concurrent requests")
			}
		}

		// Verify results
		assert.Empty(t, errs, "no errors should occur during concurrent creation")
		assert.Len(t, jobs, numJobs)

		// Verify all jobs have unique IDs
		ids := make(map[int64]bool)
		for _, job := range jobs {
			assert.False(t, ids[job.ID], "job ID should be unique")
			ids[job.ID] = true
			assert.Greater(t, job.ID, int64(0))
			assert.Equal(t, models.StatusActive, job.Status)
		}
	})
}

func TestIntegrationMiddleware(t *testing.T) {
	suite := setupIntegrationTest(t)
	defer suite.teardown()
	suite.cleanup(t)

	t.Run("request ID middleware", func(t *testing.T) {
		resp, err := http.Get(suite.testSrv.URL + "/")
		require.NoError(t, err)
		defer resp.Body.Close()

		requestID := resp.Header.Get("X-Request-Id")
		assert.NotEmpty(t, requestID, "request ID should be present")
		assert.Len(t, requestID, 20, "request ID should have expected length") // chi middleware generates 20-char IDs
	})

	t.Run("content type handling", func(t *testing.T) {
		reqBody := models.CreateJobRequest{
			Delay:  1,
			Repeat: 1,
			Webhook: models.Webhook{
				URL:    "https://example.com",
				Method: "GET",
			},
		}

		jsonBody, err := json.Marshal(reqBody)
		require.NoError(t, err)

		resp, err := http.Post(suite.testSrv.URL+"/jobs", "application/json", bytes.NewBuffer(jsonBody))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	})
}

func TestIntegrationErrorHandling(t *testing.T) {
	suite := setupIntegrationTest(t)
	defer suite.teardown()
	suite.cleanup(t)

	t.Run("malformed JSON", func(t *testing.T) {
		malformedJSON := `{"delay": 5, "repeat": 1, "webhook": {"url": "https://example.com", "method": "POST"`
		
		resp, err := http.Post(suite.testSrv.URL+"/jobs", "application/json", bytes.NewBufferString(malformedJSON))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		var errorResponse map[string]string
		err = json.NewDecoder(resp.Body).Decode(&errorResponse)
		require.NoError(t, err)
		assert.Equal(t, "Invalid request payload", errorResponse["error"])
	})

	t.Run("empty request body", func(t *testing.T) {
		resp, err := http.Post(suite.testSrv.URL+"/jobs", "application/json", bytes.NewBuffer(nil))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("wrong content type", func(t *testing.T) {
		resp, err := http.Post(suite.testSrv.URL+"/jobs", "text/plain", bytes.NewBufferString("not json"))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}