package validation

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestValidateSnapshot_ValidConfig(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Key: "test",
				InputSchema: model.InputSchema{
					"country": {Type: model.FieldTypeString, Required: true},
					"age":     {Type: model.FieldTypeNumber, Required: false},
				},
				Segments: []model.Segment{
					{
						ID:       "seg1",
						Strategy: model.StrategyRule,
						Rules: []model.Rule{
							{
								RuleName:  "check",
								Condition: &model.Condition{Field: "country", Operator: model.OpEq, Value: "US"},
							},
							{
								RuleName:  "age-check",
								Condition: &model.Condition{Field: "age", Operator: model.OpGte, Value: 18},
							},
						},
					},
				},
			},
		},
	}
	if err := ValidateSnapshot(snap); err != nil {
		t.Errorf("expected valid config, got: %v", err)
	}
}

func TestValidateSnapshot_MissingField(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Key:         "test",
				InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
				Segments: []model.Segment{
					{
						ID:       "seg1",
						Strategy: model.StrategyRule,
						Rules: []model.Rule{
							{RuleName: "bad", Condition: &model.Condition{Field: "missing_field", Operator: model.OpEq, Value: "x"}},
						},
					},
				},
			},
		},
	}
	if err := ValidateSnapshot(snap); err == nil {
		t.Error("expected validation error for missing field")
	}
}

func TestValidateSnapshot_IncompatibleOperator(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Key:         "test",
				InputSchema: model.InputSchema{"name": {Type: model.FieldTypeString}},
				Segments: []model.Segment{
					{
						ID:       "seg1",
						Strategy: model.StrategyRule,
						Rules: []model.Rule{
							{RuleName: "bad", Condition: &model.Condition{Field: "name", Operator: model.OpGt, Value: "x"}},
						},
					},
				},
			},
		},
	}
	if err := ValidateSnapshot(snap); err == nil {
		t.Error("expected validation error for gt on string")
	}
}

// crossLayerSnapshot builds a two-layer config where "test" reads "baseTier",
// declaring the dependency only when declared is true.
func crossLayerSnapshot(declared bool) *model.Snapshot {
	layer := model.Layer{
		Key:         "test",
		InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
		Segments: []model.Segment{
			{
				ID:       "seg1",
				Strategy: model.StrategyRule,
				Rules: []model.Rule{
					{RuleName: "cross", Condition: &model.Condition{Field: "layer:baseTier", Operator: model.OpEq, Value: "pro"}},
				},
			},
		},
	}
	if declared {
		layer.DependsOn = []string{"baseTier"}
	}
	return &model.Snapshot{
		Layers: []model.Layer{{Key: "baseTier"}, layer},
	}
}

func TestValidateSnapshot_CrossLayerRef_Declared(t *testing.T) {
	if err := ValidateSnapshot(crossLayerSnapshot(true)); err != nil {
		t.Errorf("declared cross-layer ref should be valid, got: %v", err)
	}
}

func TestValidateSnapshot_CrossLayerRef_Undeclared(t *testing.T) {
	err := ValidateSnapshot(crossLayerSnapshot(false))
	if err == nil {
		t.Fatal("expected an error for a cross-layer ref not declared in dependsOn")
	}
	if !strings.Contains(err.Error(), "not declared in dependsOn") {
		t.Errorf("expected a dependsOn error, got: %v", err)
	}
}

// A dependency declared purely for ordering or gating, with no layer: reference
// anywhere, is legitimate and must not be flagged.
func TestValidateSnapshot_DependencyWithoutReference(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{Key: "gate"},
			{
				Key:         "downstream",
				DependsOn:   []string{"gate"},
				InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
				Segments: []model.Segment{
					{
						ID:       "seg1",
						Strategy: model.StrategyRule,
						Rules: []model.Rule{
							{RuleName: "plain", Condition: &model.Condition{Field: "country", Operator: model.OpEq, Value: "US"}},
						},
					},
				},
			},
		},
	}
	if err := ValidateSnapshot(snap); err != nil {
		t.Errorf("unreferenced dependency should be legal, got: %v", err)
	}
}

func TestCheckRequiredFields(t *testing.T) {
	seg := &model.Segment{ID: "test"}
	schema := model.InputSchema{
		"country": {Type: model.FieldTypeString, Required: true},
		"age":     {Type: model.FieldTypeNumber, Required: false},
	}

	warnings := CheckRequiredFields(seg, schema, map[string]interface{}{"age": 25})
	if len(warnings) != 1 || warnings[0].Field != "country" {
		t.Errorf("expected warning for missing country, got %v", warnings)
	}

	warnings = CheckRequiredFields(seg, schema, map[string]interface{}{"country": "US"})
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}

// A segment id must be present and unique within its layer. Neither was
// checked at load: only CreateSegment refused a duplicate, so an imported or
// hand-edited config could hold two segments sharing an id — and findSegment
// only ever reaches the first, leaving the second uneditable while the
// evaluator still runs it.
func TestValidate_SegmentIDMustBeUniqueWithinItsLayer(t *testing.T) {
	dup := func(a, b string) *model.Snapshot {
		return &model.Snapshot{Layers: []model.Layer{{
			Key: "tier",
			Segments: []model.Segment{
				{ID: a, Strategy: model.StrategyRule, Default: "x"},
				{ID: b, Strategy: model.StrategyRule, Default: "y"},
			},
		}}}
	}

	err := ValidateSnapshot(dup("same", "same"))
	if err == nil || !strings.Contains(err.Error(), `duplicate segment id "same"`) {
		t.Errorf("expected a duplicate-id error, got %v", err)
	}

	if err := ValidateSnapshot(dup("one", "two")); err != nil {
		t.Errorf("distinct ids must validate, got %v", err)
	}

	// The same id under a different layer is fine: a segment is addressed
	// within its layer, so there is nothing to collide with.
	twoLayers := &model.Snapshot{Layers: []model.Layer{
		{Key: "a", Segments: []model.Segment{{ID: "seg", Strategy: model.StrategyRule, Default: "x"}}},
		{Key: "b", Segments: []model.Segment{{ID: "seg", Strategy: model.StrategyRule, Default: "y"}}},
	}}
	if err := ValidateSnapshot(twoLayers); err != nil {
		t.Errorf("the same id in two layers must validate, got %v", err)
	}
}

func TestValidate_SegmentIDIsRequired(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{{
		Key:      "tier",
		Segments: []model.Segment{{Strategy: model.StrategyRule, Default: "x"}},
	}}}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "has no id") {
		t.Errorf("expected a missing-id error, got %v", err)
	}
}
