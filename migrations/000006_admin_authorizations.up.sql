CREATE TABLE admin_principals (
    user_id TEXT PRIMARY KEY CHECK (length(btrim(user_id)) BETWEEN 1 AND 256),
    active BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE admin_permissions (
    user_id TEXT NOT NULL REFERENCES admin_principals(user_id) ON DELETE CASCADE,
    permission TEXT NOT NULL CHECK (permission IN ('promoter:read','promoter:review','promoter:disable','channel:position:configure')),
    PRIMARY KEY (user_id, permission)
);
