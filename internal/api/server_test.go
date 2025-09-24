package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gitlab.uis.dev/service/gocron/internal/scheduler"
)

func TestHealthCheck(t *testing.T) {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	// Pass nil scheduler because health check doesn't touch it
	srv := NewServer(log, (*scheduler.Scheduler)(nil))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
	}
}

func TestCreateJob_InvalidJSON(t *testing.T) {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	srv := NewServer(log, (*scheduler.Scheduler)(nil))

	badJSON := bytes.NewBufferString("{invalid}")
	req := httptest.NewRequest(http.MethodPost, "/jobs", badJSON)
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

