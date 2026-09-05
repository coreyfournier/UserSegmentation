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
				Name: "test",
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
				Name:        "test",
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
				Name:        "test",
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

// crossLayerSnapshot builds a two-layer config where "test" reads "base-tier",
// declaring the dependency only when declared is true.
func crossLayerSnapshot(declared bool) *model.Snapshot {
	layer := model.Layer{
		Name:        "test",
		InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
		Segments: []model.Segment{
			{
				ID:       "seg1",
				Strategy: model.StrategyRule,
				Rules: []model.Rule{
					{RuleName: "cross", Condition: &model.Condition{Field: "layer:base-tier", Operator: model.OpEq, Value: "pro"}},
				},
			},
		},
	}
	if declared {
		layer.DependsOn = []string{"base-tier"}
	}
	return &model.Snapshot{
		Layers: []model.Layer{{Name: "base-tier"}, layer},
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
			{Name: "gate"},
			{
				Name:        "downstream",
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
