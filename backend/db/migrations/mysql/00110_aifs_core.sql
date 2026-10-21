-- +goose Up
-- +goose StatementBegin

ALTER TABLE shares ADD COLUMN aifs_enabled BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS aifs_albums (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    storage_id BIGINT NOT NULL,
    node_id BIGINT NOT NULL,
    share_token VARCHAR(64),
    title VARCHAR(255) NOT NULL,
    description TEXT,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    aifs_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    total_photos INT NOT NULL DEFAULT 0,
    indexed_photos INT NOT NULL DEFAULT 0,
    detected_faces INT NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_aifs_albums_storage_node (storage_id, node_id),
    CONSTRAINT fk_aifs_albums_storage FOREIGN KEY (storage_id) REFERENCES storages(id) ON DELETE CASCADE,
    CONSTRAINT fk_aifs_albums_node FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE INDEX idx_aifs_albums_share_token ON aifs_albums(share_token);
CREATE INDEX idx_aifs_albums_node ON aifs_albums(node_id);

CREATE TABLE IF NOT EXISTS aifs_photos (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    album_id BIGINT NOT NULL,
    node_id BIGINT NOT NULL,
    filename VARCHAR(255) NOT NULL,
    file_hash VARCHAR(64) NOT NULL DEFAULT '',
    face_count INT NOT NULL DEFAULT 0,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    error TEXT,
    indexed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_aifs_photos_album_node (album_id, node_id),
    CONSTRAINT fk_aifs_photos_album FOREIGN KEY (album_id) REFERENCES aifs_albums(id) ON DELETE CASCADE,
    CONSTRAINT fk_aifs_photos_node FOREIGN KEY (node_id) REFERENCES nodes(id) ON DELETE CASCADE
);

CREATE INDEX idx_aifs_photos_album_status ON aifs_photos(album_id, status);

CREATE TABLE IF NOT EXISTS aifs_faces (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    photo_id BIGINT NOT NULL,
    album_id BIGINT NOT NULL,
    x DOUBLE NOT NULL,
    y DOUBLE NOT NULL,
    width DOUBLE NOT NULL,
    height DOUBLE NOT NULL,
    confidence DOUBLE NOT NULL,
    embedding MEDIUMTEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_aifs_faces_photo FOREIGN KEY (photo_id) REFERENCES aifs_photos(id) ON DELETE CASCADE,
    CONSTRAINT fk_aifs_faces_album FOREIGN KEY (album_id) REFERENCES aifs_albums(id) ON DELETE CASCADE
);

CREATE INDEX idx_aifs_faces_album ON aifs_faces(album_id);
CREATE INDEX idx_aifs_faces_photo ON aifs_faces(photo_id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS aifs_faces;
DROP TABLE IF EXISTS aifs_photos;
DROP TABLE IF EXISTS aifs_albums;
ALTER TABLE shares DROP COLUMN IF EXISTS aifs_enabled;
-- +goose StatementEnd
