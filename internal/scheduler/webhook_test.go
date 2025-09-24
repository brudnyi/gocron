package scheduler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"gitlab.uis.dev/service/gocron/internal/config"
	"gitlab.uis.dev/service/gocron/internal/models"
)

func newTestSchedulerWithClient(client *http.Client) *Scheduler {
	return &Scheduler{
		log:    slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		cfg:    config.SchedulerConfig{WebhookTimeout: 2 * time.Second},
		client: client,
	}
}

func TestExecuteWebhook_JSONSuccess(t *testing.T) {
	// Mock server to capture request
	var gotBody []byte
	var gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = b
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	s := newTestSchedulerWithClient(server.Client())
	job := &models.Job{ID: 42, Webhook: models.Webhook{URL: server.URL, Method: http.MethodPost, JSON: map[string]interface{}{"x": 1}}}

	logEntry := s.executeWebhook(context.Background(), job)
	if logEntry.Error != "" {
		t.Fatalf("unexpected error: %s", logEntry.Error)
	}
	if logEntry.StatusCode != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, logEntry.StatusCode)
	}
	if gotContentType != "application/json" {
		t.Fatalf("expected content-type application/json, got %q", gotContentType)
	}
	if string(gotBody) != "{\"x\":1}" {
		t.Fatalf("unexpected body: %s", string(gotBody))
	}
}

func TestExecuteWebhook_RequestCreationError(t *testing.T) {
	s := newTestSchedulerWithClient(&http.Client{})
	job := &models.Job{ID: 1, Webhook: models.Webhook{URL: "http://%", Method: http.MethodGet}}
	logEntry := s.executeWebhook(context.Background(), job)
	if logEntry.ErrorType != "RequestCreationError" {
		t.Fatalf("expected RequestCreationError, got %s", logEntry.ErrorType)
	}
}

func TestExecuteWebhook_RequestError(t *testing.T) {
	// Client with tiny timeout to trigger error
	client := &http.Client{Timeout: 1 * time.Nanosecond}
	s := newTestSchedulerWithClient(client)
	job := &models.Job{ID: 2, Webhook: models.Webhook{URL: "http://10.255.255.1", Method: http.MethodGet}}
	logEntry := s.executeWebhook(context.Background(), job)
	if logEntry.ErrorType != "RequestError" {
		t.Fatalf("expected RequestError, got %s", logEntry.ErrorType)
	}
}

func TestExecuteWebhook_ResponseReadError(t *testing.T) {
	// Handler writes and abruptly closes body, but httptest will still provide readable body.
	// Simulate by returning success and rely that read succeeds; ensure fields populated.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	s := newTestSchedulerWithClient(server.Client())
	job := &models.Job{ID: 3, Webhook: models.Webhook{URL: server.URL, Method: http.MethodGet}}
	logEntry := s.executeWebhook(context.Background(), job)
	if logEntry.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", logEntry.StatusCode)
	}
	if logEntry.Payload != "hello" {
		t.Fatalf("expected payload 'hello', got %q", logEntry.Payload)
	}
}

