-- +goose Up
-- +goose StatementBegin
--
-- Convert parallel slot-based event queue to serial position-based queues.
-- Each queue_id is a serial queue; events within a queue execute in order.
--

CREATE TABLE player_active_events_new (
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    queue_id      INTEGER NOT NULL DEFAULT 0,
    event_id      INTEGER NOT NULL,
    position      INTEGER NOT NULL,
    target_cycles INTEGER NOT NULL DEFAULT -1,
    progress      REAL    NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, queue_id, position)
);

-- Migrate old slot-based rows into queue_id=0 with position=slot.
INSERT INTO player_active_events_new (user_id, queue_id, event_id, position, target_cycles, progress)
SELECT user_id, 0, event_id, slot, -1, progress
FROM player_active_events;

DROP TABLE player_active_events;
ALTER TABLE player_active_events_new RENAME TO player_active_events;

CREATE INDEX IF NOT EXISTS active_events_user_idx ON player_active_events (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TABLE player_active_events_old (
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_id    INTEGER NOT NULL,
    slot        INTEGER NOT NULL DEFAULT 0,
    started_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    progress    REAL    NOT NULL DEFAULT 0,
    PRIMARY KEY (user_id, slot)
);

INSERT INTO player_active_events_old (user_id, event_id, slot, progress)
SELECT user_id, event_id, position, progress
FROM player_active_events
WHERE queue_id = 0;

DROP TABLE player_active_events;
ALTER TABLE player_active_events_old RENAME TO player_active_events;

CREATE INDEX IF NOT EXISTS active_events_user_idx ON player_active_events (user_id);
-- +goose StatementEnd
