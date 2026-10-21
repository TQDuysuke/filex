package aifs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Store defines operations for AIFS albums, photos, and face embeddings.
type Store interface {
	GetOrCreateAlbum(ctx context.Context, storageID, nodeID int64, shareToken, title string) (*Album, error)
	GetAlbum(ctx context.Context, albumID int64) (*Album, error)
	GetAlbumByShareToken(ctx context.Context, shareToken string) (*Album, error)
	GetAlbumByNode(ctx context.Context, storageID, nodeID int64) (*Album, error)
	UpdateAlbumStatus(ctx context.Context, albumID int64, status string, indexedPhotos, detectedFaces int) error
	
	SyncPhotos(ctx context.Context, albumID int64, photos []*Photo) error
	ListPhotos(ctx context.Context, albumID int64) ([]*Photo, error)
	GetPendingPhotos(ctx context.Context, albumID int64) ([]*Photo, error)
	UpdatePhotoIndexed(ctx context.Context, photoID int64, faceCount int, status, errStr string) error

	AddFaces(ctx context.Context, faces []*Face) error
	ListFacesByAlbum(ctx context.Context, albumID int64) ([]*Face, error)
}

// MemoryStore provides a thread-safe in-memory store for AIFS data.
type MemoryStore struct {
	mu           sync.RWMutex
	nextAlbumID  int64
	nextPhotoID  int64
	nextFaceID   int64
	albums       map[int64]*Album
	albumsByNode map[string]int64 // "storageID:nodeID" -> albumID
	albumsByTok  map[string]int64 // shareToken -> albumID
	photos       map[int64]*Photo
	photosByNode map[string]int64 // "albumID:nodeID" -> photoID
	albumPhotos  map[int64][]int64 // albumID -> []photoID
	faces        map[int64]*Face
	albumFaces   map[int64][]int64 // albumID -> []faceID
}

// NewMemoryStore constructs a new MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextAlbumID:  1,
		nextPhotoID:  1,
		nextFaceID:   1,
		albums:       make(map[int64]*Album),
		albumsByNode: make(map[string]int64),
		albumsByTok:  make(map[string]int64),
		photos:       make(map[int64]*Photo),
		photosByNode: make(map[string]int64),
		albumPhotos:  make(map[int64][]int64),
		faces:        make(map[int64]*Face),
		albumFaces:   make(map[int64][]int64),
	}
}

// DefaultStore is the shared global store instance for the AIFS subsystem.
var (
	defaultStoreInstance Store
	defaultStoreOnce     sync.Once
)

// DefaultStoreInstance returns the singleton AIFS store.
func DefaultStoreInstance() Store {
	defaultStoreOnce.Do(func() {
		defaultStoreInstance = NewMemoryStore()
	})
	return defaultStoreInstance
}

// SetDefaultStoreInstance sets the process-wide default AIFS store instance.
func SetDefaultStoreInstance(s Store) {
	if s != nil {
		defaultStoreInstance = s
	}
}

