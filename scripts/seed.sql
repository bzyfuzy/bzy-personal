-- Dev seed: default admin user
-- Password: admin  (change immediately in any non-local env)
-- Uses pgcrypto crypt() so no external tooling needed.

INSERT INTO users (id, email, username, password_hash, roles, is_active)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'admin@bzy.local',
    'admin',
    crypt('admin', gen_salt('bf', 12)),
    '{admin,user}',
    TRUE
)
ON CONFLICT (email) DO UPDATE
    SET password_hash = EXCLUDED.password_hash,
        roles         = EXCLUDED.roles,
        is_active     = TRUE;
