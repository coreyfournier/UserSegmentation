package validation

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestValidateSnapshot_ValidConfig(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "test",
				Segments: []model.Segment{
					{
						ID: "seg1",
						InputSchema: model.InputSchema{
							"country": {Type: model.FieldTypeString, Required: true},
							"age":     {Type: model.FieldTypeNumber, Required: false},
						},
						Rules: []model.Rule{
							{
								RuleName:   "check",
								Expression: &model.Expression{Field: "country", Operator: model.OpEq, Value: "US"},
							},
							{
								RuleName:   "age-check",
								Expression: &model.Expression{Field: "age", Operator: model.OpGte, Value: 18},
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
				Name: "test",
				Segments: []model.Segment{
					{
						ID:          "seg1",
						InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
						Rules: []model.Rule{
							{RuleName: "bad", Expression: &model.Expression{Field: "missing_field", Operator: model.OpEq, Value: "x"}},
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
				Name: "test",
				Segments: []model.Segment{
					{
						ID:          "seg1",
						InputSchema: model.InputSchema{"name": {Type: model.FieldTypeString}},
						Rules: []model.Rule{
							{RuleName: "bad", Expression: &model.Expression{Field: "name", Operator: model.OpGt, Value: "x"}},
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

func TestValidateSnapshot_CrossLayerRef(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "test",
				Segments: []model.Segment{
					{
						ID:          "seg1",
						InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
						Rules: []model.Rule{
							{RuleName: "cross", Expression: &model.Expression{Field: "layer:base-tier", Operator: model.OpEq, Value: "pro"}},
						},
					},
				},
			},
		},
	}
	if err := ValidateSnapshot(snap); err != nil {
		t.Errorf("cross-layer ref should be valid, got: %v", err)
	}
}

func TestCheckRequiredFields(t *testing.T) {
	seg := &model.Segment{
		ID: "test",
		InputSchema: model.InputSchema{
			"country": {Type: model.FieldTypeString, Required: true},
			"age":     {Type: model.FieldTypeNumber, Required: false},
		},
	}

	warnings := CheckRequiredFields(seg, map[string]interface{}{"age": 25})
	if len(warnings) != 1 || warnings[0].Field != "country" {
		t.Errorf("expected warning for missing country, got %v", warnings)
	}

	warnings = CheckRequiredFields(seg, map[string]interface{}{"country": "US"})
	if len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
	}
}