func (s *MemoryStore) GetOrCreateAlbum(ctx context.Context, storageID, nodeID int64, shareToken, title string) (*Album, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nodeKey := fmt.Sprintf("%d:%d", storageID, nodeID)
	if id, exists := s.albumsByNode[nodeKey]; exists {
		alb := s.albums[id]
		if shareToken != "" {
			alb.ShareToken = shareToken
			s.albumsByTok[shareToken] = id
		}
		if title != "" && alb.Title == "" {
			alb.Title = title
		}
		alb.UpdatedAt = time.Now()
		return alb, nil
	}

	albID := s.nextAlbumID
	s.nextAlbumID++

	now := time.Now()
	alb := &Album{
		ID:            albID,
		StorageID:     storageID,
		NodeID:        nodeID,
		ShareToken:    shareToken,
		Title:         title,
		Status:        "pending",
		AIFSEnabled:   true,
		TotalPhotos:   0,
		IndexedPhotos: 0,
		DetectedFaces: 0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.albums[albID] = alb
	s.albumsByNode[nodeKey] = albID
	if shareToken != "" {
		s.albumsByTok[shareToken] = albID
	}
	return alb, nil
}

func (s *MemoryStore) GetAlbum(ctx context.Context, albumID int64) (*Album, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alb, ok := s.albums[albumID]
	if !ok {
		return nil, fmt.Errorf("album %d not found", albumID)
	}
	return alb, nil
}

func (s *MemoryStore) GetAlbumByShareToken(ctx context.Context, shareToken string) (*Album, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, ok := s.albumsByTok[shareToken]
	if !ok {
		return nil, fmt.Errorf("album for share %s not found", shareToken)
	}
	return s.albums[id], nil
}

func (s *MemoryStore) GetAlbumByNode(ctx context.Context, storageID, nodeID int64) (*Album, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodeKey := fmt.Sprintf("%d:%d", storageID, nodeID)
	id, ok := s.albumsByNode[nodeKey]
	if !ok {
		return nil, fmt.Errorf("album for node %d not found", nodeID)
	}
	return s.albums[id], nil
}

func (s *MemoryStore) UpdateAlbumStatus(ctx context.Context, albumID int64, status string, indexedPhotos, detectedFaces int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	alb, ok := s.albums[albumID]
	if !ok {
		return fmt.Errorf("album %d not found", albumID)
	}
	alb.Status = status
	if indexedPhotos >= 0 {
		alb.IndexedPhotos = indexedPhotos
	}
	if detectedFaces >= 0 {
		alb.DetectedFaces = detectedFaces
	}
	alb.UpdatedAt = time.Now()
	return nil
}

func (s *MemoryStore) SyncPhotos(ctx context.Context, albumID int64, photos []*Photo) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	alb, ok := s.albums[albumID]
	if !ok {
		return fmt.Errorf("album %d not found", albumID)
	}

	now := time.Now()
	for _, p := range photos {
		photoKey := fmt.Sprintf("%d:%d", albumID, p.NodeID)
		if existingID, exists := s.photosByNode[photoKey]; exists {
			// Update filename or hash if changed
			existing := s.photos[existingID]
			existing.Filename = p.Filename
			if p.FileHash != "" && p.FileHash != existing.FileHash {
				existing.FileHash = p.FileHash
				existing.Status = "pending"
			}
			existing.UpdatedAt = now
			continue
		}

		photoID := s.nextPhotoID
		s.nextPhotoID++

		p.ID = photoID
		p.AlbumID = albumID
		p.Status = "pending"
		p.CreatedAt = now
		p.UpdatedAt = now

		s.photos[photoID] = p
		s.photosByNode[photoKey] = photoID
		s.albumPhotos[albumID] = append(s.albumPhotos[albumID], photoID)
	}

	alb.TotalPhotos = len(s.albumPhotos[albumID])
	alb.UpdatedAt = now
	return nil
}

func (s *MemoryStore) ListPhotos(ctx context.Context, albumID int64) ([]*Photo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	photoIDs := s.albumPhotos[albumID]
	res := make([]*Photo, 0, len(photoIDs))
	for _, id := range photoIDs {
		if p, ok := s.photos[id]; ok {
			res = append(res, p)
		}
	}
	return res, nil
}

func (s *MemoryStore) GetPendingPhotos(ctx context.Context, albumID int64) ([]*Photo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	photoIDs := s.albumPhotos[albumID]
	var pending []*Photo
	for _, id := range photoIDs {
		if p, ok := s.photos[id]; ok && (p.Status == "pending" || p.Status == "failed") {
			pending = append(pending, p)
		}
	}
	return pending, nil
}

func (s *MemoryStore) UpdatePhotoIndexed(ctx context.Context, photoID int64, faceCount int, status, errStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, ok := s.photos[photoID]
	if !ok {
		return fmt.Errorf("photo %d not found", photoID)
	}
	p.FaceCount = faceCount
	p.Status = status
	p.Error = errStr
	now := time.Now()
	if status == "completed" {
		p.IndexedAt = &now
	}
	p.UpdatedAt = now
	return nil
}

func (s *MemoryStore) AddFaces(ctx context.Context, faces []*Face) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, f := range faces {
		faceID := s.nextFaceID
		s.nextFaceID++

		f.ID = faceID
		f.CreatedAt = now
		s.faces[faceID] = f
		s.albumFaces[f.AlbumID] = append(s.albumFaces[f.AlbumID], faceID)
	}
	return nil
}

func (s *MemoryStore) ListFacesByAlbum(ctx context.Context, albumID int64) ([]*Face, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	faceIDs := s.albumFaces[albumID]
	res := make([]*Face, 0, len(faceIDs))
	for _, id := range faceIDs {
		if f, ok := s.faces[id]; ok {
			res = append(res, f)
		}
	}
	return res, nil
}

// SQLStore implements Store backed by the database tables aifs_albums, aifs_photos, and aifs_faces.
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore creates a new persistent SQLStore.
func NewSQLStore(db *sql.DB) *SQLStore {
	return &SQLStore{db: db}
}

