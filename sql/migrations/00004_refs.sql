-- +goose Up

CREATE TABLE refs (
    id       INTEGER PRIMARY KEY,
    doc_id   INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    name     TEXT NOT NULL,
    line     INTEGER NOT NULL,
    col      INTEGER NOT NULL,
    context  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_refs_name ON refs(name);
CREATE INDEX idx_refs_doc_id ON refs(doc_id);

-- +goose Down

DROP INDEX IF EXISTS idx_refs_doc_id;
DROP INDEX IF EXISTS idx_refs_name;
DROP TABLE IF EXISTS refs;
