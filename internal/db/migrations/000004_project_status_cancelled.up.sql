-- Adiciona o status 'cancelled' ao enum project_status recriando o tipo,
-- espelhando o padrão usado em 000002 para user_role (evita restrições de
-- ALTER TYPE ... ADD VALUE dentro de transação).
ALTER TABLE projects ALTER COLUMN status DROP DEFAULT;
ALTER TYPE project_status RENAME TO project_status_old;
CREATE TYPE project_status AS ENUM ('active', 'paused', 'done', 'cancelled');
ALTER TABLE projects
    ALTER COLUMN status TYPE project_status
    USING (status::text::project_status);
ALTER TABLE projects ALTER COLUMN status SET DEFAULT 'active';
DROP TYPE project_status_old;
