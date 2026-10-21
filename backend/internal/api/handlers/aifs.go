package handlers

import (
	"bytes"
	"database/sql"
	"image"
	"image/draw"
	"image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/brf-tech/filex/backend/internal/aifs"
	"github.com/brf-tech/filex/backend/internal/db"
	"github.com/brf-tech/filex/backend/internal/filebody"
	"github.com/brf-tech/filex/backend/internal/model"
	"github.com/brf-tech/filex/backend/internal/notify"
	"github.com/brf-tech/filex/backend/internal/share"
	"github.com/brf-tech/filex/backend/internal/storage"
)

// AIFSHandler serves the AIFS endpoints for album synchronization, face indexing, and selfie search.
type AIFSHandler struct {
	Store           db.Store
	AifsStore       aifs.Store
	ShareService    *share.Service
	StorageResolver func(int64) (storage.Driver, error)
	Body            *filebody.Resolver
	Notify          notify.Service
	Indexer         *aifs.Indexer
	Search          *aifs.SearchEngine
}

// NewAIFSHandler constructs the AIFSHandler.
func NewAIFSHandler(
	store db.Store,
	shareService *share.Service,
	storageResolver func(int64) (storage.Driver, error),
	body *filebody.Resolver,
	notifySvc notify.Service,
) *AIFSHandler {
	client := aifs.NewClient()
	var aifsStore aifs.Store
	if raw, ok := store.(interface{ RawDB() *sql.DB }); ok && raw.RawDB() != nil {
		slog.Info("[AIFS] Initialized persistent SQLStore for AIFS")
		aifsStore = aifs.NewSQLStore(raw.RawDB())
	} else {
		slog.Warn("[AIFS] Falling back to in-memory store for AIFS")
		aifsStore = aifs.DefaultStoreInstance()
	}
	return &AIFSHandler{
		Store:           store,
		AifsStore:       aifsStore,
		ShareService:    shareService,
		StorageResolver: storageResolver,
		Body:            body,
		Notify:          notifySvc,
		Indexer:         aifs.NewIndexer(client),
		Search:          aifs.NewSearchEngine(client),
	}
}

// EnableAndIndex handles POST /api/shares/{token}/aifs/enable.
// Synchronizes the album with FileX mount partitions & file nodes, then starts AI indexing.
func (h *AIFSHandler) EnableAndIndex(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing token"})
		return
	}

	sh, err := h.Store.GetShareByToken(r.Context(), token)
	if err != nil || sh == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "share not found"})
		return
	}

	node, err := h.Store.GetNode(r.Context(), sh.NodeID)
	if err != nil || node == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	// Synchronize FileX mount partition (storages) and file browsing items (nodes)
	album, err := aifs.SyncAlbumFromNode(r.Context(), h.Store, h.AifsStore, node.StorageID, node.ID, sh.Token, node.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Start background indexing
	if err := h.Indexer.StartIndexing(album.ID, h.Store, h.AifsStore, h.StorageResolver, h.Body, h.Notify); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":           true,
		"aifs_enabled": true,
		"album":        album,
		"message":      "AI indexing started",
	})
}

// Status handles GET /api/shares/{token}/aifs/status.
func (h *AIFSHandler) Status(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing token"})
		return
	}

	album, err := h.AifsStore.GetAlbumByShareToken(r.Context(), token)
	if err != nil || album == nil {
		// Try resolving share and auto-syncing if album not yet indexed
		sh, err := h.Store.GetShareByToken(r.Context(), token)
		if err != nil || sh == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "share not found"})
			return
		}

		node, err := h.Store.GetNode(r.Context(), sh.NodeID)
		if err != nil || node == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
			return
		}

		album, err = aifs.SyncAlbumFromNode(r.Context(), h.Store, h.AifsStore, node.StorageID, node.ID, sh.Token, node.Name)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// Auto-trigger background indexing if pending
	if album.Status == "pending" {
		_ = h.Indexer.StartIndexing(album.ID, h.Store, h.AifsStore, h.StorageResolver, h.Body, h.Notify)
		album.Status = "indexing"
	}

	var pct float64
	if album.TotalPhotos > 0 {
		pct = float64(album.IndexedPhotos) / float64(album.TotalPhotos) * 100.0
	}

	clustersCount := 0
	if album.DetectedFaces > 0 {
		if clusters, err := h.Search.ClusterFaces(r.Context(), h.AifsStore, album.ID, token, 0.58, 100); err == nil {
			clustersCount = len(clusters)
		}
	}

	writeJSON(w, http.StatusOK, aifs.StatusResponse{
		AlbumID:         album.ID,
		Title:           album.Title,
		Status:          album.Status,
		TotalPhotos:     album.TotalPhotos,
		IndexedPhotos:   album.IndexedPhotos,
		DetectedFaces:   album.DetectedFaces,
		ClustersCount:   clustersCount,
		PercentComplete: pct,
	})
}

