-- +goose Up
-- +goose StatementBegin
--
-- Single migration for the entire current schema.
-- Fluids are treated as items (classification = "fluid"), stored in
-- player_inventory alongside all other item types.
--

CREATE TABLE IF NOT EXISTS users (
    id            INTEGER  PRIMARY KEY AUTOINCREMENT,
    username      TEXT     NOT NULL UNIQUE,
    email         TEXT     NOT NULL DEFAULT '',
    password_hash TEXT     NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS users_username_idx ON users (username);
CREATE UNIQUE INDEX IF NOT EXISTS users_email_idx ON users (email) WHERE email <> '';

CREATE TABLE IF NOT EXISTS sessions (
    id           TEXT     PRIMARY KEY,
    user_id      INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT     NOT NULL UNIQUE,
    user_agent   TEXT     NOT NULL DEFAULT '',
    ip           TEXT     NOT NULL DEFAULT '',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at   DATETIME NOT NULL,
    revoked_at   DATETIME
);

CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions (user_id);
CREATE INDEX IF NOT EXISTS sessions_expires_at_idx ON sessions (expires_at);

-- Player skill levels. skill_id is a gameconfig.SkillID (1..N).
CREATE TABLE IF NOT EXISTS player_skills (
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_id   INTEGER  NOT NULL,
    level      REAL     NOT NULL DEFAULT 0,
    xp         REAL     NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, skill_id)
);

-- Player inventory. item_id + item_state is the complete item identity.
-- quantity is stored as REAL so fractional parts survive across settlement
-- cycles. The player-facing count is floor(quantity).
CREATE TABLE IF NOT EXISTS player_inventory (
    user_id    INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id    INTEGER  NOT NULL,
    item_state INTEGER  NOT NULL DEFAULT 0,
    quantity   REAL     NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, item_id, item_state)
);

-- Unlocked events (type = "upgrade"). event_id is a gameconfig.EventID (1..N).
CREATE TABLE IF NOT EXISTS player_unlocked_events (
    user_id     INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id    INTEGER  NOT NULL,
    unlocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, event_id)
);

-- Discovered items (bestiary entries). item_id is a gameconfig.ItemID (1..N).
-- Tracks which item types the player has discovered, independent of inventory.
CREATE TABLE IF NOT EXISTS player_discovered_items (
    user_id       INTEGER  NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id       INTEGER  NOT NULL,
    discovered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, item_id)
);

-- Active event queues. Serial queue per queue_id; events execute in order
-- of position. progress tracks accumulated seconds for the current head event.
-- target_cycles: -1 = infinite loop (default), >0 = execute N times then remove.
CREATE TABLE IF NOT EXISTS player_active_events (
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    queue_id      INTEGER NOT NULL DEFAULT 0,
    event_id      INTEGER NOT NULL,
    position      INTEGER NOT NULL,
    target_cycles INTEGER NOT NULL DEFAULT -1,
    progress      REAL    NOT NULL DEFAULT 0,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, queue_id, position)
);

-- Player equipment. One item per slot per user.
-- anchor_slot links multiple rows as one logical piece (multi-slot items).
CREATE TABLE IF NOT EXISTS player_equipment (
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slot        TEXT    NOT NULL,
    item_id     INTEGER NOT NULL,
    item_state  INTEGER NOT NULL DEFAULT 0,
    anchor_slot TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, slot)
);

-- Player initialization state. A single row per user, created on first
-- successful player init so CreateSession can detect it with one indexed
-- lookup instead of scanning child tables.
CREATE TABLE IF NOT EXISTS player_init (
    user_id       INTEGER  NOT NULL PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    initialized   INTEGER  NOT NULL DEFAULT 0,  -- 0 = pending, 1 = done
    initialized_at DATETIME,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS player_init;
DROP TABLE IF EXISTS player_equipment;
DROP TABLE IF EXISTS player_active_events;
DROP TABLE IF EXISTS player_discovered_items;
DROP TABLE IF EXISTS player_unlocked_events;
DROP TABLE IF EXISTS player_inventory;
DROP TABLE IF EXISTS player_skills;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
