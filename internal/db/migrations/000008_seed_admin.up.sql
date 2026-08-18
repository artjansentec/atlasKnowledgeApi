-- Usuário inicial (banco vazio ou e-mail ainda inexistente).
-- Senha em texto: 12345 (bcrypt cost 12).
INSERT INTO users (email, password_hash, name, role, is_active)
VALUES (
    'arthur.oliveira@aquila.com.br',
    '$2a$12$r1u.cWoLeqafd6qBSAO8He.sx1u.C10bV1NF57TF/DD9AViv05qSW',
    'Arthur Oliveira',
    'admin',
    TRUE
)
ON CONFLICT (email) DO NOTHING;
