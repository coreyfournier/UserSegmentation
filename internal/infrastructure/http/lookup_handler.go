package http

import (
	"encoding/json"
	"net/http"

	"github.com/segmentation-service/segmentation/internal/application"
	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// ListLookups handles GET /v1/admin/lookups.
func (h *AdminHandler) ListLookups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.uc.ListLookups())
}

// CreateLookup handles POST /v1/admin/lookups.
func (h *AdminHandler) CreateLookup(w http.ResponseWriter, r *http.Request) {
	var table model.LookupTable
	if err := json.NewDecoder(r.Body).Decode(&table); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	snap, err := h.uc.CreateLookup(table)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

// UpdateLookup handles PUT /v1/admin/lookups/{id}.
func (h *AdminHandler) UpdateLookup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var table model.LookupTable
	if err := json.NewDecoder(r.Body).Decode(&table); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	snap, err := h.uc.UpdateLookup(id, table)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// DeleteLookup handles DELETE /v1/admin/lookups/{id}.
func (h *AdminHandler) DeleteLookup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	snap, err := h.uc.DeleteLookup(id)
	if err != nil {
		if refErr, ok := err.(*application.LookupReferencedError); ok {
			writeJSON(w, http.StatusConflict, map[string]interface{}{
				"error":      refErr.Error(),
				"references": refErr.Refs,
			})
			return
		}
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
