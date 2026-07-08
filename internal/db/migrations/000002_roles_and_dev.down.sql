DROP TABLE IF EXISTS project_dev_responsibles;

DROP INDEX IF EXISTS idx_project_sections_kind_tree;
ALTER TABLE project_sections DROP COLUMN IF EXISTS kind;
DROP TYPE IF EXISTS section_kind;

-- Reverte o enum de perfis para ('admin','user'); consultor/desenvolvedor viram 'user'.
ALTER TABLE users ALTER COLUMN role DROP DEFAULT;
ALTER TYPE user_role RENAME TO user_role_old;
CREATE TYPE user_role AS ENUM ('admin', 'user');
ALTER TABLE users
    ALTER COLUMN role TYPE user_role
    USING (CASE role::text WHEN 'admin' THEN 'admin' ELSE 'user' END::user_role);
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'user';
DROP TYPE user_role_old;
