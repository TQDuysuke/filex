-- +goose Up
-- +goose StatementBegin

ALTER TABLE shares ADD COLUMN aifs_enabled INTEGER NOT NULL DEFAULT 0;

CREATE TABLE IF NOT EXISTS aifs_albums (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    storage_id INTEGER NOT NULL REFERENCES storages(id) ON DELETE CASCADE,
    node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    share_token TEXT,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    aifs_enabled INTEGER NOT NULL DEFAULT 1,
    total_photos INTEGER NOT NULL DEFAULT 0,
    indexed_photos INTEGER NOT NULL DEFAULT 0,
    detected_faces INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (storage_id, node_id)
);

CREATE INDEX IF NOT EXISTS idx_aifs_albums_share_token ON aifs_albums(share_token);
CREATE INDEX IF NOT EXISTS idx_aifs_albums_node ON aifs_albums(node_id);

CREATE TABLE IF NOT EXISTS aifs_photos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    album_id INTEGER NOT NULL REFERENCES aifs_albums(id) ON DELETE CASCADE,
    node_id INTEGER NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    file_hash TEXT NOT NULL DEFAULT '',
    face_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    error TEXT,
    indexed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (album_id, node_id)
);

CREATE INDEX IF NOT EXISTS idx_aifs_photos_album_status ON aifs_photos(album_id, status);

CREATE TABLE IF NOT EXISTS aifs_faces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    photo_id INTEGER NOT NULL REFERENCES aifs_photos(id) ON DELETE CASCADE,
    album_id INTEGER NOT NULL REFERENCES aifs_albums(id) ON DELETE CASCADE,
    x REAL NOT NULL,
    y REAL NOT NULL,
    width REAL NOT NULL,
    height REAL NOT NULL,
    confidence REAL NOT NULL,
    embedding TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_aifs_faces_album ON aifs_faces(album_id);
CREATE INDEX IF NOT EXISTS idx_aifs_faces_photo ON aifs_faces(photo_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS aifs_faces;
DROP TABLE IF EXISTS aifs_photos;
DROP TABLE IF EXISTS aifs_albums;
-- +goose StatementEnd
