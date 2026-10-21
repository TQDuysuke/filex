package aifs

import (
	"context"
	"fmt"
	"math"
	"sort"
)

// SearchEngine handles face similarity searches scoped to an album.
type SearchEngine struct {
	client *Client
}

// NewSearchEngine constructs an AIFS search engine.
func NewSearchEngine(client *Client) *SearchEngine {
	if client == nil {
		client = NewClient()
	}
	return &SearchEngine{client: client}
}

// CosineDistance calculates cosine distance between two 512-dim unit vector embeddings.
// Distance = 1.0 - dot_product(a, b). 0.0 means identical, >= 1.0 means dissimilar.
func CosineDistance(a, b []float32) float64 {
	if len(a) == 0 || len(b) == 0 || len(a) != len(b) {
		return 1.0
	}
	var dot float64
	var normA float64
	var normB float64

	for i := 0; i < len(a); i++ {
		dot += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 1.0
	}

	sim := dot / (math.Sqrt(normA) * math.Sqrt(normB))
	return math.Max(0.0, 1.0-sim)
}

// SearchBySelfieBytes performs an album-scoped face search using an uploaded selfie image.
func (s *SearchEngine) SearchBySelfieBytes(
	ctx context.Context,
	aifsStore Store,
	albumID int64,
	selfieBytes []byte,
	filename string,
	threshold float64,
	limit int,
) ([]MatchPhotoResult, error) {
	if aifsStore == nil {
		aifsStore = DefaultStoreInstance()
	}
	if threshold <= 0 {
		threshold = 0.58 // Optimal threshold for InsightFace 512d embeddings
	}
	if limit <= 0 {
		limit = 100
	}

	// 1. Trích xuất embedding từ ảnh selfie
	selfieResult, err := s.client.EmbedSelfie(ctx, selfieBytes, filename)
	if err != nil {
		return nil, fmt.Errorf("aifs face search: embed selfie: %w", err)
	}

	queryEmbedding := selfieResult.Face.Embedding
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("no face embedding extracted from selfie")
	}

	// 2. Lấy tất cả khuôn mặt đã index trong album
	faces, err := aifsStore.ListFacesByAlbum(ctx, albumID)
	if err != nil {
		return nil, fmt.Errorf("list album faces: %w", err)
	}

	// Lấy danh sách ảnh để map ID -> Filename, NodeID
	photos, err := aifsStore.ListPhotos(ctx, albumID)
	if err != nil {
		return nil, fmt.Errorf("list album photos: %w", err)
	}
	photoMap := make(map[int64]*Photo, len(photos))
	for _, p := range photos {
		photoMap[p.ID] = p
	}

	// 3. Đối chiếu cosine distance với các khuôn mặt trong album
	bestMatchByPhoto := make(map[int64]MatchPhotoResult)

	for _, f := range faces {
		dist := CosineDistance(f.Embedding, queryEmbedding)
		if dist <= threshold {
			p, exists := photoMap[f.PhotoID]
			if !exists {
				continue
			}

			sim := math.Max(0.0, math.Min(1.0, 1.0-dist))
			match := MatchPhotoResult{
				PhotoID:    f.PhotoID,
				NodeID:     p.NodeID,
				Filename:   p.Filename,
				Distance:   dist,
				Similarity: sim,
				Confidence: f.Confidence,
				BBox:       f.BBox,
			}

			// Giữ kết quả khớp nhất cho mỗi bức ảnh
			if prev, ok := bestMatchByPhoto[f.PhotoID]; !ok || dist < prev.Distance {
				bestMatchByPhoto[f.PhotoID] = match
			}
		}
	}

	results := make([]MatchPhotoResult, 0, len(bestMatchByPhoto))
	for _, m := range bestMatchByPhoto {
		results = append(results, m)
	}

	// Sắp xếp theo độ tương đồng giảm dần (distance tăng dần)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// SearchByFaceID performs face search within the album using an existing face ID.
