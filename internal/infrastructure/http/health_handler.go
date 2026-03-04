package http

import (
	"net/http"

	"github.com/segmentation-service/segmentation/internal/domain/ports"
)

// HealthHandler handles GET /v1/health.
type HealthHandler struct {
	store ports.SegmentStore
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	snap := h.store.Get()
	status := "healthy"
	version := 0
	if snap == nil {
		status = "degraded"
	} else {
		version = snap.Version
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  status,
		"version": version,
	})
}
