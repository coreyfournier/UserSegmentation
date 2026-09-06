package application

import (
	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/ports"
)

// SearchUseCase answers free-text queries over the configuration.
//
// It holds the port, not a store, so replacing the snapshot scan with a
// database query is a change in the composition root and nowhere else.
type SearchUseCase struct {
	searcher ports.Searcher
}

// NewSearchUseCase creates a search use case over the given searcher.
func NewSearchUseCase(searcher ports.Searcher) *SearchUseCase {
	return &SearchUseCase{searcher: searcher}
}

// Execute runs one query. A non-positive limit means the searcher's default.
func (uc *SearchUseCase) Execute(query string, limit int) (*model.SearchResult, error) {
	return uc.searcher.Search(query, limit)
}
