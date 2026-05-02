-- name: LoadUnlockedEvents :many
SELECT user_id, event_id, unlocked_at
FROM player_unlocked_events
WHERE user_id = ?
ORDER BY event_id;

-- name: UpsertUnlockedEvent :exec
INSERT INTO player_unlocked_events (user_id, event_id, unlocked_at)
VALUES (?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(user_id, event_id) DO NOTHING;

-- name: LoadActiveEvents :many
SELECT user_id, queue_id, event_id, position, target_cycles, progress
FROM player_active_events
WHERE user_id = ?
ORDER BY queue_id, position;

-- name: UpsertActiveEvent :exec
INSERT INTO player_active_events (user_id, queue_id, event_id, position, target_cycles, progress, updated_at)
VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(user_id, queue_id, position) DO UPDATE SET
    event_id = excluded.event_id,
    target_cycles = excluded.target_cycles,
    progress = excluded.progress,
    updated_at = CURRENT_TIMESTAMP
WHERE event_id IS NOT excluded.event_id OR target_cycles IS NOT excluded.target_cycles OR progress IS NOT excluded.progress;

-- name: ClearTailPositions :exec
UPDATE player_active_events
SET event_id = 0, target_cycles = 0, progress = 0, updated_at = CURRENT_TIMESTAMP
WHERE user_id = ? AND queue_id = ? AND position >= ?;
