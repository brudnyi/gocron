package models

import "time"

// StatusEnum represents the status of a job.
type StatusEnum string

const (
	StatusActive     StatusEnum = "ACTIVE"
	StatusProcessing StatusEnum = "PROCESSING"
	StatusCompleted  StatusEnum = "COMPLETED"
	StatusCancelled  StatusEnum = "CANCELLED"
)

// Webhook represents the HTTP request to be made by a job.
type Webhook struct {
	URL     string            `json:"url" validate:"required,url"`
	Method  string            `json:"method" validate:"required"`
	Headers map[string]string `json:"headers"`
	Data    string            `json:"data"`
	JSON    map[string]interface{} `json:"json"`
}

// Job represents a scheduled task.
type Job struct {
	ID          int64      `json:"id"`
	CustomID    *string    `json:"custom_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Delay       int        `json:"delay"`
	Repeat      int        `json:"repeat"`
	Webhook     Webhook    `json:"webhook"`
	Status      StatusEnum `json:"status"`
	Executions  int        `json:"executions"`
	DeadlineAt  time.Time  `json:"deadline_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// CreateJobRequest represents the request to create a new job.
type CreateJobRequest struct {
	CustomID *string `json:"custom_id"`
	Delay    int     `json:"delay" validate:"gte=0"`
	Repeat   int     `json:"repeat" validate:"gte=0"`
	Webhook  Webhook `json:"webhook" validate:"required"`
}
