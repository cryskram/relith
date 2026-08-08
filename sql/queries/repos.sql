-- name: ListRepos :many
SELECT * FROM repositories
ORDER BY name;

-- name: GetRepo :one
SELECT * FROM repositories
WHERE id = ?;

-- name: GetRepoByPath :one
SELECT * FROM repositories
WHERE path = ?;

-- name: GetRepoByName :one
SELECT * FROM repositories
WHERE name = ?;

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

-- name: GetStats :one
SELECT
  (SELECT COUNT(*) FROM repositories) AS repo_count,
  (SELECT COUNT(*) FROM documents) AS doc_count,
  (SELECT COUNT(*) FROM chunks) AS chunk_count,
  (SELECT COALESCE(SUM(size), 0) FROM documents) AS total_raw_bytes,
  (SELECT COALESCE(SUM(LENGTH(content)), 0) FROM chunks) AS total_chunk_bytes,
  (SELECT COUNT(*) FROM symbols) AS symbol_count,
  (SELECT COUNT(*) FROM refs) AS ref_count;