// IndexAlbum handles POST /api/public/s/{token}/aifs/index.
// Manually triggers or restarts face indexing for the public shared album.
func (h *AIFSHandler) IndexAlbum(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing token"})
		return
	}

	sh, err := h.Store.GetShareByToken(r.Context(), token)
	if err != nil || sh == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "share not found"})
		return
	}

	node, err := h.Store.GetNode(r.Context(), sh.NodeID)
	if err != nil || node == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	album, err := aifs.SyncAlbumFromNode(r.Context(), h.Store, h.AifsStore, node.StorageID, node.ID, sh.Token, node.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := h.Indexer.StartIndexing(album.ID, h.Store, h.AifsStore, h.StorageResolver, h.Body, h.Notify); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"album":   album,
		"status":  "indexing",
		"message": "AI face indexing triggered",
	})
}

// FaceSearch handles POST /api/public/s/{token}/face-search.
// Guests search by face_id query param or upload a selfie image to find matching photos within the shared album.
func (h *AIFSHandler) FaceSearch(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing share token"})
		return
	}

	threshold := 0.58
	if tStr := r.URL.Query().Get("threshold"); tStr != "" {
		if t, err := strconv.ParseFloat(tStr, 64); err == nil && t > 0 {
			threshold = t
		}
	}

	limit := 100
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	// Resolve album
	album, err := h.AifsStore.GetAlbumByShareToken(r.Context(), token)
	if err != nil || album == nil {
		sh, err := h.Store.GetShareByToken(r.Context(), token)
		if err != nil || sh == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "share not found"})
			return
		}
		node, err := h.Store.GetNode(r.Context(), sh.NodeID)
		if err != nil || node == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
			return
		}
		album, err = aifs.SyncAlbumFromNode(r.Context(), h.Store, h.AifsStore, node.StorageID, node.ID, sh.Token, node.Name)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	// Fast path: search by an existing face ID from clustered faces
	if faceIDStr := r.URL.Query().Get("face_id"); faceIDStr != "" {
		faceID, err := strconv.ParseInt(faceIDStr, 10, 64)
		if err == nil && faceID > 0 {
			matches, err := h.Search.SearchByFaceID(r.Context(), h.AifsStore, album.ID, faceID, threshold, limit)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":          true,
				"album_id":    album.ID,
				"album_title": album.Title,
				"count":       len(matches),
				"matches":     matches,
			})
			return
		}
	}

	// Max 15 MB for uploaded selfie
	if err := r.ParseMultipartForm(15 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form: " + err.Error()})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing selfie file in request (field: 'file')"})
		return
	}
	defer file.Close()

	selfieBytes, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read file bytes"})
		return
	}

	matches, err := h.Search.SearchBySelfieBytes(r.Context(), h.AifsStore, album.ID, selfieBytes, header.Filename, threshold, limit)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          true,
		"album_id":    album.ID,
		"album_title": album.Title,
		"count":       len(matches),
		"matches":     matches,
	})
}

