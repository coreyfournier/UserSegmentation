package http

import (
	"encoding/json"
	"net/http"

	"github.com/segmentation-service/segmentation/internal/application"
)

// EvaluateHandler handles POST /v1/evaluate.
type EvaluateHandler struct {
	uc *application.EvaluateUseCase
}

func (h *EvaluateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req application.EvaluateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.UserKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_key is required"})
		return
	}

	resp, err := h.uc.Execute(req)
	if err != nil {
		if err == application.ErrNoConfig {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no configuration loaded"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
