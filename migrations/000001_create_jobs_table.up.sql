--bun:split

CREATE TYPE job_status AS ENUM ('ACTIVE', 'PROCESSING', 'COMPLETED', 'CANCELLED');

CREATE TABLE jobs (
    id BIGSERIAL PRIMARY KEY,
    custom_id TEXT UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delay INTEGER NOT NULL,
    repeat INTEGER NOT NULL DEFAULT 0,
    webhook JSONB NOT NULL,
    status job_status NOT NULL DEFAULT 'ACTIVE',
    executions INTEGER NOT NULL DEFAULT 0,
    deadline_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_jobs_status_deadline_at ON jobs (status, deadline_at);
