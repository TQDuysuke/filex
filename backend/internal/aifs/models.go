package aifs

import (
	"time"
)

// Album represents an AIFS AI photo album synchronized with a FileX folder node and mount partition (storage).
type Album struct {
	ID             int64     `json:"id"`
	StorageID      int64     `json:"storage_id"`
	NodeID         int64     `json:"node_id"`
	ShareToken     string    `json:"share_token,omitempty"`
	Title          string    `json:"title"`
	Description    string    `json:"description,omitempty"`
	Status         string    `json:"status"` // "pending", "indexing", "ready", "failed"
	AIFSEnabled    bool      `json:"aifs_enabled"`
	TotalPhotos    int       `json:"total_photos"`
	IndexedPhotos  int       `json:"indexed_photos"`
	DetectedFaces  int       `json:"detected_faces"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Photo represents a photo inside an AIFS album synchronized with a FileX file node.
type Photo struct {
	ID        int64      `json:"id"`
	AlbumID   int64      `json:"album_id"`
	NodeID    int64      `json:"node_id"`
	Filename  string     `json:"filename"`
	FileHash  string     `json:"file_hash"`
	FaceCount int        `json:"face_count"`
	Status    string     `json:"status"` // "pending", "processing", "completed", "failed"
	Error     string     `json:"error,omitempty"`
	IndexedAt *time.Time `json:"indexed_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// BoundingBox holds normalized or pixel coordinates of a detected face.
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Face represents a detected face within a photo with its 512-dimensional vector embedding.
type Face struct {
	ID         int64       `json:"id"`
	PhotoID    int64       `json:"photo_id"`
	AlbumID    int64       `json:"album_id"`
	BBox       BoundingBox `json:"bbox"`
	Confidence float64     `json:"confidence"`
	Embedding  []float32   `json:"embedding"`
	CreatedAt  time.Time   `json:"created_at"`
}

// DetectionResult is the response returned by the AI service /detect-and-embed.
type DetectionResult struct {
	Count        int `json:"count"`
	Faces        []struct {
		BBox       BoundingBox `json:"bbox"`
		Confidence float64     `json:"confidence"`
		Embedding  []float32   `json:"embedding"`
	} `json:"faces"`
	ModelName    string `json:"model_name"`
	ModelVersion string `json:"model_version"`
}

// SelfieFaceResult is the response returned by /embed-selfie.
type SelfieFaceResult struct {
	Face struct {
		BBox       BoundingBox `json:"bbox"`
		Confidence float64     `json:"confidence"`
		Embedding  []float32   `json:"embedding"`
	} `json:"face"`
	ModelName    string `json:"model_name"`
	ModelVersion string `json:"model_version"`
}

// MatchPhotoResult represents a photo matched during face similarity search.
type MatchPhotoResult struct {
	PhotoID     int64       `json:"photo_id"`
	NodeID      int64       `json:"node_id"`
	Filename    string      `json:"filename"`
	Distance    float64     `json:"distance"`
	Similarity  float64     `json:"similarity"`
	Confidence  float64     `json:"confidence"`
	BBox        BoundingBox `json:"bbox"`
}

// StatusResponse describes the real-time AI indexing progress.
type StatusResponse struct {
	AlbumID         int64   `json:"album_id"`
	Title           string  `json:"title"`
	Status          string  `json:"status"`
	TotalPhotos     int     `json:"total_photos"`
	IndexedPhotos   int     `json:"indexed_photos"`
	DetectedFaces   int     `json:"detected_faces"`
	ClustersCount   int     `json:"clusters_count"`
	PercentComplete float64 `json:"percent_complete"`
}

// ClusteredFaceResult represents a representative face for a detected person.
type ClusteredFaceResult struct {
	FaceID     int64       `json:"face_id"`
	PhotoID    int64       `json:"photo_id"`
	NodeID     int64       `json:"node_id"`
	Filename   string      `json:"filename"`
	PhotoCount int         `json:"photo_count"`
	Confidence float64     `json:"confidence"`
	BBox           BoundingBox `json:"bbox"`
	CropURL        string      `json:"crop_url"`
	PhotoFilenames []string    `json:"photo_filenames,omitempty"`
}

