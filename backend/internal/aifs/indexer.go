package aifs

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/brf-tech/filex/backend/internal/db"
	"github.com/brf-tech/filex/backend/internal/filebody"
	"github.com/brf-tech/filex/backend/internal/model"
	"github.com/brf-tech/filex/backend/internal/notify"
	"github.com/brf-tech/filex/backend/internal/storage"
)

var (
	globalIndexerMu sync.Mutex
	globalActive    = make(map[int64]bool)
)

// Indexer coordinates the AI indexing worker for photo albums.
type Indexer struct {
	client *Client
}

// NewIndexer creates a new AIFS indexer.
func NewIndexer(client *Client) *Indexer {
	if client == nil {
		client = NewClient()
	}
	return &Indexer{
		client: client,
	}
}

// IsActive reports whether a background indexing task is currently running for the album.
func (idx *Indexer) IsActive(albumID int64) bool {
	globalIndexerMu.Lock()
	defer globalIndexerMu.Unlock()
	return globalActive[albumID]
}

// StartIndexing initiates the background AI indexing process for an album.
func (idx *Indexer) StartIndexing(
	albumID int64,
	filexStore db.Store,
	aifsStore Store,
	storageResolver func(int64) (storage.Driver, error),
	bodyResolver *filebody.Resolver,
	notifySvc notify.Service,
) error {
	globalIndexerMu.Lock()
	if globalActive[albumID] {
		globalIndexerMu.Unlock()
		slog.Debug("[AIFS] Indexing worker already active for album", "album_id", albumID)
		return nil
	}
	globalActive[albumID] = true
	globalIndexerMu.Unlock()

	if aifsStore == nil {
		aifsStore = DefaultStoreInstance()
	}

	album, err := aifsStore.GetAlbum(context.Background(), albumID)
	if err != nil {
		globalIndexerMu.Lock()
		delete(globalActive, albumID)
		globalIndexerMu.Unlock()
		return fmt.Errorf("aifs indexer: album %d not found: %w", albumID, err)
	}

	// Launch worker in background goroutine with detached context
	go func() {
		defer func() {
			globalIndexerMu.Lock()
			delete(globalActive, albumID)
			globalIndexerMu.Unlock()
		}()
		idx.runIndexing(album, filexStore, aifsStore, storageResolver, bodyResolver, notifySvc)
	}()
	return nil
}