func (s *SQLStore) GetOrCreateAlbum(ctx context.Context, storageID, nodeID int64, shareToken, title string) (*Album, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, storage_id, node_id, COALESCE(share_token, ''), title, COALESCE(description, ''),
		        status, aifs_enabled != 0, total_photos, indexed_photos, detected_faces, created_at, updated_at
		 FROM aifs_albums WHERE storage_id = ? AND node_id = ?`,
		storageID, nodeID,
	)
	var alb Album
	var aifsEnabled bool
	err := row.Scan(&alb.ID, &alb.StorageID, &alb.NodeID, &alb.ShareToken, &alb.Title, &alb.Description,
		&alb.Status, &aifsEnabled, &alb.TotalPhotos, &alb.IndexedPhotos, &alb.DetectedFaces, &alb.CreatedAt, &alb.UpdatedAt)
	if err == nil {
		alb.AIFSEnabled = aifsEnabled
		if shareToken != "" && alb.ShareToken != shareToken {
			alb.ShareToken = shareToken
			_, _ = s.db.ExecContext(ctx, `UPDATE aifs_albums SET share_token = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, shareToken, alb.ID)
		}
		if title != "" && alb.Title != title {
			alb.Title = title
			_, _ = s.db.ExecContext(ctx, `UPDATE aifs_albums SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, title, alb.ID)
		}
		if !alb.AIFSEnabled {
			alb.AIFSEnabled = true
			_, _ = s.db.ExecContext(ctx, `UPDATE aifs_albums SET aifs_enabled = 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, alb.ID)
		}
		return &alb, nil
	}

	res, err := s.db.ExecContext(ctx,
		`INSERT INTO aifs_albums (storage_id, node_id, share_token, title, status, aifs_enabled, total_photos, indexed_photos, detected_faces, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 'pending', 1, 0, 0, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		storageID, nodeID, shareToken, title,
	)
	if err != nil {
		return s.GetAlbumByNode(ctx, storageID, nodeID)
	}

	id, _ := res.LastInsertId()
	now := time.Now()
	return &Album{
		ID:            id,
		StorageID:     storageID,
		NodeID:        nodeID,
		ShareToken:    shareToken,
		Title:         title,
		Status:        "pending",
		AIFSEnabled:   true,
		TotalPhotos:   0,
		IndexedPhotos: 0,
		DetectedFaces: 0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (s *SQLStore) GetAlbum(ctx context.Context, albumID int64) (*Album, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, storage_id, node_id, COALESCE(share_token, ''), title, COALESCE(description, ''),
		        status, aifs_enabled != 0, total_photos, indexed_photos, detected_faces, created_at, updated_at
		 FROM aifs_albums WHERE id = ?`,
		albumID,
	)
	var alb Album
	var aifsEnabled bool
	if err := row.Scan(&alb.ID, &alb.StorageID, &alb.NodeID, &alb.ShareToken, &alb.Title, &alb.Description,
		&alb.Status, &aifsEnabled, &alb.TotalPhotos, &alb.IndexedPhotos, &alb.DetectedFaces, &alb.CreatedAt, &alb.UpdatedAt); err != nil {
		return nil, fmt.Errorf("album %d not found: %w", albumID, err)
	}
	alb.AIFSEnabled = aifsEnabled
	return &alb, nil
}

func (s *SQLStore) GetAlbumByShareToken(ctx context.Context, shareToken string) (*Album, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, storage_id, node_id, COALESCE(share_token, ''), title, COALESCE(description, ''),
		        status, aifs_enabled != 0, total_photos, indexed_photos, detected_faces, created_at, updated_at
		 FROM aifs_albums WHERE share_token = ?`,
		shareToken,
	)
	var alb Album
	var aifsEnabled bool
	if err := row.Scan(&alb.ID, &alb.StorageID, &alb.NodeID, &alb.ShareToken, &alb.Title, &alb.Description,
		&alb.Status, &aifsEnabled, &alb.TotalPhotos, &alb.IndexedPhotos, &alb.DetectedFaces, &alb.CreatedAt, &alb.UpdatedAt); err != nil {
		return nil, fmt.Errorf("album for share %s not found: %w", shareToken, err)
	}
	alb.AIFSEnabled = aifsEnabled
	return &alb, nil
}

func (s *SQLStore) GetAlbumByNode(ctx context.Context, storageID, nodeID int64) (*Album, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, storage_id, node_id, COALESCE(share_token, ''), title, COALESCE(description, ''),
		        status, aifs_enabled != 0, total_photos, indexed_photos, detected_faces, created_at, updated_at
		 FROM aifs_albums WHERE storage_id = ? AND node_id = ?`,
		storageID, nodeID,
	)
	var alb Album
	var aifsEnabled bool
	if err := row.Scan(&alb.ID, &alb.StorageID, &alb.NodeID, &alb.ShareToken, &alb.Title, &alb.Description,
		&alb.Status, &aifsEnabled, &alb.TotalPhotos, &alb.IndexedPhotos, &alb.DetectedFaces, &alb.CreatedAt, &alb.UpdatedAt); err != nil {
		return nil, fmt.Errorf("album for node %d not found: %w", nodeID, err)
	}
	alb.AIFSEnabled = aifsEnabled
	return &alb, nil
}

