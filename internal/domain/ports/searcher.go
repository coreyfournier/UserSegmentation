package ports

import "github.com/segmentation-service/segmentation/internal/domain/model"

// Searcher finds layers and segments matching a free-text query.
//
// It is a port rather than a helper over SegmentStore because searching is the
// one read whose implementation genuinely changes with the backing store. The
// in-memory store scans a snapshot; a database issues a query and never
// materialises the config at all. Callers depend on this interface, so that
// swap is a wiring change in the composition root and nothing more.
//
// That intent is what the signature is shaped around:
//
//   - limit is passed in, not applied by the caller after the fact, because a
//     database must be told how much to fetch. A non-positive limit means the
//     implementation's default.
//   - the error return exists for implementations that can fail. Scanning a
//     snapshot in memory cannot, and that one returns nil.
//
// Matching is case-insensitive substring. It is the honest common denominator
// between a scan and a SQL LIKE; anything richer (prefix ranking, full-text,
// fuzzy) would be a promise the in-memory implementation cannot keep.
type Searcher interface {
	Search(query string, limit int) (*model.SearchResult, error)
}