func (idx *Indexer) runIndexing(
	album *Album,
	filexStore db.Store,
	aifsStore Store,
	storageResolver func(int64) (storage.Driver, error),
	bodyResolver *filebody.Resolver,
	notifySvc notify.Service,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	slog.Info("[AIFS] Starting AI Indexing for album", "album_id", album.ID, "title", album.Title)
	_ = aifsStore.UpdateAlbumStatus(ctx, album.ID, "indexing", album.IndexedPhotos, album.DetectedFaces)

	indexedCount := album.IndexedPhotos
	detectedFaces := album.DetectedFaces

	for {
		pending, err := aifsStore.GetPendingPhotos(ctx, album.ID)
		if err != nil {
			slog.Error("[AIFS] Failed to get pending photos", "album_id", album.ID, "error", err)
			_ = aifsStore.UpdateAlbumStatus(ctx, album.ID, "failed", indexedCount, detectedFaces)
			return
		}
		if len(pending) == 0 {
			break
		}

		for _, p := range pending {
			select {
			case <-ctx.Done():
				slog.Warn("[AIFS] Indexing timeout or cancelled", "album_id", album.ID)
				return
			default:
			}

			node, err := filexStore.GetNode(ctx, p.NodeID)
			if err != nil || node == nil {
				slog.Warn("[AIFS] File node missing", "node_id", p.NodeID)
				_ = aifsStore.UpdatePhotoIndexed(ctx, p.ID, 0, "failed", "node not found")
				continue
			}

			imgBytes, err := readNodeBytes(ctx, node, storageResolver, bodyResolver)
			if err != nil {
				slog.Warn("[AIFS] Failed to read image bytes", "filename", node.Name, "error", err)
				_ = aifsStore.UpdatePhotoIndexed(ctx, p.ID, 0, "failed", err.Error())
				continue
			}

			// Call InsightFace detection and embedding
			detResult, err := idx.client.DetectAndEmbed(ctx, imgBytes, node.Name)
			if err != nil {
				slog.Warn("[AIFS] AI detection failed", "filename", node.Name, "error", err)
				_ = aifsStore.UpdatePhotoIndexed(ctx, p.ID, 0, "failed", err.Error())
				continue
			}

			var faces []*Face
			for _, f := range detResult.Faces {
				faces = append(faces, &Face{
					PhotoID:    p.ID,
					AlbumID:    album.ID,
					BBox:       f.BBox,
					Confidence: f.Confidence,
					Embedding:  f.Embedding,
				})
			}

			if len(faces) > 0 {
				if err := aifsStore.AddFaces(ctx, faces); err != nil {
					slog.Error("[AIFS] Failed to save detected faces", "photo_id", p.ID, "error", err)
				}
			}

			_ = aifsStore.UpdatePhotoIndexed(ctx, p.ID, len(faces), "completed", "")
			indexedCount++
			detectedFaces += len(faces)

			_ = aifsStore.UpdateAlbumStatus(ctx, album.ID, "indexing", indexedCount, detectedFaces)
		}
	}

	// Mark album as ready
	_ = aifsStore.UpdateAlbumStatus(ctx, album.ID, "ready", indexedCount, detectedFaces)
	slog.Info("[AIFS] Completed AI Indexing for album!", "album_id", album.ID, "title", album.Title, "photos", indexedCount, "faces", detectedFaces)

	// Dispatch notification via notify.Service
	if notifySvc != nil {
		ev := notify.Event{
			Event:    notify.EventType("aifs_indexing_completed"),
			Severity: notify.SeverityInfo,
			Body:     fmt.Sprintf("AI indexing cho album '%s' đã hoàn thành (%d ảnh, %d khuôn mặt phát hiện).", album.Title, indexedCount, detectedFaces),
			Meta: map[string]any{
				"title_vi":       "Hoàn tất AI Indexing",
				"body_vi":        fmt.Sprintf("AI indexing cho album '%s' đã hoàn tất. Đã nhận diện %d khuôn mặt trên %d bức ảnh.", album.Title, detectedFaces, indexedCount),
				"title_en":       "AI Indexing Completed",
				"body_en":        fmt.Sprintf("AI indexing for album '%s' has finished (%d photos, %d faces detected).", album.Title, indexedCount, detectedFaces),
				"album_id":       album.ID,
				"album_title":    album.Title,
				"share_token":    album.ShareToken,
				"total_photos":   album.TotalPhotos,
				"indexed_photos": indexedCount,
				"detected_faces": detectedFaces,
				"status":         "ready",
			},
		}
		if album.ShareToken != "" {
			ev.Share = &notify.ShareRef{Token: album.ShareToken}
		}

		if _, err := notifySvc.Send(context.Background(), ev); err != nil {
			slog.Warn("[AIFS] Failed to emit completion notification", "error", err)
		} else {
			slog.Info("[AIFS] Successfully emitted completion notification for album", "album_id", album.ID)
		}
	}
}

func readNodeBytes(
	ctx context.Context,
	node *model.Node,
	storageResolver func(int64) (storage.Driver, error),
	bodyResolver *filebody.Resolver,
) ([]byte, error) {
	if storageResolver == nil {
		return nil, fmt.Errorf("no storage resolver available")
	}

	drv, err := storageResolver(node.StorageID)
	if err != nil {
		return nil, fmt.Errorf("resolve driver: %w", err)
	}

	var reader io.ReadCloser
	if bodyResolver != nil {
		src, err := bodyResolver.Resolve(ctx, drv, node.StorageID, node.Path, node)
		if err == nil && src != nil {
			rc, openErr := src.Open(ctx)
			if openErr == nil {
				reader = rc
			}
		}
	}

	if reader == nil {
		rc, err := drv.Read(ctx, node.Path)
		if err != nil {
			return nil, fmt.Errorf("driver read %s: %w", node.Path, err)
		}
		reader = rc
	}
	defer reader.Close()

	return io.ReadAll(reader)
}
