-- Configuração global do provedor de IA (fonte de verdade no Atlas).
-- Singleton: sempre uma única linha (id = 1). O Mnemos consome via
-- GET /api/v1/internal/ai-settings e/ou campos multipart na geração.
CREATE TABLE ai_settings (
    id         SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    provider   VARCHAR(64) NOT NULL DEFAULT 'openai',
    model      VARCHAR(255) NOT NULL DEFAULT 'gpt-4o-mini',
    api_key    TEXT NOT NULL DEFAULT '',
    base_url   TEXT NOT NULL DEFAULT '',
    updated_by UUID NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO ai_settings (id) VALUES (1);
