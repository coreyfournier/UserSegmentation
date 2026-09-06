// Package search implements the Searcher port against the in-memory snapshot.
package search

import (
	"sort"
	"strings"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/ports"
)

// DefaultLimit caps a result set when the caller does not ask for a size.
// Generous against any config that fits in a file, and present so a caller is
// already written against a bounded result when the store is a database.
const DefaultLimit = 100

// SnapshotSearcher scans the current snapshot for matches.
type SnapshotSearcher struct {
	store ports.SegmentStore
}

// NewSnapshotSearcher returns a Searcher backed by the given store.
func NewSnapshotSearcher(store ports.SegmentStore) *SnapshotSearcher {
	return &SnapshotSearcher{store: store}
}

// Search reports every layer and segment matching query, case-insensitively.
//
// An empty query matches nothing rather than everything. The caller listing
// all layers has GET /v1/admin/layers for that, and a database asked for
// "every row, unfiltered" through a search path is the shape of an accident.
func (s *SnapshotSearcher) Search(query string, limit int) (*model.SearchResult, error) {
	if limit <= 0 {
		limit = DefaultLimit
	}
	result := &model.SearchResult{Query: query, Hits: []model.SearchHit{}}

	needle := strings.ToLower(strings.TrimSpace(query))
	if needle == "" {
		return result, nil
	}

	snap := s.store.Get()
	if snap == nil {
		return result, nil
	}

	// Layers are scanned in name order, not config order, so the same query
	// returns the same list from one call to the next. Config order shifts
	// whenever a layer is created or deleted, which would reorder a result
	// list for reasons that have nothing to do with the query.
	layers := make([]*model.Layer, len(snap.Layers))
	for i := range snap.Layers {
		layers[i] = &snap.Layers[i]
	}
	sort.Slice(layers, func(i, j int) bool { return layers[i].Name < layers[j].Name })

	for _, layer := range layers {
		if strings.Contains(strings.ToLower(layer.Name), needle) {
			if !appendHit(result, limit, model.SearchHit{
				Kind:  "layer",
				Layer: layer.Name,
				Field: "name",
				Value: layer.Name,
			}) {
				return result, nil
			}
		}

		for i := range layer.Segments {
			seg := &layer.Segments[i]
			// A segment matches on its id or its strategy. Strategy is
			// included because it is shown beside the id and is the natural
			// way to ask "which segments are checklists?".
			field, value := "", ""
			switch {
			case strings.Contains(strings.ToLower(seg.ID), needle):
				field, value = "id", seg.ID
			case strings.Contains(strings.ToLower(seg.Strategy), needle):
				field, value = "strategy", seg.Strategy
			default:
				continue
			}
			if !appendHit(result, limit, model.SearchHit{
				Kind:    "segment",
				Layer:   layer.Name,
				Segment: seg.ID,
				Field:   field,
				Value:   value,
			}) {
				return result, nil
			}
		}
	}

	return result, nil
}

// appendHit adds a hit, reporting whether scanning should continue. At the
// limit it marks the result truncated and stops, so the caller is told the
// list is partial rather than being handed a silently short answer.
func appendHit(result *model.SearchResult, limit int, hit model.SearchHit) bool {
	if len(result.Hits) >= limit {
		result.Truncated = true
		return false
	}
	result.Hits = append(result.Hits, hit)
	return true
}
