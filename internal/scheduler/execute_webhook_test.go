package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
	"gitlab.uis.dev/service/gocron/internal/models"
	"gitlab.uis.dev/service/gocron/internal/storage/postgres"
)

func newTestSchedulerWithClient(client *http.Client) *Scheduler {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &Scheduler{
		log:    logger,
		client: client,
	}
}

func TestExecuteWebhook_Success_GET(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	defer ts.Close()

	s := newTestSchedulerWithClient(ts.Client())
	job := &models.Job{
		ID: 1,
		Webhook: models.Webhook{
			Method: "GET",
			URL:    ts.URL,
		},
	}

	log := s.executeWebhook(context.Background(), job)
	require.Equal(t, int32(200), log.StatusCode)
	require.Equal(t, "200 OK", log.Reason)
	require.Equal(t, "hello", log.Payload)
	require.Empty(t, log.Error)
	require.False(t, log.StartedAt.IsZero())
	require.False(t, log.CompletedAt.IsZero())
}

func TestExecuteWebhook_JSON_SetsContentTypeAndBody(t *testing.T) {
	t.Parallel()

	var receivedCT string
	var receivedBody []byte

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCT = r.Header.Get("Content-Type")
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		_, _ = w.Write(receivedBody) // echo back
	}))
	defer ts.Close()

	payload := map[string]interface{}{"a": "b", "n": 123.0}
	s := newTestSchedulerWithClient(ts.Client())
	job := &models.Job{
		ID: 2,
		Webhook: models.Webhook{
			Method: "POST",
			URL:    ts.URL,
			JSON:   payload,
			Headers: map[string]string{
				"X-Custom": "yes",
			},
		},
	}

	log := s.executeWebhook(context.Background(), job)
	expectedJSON, _ := json.Marshal(payload)

	require.Equal(t, "application/json", receivedCT)
	require.Equal(t, string(expectedJSON), string(receivedBody))
	require.Equal(t, int32(200), log.StatusCode)
	require.Equal(t, "200 OK", log.Reason)
	require.Equal(t, string(expectedJSON), log.Payload)
	require.Empty(t, log.Error)
}

func TestExecuteWebhook_Data_UsesRawBodyAndHeaders(t *testing.T) {
	t.Parallel()

	var hdr string
	var body string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hdr = r.Header.Get("X-Token")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		_, _ = w.Write([]byte("ok:" + body + ":" + hdr))
	}))
	defer ts.Close()

	s := newTestSchedulerWithClient(ts.Client())
	job := &models.Job{
		ID: 3,
		Webhook: models.Webhook{
			Method: "PUT",
			URL:    ts.URL,
			Data:   "raw-data",
			Headers: map[string]string{
				"X-Token": "abc123",
			},
		},
	}

	log := s.executeWebhook(context.Background(), job)
	require.Equal(t, "abc123", hdr)
	require.Equal(t, "raw-data", body)
	require.Equal(t, "ok:raw-data:abc123", log.Payload)
	require.Equal(t, int32(200), log.StatusCode)
	require.Empty(t, log.Error)
}

func TestExecuteWebhook_RequestCreationError(t *testing.T) {
	t.Parallel()

	// Invalid method triggers request creation error
	s := newTestSchedulerWithClient(http.DefaultClient)
	job := &models.Job{
		ID: 4,
		Webhook: models.Webhook{
			Method: "",
			URL:    "http://example.com",
		},
	}

	log := s.executeWebhook(context.Background(), job)
	require.Equal(t, "RequestCreationError", log.ErrorType)
	require.NotEmpty(t, log.Error)
	require.True(t, log.CompletedAt.After(log.StartedAt) || log.CompletedAt.Equal(log.StartedAt))
	require.Zero(t, log.StatusCode)
}

type errorRoundTripper struct{}

func (e errorRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("transport failure")
}

func TestExecuteWebhook_RequestError(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: errorRoundTripper{}}
	s := newTestSchedulerWithClient(client)
	job := &models.Job{
		ID: 5,
		Webhook: models.Webhook{
			Method: "GET",
			URL:    "http://nonexistent.invalid",
		},
	}

	log := s.executeWebhook(context.Background(), job)
	require.Equal(t, "RequestError", log.ErrorType)
	require.NotEmpty(t, log.Error)
	require.True(t, log.CompletedAt.After(log.StartedAt) || log.CompletedAt.Equal(log.StartedAt))
}

type brokenReadCloser struct{}

func (b brokenReadCloser) Read(_ []byte) (int, error) { return 0, fmt.Errorf("read error") }
func (b brokenReadCloser) Close() error               { return nil }

type responseRoundTripper struct {
	status int
}

func (r responseRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: r.status,
		Status:     fmt.Sprintf("%d %s", r.status, http.StatusText(r.status)),
		Header:     make(http.Header),
		Body:       brokenReadCloser{},
		Request:    req,
	}, nil
}

