package models

import "time"

// JobLog represents a log entry for a job execution attempt.
type JobLog struct {
	ID          int64     `json:"id"`
	JobID       int64     `json:"job_id"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	StatusCode  int32     `json:"status_code"`
	Reason      string    `json:"reason"`
	Payload     string    `json:"payload"`
	Error       string    `json:"error"`
	ErrorType   string    `json:"error_type"`
}
