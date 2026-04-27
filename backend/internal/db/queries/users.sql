-- name: CreateUser :one
INSERT INTO users (username, password_hash)
VALUES (?, ?)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = ?,
    updated_at    = CURRENT_TIMESTAMP
WHERE id = ?;
