-- name: IsPlayerInit :one
SELECT initialized FROM player_init WHERE user_id = ?;

-- name: MarkPlayerInit :exec
INSERT INTO player_init (user_id, initialized, initialized_at)
VALUES (?, 1, CURRENT_TIMESTAMP)
ON CONFLICT(user_id) DO UPDATE SET
    initialized = 1,
    initialized_at = CURRENT_TIMESTAMP
WHERE initialized IS NOT 1;
