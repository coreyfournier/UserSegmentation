package validation

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// 1. A rule reading a field declared on its layer validates; one reading an
// undeclared field errors.
func TestLayerSchema_RuleFieldDeclaredOnLayer(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name:        "tier",
			InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyRule,
				Rules: []model.Rule{{
					RuleName:  "check",
					Condition: &model.Condition{Field: "country", Operator: model.OpEq, Value: "US"},
				}},
			}},
		}},
	}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("field declared on the layer should validate, got: %v", err)
	}
}

func TestLayerSchema_RuleFieldUndeclaredOnLayer(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name:        "tier",
			InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyRule,
				Rules: []model.Rule{{
					RuleName:  "check",
					Condition: &model.Condition{Field: "unknown_field", Operator: model.OpEq, Value: "US"},
				}},
			}},
		}},
	}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "not in inputSchema") {
		t.Fatalf("expected an undeclared-field error, got: %v", err)
	}
}

// 2. A layer schema plus a segment's computed — a rule reading the computed
// field validates, proving the merge still happens per segment.
func TestLayerSchema_ComputedFieldMergesWithLayerSchema(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name:        "tier",
			InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyRule,
				Computed: []model.ComputedField{
					{Name: "score", Type: model.FieldTypeNumber, Formula: "1 + 1"},
				},
				Rules: []model.Rule{{
					RuleName:  "check",
					Condition: &model.Condition{Field: "score", Operator: model.OpGt, Value: 0},
				}},
			}},
		}},
	}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("a rule reading a segment's computed field should validate, got: %v", err)
	}
}

// 3. The escape hatch: a layer with no input schema whose segments declare no
// computed fields still skips rule-field validation entirely.
func TestLayerSchema_EscapeHatchWhenNoInputSchemaAndNoComputed(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "tier",
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyRule,
				Rules: []model.Rule{{
					RuleName:  "check",
					Condition: &model.Condition{Field: "whatever", Operator: model.OpEq, Value: "US"},
				}},
			}},
		}},
	}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("a layer with no input schema and no computed fields should skip rule-field validation, got: %v", err)
	}
}

// 4. A segment carrying a legacy inputSchema is rejected, naming the segment
// and telling the author to move it to the layer. Same for outputSchema.
func TestLayerSchema_LegacySegmentInputSchemaIsRejected(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "tier",
			Segments: []model.Segment{{
				ID:                "seg",
				Strategy:          model.StrategyRule,
				LegacyInputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
			}},
		}},
	}
	err := ValidateSnapshot(snap)
	if err == nil {
		t.Fatal("expected a legacy inputSchema on a segment to be rejected")
	}
	if !strings.Contains(err.Error(), `segment "seg"`) || !strings.Contains(err.Error(), "inputSchema") ||
		!strings.Contains(err.Error(), "layer") {
		t.Fatalf("expected an error naming the segment and pointing to the layer, got: %v", err)
	}
}

func TestLayerSchema_LegacySegmentOutputSchemaIsRejected(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "tier",
			Segments: []model.Segment{{
				ID:                 "seg",
				Strategy:           model.StrategyStatic,
				LegacyOutputSchema: model.OutputSchema{"category": {Type: model.FieldTypeString}},
			}},
		}},
	}
	err := ValidateSnapshot(snap)
	if err == nil {
		t.Fatal("expected a legacy outputSchema on a segment to be rejected")
	}
	if !strings.Contains(err.Error(), `segment "seg"`) || !strings.Contains(err.Error(), "outputSchema") ||
		!strings.Contains(err.Error(), "layer") {
		t.Fatalf("expected an error naming the segment and pointing to the layer, got: %v", err)
	}
}

// 5. A required output field on the layer is enforced against a segment that
// authors nothing.
func TestLayerSchema_RequiredOutputFieldEnforcedFromLayer(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name:         "diagnostics",
			InputSchema:  model.InputSchema{"x": {Type: model.FieldTypeString}},
			OutputSchema: model.OutputSchema{"category": {Type: model.FieldTypeString, Required: true}},
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyChecklist,
				Rules: []model.Rule{{
					RuleName:  "someCheck",
					Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
				}},
			}},
		}},
	}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "someCheck") {
		t.Fatalf("expected the unauthored required output to be reported, got: %v", err)
	}
}

// 6. An undeclared output key is rejected against the layer's output schema.
func TestLayerSchema_UndeclaredOutputKeyRejectedFromLayer(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name:         "diagnostics",
			InputSchema:  model.InputSchema{"x": {Type: model.FieldTypeString}},
			OutputSchema: model.OutputSchema{"category": {Type: model.FieldTypeString}},
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyRule,
				Outputs:  map[string]string{"typo": "x"},
				Rules: []model.Rule{{
					RuleName:  "check",
					Condition: &model.Condition{Field: "x", Operator: model.OpEq, Value: "US"},
				}},
			}},
		}},
	}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "typo") || !strings.Contains(err.Error(), `segment "seg"`) {
		t.Fatalf("expected the unknown output key and segment to be named, got: %v", err)
	}
}

// 7. A static segment in a layer with an output schema is still exempt.
func TestLayerSchema_StaticSegmentExemptFromLayerOutputSchema(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "tier",
			OutputSchema: model.OutputSchema{
				"category": {Type: model.FieldTypeString, Required: true},
			},
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyStatic,
				Static:   &model.StaticConfig{Default: "basic"},
				// Neither an unauthored required field nor an undeclared key
				// is enforced for a static segment.
				Outputs: map[string]string{"unlisted": "x"},
			}},
		}},
	}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("a static segment should be exempt from the layer's output schema, got: %v", err)
	}
}
