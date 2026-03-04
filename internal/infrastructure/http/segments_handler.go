package http

import (
	"net/http"

	"github.com/segmentation-service/segmentation/internal/domain/ports"
)

// SegmentsHandler handles GET /v1/segments.
type SegmentsHandler struct {
	store ports.SegmentStore
}

func (h *SegmentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snap := h.store.Get()
	if snap == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no configuration loaded"})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
