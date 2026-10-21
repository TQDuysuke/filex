package aifs

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/brf-tech/filex/backend/internal/db"
	"github.com/brf-tech/filex/backend/internal/model"
)

var supportedImageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".webp": true,
	".gif":  true,
	".heic": true,
	".avif": true,
}

// IsImageNode determines whether a FileX node represents a supported photo.
func IsImageNode(n *model.Node) bool {
	if n == nil || n.Type != model.NodeTypeFile {
		return false
	}
	if strings.HasPrefix(strings.ToLower(n.Mime), "image/") {
		return true
	}
	ext := strings.ToLower(path.Ext(n.Name))
	return supportedImageExts[ext]
}

// SyncAlbumFromNode synchronizes FileX's file browsing items (nodes) and mount partition (storages)
// into the shared AIFS album and photos database.
func SyncAlbumFromNode(
	ctx context.Context,
	filexStore db.Store,
	aifsStore Store,
	storageID int64,
	nodeID int64,
	shareToken string,
	albumTitle string,
) (*Album, error) {
	if aifsStore == nil {
		aifsStore = DefaultStoreInstance()
	}

	folderNode, err := filexStore.GetNode(ctx, nodeID)
	if err != nil {
		return nil, fmt.Errorf("aifs sync: get folder node %d: %w", nodeID, err)
	}
	if folderNode == nil {
		return nil, fmt.Errorf("aifs sync: folder node %d not found", nodeID)
	}

	if albumTitle == "" {
		albumTitle = folderNode.Name
	}

	// 1. Ensure AIFS album entry exists for this (storage_id, node_id) partition mapping
	album, err := aifsStore.GetOrCreateAlbum(ctx, storageID, nodeID, shareToken, albumTitle)
	if err != nil {
		return nil, fmt.Errorf("aifs sync: get/create album: %w", err)
	}

	// 2. Discover all image nodes under this folder from FileX's mount partition
	descendants, err := filexStore.ListNodesUnder(ctx, storageID, folderNode.Path, false)
	if err != nil {
		return nil, fmt.Errorf("aifs sync: list nodes under %s: %w", folderNode.Path, err)
	}

	var photoEntries []*Photo
	for _, n := range descendants {
		// Only collect images that are strictly inside the folder (not the folder itself)
		if n.ID == folderNode.ID || !IsImageNode(n) {
			continue
		}

		hash := n.Etag
		if hash == "" {
			hash = fmt.Sprintf("node_%d_%d", n.ID, n.Size)
		}

		photoEntries = append(photoEntries, &Photo{
			NodeID:   n.ID,
			Filename: n.Name,
			FileHash: hash,
			Status:   "pending",
		})
	}

	// 3. Synchronize photos in the shared database
	if err := aifsStore.SyncPhotos(ctx, album.ID, photoEntries); err != nil {
		return nil, fmt.Errorf("aifs sync: sync photos: %w", err)
	}

	return aifsStore.GetAlbum(ctx, album.ID)
}