func TestExecuteWebhook_ResponseReadError(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: responseRoundTripper{status: 202}}
	s := newTestSchedulerWithClient(client)
	job := &models.Job{
		ID: 6,
		Webhook: models.Webhook{
			Method: "POST",
			URL:    "http://example.com/any",
			Data:   "ignored",
		},
	}

	log := s.executeWebhook(context.Background(), job)
	require.Equal(t, "ResponseReadError", log.ErrorType)
	require.Equal(t, int32(202), log.StatusCode)
	require.Equal(t, "202 Accepted", log.Reason)
	require.True(t, log.CompletedAt.After(log.StartedAt) || log.CompletedAt.Equal(log.StartedAt))
}

func TestExecuteWebhook_RequestMarshalError(t *testing.T) {
	t.Parallel()

	// JSON with non-marshalable value (channel) to force marshal error
	bad := map[string]interface{}{"ch": make(chan int)}
	s := newTestSchedulerWithClient(http.DefaultClient)
	job := &models.Job{
		ID: 7,
		Webhook: models.Webhook{
			Method: "POST",
			URL:    "http://example.com",
			JSON:   bad,
		},
	}

	log := s.executeWebhook(context.Background(), job)
	require.Equal(t, "RequestMarshalError", log.ErrorType)
	require.NotEmpty(t, log.Error)
	require.True(t, log.CompletedAt.After(log.StartedAt) || log.CompletedAt.Equal(log.StartedAt))
}

func TestDBJobToModel_Success(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(30 * time.Second)

	webhook := models.Webhook{
		URL:    "http://example.com",
		Method: "GET",
		Headers: map[string]string{
			"A": "B",
		},
	}
	webhookBytes, _ := json.Marshal(webhook)

	dbJob := postgres.Job{
		ID:         42,
		CustomID:   pgtype.Text{String: "custom-42", Valid: true},
		CreatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		Delay:      5,
		Repeat:     3,
		Webhook:    webhookBytes,
		Status:     postgres.JobStatusACTIVE,
		Executions: 1,
		DeadlineAt: pgtype.Timestamptz{Time: deadline, Valid: true},
		CompletedAt: pgtype.Timestamptz{
			// Intentionally not valid to ensure pointer is nil
			Valid: false,
		},
	}

	job, err := dbJobToModel(dbJob)
	require.NoError(t, err)
	require.NotNil(t, job)
	require.Equal(t, int64(42), job.ID)
	require.NotNil(t, job.CustomID)
	require.Equal(t, "custom-42", *job.CustomID)
	require.Equal(t, now, job.CreatedAt)
	require.Equal(t, now, job.UpdatedAt)
	require.Equal(t, 5, job.Delay)
	require.Equal(t, 3, job.Repeat)
	require.Equal(t, models.StatusActive, job.Status)
	require.Equal(t, 1, job.Executions)
	require.Equal(t, deadline, job.DeadlineAt)
	require.Nil(t, job.CompletedAt)
	require.Equal(t, webhook, job.Webhook)
}

func TestDBJobToModel_HandlesCompletedAndNilCustomID(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	webhook := models.Webhook{URL: "http://example.com", Method: "POST"}
	webhookBytes, _ := json.Marshal(webhook)

	completedAt := now.Add(time.Minute)

	dbJob := postgres.Job{
		ID:         7,
		CustomID:   pgtype.Text{Valid: false},
		CreatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Time: now, Valid: true},
		Delay:      0,
		Repeat:     1,
		Webhook:    webhookBytes,
		Status:     postgres.JobStatusCOMPLETED,
		Executions: 1,
		DeadlineAt: pgtype.Timestamptz{Time: now, Valid: true},
		CompletedAt: pgtype.Timestamptz{
			Time:  completedAt,
			Valid: true,
		},
	}

	job, err := dbJobToModel(dbJob)
	require.NoError(t, err)
	require.Nil(t, job.CustomID)
	require.NotNil(t, job.CompletedAt)
	require.WithinDuration(t, completedAt, *job.CompletedAt, time.Second)
	require.Equal(t, models.StatusCompleted, job.Status)
}

func TestDBJobToModel_InvalidWebhookJSON(t *testing.T) {
	t.Parallel()

	dbJob := postgres.Job{
		ID:         1,
		CustomID:   pgtype.Text{String: "x", Valid: true},
		CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		Delay:      1,
		Repeat:     1,
		Webhook:    []byte("{invalid-json"),
		Status:     postgres.JobStatusACTIVE,
		Executions: 0,
		DeadlineAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}

	job, err := dbJobToModel(dbJob)
	require.Nil(t, job)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal webhook")
}

// Ensure JSON takes precedence over Data
func TestExecuteWebhook_JSONPrecedenceOverData(t *testing.T) {
	t.Parallel()

	var reqBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		reqBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	s := newTestSchedulerWithClient(ts.Client())
	jsonPayload := map[string]interface{}{"x": "y"}
	dataPayload := "should-not-be-used"
	job := &models.Job{
		ID: 99,
		Webhook: models.Webhook{
			Method: "POST",
			URL:    ts.URL,
			JSON:   jsonPayload,
			Data:   dataPayload,
		},
	}

	_ = s.executeWebhook(context.Background(), job)
	expected, _ := json.Marshal(jsonPayload)
	require.Equal(t, string(expected), string(reqBody))
	require.NotEqual(t, dataPayload, string(reqBody))
}
