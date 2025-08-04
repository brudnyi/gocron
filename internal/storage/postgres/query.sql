-- name: GetJob :one
SELECT * FROM jobs
WHERE id = $1 LIMIT 1;

-- name: GetJobByCustomID :one
SELECT * FROM jobs
WHERE custom_id = $1 LIMIT 1;

-- name: GetActiveJobs :many
SELECT * FROM jobs
WHERE status = 'ACTIVE';

-- name: CreateJob :one
INSERT INTO jobs (
    custom_id,
    delay,
    repeat,
    webhook,
    deadline_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING *;

-- name: UpdateJobStatus :one
UPDATE jobs
SET
    status = $2,
    updated_at = NOW(),
    completed_at = $3
WHERE id = $1
RETURNING *;

-- name: UpdateJobAfterExecution :one
UPDATE jobs
SET
    status = $2,
    updated_at = $3,
    deadline_at = $4,
    executions = executions + 1
WHERE id = $1
RETURNING *;


-- name: ProcessJob :one
UPDATE jobs
SET
    status = 'PROCESSING',
    updated_at = NOW()
WHERE
    id = $1 AND status = 'ACTIVE'
RETURNING *;

-- name: DeleteJob :exec
DELETE FROM jobs
WHERE id = $1;

-- name: CreateJobLog :one
INSERT INTO job_logs (
    job_id,
    started_at,
    completed_at,
    status_code,
    reason,
    payload,
    error,
    error_type
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetJobLogs :many
SELECT * FROM job_logs
WHERE job_id = $1
ORDER BY id
LIMIT $2
OFFSET $3;
