-- Perfis: substitui o enum antigo ('admin','user') por ('admin','consultor','desenvolvedor').
-- O role antigo 'user' é migrado para 'consultor'.
ALTER TABLE users ALTER COLUMN role DROP DEFAULT;
ALTER TYPE user_role RENAME TO user_role_old;
CREATE TYPE user_role AS ENUM ('admin', 'consultor', 'desenvolvedor');
ALTER TABLE users
    ALTER COLUMN role TYPE user_role
    USING (CASE role::text WHEN 'user' THEN 'consultor' ELSE role::text END::user_role);
ALTER TABLE users ALTER COLUMN role SET DEFAULT 'consultor';
DROP TYPE user_role_old;

-- Seções ganham um "kind" para distinguir documentação (aba Projeto) de
-- requisitos de desenvolvimento (aba Desenvolvimento). Rotas espelhadas.
CREATE TYPE section_kind AS ENUM ('doc', 'dev');
ALTER TABLE project_sections ADD COLUMN kind section_kind NOT NULL DEFAULT 'doc';
CREATE INDEX idx_project_sections_kind_tree ON project_sections (project_id, kind, parent_id, sort_order);

-- Dev-responsáveis: usuários que podem editar a aba Desenvolvimento de um projeto.
CREATE TABLE project_dev_responsibles (
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    PRIMARY KEY (project_id, user_id)
);

CREATE INDEX idx_project_dev_responsibles_user ON project_dev_responsibles (user_id);
