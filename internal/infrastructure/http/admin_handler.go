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
	// A caller that supplies only a friendly name gets a key derived from it,
	// the way CreateLookup derives a table's id. An explicit key always wins.
	if layer.Key == "" && layer.Name != "" {
		layer.Key = model.DeriveLayerKey(layer.Name)
	}
	// Checked here as well as at load so a bad key is a 400 naming the problem
	// rather than a 409 carrying a validation dump.
	if msg := model.ValidateLayerKey(layer.Key); msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	snap, err := h.uc.CreateLayer(layer)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, snap)
}

// UpdateLayer handles PUT /v1/admin/layers/{key}.
func (h *AdminHandler) UpdateLayer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("key")
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

// DeleteLayer handles DELETE /v1/admin/layers/{key}.
func (h *AdminHandler) DeleteLayer(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("key")
	snap, err := h.uc.DeleteLayer(name)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// ListSegments handles GET /v1/admin/layers/{key}/segments.
func (h *AdminHandler) ListSegments(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	snap := h.uc.GetSnapshot()
	if snap == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no configuration loaded"})
		return
	}
	for _, l := range snap.Layers {
		if l.Key == key {
			writeJSON(w, http.StatusOK, l.Segments)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "layer not found"})
}

// CreateSegment handles POST /v1/admin/layers/{key}/segments.
func (h *AdminHandler) CreateSegment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("key")
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

// UpdateSegment handles PUT /v1/admin/layers/{key}/segments/{id}.
func (h *AdminHandler) UpdateSegment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("key")
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

// DeleteSegment handles DELETE /v1/admin/layers/{key}/segments/{id}.
func (h *AdminHandler) DeleteSegment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("key")
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

// PreviewRekey handles GET /v1/admin/layers/{key}/rekey-preview?to=<newKey>.
//
// Read-only: it reports what changing the key would rewrite so the editor can
// show the blast radius before the author commits to it. Computed server-side
// rather than in the browser because the references live across other layers'
// rules and messages, and the rewrite that follows is server-side too — one
// implementation, so the preview cannot drift from what actually happens.
func (h *AdminHandler) PreviewRekey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	to := r.URL.Query().Get("to")
	if to == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "to is required"})
		return
	}

	refs, err := h.uc.PreviewRekey(key, to)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// Never null: the UI counts this, and "no references" is a result rather
	// than an absence.
	if refs == nil {
		refs = []application.RekeyRef{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"from": key, "to": to, "references": refs})
}
