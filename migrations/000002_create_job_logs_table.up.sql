CREATE TABLE job_logs (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    status_code INT,
    reason TEXT,
    payload TEXT,
    error TEXT,
    error_type TEXT,
    CONSTRAINT fk_jobs
        FOREIGN KEY(job_id)
        REFERENCES jobs(id)
        ON DELETE CASCADE
);

CREATE INDEX idx_job_logs_job_id ON job_logs (job_id);