// ClusteredFaces handles GET /api/public/s/{token}/faces.
// Returns representative clustered people and detected faces for the shared album.
func (h *AIFSHandler) ClusteredFaces(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing token"})
		return
	}

	album, err := h.AifsStore.GetAlbumByShareToken(r.Context(), token)
	if err != nil || album == nil {
		sh, err := h.Store.GetShareByToken(r.Context(), token)
		if err != nil || sh == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "share not found"})
			return
		}
		node, err := h.Store.GetNode(r.Context(), sh.NodeID)
		if err != nil || node == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
			return
		}
		album, err = aifs.SyncAlbumFromNode(r.Context(), h.Store, h.AifsStore, node.StorageID, node.ID, sh.Token, node.Name)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	threshold := 0.58
	if tStr := r.URL.Query().Get("threshold"); tStr != "" {
		if t, err := strconv.ParseFloat(tStr, 64); err == nil && t > 0 {
			threshold = t
		}
	}

	limit := 35
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	faces, err := h.Search.ClusterFaces(r.Context(), h.AifsStore, album.ID, token, threshold, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"album_id":       album.ID,
		"album_title":    album.Title,
		"faces":          faces,
		"total_clusters": len(faces),
	})
}

// FaceCrop handles GET /api/public/s/{token}/faces/{faceID}/crop.
// Returns a cropped avatar JPEG for the specified detected face.
func (h *AIFSHandler) FaceCrop(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	faceIDStr := chi.URLParam(r, "faceID")
	faceID, err := strconv.ParseInt(faceIDStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid face id", http.StatusBadRequest)
		return
	}

	album, err := h.AifsStore.GetAlbumByShareToken(r.Context(), token)
	if err != nil || album == nil {
		sh, _ := h.Store.GetShareByToken(r.Context(), token)
		if sh != nil {
			node, _ := h.Store.GetNode(r.Context(), sh.NodeID)
			if node != nil {
				album, _ = aifs.SyncAlbumFromNode(r.Context(), h.Store, h.AifsStore, node.StorageID, node.ID, sh.Token, node.Name)
			}
		}
	}
	if album == nil {
		http.Error(w, "album not found", http.StatusNotFound)
		return
	}

	faces, err := h.AifsStore.ListFacesByAlbum(r.Context(), album.ID)
	if err != nil {
		http.Error(w, "faces not found", http.StatusNotFound)
		return
	}

	var targetFace *aifs.Face
	for _, f := range faces {
		if f.ID == faceID {
			targetFace = f
			break
		}
	}
	if targetFace == nil {
		http.Error(w, "face not found", http.StatusNotFound)
		return
	}

	photos, err := h.AifsStore.ListPhotos(r.Context(), album.ID)
	if err != nil {
		http.Error(w, "photo not found", http.StatusNotFound)
		return
	}

	var targetPhoto *aifs.Photo
	for _, p := range photos {
		if p.ID == targetFace.PhotoID {
			targetPhoto = p
			break
		}
	}
	if targetPhoto == nil {
		http.Error(w, "photo not found", http.StatusNotFound)
		return
	}

	node, err := h.Store.GetNode(r.Context(), targetPhoto.NodeID)
	if err != nil || node == nil {
		http.Error(w, "node not found", http.StatusNotFound)
		return
	}

	drv, err := h.StorageResolver(node.StorageID)
	if err != nil {
		http.Error(w, "storage driver unavailable", http.StatusInternalServerError)
		return
	}
	rc, err := drv.Read(r.Context(), node.Path)
	if err != nil {
		http.Error(w, "read failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer rc.Close()
	imgBytes, err := io.ReadAll(rc)
	if err != nil {
		http.Error(w, "read bytes failed", http.StatusInternalServerError)
		return
	}

	srcImg, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		_, _ = w.Write(imgBytes)
		return
	}

	b := srcImg.Bounds()
	imgW := float64(b.Dx())
	imgH := float64(b.Dy())

	bx := targetFace.BBox.X
	by := targetFace.BBox.Y
	bw := targetFace.BBox.Width
	bh := targetFace.BBox.Height

	if bx <= 1.0 && bw <= 1.0 {
		bx *= imgW
		by *= imgH
		bw *= imgW
		bh *= imgH
	}

	padX := bw * 0.25
	padY := bh * 0.25
	minX := int(math.Max(0, bx-padX))
	minY := int(math.Max(0, by-padY))
	maxX := int(math.Min(imgW, bx+bw+padX))
	maxY := int(math.Min(imgH, by+bh+padY))

	cropRect := image.Rect(minX, minY, maxX, maxY)
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}

	var cropped image.Image
	if si, ok := srcImg.(subImager); ok {
		cropped = si.SubImage(cropRect)
	} else {
		dst := image.NewRGBA(cropRect)
		draw.Draw(dst, cropRect, srcImg, cropRect.Min, draw.Src)
		cropped = dst
	}

	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	_ = jpeg.Encode(w, cropped, &jpeg.Options{Quality: 88})
}


