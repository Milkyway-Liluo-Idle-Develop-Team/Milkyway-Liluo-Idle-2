-- name: CreateUser :one
INSERT INTO users (username, email, password_hash)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ?;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = ?;

-- name: UpdateUserEmail :exec
UPDATE users
SET email      = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = ?,
    updated_at    = CURRENT_TIMESTAMP
WHERE id = ?;
