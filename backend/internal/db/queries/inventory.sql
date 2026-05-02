-- name: LoadInventory :many
SELECT user_id, item_id, item_state, quantity
FROM player_inventory
WHERE user_id = ?
ORDER BY item_id, item_state;

-- name: UpsertInventory :exec
INSERT INTO player_inventory (user_id, item_id, item_state, quantity, updated_at)
VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(user_id, item_id, item_state) DO UPDATE SET
    quantity = excluded.quantity,
    updated_at = CURRENT_TIMESTAMP
WHERE quantity IS NOT excluded.quantity;
