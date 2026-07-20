-- +goose Up

CREATE TABLE repositories (
    id INTEGER PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    remote_url TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','indexing','ready','failed')),
    last_indexed_at DATETIME,
    file_count INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE documents (
    id INTEGER PRIMARY KEY,
    repo_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    size INTEGER NOT NULL,
    hash TEXT NOT NULL,
    mod_time DATETIME NOT NULL,
    mime_type TEXT,
    language TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(repo_id, path)
);
CREATE INDEX idx_documents_repo_id ON documents(repo_id);
CREATE INDEX idx_documents_language ON documents(language);

CREATE TABLE chunks (
    id INTEGER PRIMARY KEY,
    doc_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    chunk_index INTEGER NOT NULL,
    content TEXT NOT NULL,
    UNIQUE(doc_id, chunk_index)
);
CREATE INDEX idx_chunks_doc_id ON chunks(doc_id);

CREATE VIRTUAL TABLE chunks_fts USING fts5(
    content,
    doc_id UNINDEXED,
    content=chunks,
    content_rowid=id,
    tokenize='porter unicode61'
);

-- +goose StatementBegin
CREATE TRIGGER chunks_ai AFTER INSERT ON chunks BEGIN
    INSERT INTO chunks_fts(rowid, doc_id, content) VALUES (new.id, new.doc_id, new.content);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER chunks_ad AFTER DELETE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, doc_id, content) VALUES ('delete', old.id, old.doc_id, old.content);
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER chunks_au AFTER UPDATE ON chunks BEGIN
    INSERT INTO chunks_fts(chunks_fts, rowid, doc_id, content) VALUES ('delete', old.id, old.doc_id, old.content);
    INSERT INTO chunks_fts(rowid, doc_id, content) VALUES (new.id, new.doc_id, new.content);
END;
-- +goose StatementEnd

-- +goose Down

DROP TRIGGER IF EXISTS chunks_au;
DROP TRIGGER IF EXISTS chunks_ad;
DROP TRIGGER IF EXISTS chunks_ai;
DROP TABLE IF EXISTS chunks_fts;
DROP TABLE IF EXISTS chunks;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS repositories;
