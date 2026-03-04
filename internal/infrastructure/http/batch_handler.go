package http

import (
	"encoding/json"
	"net/http"

	"github.com/segmentation-service/segmentation/internal/application"
)

// BatchHandler handles POST /v1/evaluate/batch.
type BatchHandler struct {
	uc *application.BatchEvaluateUseCase
}

func (h *BatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req application.BatchEvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if len(req.Users) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "users array is required"})
		return
	}

	resp, err := h.uc.Execute(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
