package application

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/infrastructure/store"
)

type mockSink struct {
	saved *model.Snapshot
	err   error
}

func (m *mockSink) Save(snap *model.Snapshot) error {
	if m.err != nil {
		return m.err
	}
	m.saved = snap
	return nil
}

func newTestAdminUC() (*AdminUseCase, *store.Memory, *mockSink) {
	s := store.NewMemory()
	s.Swap(&model.Snapshot{Version: 1, Layers: []model.Layer{
		{Name: "base", Order: 1, Segments: []model.Segment{
			{ID: "seg1", Strategy: "static", Static: &model.StaticConfig{Mappings: map[string]string{}, Default: "x"}},
		}},
	}})
	sink := &mockSink{}
	uc := NewAdminUseCase(s, sink)
	return uc, s, sink
}

// --- GetSnapshot ---

func TestAdminUseCase_GetSnapshot(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap := uc.GetSnapshot()
	if snap == nil || snap.Version != 1 {
		t.Error("expected version 1 snapshot")
	}
}

// --- CreateLayer ---

func TestAdminUseCase_CreateLayer(t *testing.T) {
	uc, s, sink := newTestAdminUC()
	snap, err := uc.CreateLayer(model.Layer{Name: "new", Order: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Version != 2 {
		t.Errorf("expected version 2, got %d", snap.Version)
	}
	if len(snap.Layers) != 2 {
		t.Errorf("expected 2 layers, got %d", len(snap.Layers))
	}
	if sink.saved == nil {
		t.Error("expected sink.Save to be called")
	}
	got := s.Get()
	if got.Version != 2 {
		t.Error("store should contain the new snapshot")
	}
}

func TestAdminUseCase_CreateLayer_Duplicate(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	_, err := uc.CreateLayer(model.Layer{Name: "base", Order: 2})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func TestAdminUseCase_CreateLayer_NilSegments(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap, err := uc.CreateLayer(model.Layer{Name: "empty", Order: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, l := range snap.Layers {
		if l.Name == "empty" {
			if l.Segments == nil {
				t.Error("segments should be initialized to empty slice, not nil")
			}
			return
		}
	}
	t.Error("empty layer not found")
}

// --- UpdateLayer ---

func TestAdminUseCase_UpdateLayer(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap, err := uc.UpdateLayer("base", model.Layer{Name: "renamed", Order: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Layers[0].Name != "renamed" || snap.Layers[0].Order != 10 {
		t.Errorf("expected renamed/10, got %s/%d", snap.Layers[0].Name, snap.Layers[0].Order)
	}
	// Segments should be preserved
	if len(snap.Layers[0].Segments) != 1 {
		t.Error("segments should be preserved after update")
	}
}

func TestAdminUseCase_UpdateLayer_NotFound(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	_, err := uc.UpdateLayer("nonexistent", model.Layer{Name: "x", Order: 1})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

// --- DeleteLayer ---

func TestAdminUseCase_DeleteLayer(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap, err := uc.DeleteLayer("base")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Layers) != 0 {
		t.Errorf("expected 0 layers, got %d", len(snap.Layers))
	}
}

func TestAdminUseCase_DeleteLayer_NotFound(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	_, err := uc.DeleteLayer("nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

// --- CreateSegment ---

func TestAdminUseCase_CreateSegment(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap, err := uc.CreateSegment("base", model.Segment{
		ID: "seg2", Strategy: "static",
		Static: &model.StaticConfig{Mappings: map[string]string{}, Default: "y"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Layers[0].Segments) != 2 {
		t.Errorf("expected 2 segments, got %d", len(snap.Layers[0].Segments))
	}
}

func TestAdminUseCase_CreateSegment_DuplicateID(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	_, err := uc.CreateSegment("base", model.Segment{
		ID: "seg1", Strategy: "static",
		Static: &model.StaticConfig{Mappings: map[string]string{}, Default: "y"},
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected duplicate segment error, got %v", err)
	}
}

func TestAdminUseCase_CreateSegment_LayerNotFound(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	_, err := uc.CreateSegment("nonexistent", model.Segment{ID: "s", Strategy: "static"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

// --- UpdateSegment ---

func TestAdminUseCase_UpdateSegment(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap, err := uc.UpdateSegment("base", "seg1", model.Segment{
		Strategy: "static",
		Static:   &model.StaticConfig{Mappings: map[string]string{}, Default: "updated"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if snap.Layers[0].Segments[0].Static.Default != "updated" {
		t.Error("expected default to be updated")
	}
	if snap.Layers[0].Segments[0].ID != "seg1" {
		t.Error("ID should be preserved")
	}
}

func TestAdminUseCase_UpdateSegment_NotFound(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	_, err := uc.UpdateSegment("base", "nonexistent", model.Segment{Strategy: "static"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

func TestAdminUseCase_UpdateSegment_LayerNotFound(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	_, err := uc.UpdateSegment("nope", "seg1", model.Segment{Strategy: "static"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

// --- DeleteSegment ---

func TestAdminUseCase_DeleteSegment(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap, err := uc.DeleteSegment("base", "seg1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snap.Layers[0].Segments) != 0 {
		t.Errorf("expected 0 segments, got %d", len(snap.Layers[0].Segments))
	}
}

func TestAdminUseCase_DeleteSegment_NotFound(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	_, err := uc.DeleteSegment("base", "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not found error, got %v", err)
	}
}

// --- ReplaceSnapshot ---

func TestAdminUseCase_ReplaceSnapshot(t *testing.T) {
	uc, s, sink := newTestAdminUC()
	newSnap := &model.Snapshot{Version: 5, Layers: []model.Layer{
		{Name: "replaced", Order: 1, Segments: []model.Segment{}},
	}}
	err := uc.ReplaceSnapshot(newSnap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Get().Version != 5 {
		t.Errorf("expected version 5, got %d", s.Get().Version)
	}
	if sink.saved == nil {
		t.Error("expected sink.Save to be called")
	}
}

func TestAdminUseCase_ReplaceSnapshot_ValidationError(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	// Incompatible operator/type should fail validation
	bad := &model.Snapshot{
		Version: 1,
		Layers: []model.Layer{{
			Name: "bad", Order: 1,
			Segments: []model.Segment{{
				ID: "s", Strategy: "rule",
				Rules: []model.Rule{{
					RuleName:     "r",
					Expression:   &model.Expression{Field: "age", Operator: "gt", Value: 18},
					SuccessEvent: "x",
				}},
				InputSchema: model.InputSchema{
					"age": {Type: model.FieldTypeString, Required: true},
				},
			}},
		}},
	}
	err := uc.ReplaceSnapshot(bad)
	if err == nil {
		t.Error("expected validation error")
	}
}

// --- Version increment ---

func TestAdminUseCase_VersionIncrement(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap1, _ := uc.CreateLayer(model.Layer{Name: "l1", Order: 2})
	snap2, _ := uc.CreateLayer(model.Layer{Name: "l2", Order: 3})
	if snap2.Version != snap1.Version+1 {
		t.Errorf("expected version to increment: %d → %d", snap1.Version, snap2.Version)
	}
}

// --- Immutability (clone) ---

func TestAdminUseCase_CloneImmutability(t *testing.T) {
	uc, s, _ := newTestAdminUC()
	before := s.Get()
	_, _ = uc.CreateLayer(model.Layer{Name: "new", Order: 2})
	after := s.Get()
	// Original pointer should not have been mutated
	if len(before.Layers) == len(after.Layers) {
		t.Error("original snapshot should not be mutated by CreateLayer")
	}
}