func (s *SearchEngine) SearchByFaceID(
	ctx context.Context,
	aifsStore Store,
	albumID int64,
	faceID int64,
	threshold float64,
	limit int,
) ([]MatchPhotoResult, error) {
	if aifsStore == nil {
		aifsStore = DefaultStoreInstance()
	}
	if threshold <= 0 {
		threshold = 0.58
	}
	if limit <= 0 {
		limit = 100
	}

	faces, err := aifsStore.ListFacesByAlbum(ctx, albumID)
	if err != nil {
		return nil, fmt.Errorf("list faces: %w", err)
	}

	var targetEmbedding []float32
	for _, f := range faces {
		if f.ID == faceID {
			targetEmbedding = f.Embedding
			break
		}
	}
	if len(targetEmbedding) == 0 {
		return nil, fmt.Errorf("face %d not found in album", faceID)
	}

	photos, err := aifsStore.ListPhotos(ctx, albumID)
	if err != nil {
		return nil, fmt.Errorf("list photos: %w", err)
	}
	photoMap := make(map[int64]*Photo, len(photos))
	for _, p := range photos {
		photoMap[p.ID] = p
	}

	bestMatchByPhoto := make(map[int64]MatchPhotoResult)
	for _, f := range faces {
		dist := CosineDistance(f.Embedding, targetEmbedding)
		if dist <= threshold {
			p, exists := photoMap[f.PhotoID]
			if !exists {
				continue
			}
			sim := math.Max(0.0, math.Min(1.0, 1.0-dist))
			match := MatchPhotoResult{
				PhotoID:    f.PhotoID,
				NodeID:     p.NodeID,
				Filename:   p.Filename,
				Distance:   dist,
				Similarity: sim,
				Confidence: f.Confidence,
				BBox:       f.BBox,
			}
			if prev, ok := bestMatchByPhoto[f.PhotoID]; !ok || dist < prev.Distance {
				bestMatchByPhoto[f.PhotoID] = match
			}
		}
	}

	results := make([]MatchPhotoResult, 0, len(bestMatchByPhoto))
	for _, m := range bestMatchByPhoto {
		results = append(results, m)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// ClusterFaces clusters detected faces in an album using single-linkage cosine similarity.
func (s *SearchEngine) ClusterFaces(
	ctx context.Context,
	aifsStore Store,
	albumID int64,
	token string,
	threshold float64,
	limit int,
) ([]ClusteredFaceResult, error) {
	if aifsStore == nil {
		aifsStore = DefaultStoreInstance()
	}
	if threshold <= 0 {
		threshold = 0.58
	}
	if limit <= 0 {
		limit = 35
	}

	faces, err := aifsStore.ListFacesByAlbum(ctx, albumID)
	if err != nil {
		return nil, fmt.Errorf("list faces: %w", err)
	}
	if len(faces) == 0 {
		return nil, nil
	}

	photos, err := aifsStore.ListPhotos(ctx, albumID)
	if err != nil {
		return nil, fmt.Errorf("list photos: %w", err)
	}
	photoMap := make(map[int64]*Photo, len(photos))
	for _, p := range photos {
		photoMap[p.ID] = p
	}

	sortedFaces := make([]*Face, len(faces))
	copy(sortedFaces, faces)
	sort.Slice(sortedFaces, func(i, j int) bool {
		return sortedFaces[i].Confidence > sortedFaces[j].Confidence
	})

	type cluster struct {
		repFace   *Face
		exemplars [][]float32
		photos    map[int64]bool
	}

	var clusters []*cluster

	for _, f := range sortedFaces {
		var matched *cluster
		minDist := 999.0

		for _, c := range clusters {
			for _, ex := range c.exemplars {
				dist := CosineDistance(f.Embedding, ex)
				if dist < threshold && dist < minDist {
					minDist = dist
					matched = c
				}
			}
		}

		if matched != nil {
			matched.photos[f.PhotoID] = true
			if len(matched.exemplars) < 6 {
				matched.exemplars = append(matched.exemplars, f.Embedding)
			}
		} else {
			c := &cluster{
				repFace:   f,
				exemplars: [][]float32{f.Embedding},
				photos:    map[int64]bool{f.PhotoID: true},
			}
			clusters = append(clusters, c)
		}
	}

	sort.Slice(clusters, func(i, j int) bool {
		return len(clusters[i].photos) > len(clusters[j].photos)
	})

	if len(clusters) > limit {
		clusters = clusters[:limit]
	}

	res := make([]ClusteredFaceResult, 0, len(clusters))
	for _, c := range clusters {
		p := photoMap[c.repFace.PhotoID]
		fn := ""
		nodeID := int64(0)
		if p != nil {
			fn = p.Filename
			nodeID = p.NodeID
		}
		cropURL := fmt.Sprintf("/api/public/s/%s/faces/%d/crop", token, c.repFace.ID)
		filenames := make([]string, 0, len(c.photos))
		for pID := range c.photos {
			if ph := photoMap[pID]; ph != nil && ph.Filename != "" {
				filenames = append(filenames, ph.Filename)
			}
		}
		res = append(res, ClusteredFaceResult{
			FaceID:         c.repFace.ID,
			PhotoID:        c.repFace.PhotoID,
			NodeID:         nodeID,
			Filename:       fn,
			PhotoCount:     len(c.photos),
			Confidence:     c.repFace.Confidence,
			BBox:           c.repFace.BBox,
			CropURL:        cropURL,
			PhotoFilenames: filenames,
		})
	}

	return res, nil
}