func (s *SQLStore) UpdateAlbumStatus(ctx context.Context, albumID int64, status string, indexedPhotos, detectedFaces int) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE aifs_albums SET status = ?, indexed_photos = ?, detected_faces = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, indexedPhotos, detectedFaces, albumID,
	)
	return err
}

func (s *SQLStore) SyncPhotos(ctx context.Context, albumID int64, photos []*Photo) error {
	for _, p := range photos {
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO aifs_photos (album_id, node_id, filename, file_hash, face_count, status, created_at, updated_at)
			 VALUES (?, ?, ?, ?, 0, 'pending', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
			 ON CONFLICT(album_id, node_id) DO UPDATE SET filename = excluded.filename, file_hash = excluded.file_hash, updated_at = CURRENT_TIMESTAMP, status = CASE WHEN aifs_photos.file_hash != excluded.file_hash THEN 'pending' ELSE aifs_photos.status END`,
			albumID, p.NodeID, p.Filename, p.FileHash,
		)
		if err != nil {
			return fmt.Errorf("sync photo %s: %w", p.Filename, err)
		}
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE aifs_albums SET total_photos = (SELECT COUNT(*) FROM aifs_photos WHERE album_id = ?), updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		albumID, albumID,
	)
	return err
}

func (s *SQLStore) ListPhotos(ctx context.Context, albumID int64) ([]*Photo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, album_id, node_id, filename, file_hash, face_count, status, COALESCE(error, ''), indexed_at, created_at, updated_at
		 FROM aifs_photos WHERE album_id = ? ORDER BY id ASC`,
		albumID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Photo
	for rows.Next() {
		var p Photo
		if err := rows.Scan(&p.ID, &p.AlbumID, &p.NodeID, &p.Filename, &p.FileHash, &p.FaceCount, &p.Status, &p.Error, &p.IndexedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (s *SQLStore) GetPendingPhotos(ctx context.Context, albumID int64) ([]*Photo, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, album_id, node_id, filename, file_hash, face_count, status, COALESCE(error, ''), indexed_at, created_at, updated_at
		 FROM aifs_photos WHERE album_id = ? AND (status = 'pending' OR status = 'failed') ORDER BY id ASC`,
		albumID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Photo
	for rows.Next() {
		var p Photo
		if err := rows.Scan(&p.ID, &p.AlbumID, &p.NodeID, &p.Filename, &p.FileHash, &p.FaceCount, &p.Status, &p.Error, &p.IndexedAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

func (s *SQLStore) UpdatePhotoIndexed(ctx context.Context, photoID int64, faceCount int, status, errStr string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE aifs_photos SET face_count = ?, status = ?, error = ?, indexed_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		faceCount, status, errStr, photoID,
	)
	return err
}

func (s *SQLStore) AddFaces(ctx context.Context, faces []*Face) error {
	for _, f := range faces {
		embBytes, err := json.Marshal(f.Embedding)
		if err != nil {
			continue
		}
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO aifs_faces (photo_id, album_id, x, y, width, height, confidence, embedding, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			f.PhotoID, f.AlbumID, f.BBox.X, f.BBox.Y, f.BBox.Width, f.BBox.Height, f.Confidence, string(embBytes),
		)
		if err == nil {
			if id, err := res.LastInsertId(); err == nil {
				f.ID = id
			}
		}
	}
	return nil
}

func (s *SQLStore) ListFacesByAlbum(ctx context.Context, albumID int64) ([]*Face, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, photo_id, album_id, x, y, width, height, confidence, embedding, created_at
		 FROM aifs_faces WHERE album_id = ? ORDER BY id ASC`,
		albumID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Face
	for rows.Next() {
		var f Face
		var embStr string
		if err := rows.Scan(&f.ID, &f.PhotoID, &f.AlbumID, &f.BBox.X, &f.BBox.Y, &f.BBox.Width, &f.BBox.Height, &f.Confidence, &embStr, &f.CreatedAt); err != nil {
			return nil, err
		}
		if embStr != "" {
			_ = json.Unmarshal([]byte(embStr), &f.Embedding)
		}
		out = append(out, &f)
	}
	return out, rows.Err()
}
