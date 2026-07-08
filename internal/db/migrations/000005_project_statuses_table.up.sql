-- Tabela de status de projeto: passa a ser a fonte de verdade para os status.
-- Adicionar um novo status vira um simples INSERT (sem migration de enum), e
-- cada status carrega rótulo e cores (texto/fundo) para o front renderizar os badges.
-- color/background aceitam qualquer valor CSS válido (hex, oklch, gradiente,
-- var(--...)), por isso são strings amplas.
CREATE TABLE project_statuses (
    code       VARCHAR(32) PRIMARY KEY,
    label      VARCHAR(64) NOT NULL,
    color      VARCHAR(255) NOT NULL,
    background VARCHAR(255) NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO project_statuses (code, label, color, background, sort_order) VALUES
    ('active',    'Ativo',     '#40a937', '#e4f4e1', 1),
    ('paused',    'Pausado',   '#b45309', '#fef3c7', 2),
    ('done',      'Concluído', '#052e16', '#40a937', 3),
    ('cancelled', 'Cancelado', '#4b5563', '#e5e7eb', 4);

-- Converte projects.status do enum project_status para texto referenciando a tabela.
ALTER TABLE projects ALTER COLUMN status DROP DEFAULT;
ALTER TABLE projects ALTER COLUMN status TYPE VARCHAR(32) USING status::text;
ALTER TABLE projects ALTER COLUMN status SET DEFAULT 'active';
ALTER TABLE projects
    ADD CONSTRAINT fk_projects_status
    FOREIGN KEY (status) REFERENCES project_statuses(code);

DROP TYPE project_status;
