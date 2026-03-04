package http

import (
	"encoding/json"
	"net/http"

	"github.com/segmentation-service/segmentation/internal/application"
	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// AdminHandler handles all admin CRUD endpoints.
type AdminHandler struct {
	uc *application.AdminUseCase
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(uc *application.AdminUseCase) *AdminHandler {
	return &AdminHandler{uc: uc}
}

// ListLayers handles GET /v1/admin/layers.
func (h *AdminHandler) ListLayers(w http.ResponseWriter, r *http.Request) {
	snap := h.uc.GetSnapshot()
	if snap == nil {
		writeJSON(w, http.StatusOK, []model.Layer{})
		return
	}
	writeJSON(w, http.StatusOK, snap.Layers)
}

// CreateLayer handles POST /v1/admin/layers.
func (h *AdminHandler) CreateLayer(w http.ResponseWriter, r *http.Request) {
	var layer model.Layer
	if err := json.NewDecoder(r.Body).Decode(&layer); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if layer.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	snap, err := h.uc.CreateLayer(layer)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

// UpdateLayer handles PUT /v1/admin/layers/{name}.
func (h *AdminHandler) UpdateLayer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var layer model.Layer
	if err := json.NewDecoder(r.Body).Decode(&layer); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	snap, err := h.uc.UpdateLayer(name, layer)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// DeleteLayer handles DELETE /v1/admin/layers/{name}.
func (h *AdminHandler) DeleteLayer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	snap, err := h.uc.DeleteLayer(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// ListSegments handles GET /v1/admin/layers/{name}/segments.
func (h *AdminHandler) ListSegments(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	snap := h.uc.GetSnapshot()
	if snap == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no configuration loaded"})
		return
	}
	for _, l := range snap.Layers {
		if l.Name == name {
			writeJSON(w, http.StatusOK, l.Segments)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "layer not found"})
}

// CreateSegment handles POST /v1/admin/layers/{name}/segments.
func (h *AdminHandler) CreateSegment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var seg model.Segment
	if err := json.NewDecoder(r.Body).Decode(&seg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if seg.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}
	if seg.Strategy == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "strategy is required"})
		return
	}
	snap, err := h.uc.CreateSegment(name, seg)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

// UpdateSegment handles PUT /v1/admin/layers/{name}/segments/{id}.
func (h *AdminHandler) UpdateSegment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")
	var seg model.Segment
	if err := json.NewDecoder(r.Body).Decode(&seg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	snap, err := h.uc.UpdateSegment(name, id, seg)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// DeleteSegment handles DELETE /v1/admin/layers/{name}/segments/{id}.
func (h *AdminHandler) DeleteSegment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	id := r.PathValue("id")
	snap, err := h.uc.DeleteSegment(name, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// ImportSnapshot handles POST /v1/admin/import.
func (h *AdminHandler) ImportSnapshot(w http.ResponseWriter, r *http.Request) {
	var snap model.Snapshot
	if err := json.NewDecoder(r.Body).Decode(&snap); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := h.uc.ReplaceSnapshot(&snap); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// ExportSnapshot handles GET /v1/admin/export.
func (h *AdminHandler) ExportSnapshot(w http.ResponseWriter, r *http.Request) {
	snap := h.uc.GetSnapshot()
	if snap == nil {
		writeJSON(w, http.StatusOK, model.Snapshot{Layers: []model.Layer{}})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}
