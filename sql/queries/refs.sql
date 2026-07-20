-- name: CreateRef :one
INSERT INTO refs (doc_id, name, line, col, context)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteRefsByDoc :exec
DELETE FROM refs
WHERE doc_id = ?;

-- name: FindRefsByName :many
SELECT r.*, d.path, d.repo_id, r2.name AS repo_name
FROM refs r
JOIN documents d ON d.id = r.doc_id
JOIN repositories r2 ON r2.id = d.repo_id
WHERE r.name LIKE ? || '%'
ORDER BY r2.name, d.path, r.line;

-- name: FindRefsByNameAndRepo :many
SELECT r.*, d.path, d.repo_id, r2.name AS repo_name
FROM refs r
JOIN documents d ON d.id = r.doc_id
JOIN repositories r2 ON r2.id = d.repo_id
WHERE r2.name = ? AND r.name LIKE ? || '%'
ORDER BY d.path, r.line;
