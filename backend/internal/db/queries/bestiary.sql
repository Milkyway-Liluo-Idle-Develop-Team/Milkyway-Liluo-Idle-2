-- name: LoadDiscoveredItems :many
SELECT user_id, item_id, discovered_at
FROM player_discovered_items
WHERE user_id = ?
ORDER BY item_id;

-- name: UpsertDiscoveredItem :exec
INSERT INTO player_discovered_items (user_id, item_id, discovered_at)
VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(user_id, item_id) DO NOTHING;
