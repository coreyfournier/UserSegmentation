package application

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// These tests exist because of a defect found once already on this branch:
// UpdateLayer copies only the fields it names onto the stored layer, so any
// field it doesn't name is silently discarded on every save. Before this
// change UpdateLayer named exactly Name, DependsOn and DefaultLanguage —
// InputSchema and OutputSchema were dropped on every layer update. CreateLayer
// was never affected: it appends the whole incoming model.Layer value, so it
// already carried both schemas through untouched.

// --- CreateLayer persists both schemas ---

func TestAdminUseCase_CreateLayer_PersistsSchemas(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap, err := uc.CreateLayer(model.Layer{
		Key:          "withSchemas",
		InputSchema:  model.InputSchema{"age": {Type: model.FieldTypeNumber}},
		OutputSchema: model.OutputSchema{"category": {Type: model.FieldTypeString}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idx := -1
	for i, l := range snap.Layers {
		if l.Key == "withSchemas" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("created layer not found")
	}
	got := snap.Layers[idx]
	if got.InputSchema["age"].Type != model.FieldTypeNumber {
		t.Errorf("expected inputSchema to persist, got %+v", got.InputSchema)
	}
	if got.OutputSchema["category"].Type != model.FieldTypeString {
		t.Errorf("expected outputSchema to persist, got %+v", got.OutputSchema)
	}
}

// --- UpdateLayer carries both schemas through ---

func TestLayerSchema_UpdateLayerCarriesSchemasThrough(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap, err := uc.UpdateLayer("baseLayer", model.Layer{
		Key: "baseLayer",
		// subjectKey rides along because a PUT replaces the schema wholesale
		// and the layer holds a static segment that requires it.
		InputSchema: model.InputSchema{
			"age":                 {Type: model.FieldTypeNumber},
			model.SubjectKeyField: {Type: model.FieldTypeString},
		},
		OutputSchema: model.OutputSchema{"category": {Type: model.FieldTypeString}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := snap.Layers[0]
	if got.InputSchema["age"].Type != model.FieldTypeNumber {
		t.Errorf("expected inputSchema to carry through the update, got %+v", got.InputSchema)
	}
	if got.OutputSchema["category"].Type != model.FieldTypeString {
		t.Errorf("expected outputSchema to carry through the update, got %+v", got.OutputSchema)
	}
	// Segments must still be preserved — UpdateLayer never touches them.
	if len(got.Segments) != 1 {
		t.Error("segments should be preserved after update")
	}
}

// --- Deliberate choice: an update whose layer omits a schema clears it ---
//
// This is chosen (over preserving the stored schema) for consistency with how
// UpdateLayer already treats DependsOn and DefaultLanguage: the whole field is
// replaced by whatever the request carries, never merged. It is also the
// shape a layer-editing UI naturally produces, since it always round-trips a
// complete layer.
func TestLayerSchema_UpdateLayer_OmittedSchemaIsCleared(t *testing.T) {
	uc, _, _ := newTestAdminUC()

	// A layer of its own, with no segment that needs a declared field. The
	// shared fixture's layer holds a static segment, whose strategy requires
	// the layer to declare subjectKey — so clearing its input schema is
	// correctly refused, and would test the refusal rather than the clearing.
	if _, err := uc.CreateLayer(model.Layer{Key: "clearable"}); err != nil {
		t.Fatalf("unexpected error creating the layer: %v", err)
	}

	// First give it both schemas.
	_, err := uc.UpdateLayer("clearable", model.Layer{
		Key:          "clearable",
		InputSchema:  model.InputSchema{"age": {Type: model.FieldTypeNumber}},
		OutputSchema: model.OutputSchema{"category": {Type: model.FieldTypeString}},
	})
	if err != nil {
		t.Fatalf("unexpected error priming schemas: %v", err)
	}

	// Now update again with a layer that omits both schemas.
	snap, err := uc.UpdateLayer("clearable", model.Layer{Key: "clearable"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got model.Layer
	for _, l := range snap.Layers {
		if l.Key == "clearable" {
			got = l
		}
	}
	if len(got.InputSchema) != 0 {
		t.Errorf("expected an omitted inputSchema to clear the stored one, got %+v", got.InputSchema)
	}
	if len(got.OutputSchema) != 0 {
		t.Errorf("expected an omitted outputSchema to clear the stored one, got %+v", got.OutputSchema)
	}
}

// --- Hazard: replacing a layer's schema can invalidate its own segments ---
//
// A rule that reads a field declared in the layer's current inputSchema stops
// validating once that field is dropped from a new inputSchema. commitSnapshot
// validates the whole snapshot before saving, so the update must be rejected
// and the stored snapshot must be left exactly as it was.
func TestLayerSchema_UpdateLayer_InvalidatingSegmentRuleIsRejected(t *testing.T) {
	uc, s, sink := newTestAdminUC()

	_, err := uc.CreateLayer(model.Layer{
		Key:         "hazard",
		InputSchema: model.InputSchema{"age": {Type: model.FieldTypeNumber}},
		Segments: []model.Segment{{
			ID:       "seg1",
			Strategy: model.StrategyRule,
			Rules: []model.Rule{{
				RuleName:  "adult",
				Condition: &model.Condition{Field: "age", Operator: model.OpGt, Value: 18},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("unexpected error creating hazard layer: %v", err)
	}

	before := s.Get()
	sink.saved = nil // reset so we can tell whether the rejected update tried to save

	// Replace the inputSchema with one that no longer declares "age" — the
	// rule above now reads an undeclared field.
	_, err = uc.UpdateLayer("hazard", model.Layer{
		Key:         "hazard",
		InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
	})
	if err == nil {
		t.Fatal("expected the update to be rejected — the rule reads a field the new schema doesn't declare")
	}
	if !strings.Contains(err.Error(), "age") || !strings.Contains(err.Error(), "inputSchema") {
		t.Errorf("expected an error naming the undeclared field and inputSchema, got: %v", err)
	}

	// Nothing should have been persisted: neither the sink nor the store.
	if sink.saved != nil {
		t.Error("expected sink.Save not to be called for a rejected update")
	}
	after := s.Get()
	if after != before {
		t.Error("expected the store to still hold the original snapshot pointer")
	}
	var hazard *model.Layer
	for i := range after.Layers {
		if after.Layers[i].Key == "hazard" {
			hazard = &after.Layers[i]
		}
	}
	if hazard == nil {
		t.Fatal("hazard layer missing from stored snapshot")
	}
	if _, ok := hazard.InputSchema["age"]; !ok {
		t.Error("expected the stored hazard layer's inputSchema to be unchanged (still declaring age)")
	}
}
