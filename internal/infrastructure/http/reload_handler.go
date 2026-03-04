package http

import (
	"net/http"

	"github.com/segmentation-service/segmentation/internal/application"
)

// ReloadHandler handles POST /v1/reload.
type ReloadHandler struct {
	uc *application.ReloadUseCase
}

func (h *ReloadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.uc.Execute(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reloaded"})
}
