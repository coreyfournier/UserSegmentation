package validation

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// A layer using the escape hatch (no inputSchema, no segment computed fields)
// must still be visible, even though it is not an error — Finding 3 in the
// branch review. Without this, clearing a layer's last input field silently
// turns off rule-field validation for the whole layer.
func TestWarnMissingInputSchemas_WarnsWhenSegmentsCarryRules(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Key: "tier",
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

	warnings := WarnMissingInputSchemas(snap)
	if len(warnings) != 1 {
		t.Fatalf("expected exactly one warning, got: %v", warnings)
	}
	if !strings.Contains(warnings[0], `layer "tier"`) || !strings.Contains(warnings[0], "no inputSchema") ||
		!strings.Contains(warnings[0], "1 segment") {
		t.Fatalf("expected a warning naming the layer and counting its affected segments, got: %q", warnings[0])
	}
}

// A layer that declares an inputSchema is never warned about, regardless of
// what its segments do.
func TestWarnMissingInputSchemas_SilentWhenLayerDeclaresInputSchema(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Key:         "tier",
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

	if warnings := WarnMissingInputSchemas(snap); len(warnings) != 0 {
		t.Fatalf("expected no warnings, got: %v", warnings)
	}
}

// A layer with no inputSchema is also silent when none of its segments carry
// a rule, override, or when predicate to leave unvalidated — matching the
// shipped config's baseTier/transfer-fee layers, which rely on the escape
// hatch for segments that read no fields at all.
func TestWarnMissingInputSchemas_SilentWhenNoSegmentHasRules(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Key: "tier",
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyStatic,
				Static:   &model.StaticConfig{Default: "basic"},
			}},
		}},
	}

	if warnings := WarnMissingInputSchemas(snap); len(warnings) != 0 {
		t.Fatalf("expected no warnings, got: %v", warnings)
	}
}

// A segment carrying its own Computed fields is still validated against
// those even when the layer has no inputSchema (buildEffectiveSchema merges
// computed fields into an empty schema), so it must not count toward the
// warning.
func TestWarnMissingInputSchemas_ExemptsSegmentWithComputedFields(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Key: "tier",
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

	if warnings := WarnMissingInputSchemas(snap); len(warnings) != 0 {
		t.Fatalf("expected no warnings for a segment validated via its own computed fields, got: %v", warnings)
	}
}

// The affected count only includes segments that actually carry a rule tree
// (Rules, Overrides, or When) — a segment with none of those has nothing for
// the escape hatch to leave unvalidated.
func TestWarnMissingInputSchemas_CountsOnlySegmentsWithRuleTrees(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Key: "tier",
			Segments: []model.Segment{
				{ID: "s1", Strategy: model.StrategyStatic, Static: &model.StaticConfig{Default: "x"}},
				{
					ID:       "s2",
					Strategy: model.StrategyRule,
					Rules: []model.Rule{{
						RuleName:  "r",
						Condition: &model.Condition{Field: "f", Operator: model.OpEq, Value: 1},
					}},
				},
				{
					ID:       "s3",
					Strategy: model.StrategyRule,
					When: &model.Rule{
						RuleName:  "w",
						Condition: &model.Condition{Field: "f2", Operator: model.OpEq, Value: 1},
					},
				},
			},
		}},
	}

	warnings := WarnMissingInputSchemas(snap)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "2 segment(s)") {
		t.Fatalf("expected one warning counting 2 segments, got: %v", warnings)
	}
}
