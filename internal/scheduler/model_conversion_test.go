package scheduler

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"gitlab.uis.dev/service/gocron/internal/models"
	"gitlab.uis.dev/service/gocron/internal/storage/postgres"
)

func TestDbJobToModel(t *testing.T) {
	wh := models.Webhook{URL: "http://example.com", Method: "GET", Headers: map[string]string{"A": "B"}}
	whBytes, _ := json.Marshal(wh)
	now := time.Now()
	comp := now.Add(time.Minute)

	dbJob := postgres.Job{
		ID:       7,
		CustomID: pgtype.Text{String: "cid", Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt: pgtype.Timestamptz{Time: now, Valid: true},
		Delay:    5,
		Repeat:   3,
		Webhook:  whBytes,
		Status:   postgres.JobStatusACTIVE,
		Executions: 1,
		DeadlineAt: pgtype.Timestamptz{Time: now.Add(5 * time.Second), Valid: true},
		CompletedAt: pgtype.Timestamptz{Time: comp, Valid: true},
	}

	job, err := dbJobToModel(dbJob)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job.ID != dbJob.ID || *job.CustomID != dbJob.CustomID.String {
		t.Fatalf("id/custom mismatch")
	}
	if job.Webhook.URL != wh.URL || job.Webhook.Method != wh.Method || job.Webhook.Headers["A"] != "B" {
		t.Fatalf("webhook mismatch")
	}
	if job.Status != models.StatusEnum(dbJob.Status) {
		t.Fatalf("status mismatch")
	}
	if job.Executions != int(dbJob.Executions) {
		t.Fatalf("executions mismatch")
	}
	if job.CompletedAt == nil || !job.CompletedAt.Equal(comp) {
		t.Fatalf("completedAt mismatch")
	}
}

