DROP INDEX IF EXISTS idx_project_attachments_kind;
ALTER TABLE project_attachments DROP COLUMN IF EXISTS kind;
DROP TYPE IF EXISTS attachment_kind;
