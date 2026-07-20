-- +goose Up

CREATE TABLE metadata (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- +goose Down

DROP TABLE metadata;