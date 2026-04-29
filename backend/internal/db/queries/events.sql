-- name: LoadUnlockedEvents :many
SELECT user_id, event_id, unlocked_at
FROM player_unlocked_events
WHERE user_id = ?
ORDER BY event_id;

-- name: UpsertUnlockedEvent :exec
INSERT INTO player_unlocked_events (user_id, event_id, unlocked_at)
VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(user_id, event_id) DO NOTHING;
