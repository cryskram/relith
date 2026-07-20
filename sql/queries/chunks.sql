-- name: ListChunks :many
SELECT * FROM chunks
WHERE doc_id = ?
ORDER BY chunk_index;

-- name: GetChunk :one
SELECT * FROM chunks
WHERE id = ?;

-- name: CreateChunk :one
INSERT INTO chunks (doc_id, chunk_index, content)
VALUES (?, ?, ?)
RETURNING *;

-- name: DeleteChunksByDoc :exec
DELETE FROM chunks
WHERE doc_id = ?;

-- name: CountDocChunks :one
SELECT COUNT(*) FROM chunks
WHERE doc_id = ?;

-- name: GetChunkCountsByRepo :many
SELECT d.id AS doc_id, COUNT(c.id) AS chunk_count
FROM documents d
LEFT JOIN chunks c ON c.doc_id = d.id
WHERE d.repo_id = ?
GROUP BY d.id;
