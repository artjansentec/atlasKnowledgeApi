-- Remove 'cancelled' do enum recriando o tipo. Projetos cancelados passam a 'done'.
ALTER TABLE projects ALTER COLUMN status DROP DEFAULT;
ALTER TYPE project_status RENAME TO project_status_old;
CREATE TYPE project_status AS ENUM ('active', 'paused', 'done');
ALTER TABLE projects
    ALTER COLUMN status TYPE project_status
    USING (CASE status::text WHEN 'cancelled' THEN 'done' ELSE status::text END::project_status);
ALTER TABLE projects ALTER COLUMN status SET DEFAULT 'active';
DROP TYPE project_status_old;
