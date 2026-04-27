-- name: CreateSession :one
INSERT INTO sessions (id, user_id, token_hash, user_agent, ip, expires_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions
WHERE token_hash = ?
  AND revoked_at IS NULL
  AND expires_at > CURRENT_TIMESTAMP;

-- name: TouchSession :exec
UPDATE sessions
SET last_used_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: RevokeSession :exec
UPDATE sessions
SET revoked_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: RevokeAllSessionsForUser :exec
UPDATE sessions
SET revoked_at = CURRENT_TIMESTAMP
WHERE user_id = ?
  AND revoked_at IS NULL;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at < CURRENT_TIMESTAMP
   OR (revoked_at IS NOT NULL AND revoked_at < datetime('now', '-7 days'));
