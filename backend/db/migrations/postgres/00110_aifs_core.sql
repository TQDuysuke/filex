-- +goose Up
-- +goose StatementBegin

-- 1. Extend shares with aifs_enabled
ALTER TABLE shares ADD COLUMN IF NOT EXISTS aifs_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- 2. AIFS Albums (Synchronized with FileX mount partitions and folder nodes)
CREATE TABLE IF NOT EXISTS aifs_albums (
    id BIGSERIAL PRIMARY KEY,
    storage_id BIGINT NOT NULL REFERENCES storages(id) ON DELETE CASCADE,
    node_id BIGINT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    share_token TEXT,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    aifs_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    total_photos INTEGER NOT NULL DEFAULT 0,
    indexed_photos INTEGER NOT NULL DEFAULT 0,
    detected_faces INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (storage_id, node_id)
);

CREATE INDEX IF NOT EXISTS idx_aifs_albums_share_token ON aifs_albums(share_token);
CREATE INDEX IF NOT EXISTS idx_aifs_albums_node ON aifs_albums(node_id);

-- 3. AIFS Photos (Synchronized with FileX image file nodes)
CREATE TABLE IF NOT EXISTS aifs_photos (
    id BIGSERIAL PRIMARY KEY,
    album_id BIGINT NOT NULL REFERENCES aifs_albums(id) ON DELETE CASCADE,
    node_id BIGINT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    filename TEXT NOT NULL,
    file_hash TEXT NOT NULL DEFAULT '',
    face_count INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    error TEXT,
    indexed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (album_id, node_id)
);

CREATE INDEX IF NOT EXISTS idx_aifs_photos_album_status ON aifs_photos(album_id, status);

-- 4. AIFS Faces (Faces detected by AI InsightFace model with 512-dim embedding)
CREATE TABLE IF NOT EXISTS aifs_faces (
    id BIGSERIAL PRIMARY KEY,
    photo_id BIGINT NOT NULL REFERENCES aifs_photos(id) ON DELETE CASCADE,
    album_id BIGINT NOT NULL REFERENCES aifs_albums(id) ON DELETE CASCADE,
    x DOUBLE PRECISION NOT NULL,
    y DOUBLE PRECISION NOT NULL,
    width DOUBLE PRECISION NOT NULL,
    height DOUBLE PRECISION NOT NULL,
    confidence DOUBLE PRECISION NOT NULL,
    embedding TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_aifs_faces_album ON aifs_faces(album_id);
CREATE INDEX IF NOT EXISTS idx_aifs_faces_photo ON aifs_faces(photo_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS aifs_faces;
DROP TABLE IF EXISTS aifs_photos;
DROP TABLE IF EXISTS aifs_albums;
ALTER TABLE shares DROP COLUMN IF EXISTS aifs_enabled;
-- +goose StatementEnd
