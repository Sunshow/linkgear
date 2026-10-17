-- linkgear schema (sqlite + postgres compatible; timestamps as TEXT/RFC3339 via GORM or INTEGER epoch — use DATETIME-ish TEXT)

CREATE TABLE IF NOT EXISTS linkers (
    id            TEXT PRIMARY KEY NOT NULL,
    name          TEXT NOT NULL,
    admin_token   TEXT NOT NULL UNIQUE,
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS join_tokens (
    id                 TEXT PRIMARY KEY NOT NULL,
    linker_id          TEXT NOT NULL REFERENCES linkers(id) ON DELETE CASCADE,
    token              TEXT NOT NULL UNIQUE,
    bot_type           TEXT NOT NULL,
    nickname           TEXT NOT NULL,
    expires_at         TIMESTAMP NOT NULL,
    used_at            TIMESTAMP NULL,
    used_by_member_id  TEXT NULL,
    created_at         TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_join_tokens_linker ON join_tokens(linker_id);
CREATE INDEX IF NOT EXISTS idx_join_tokens_token ON join_tokens(token);

CREATE TABLE IF NOT EXISTS members (
    id                    TEXT PRIMARY KEY NOT NULL,
    linker_id             TEXT NOT NULL REFERENCES linkers(id) ON DELETE CASCADE,
    bot_type              TEXT NOT NULL,
    nickname              TEXT NOT NULL,
    webhook_url           TEXT NOT NULL DEFAULT '',
    webhook_auth_header   TEXT NOT NULL DEFAULT '',
    member_token          TEXT NOT NULL UNIQUE,
    status                TEXT NOT NULL DEFAULT 'online',
    last_sent_at          TIMESTAMP NULL,
    last_deliver_ok_at    TIMESTAMP NULL,
    last_deliver_err_at   TIMESTAMP NULL,
    last_deliver_error    TEXT NOT NULL DEFAULT '',
    last_heartbeat_at     TIMESTAMP NULL,
    joined_at             TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_members_linker ON members(linker_id);
CREATE INDEX IF NOT EXISTS idx_members_token ON members(member_token);

CREATE TABLE IF NOT EXISTS messages (
    id              TEXT PRIMARY KEY NOT NULL,
    linker_id       TEXT NOT NULL REFERENCES linkers(id) ON DELETE CASCADE,
    from_member_id  TEXT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    body            TEXT NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_linker_created ON messages(linker_id, created_at);

CREATE TABLE IF NOT EXISTS deliveries (
    id             TEXT PRIMARY KEY NOT NULL,
    message_id     TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    to_member_id   TEXT NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    status         TEXT NOT NULL DEFAULT 'pending',
    attempts       INTEGER NOT NULL DEFAULT 0,
    last_error     TEXT NOT NULL DEFAULT '',
    delivered_at   TIMESTAMP NULL,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_deliveries_status ON deliveries(status);
CREATE INDEX IF NOT EXISTS idx_deliveries_message ON deliveries(message_id);
