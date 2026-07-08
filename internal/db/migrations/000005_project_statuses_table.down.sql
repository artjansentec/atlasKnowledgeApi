-- Reverte para o enum project_status. Qualquer status fora dos quatro originais
-- é convertido para 'done' para não violar o enum.
ALTER TABLE projects DROP CONSTRAINT IF EXISTS fk_projects_status;

CREATE TYPE project_status AS ENUM ('active', 'paused', 'done', 'cancelled');
ALTER TABLE projects ALTER COLUMN status DROP DEFAULT;
ALTER TABLE projects
    ALTER COLUMN status TYPE project_status
    USING (CASE
        WHEN status IN ('active', 'paused', 'done', 'cancelled') THEN status
        ELSE 'done'
    END::project_status);
ALTER TABLE projects ALTER COLUMN status SET DEFAULT 'active';

DROP TABLE project_statuses;
