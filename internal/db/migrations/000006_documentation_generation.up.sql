CREATE TYPE documentation_job_status AS ENUM (
    'PENDING',
    'VALIDATING',
    'UPLOADING_FILES',
    'WAITING_AI',
    'PROCESSING',
    'COMPLETED',
    'FAILED',
    'CANCELLED'
);

CREATE TABLE documentation_jobs (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_by           UUID NOT NULL REFERENCES users(id),
    status               documentation_job_status NOT NULL DEFAULT 'PENDING',
    progress             INT NOT NULL DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    current_step         VARCHAR(255) NOT NULL DEFAULT '',
    project_name         VARCHAR(255) NOT NULL DEFAULT '',
    description          TEXT NOT NULL DEFAULT '',
    generation_options   JSONB NOT NULL DEFAULT '{}'::jsonb,
    error_message        TEXT NULL,
    version_id           UUID NULL,
    file_count           INT NOT NULL DEFAULT 0,
    total_size_bytes     BIGINT NOT NULL DEFAULT 0,
    started_at           TIMESTAMPTZ NULL,
    finished_at          TIMESTAMPTZ NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documentation_jobs_project ON documentation_jobs (project_id, created_at DESC);
CREATE INDEX idx_documentation_jobs_status ON documentation_jobs (status);
CREATE UNIQUE INDEX idx_documentation_jobs_active_project
    ON documentation_jobs (project_id)
    WHERE status IN ('PENDING', 'VALIDATING', 'UPLOADING_FILES', 'WAITING_AI', 'PROCESSING');

CREATE TABLE documentation_versions (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id           UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    job_id               UUID NOT NULL REFERENCES documentation_jobs(id) ON DELETE RESTRICT,
    created_by           UUID NOT NULL REFERENCES users(id),
    version_number       INT NOT NULL,
    content              JSONB NOT NULL,
    model_used           VARCHAR(255) NOT NULL DEFAULT '',
    language             VARCHAR(64) NOT NULL DEFAULT '',
    processing_ms        BIGINT NOT NULL DEFAULT 0,
    file_count           INT NOT NULL DEFAULT 0,
    total_size_bytes     BIGINT NOT NULL DEFAULT 0,
    generation_options   JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at           TIMESTAMPTZ NULL,
    UNIQUE (project_id, version_number)
);

CREATE INDEX idx_documentation_versions_project
    ON documentation_versions (project_id, version_number DESC)
    WHERE deleted_at IS NULL;

ALTER TABLE documentation_jobs
    ADD CONSTRAINT fk_documentation_jobs_version
    FOREIGN KEY (version_id) REFERENCES documentation_versions(id) ON DELETE SET NULL;

CREATE TABLE documentation_files (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id        UUID NOT NULL REFERENCES documentation_jobs(id) ON DELETE CASCADE,
    version_id    UUID NULL REFERENCES documentation_versions(id) ON DELETE SET NULL,
    file_id       UUID NOT NULL REFERENCES files(id) ON DELETE RESTRICT,
    content_hash  VARCHAR(128) NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_documentation_files_job ON documentation_files (job_id);
CREATE INDEX idx_documentation_files_version ON documentation_files (version_id);

CREATE TRIGGER trg_documentation_jobs_updated
    BEFORE UPDATE ON documentation_jobs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
