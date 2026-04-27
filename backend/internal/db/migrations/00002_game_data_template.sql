-- +goose Up
-- +goose StatementBegin
--
-- This migration demonstrates the pattern for game data tables that use
-- numeric IDs (allocated by gameconfig at startup) instead of string IDs.
--
-- Key principle: storage and settlement use INTEGER; string IDs are only
-- used at the transport layer (HTTP/WS) for human readability.
--

-- Player skill levels. skill_id is a gameconfig.SkillID (1..N).
CREATE TABLE IF NOT EXISTS player_skills (
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    skill_id  INTEGER NOT NULL,
    level     REAL    NOT NULL DEFAULT 0,
    xp        REAL    NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, skill_id)
);

-- Player inventory. item_id is a gameconfig.ItemID (1..N).
-- item_state is the 32-bit variant descriptor mentioned in the design doc.
-- quantity is INTEGER; fractional accumulation is stored in fraction.
CREATE TABLE IF NOT EXISTS player_inventory (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id    INTEGER NOT NULL,
    item_state INTEGER NOT NULL DEFAULT 0,
    quantity   INTEGER NOT NULL DEFAULT 0,
    fraction   REAL    NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, item_id, item_state)
);

CREATE INDEX IF NOT EXISTS inv_user_id_idx ON player_inventory (user_id);

-- Unlocked events (type = "upgrade"). event_id is a gameconfig.EventID (1..N).
CREATE TABLE IF NOT EXISTS player_unlocked_events (
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id  INTEGER NOT NULL,
    unlocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, event_id)
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
    PRIMARY KEY (user_id, queue_id, position)
);

CREATE INDEX IF NOT EXISTS active_events_user_idx ON player_active_events (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS player_active_events;
DROP TABLE IF EXISTS player_unlocked_events;
DROP TABLE IF EXISTS player_inventory;
DROP TABLE IF EXISTS player_skills;
-- +goose StatementEnd
