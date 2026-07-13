DROP TRIGGER IF EXISTS trg_documentation_jobs_updated ON documentation_jobs;

DROP TABLE IF EXISTS documentation_files;
ALTER TABLE IF EXISTS documentation_jobs DROP CONSTRAINT IF EXISTS fk_documentation_jobs_version;
DROP TABLE IF EXISTS documentation_versions;
DROP TABLE IF EXISTS documentation_jobs;
DROP TYPE IF EXISTS documentation_job_status;
