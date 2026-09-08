package application

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func rev(n int) *int { return &n }

// The layer is the aggregate, so its revision advances on any write that
// touches it — its own fields or any of its segments.
func TestRevision_AdvancesOnEveryWriteToTheLayer(t *testing.T) {
	uc, _, _ := newTestAdminUC()

	start := uc.GetSnapshot().Layers[0].Revision

	schema := model.InputSchema{model.SubjectKeyField: {Type: model.FieldTypeString}}
	snap, err := uc.UpdateLayer("baseLayer", model.Layer{Key: "baseLayer", InputSchema: schema}, nil)
	if err != nil {
		t.Fatalf("update layer: %v", err)
	}
	afterLayer := snap.Layers[0].Revision
	if afterLayer <= start {
		t.Fatalf("expected the revision to advance past %d, got %d", start, afterLayer)
	}
	if snap.Layers[0].UpdatedAt == nil {
		t.Error("expected a timestamp alongside the revision")
	}

	// A segment write advances the layer too: a segment belongs to the layer,
	// so changing it changes the layer.
	snap, err = uc.UpdateSegment("baseLayer", "seg1", model.Segment{
		ID: "seg1", Strategy: model.StrategyStatic,
		Static: &model.StaticConfig{Mappings: map[string]string{}, Default: "x"},
	}, nil)
	if err != nil {
		t.Fatalf("update segment: %v", err)
	}
	if snap.Layers[0].Revision <= afterLayer {
		t.Errorf("a segment write must advance the layer's revision, got %d", snap.Layers[0].Revision)
	}
}

// The case the whole feature exists for: two editors, one stale.
func TestRevision_StaleWriteIsRefused(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	schema := model.InputSchema{model.SubjectKeyField: {Type: model.FieldTypeString}}

	loaded := uc.GetSnapshot().Layers[0].Revision

	// Someone else saves first.
	if _, err := uc.UpdateLayer("baseLayer", model.Layer{
		Key: "baseLayer", Name: "theirs", InputSchema: schema,
	}, rev(loaded)); err != nil {
		t.Fatalf("first write: %v", err)
	}

	// Our copy is now behind.
	_, err := uc.UpdateLayer("baseLayer", model.Layer{
		Key: "baseLayer", Name: "mine", InputSchema: schema,
	}, rev(loaded))
	c, ok := model.AsConflict(err)
	if !ok {
		t.Fatalf("expected a conflict, got %v", err)
	}
	if c.Expected != loaded || c.Actual != loaded+1 {
		t.Errorf("expected %d vs %d, got %d vs %d", loaded, loaded+1, c.Expected, c.Actual)
	}
	if c.Key != "baseLayer" || c.Kind != "layer" {
		t.Errorf("conflict must name the aggregate, got %+v", c)
	}
	if c.ChangedAt == nil {
		t.Error("expected the conflict to say when the stored revision was written")
	}

	// And the refused write changed nothing.
	if got := uc.GetSnapshot().Layers[0].Name; got != "theirs" {
		t.Errorf("a refused write must not land, got name %q", got)
	}
}

// Overwrite: the editor re-sends with the revision it was just told about.
func TestRevision_OverwriteSucceedsWithTheCurrentRevision(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	schema := model.InputSchema{model.SubjectKeyField: {Type: model.FieldTypeString}}

	loaded := uc.GetSnapshot().Layers[0].Revision
	if _, err := uc.UpdateLayer("baseLayer", model.Layer{
		Key: "baseLayer", Name: "theirs", InputSchema: schema,
	}, rev(loaded)); err != nil {
		t.Fatalf("first write: %v", err)
	}

	_, err := uc.UpdateLayer("baseLayer", model.Layer{
		Key: "baseLayer", Name: "mine", InputSchema: schema,
	}, rev(loaded))
	c, _ := model.AsConflict(err)

	snap, err := uc.UpdateLayer("baseLayer", model.Layer{
		Key: "baseLayer", Name: "mine", InputSchema: schema,
	}, rev(c.Actual))
	if err != nil {
		t.Fatalf("overwrite with the current revision: %v", err)
	}
	if snap.Layers[0].Name != "mine" {
		t.Errorf("expected the overwrite to land, got %q", snap.Layers[0].Name)
	}
}

// A nil expectation skips the check, which is what scripts and imports rely on
// — they are the only writer and have no revision to offer.
func TestRevision_NoExpectationSkipsTheCheck(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	schema := model.InputSchema{model.SubjectKeyField: {Type: model.FieldTypeString}}

	if _, err := uc.UpdateLayer("baseLayer", model.Layer{
		Key: "baseLayer", Name: "first", InputSchema: schema,
	}, nil); err != nil {
		t.Fatalf("first: %v", err)
	}
	// Deliberately stale, and deliberately unchecked.
	if _, err := uc.UpdateLayer("baseLayer", model.Layer{
		Key: "baseLayer", Name: "second", InputSchema: schema,
	}, nil); err != nil {
		t.Errorf("a nil expectation must not be checked, got %v", err)
	}
}

// A segment save carries the layer's revision, so a schema change that landed
// meanwhile refuses it — which is the reason the scope is the layer and not the
// segment: the schema decides what the segment is allowed to author.
func TestRevision_SegmentSaveIsRefusedAfterASchemaChange(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	schema := model.InputSchema{model.SubjectKeyField: {Type: model.FieldTypeString}}

	loaded := uc.GetSnapshot().Layers[0].Revision

	if _, err := uc.UpdateLayer("baseLayer", model.Layer{
		Key: "baseLayer", InputSchema: schema,
	}, rev(loaded)); err != nil {
		t.Fatalf("schema change: %v", err)
	}

	_, err := uc.UpdateSegment("baseLayer", "seg1", model.Segment{
		ID: "seg1", Strategy: model.StrategyStatic,
		Static: &model.StaticConfig{Mappings: map[string]string{}, Default: "x"},
	}, rev(loaded))
	if _, ok := model.AsConflict(err); !ok {
		t.Errorf("expected the stale segment save to conflict, got %v", err)
	}
}

// Creating and deleting a segment are writes to the layer as well.
func TestRevision_SegmentCreateAndDeleteAreGuarded(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	stale := uc.GetSnapshot().Layers[0].Revision

	snap, err := uc.CreateSegment("baseLayer", model.Segment{
		ID: "seg2", Strategy: model.StrategyStatic,
		Static: &model.StaticConfig{Mappings: map[string]string{}, Default: "y"},
	}, rev(stale))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	current := snap.Layers[0].Revision
	if current <= stale {
		t.Fatalf("create must advance the revision, got %d", current)
	}

	if _, err := uc.DeleteSegment("baseLayer", "seg2", rev(stale)); err == nil {
		t.Error("expected a stale delete to conflict")
	}
	if _, err := uc.DeleteSegment("baseLayer", "seg2", rev(current)); err != nil {
		t.Errorf("delete with the current revision: %v", err)
	}
}
