package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/segmentation-service/segmentation/internal/application"
	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// mockSink implements ports.ConfigSink for testing.
type mockSink struct{}

func (m *mockSink) Save(_ *model.Snapshot) error { return nil }

// mockStore implements ports.SegmentStore for testing.
type mockStore struct {
	snap *model.Snapshot
}

func (m *mockStore) Get() *model.Snapshot  { return m.snap }
func (m *mockStore) Swap(s *model.Snapshot) { m.snap = s }

func newTestAdmin() (*AdminHandler, *mockStore) {
	store := &mockStore{snap: &model.Snapshot{
		Version: 1,
		Layers: []model.Layer{
			{Name: "test-layer", Order: 1, Segments: []model.Segment{
				{ID: "seg-1", Strategy: "static", Static: &model.StaticConfig{
					Mappings: map[string]string{}, Default: "default",
				}},
			}},
		},
	}}
	uc := application.NewAdminUseCase(store, &mockSink{})
	return NewAdminHandler(uc), store
}

func TestAdminHandler_ListLayers(t *testing.T) {
	h, _ := newTestAdmin()
	req := httptest.NewRequest("GET", "/v1/admin/layers", nil)
	w := httptest.NewRecorder()
	h.ListLayers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var layers []model.Layer
	json.NewDecoder(w.Body).Decode(&layers)
	if len(layers) != 1 || layers[0].Name != "test-layer" {
		t.Errorf("unexpected layers: %v", layers)
	}
}

func TestAdminHandler_CreateLayer(t *testing.T) {
	h, _ := newTestAdmin()
	body, _ := json.Marshal(model.Layer{Name: "new-layer", Order: 2})
	req := httptest.NewRequest("POST", "/v1/admin/layers", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateLayer(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminHandler_CreateLayerDuplicate(t *testing.T) {
	h, _ := newTestAdmin()
	body, _ := json.Marshal(model.Layer{Name: "test-layer", Order: 2})
	req := httptest.NewRequest("POST", "/v1/admin/layers", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.CreateLayer(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestAdminHandler_DeleteLayer(t *testing.T) {
	h, _ := newTestAdmin()
	req := httptest.NewRequest("DELETE", "/v1/admin/layers/test-layer", nil)
	req.SetPathValue("name", "test-layer")
	w := httptest.NewRecorder()
	h.DeleteLayer(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminHandler_DeleteLayerNotFound(t *testing.T) {
	h, _ := newTestAdmin()
	req := httptest.NewRequest("DELETE", "/v1/admin/layers/nope", nil)
	req.SetPathValue("name", "nope")
	w := httptest.NewRecorder()
	h.DeleteLayer(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAdminHandler_CreateSegment(t *testing.T) {
	h, _ := newTestAdmin()
	seg := model.Segment{ID: "seg-2", Strategy: "static", Static: &model.StaticConfig{
		Mappings: map[string]string{}, Default: "x",
	}}
	body, _ := json.Marshal(seg)
	req := httptest.NewRequest("POST", "/v1/admin/layers/test-layer/segments", bytes.NewReader(body))
	req.SetPathValue("name", "test-layer")
	w := httptest.NewRecorder()
	h.CreateSegment(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminHandler_DeleteSegment(t *testing.T) {
	h, _ := newTestAdmin()
	req := httptest.NewRequest("DELETE", "/v1/admin/layers/test-layer/segments/seg-1", nil)
	req.SetPathValue("name", "test-layer")
	req.SetPathValue("id", "seg-1")
	w := httptest.NewRecorder()
	h.DeleteSegment(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAdminHandler_ExportImport(t *testing.T) {
	h, store := newTestAdmin()

	// Export
	req := httptest.NewRequest("GET", "/v1/admin/export", nil)
	w := httptest.NewRecorder()
	h.ExportSnapshot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("export: expected 200, got %d", w.Code)
	}

	// Import with new version
	var snap model.Snapshot
	json.NewDecoder(w.Body).Decode(&snap)
	snap.Version = 99
	body, _ := json.Marshal(snap)
	req = httptest.NewRequest("POST", "/v1/admin/import", bytes.NewReader(body))
	w = httptest.NewRecorder()
	h.ImportSnapshot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("import: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if store.snap.Version != 99 {
		t.Errorf("expected version 99, got %d", store.snap.Version)
	}
}

func TestAdminHandler_ListSegments(t *testing.T) {
	h, _ := newTestAdmin()
	req := httptest.NewRequest("GET", "/v1/admin/layers/test-layer/segments", nil)
	req.SetPathValue("name", "test-layer")
	w := httptest.NewRecorder()
	h.ListSegments(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var segs []model.Segment
	json.NewDecoder(w.Body).Decode(&segs)
	if len(segs) != 1 || segs[0].ID != "seg-1" {
		t.Errorf("unexpected segments: %v", segs)
	}
}
