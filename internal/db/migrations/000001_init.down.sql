DROP TRIGGER IF EXISTS trg_project_lessons_updated_at ON project_lessons;
DROP TRIGGER IF EXISTS trg_project_sections_updated_at ON project_sections;
DROP TRIGGER IF EXISTS trg_projects_updated_at ON projects;
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS lesson_tags;
DROP TABLE IF EXISTS project_tech;
DROP TABLE IF EXISTS project_tags;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS project_lessons;
DROP TABLE IF EXISTS project_attachments;
DROP TABLE IF EXISTS files;
DROP TABLE IF EXISTS project_sections;
DROP TABLE IF EXISTS project_members;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS tag_kind;
DROP TYPE IF EXISTS member_role;
DROP TYPE IF EXISTS lesson_type;
DROP TYPE IF EXISTS project_status;
DROP TYPE IF EXISTS user_role;
