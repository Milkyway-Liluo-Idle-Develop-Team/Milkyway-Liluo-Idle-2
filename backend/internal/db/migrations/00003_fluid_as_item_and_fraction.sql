-- +goose Up
-- +goose StatementBegin
--
-- 1. Fluids are now treated as a special form of items.
--    Delete the standalone player_fluids table; fluid quantities move to
--    player_inventory.
-- 2. player_inventory.quantity changes from REAL to INTEGER.
-- 3. player_inventory gets a fraction column to support fractional
--    accumulation (e.g. buffs that grant 1.3 items per cycle).
--

-- Drop the now-redundant fluid table.
DROP TABLE IF EXISTS player_fluids;

-- SQLite does not support ALTER COLUMN. Recreate the table.
CREATE TABLE player_inventory_new (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id    INTEGER NOT NULL,
    item_state INTEGER NOT NULL DEFAULT 0,
    quantity   INTEGER NOT NULL DEFAULT 0,
    fraction   REAL    NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, item_id, item_state)
);

INSERT INTO player_inventory_new (user_id, item_id, item_state, quantity, fraction, updated_at)
SELECT user_id, item_id, item_state, CAST(quantity AS INTEGER), quantity - CAST(quantity AS INTEGER), updated_at
FROM player_inventory;

DROP TABLE player_inventory;
ALTER TABLE player_inventory_new RENAME TO player_inventory;

CREATE INDEX IF NOT EXISTS inv_user_id_idx ON player_inventory (user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- Recreate player_fluids.
CREATE TABLE IF NOT EXISTS player_fluids (
    user_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    fluid_id  INTEGER NOT NULL,
    quantity  REAL    NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, fluid_id)
);

-- Revert player_inventory to REAL quantity without fraction.
CREATE TABLE player_inventory_old (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id    INTEGER NOT NULL,
    item_state INTEGER NOT NULL DEFAULT 0,
    quantity   REAL    NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, item_id, item_state)
);

INSERT INTO player_inventory_old (user_id, item_id, item_state, quantity, updated_at)
SELECT user_id, item_id, item_state, CAST(quantity AS REAL) + fraction, updated_at
FROM player_inventory;

DROP TABLE player_inventory;
ALTER TABLE player_inventory_old RENAME TO player_inventory;

CREATE INDEX IF NOT EXISTS inv_user_id_idx ON player_inventory (user_id);
-- +goose StatementEnd
