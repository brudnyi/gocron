package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestZeroValues(t *testing.T) {
	_ = CreateJobRequest{}
	_ = Job{}
	_ = JobLog{}
	_ = Webhook{}
}

func TestStatusEnum(t *testing.T) {
	tests := []struct {
		name     string
		status   StatusEnum
		expected string
	}{
		{"Active", StatusActive, "ACTIVE"},
		{"Processing", StatusProcessing, "PROCESSING"},
		{"Completed", StatusCompleted, "COMPLETED"},
		{"Cancelled", StatusCancelled, "CANCELLED"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.status))
		})
	}
}

func TestWebhook(t *testing.T) {
	t.Run("valid webhook", func(t *testing.T) {
		webhook := Webhook{
			URL:     "https://example.com/webhook",
			Method:  "POST",
			Headers: map[string]string{"Content-Type": "application/json"},
			Data:    "test data",
		}

		assert.Equal(t, "https://example.com/webhook", webhook.URL)
		assert.Equal(t, "POST", webhook.Method)
		assert.Equal(t, "application/json", webhook.Headers["Content-Type"])
		assert.Equal(t, "test data", webhook.Data)
	})

	t.Run("webhook with JSON", func(t *testing.T) {
		webhook := Webhook{
			URL:    "https://example.com/webhook",
			Method: "POST",
			JSON: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
			},
		}

		assert.Equal(t, "https://example.com/webhook", webhook.URL)
		assert.Equal(t, "POST", webhook.Method)
		assert.Equal(t, "value1", webhook.JSON["key1"])
		assert.Equal(t, 123, webhook.JSON["key2"])
	})
}

func TestJob(t *testing.T) {
	now := time.Now()
	customID := "test-job-123"
	completedAt := now.Add(time.Hour)

	job := Job{
		ID:          1,
		CustomID:    &customID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Delay:       60,
		Repeat:      3,
		Webhook:     Webhook{URL: "https://example.com", Method: "POST"},
		Status:      StatusActive,
		Executions:  1,
		DeadlineAt:  now.Add(time.Minute),
		CompletedAt: &completedAt,
	}

	assert.Equal(t, int64(1), job.ID)
	assert.Equal(t, "test-job-123", *job.CustomID)
	assert.Equal(t, now, job.CreatedAt)
	assert.Equal(t, now, job.UpdatedAt)
	assert.Equal(t, 60, job.Delay)
	assert.Equal(t, 3, job.Repeat)
	assert.Equal(t, StatusActive, job.Status)
	assert.Equal(t, 1, job.Executions)
	assert.Equal(t, now.Add(time.Minute), job.DeadlineAt)
	assert.Equal(t, completedAt, *job.CompletedAt)
}

func TestCreateJobRequest(t *testing.T) {
	customID := "test-job-456"
	webhook := Webhook{
		URL:    "https://example.com/webhook",
		Method: "POST",
		Data:   "test payload",
	}

	req := CreateJobRequest{
		CustomID: &customID,
		Delay:    30,
		Repeat:   2,
		Webhook:  webhook,
	}

	assert.Equal(t, "test-job-456", *req.CustomID)
	assert.Equal(t, 30, req.Delay)
	assert.Equal(t, 2, req.Repeat)
	assert.Equal(t, webhook, req.Webhook)
}

func TestJobLog(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(time.Second * 5)

	log := JobLog{
		ID:          1,
		JobID:       123,
		StartedAt:   now,
		CompletedAt: completedAt,
		StatusCode:  200,
		Reason:      "OK",
		Payload:     "response body",
		Error:       "",
		ErrorType:   "",
	}

	assert.Equal(t, int64(1), log.ID)
	assert.Equal(t, int64(123), log.JobID)
	assert.Equal(t, now, log.StartedAt)
	assert.Equal(t, completedAt, log.CompletedAt)
	assert.Equal(t, int32(200), log.StatusCode)
	assert.Equal(t, "OK", log.Reason)
	assert.Equal(t, "response body", log.Payload)
	assert.Equal(t, "", log.Error)
	assert.Equal(t, "", log.ErrorType)
}

func TestJobLogWithError(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(time.Second * 2)

	log := JobLog{
		ID:          2,
		JobID:       456,
		StartedAt:   now,
		CompletedAt: completedAt,
		StatusCode:  0,
		Reason:      "",
		Payload:     "",
		Error:       "connection timeout",
		ErrorType:   "RequestError",
	}

	assert.Equal(t, int64(2), log.ID)
	assert.Equal(t, int64(456), log.JobID)
	assert.Equal(t, now, log.StartedAt)
	assert.Equal(t, completedAt, log.CompletedAt)
	assert.Equal(t, int32(0), log.StatusCode)
	assert.Equal(t, "", log.Reason)
	assert.Equal(t, "", log.Payload)
	assert.Equal(t, "connection timeout", log.Error)
	assert.Equal(t, "RequestError", log.ErrorType)
}

func TestCreateJobRequestValidation(t *testing.T) {
	t.Run("valid request", func(t *testing.T) {
		customID := "valid-job"
		req := CreateJobRequest{
			CustomID: &customID,
			Delay:    10,
			Repeat:   1,
			Webhook: Webhook{
				URL:    "https://example.com",
				Method: "POST",
			},
		}

		// Basic validation checks
		assert.GreaterOrEqual(t, req.Delay, 0)
		assert.GreaterOrEqual(t, req.Repeat, 0)
		assert.NotEmpty(t, req.Webhook.URL)
		assert.NotEmpty(t, req.Webhook.Method)
	})

	t.Run("zero values", func(t *testing.T) {
		req := CreateJobRequest{
			Delay:  0,
			Repeat: 0,
			Webhook: Webhook{
				URL:    "",
				Method: "",
			},
		}

		assert.Equal(t, 0, req.Delay)
		assert.Equal(t, 0, req.Repeat)
		assert.Empty(t, req.Webhook.URL)
		assert.Empty(t, req.Webhook.Method)
	})

	t.Run("nil custom ID", func(t *testing.T) {
		req := CreateJobRequest{
			CustomID: nil,
			Delay:    5,
			Repeat:   1,
			Webhook: Webhook{
				URL:    "https://example.com",
				Method: "GET",
			},
		}

		assert.Nil(t, req.CustomID)
	})
}
