-- name: ListDocuments :many
SELECT * FROM documents
WHERE repo_id = ?
ORDER BY path;

-- name: GetDocument :one
SELECT * FROM documents
WHERE id = ?;

-- name: GetDocumentByPath :one
SELECT * FROM documents
WHERE repo_id = ? AND path = ?;

-- name: CreateDocument :one
INSERT INTO documents (repo_id, path, size, hash, mod_time, mime_type, language)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: UpdateDocument :exec
UPDATE documents
SET size = ?,
    hash = ?,
    mod_time = ?,
    mime_type = ?,
    language = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE id = ?;

-- name: DeleteDocument :exec
DELETE FROM documents
WHERE id = ?;

-- name: DeleteDocumentsByRepo :exec
DELETE FROM documents
WHERE repo_id = ?;

-- name: CountRepoDocuments :one
SELECT COUNT(*) FROM documents
WHERE repo_id = ?;

-- name: DocumentExists :one
SELECT COUNT(*) FROM documents
WHERE repo_id = ? AND path = ?;
