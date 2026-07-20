-- name: ListRepos :many
SELECT * FROM repositories
ORDER BY name;

-- name: GetRepo :one
SELECT * FROM repositories
WHERE id = ?;

-- name: GetRepoByPath :one
SELECT * FROM repositories
WHERE path = ?;

-- name: CreateRepo :one
INSERT INTO repositories (path, name, remote_url)
VALUES (?, ?, ?)
RETURNING *;

-- name: UpdateRepoStatus :exec
UPDATE repositories
SET status = ?,
    last_indexed_at = ?,
    file_count = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteRepo :exec
DELETE FROM repositories
WHERE id = ?;
