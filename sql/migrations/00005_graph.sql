-- +goose Up

CREATE TABLE graph_edges (
    id INTEGER PRIMARY KEY,
    repo_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    source_doc_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    target_doc_id INTEGER NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK(kind IN ('imports', 'calls', 'references', 'implements')),
    weight INTEGER NOT NULL DEFAULT 1,
    metadata TEXT DEFAULT '{}',
    UNIQUE(source_doc_id, target_doc_id, kind)
);
CREATE INDEX idx_graph_edges_repo ON graph_edges(repo_id);
CREATE INDEX idx_graph_edges_source ON graph_edges(source_doc_id);
CREATE INDEX idx_graph_edges_target ON graph_edges(target_doc_id);
CREATE INDEX idx_graph_edges_kind ON graph_edges(kind);

-- +goose Down

DROP INDEX IF EXISTS idx_graph_edges_kind;
DROP INDEX IF EXISTS idx_graph_edges_target;
DROP INDEX IF EXISTS idx_graph_edges_source;
DROP INDEX IF EXISTS idx_graph_edges_repo;
DROP TABLE IF EXISTS graph_edges;
