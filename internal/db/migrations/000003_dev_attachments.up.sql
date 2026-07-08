-- Anexos ganham um "kind" para separar arquivos da aba Projeto dos da aba
-- Desenvolvimento. Rotas espelhadas: attachments (project) e dev-attachments (dev).
CREATE TYPE attachment_kind AS ENUM ('project', 'dev');
ALTER TABLE project_attachments ADD COLUMN kind attachment_kind NOT NULL DEFAULT 'project';
CREATE INDEX idx_project_attachments_kind ON project_attachments (project_id, kind, created_at DESC);
