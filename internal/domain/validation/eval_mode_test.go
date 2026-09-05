package validation

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// A field's eval mode is no longer declared — it is derived from its type: a
// string is a template (never syntax-checked at load), and everything else is
// an expression (always syntax-checked at load). These tests cover that
// derivation from validation's point of view.

func TestValidate_StringFieldWithTemplateTokenValidates(t *testing.T) {
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeString})
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "${x}"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("a string field's template value should validate, got %v", err)
	}
}

func TestValidate_NumberFieldWithBrokenExpressionErrors(t *testing.T) {
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeNumber})
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "amount *"}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "field") {
		t.Fatalf("expected a syntax error naming the field, got %v", err)
	}
}

func TestValidate_NumberFieldWithBareNumeralValidates(t *testing.T) {
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeNumber})
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "3"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("a bare numeral is a valid expression, got %v", err)
	}
}

func TestValidate_LegacyEvalKeyIsRejected(t *testing.T) {
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeString, LegacyEval: "expression"})
	err := ValidateSnapshot(snap)
	if err == nil {
		t.Fatal("expected a legacy-eval error")
	}
	if !strings.Contains(err.Error(), "field") || !strings.Contains(err.Error(), "eval") {
		t.Fatalf("expected the error to name the field and mention \"eval\", got %v", err)
	}
}

// A legacy "eval" key lives on the layer's OutputSchema field, not on any
// segment, so it must be caught even for a layer with zero segments —
// reachable via CreateLayer, which starts a new layer with an empty
// Segments slice, before any segment is added in the UI.
func TestValidate_LegacyEvalKeyRejectedOnZeroSegmentLayer(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "diagnostics",
			OutputSchema: model.OutputSchema{
				"field": {Type: model.FieldTypeString, LegacyEval: "template"},
			},
			Segments: []model.Segment{},
		}},
	}
	err := ValidateSnapshot(snap)
	if err == nil {
		t.Fatal("expected a legacy-eval error even with zero segments")
	}
	if !strings.Contains(err.Error(), "diagnostics") || !strings.Contains(err.Error(), "field") || !strings.Contains(err.Error(), "eval") {
		t.Fatalf("expected the error to name the layer and field and mention \"eval\", got %v", err)
	}
}

func TestValidate_ObjectFieldValidatesWithNoEval(t *testing.T) {
	// Previously an object field required eval: "expression" to be declared;
	// now the mode is derived from the type, so an object field with no eval
	// anywhere on it is valid on its own.
	if err := ValidateSnapshot(snapWithOutputField(model.OutputField{
		Type: model.FieldTypeObject,
	})); err != nil {
		t.Fatalf("an object field should validate with no eval declared, got %v", err)
	}
}

func TestValidate_ArrayFieldValidatesWithNoEval(t *testing.T) {
	// array is not expressible as a literal or a template either, and used to
	// carry the same expression-mode requirement as object; now the mode is
	// derived from the type, so an array field with no eval anywhere on it is
	// valid on its own.
	if err := ValidateSnapshot(snapWithOutputField(model.OutputField{
		Type: model.FieldTypeArray,
	})); err != nil {
		t.Fatalf("an array field should validate with no eval declared, got %v", err)
	}
}

func TestValidate_DisabledRuleBrokenExpressionStillExempt(t *testing.T) {
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeNumber})
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "amount *"}
	disabled := false
	snap.Layers[0].Segments[0].Rules[0].Enabled = &disabled

	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("a disabled rule's broken expression should be exempt, got %v", err)
	}
}
