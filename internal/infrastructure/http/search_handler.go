package http

import (
	"net/http"
	"strconv"

	"github.com/segmentation-service/segmentation/internal/application"
)

// SearchHandler handles GET /v1/admin/search.
type SearchHandler struct {
	uc *application.SearchUseCase
}

// ServeHTTP answers ?q=<term>&limit=<n>.
//
// A blank or missing q is an empty result, not a 400: the field it backs is
// cleared by deleting what was typed, and an error banner appearing on the way
// back to empty would be noise. limit is optional; a value that is not a
// positive integer falls back to the searcher's default rather than failing,
// since nothing about the query itself is wrong.
func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")

	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			limit = n
		}
	}

	result, err := h.uc.Execute(query, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