// NodeStatus handles GET /api/files/nodes/{id}/aifs/status.
// Returns the face indexing status and progress for a folder node.
func (h *AIFSHandler) NodeStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	nodeID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid node id"})
		return
	}

	node, err := h.Store.GetNode(r.Context(), nodeID)
	if err != nil || node == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	folderNode := node
	if node.Type != model.NodeTypeDirectory && node.ParentID != nil {
		if parent, err := h.Store.GetNode(r.Context(), *node.ParentID); err == nil && parent != nil {
			folderNode = parent
		}
	}

	album, err := h.AifsStore.GetAlbumByNode(r.Context(), folderNode.StorageID, folderNode.ID)
	if err != nil || album == nil {
		// Check if any shares for this folder resolve an AIFS album
		shares, _ := h.Store.ListSharesByNode(r.Context(), folderNode.ID)
		for _, sh := range shares {
			if a, _ := h.AifsStore.GetAlbumByShareToken(r.Context(), sh.Token); a != nil {
				album = a
				break
			}
		}
		// If album is still nil and shares exist for this folder, sync album from node
		if album == nil && len(shares) > 0 {
			for _, sh := range shares {
				if a, syncErr := aifs.SyncAlbumFromNode(r.Context(), h.Store, h.AifsStore, folderNode.StorageID, folderNode.ID, sh.Token, folderNode.Name); syncErr == nil && a != nil {
					album = a
					// Start indexing if not yet indexed
					_ = h.Indexer.StartIndexing(album.ID, h.Store, h.AifsStore, h.StorageResolver, h.Body, h.Notify)
					break
				}
			}
		}
	}

	if album == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled": false,
			"status":  "not_indexed",
			"node_id": folderNode.ID,
		})
		return
	}

	// Auto-trigger background indexing if pending
	if album.Status == "pending" {
		_ = h.Indexer.StartIndexing(album.ID, h.Store, h.AifsStore, h.StorageResolver, h.Body, h.Notify)
		album.Status = "indexing"
	}

	var pct float64
	if album.TotalPhotos > 0 {
		pct = float64(album.IndexedPhotos) / float64(album.TotalPhotos) * 100.0
	}

	clustersCount := 0
	if album.DetectedFaces > 0 {
		if clusters, err := h.Search.ClusterFaces(r.Context(), h.AifsStore, album.ID, album.ShareToken, 0.58, 100); err == nil {
			clustersCount = len(clusters)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"enabled":          true,
		"album_id":         album.ID,
		"node_id":          folderNode.ID,
		"title":            album.Title,
		"status":           album.Status,
		"total_photos":     album.TotalPhotos,
		"indexed_photos":   album.IndexedPhotos,
		"detected_faces":   album.DetectedFaces,
		"clusters_count":   clustersCount,
		"percent_complete": pct,
	})
}

// IndexNode handles POST /api/files/nodes/{id}/aifs/index.
// Starts or restarts AI face indexing on a folder node.
func (h *AIFSHandler) IndexNode(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	nodeID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid node id"})
		return
	}

	node, err := h.Store.GetNode(r.Context(), nodeID)
	if err != nil || node == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "node not found"})
		return
	}

	folderNode := node
	if node.Type != model.NodeTypeDirectory && node.ParentID != nil {
		if parent, err := h.Store.GetNode(r.Context(), *node.ParentID); err == nil && parent != nil {
			folderNode = parent
		}
	}

	// Find active share token if exists
	shareToken := ""
	shares, _ := h.Store.ListSharesByNode(r.Context(), folderNode.ID)
	if len(shares) > 0 {
		shareToken = shares[0].Token
	}

	album, err := aifs.SyncAlbumFromNode(r.Context(), h.Store, h.AifsStore, folderNode.StorageID, folderNode.ID, shareToken, folderNode.Name)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := h.Indexer.StartIndexing(album.ID, h.Store, h.AifsStore, h.StorageResolver, h.Body, h.Notify); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"album":   album,
		"status":  "indexing",
		"message": "AI face indexing started",
	})
}
