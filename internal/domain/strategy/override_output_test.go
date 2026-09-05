package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// An override's output values resolve against the computed-enriched context,
// so a template token can read a field the segment's own formulas derive —
// even though the override's condition never sees that field (pinned by
// TestEvalOverrides_ConditionCannotSeeComputedFields below).
func TestEvalOverrides_Outputs(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Computed: []model.ComputedField{
			{Name: "derived", Type: model.FieldTypeNumber, Formula: "base * 2"},
		},
		Overrides: []model.Rule{
			{
				RuleName:     "vip-override",
				SuccessEvent: "vip-segment",
				Condition:    &model.Condition{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
				Outputs: map[string]string{
					"tier":    "gold",
					"derived": "${derived}",
				},
			},
		},
	}

	ctx := &EvalContext{
		Context: map[string]interface{}{"plan": "enterprise", "base": 21},
		OutputSchema: model.OutputSchema{
			"tier":    model.OutputField{Type: model.FieldTypeString},
			"derived": model.OutputField{Type: model.FieldTypeString, Eval: model.EvalTemplate},
		},
	}
	res, ok := EvalOverrides(seg, ctx)
	if !ok || res.Segment != "vip-segment" {
		t.Fatalf("expected vip-segment, got %v %v", res, ok)
	}
	if got := res.Outputs["tier"]; got != "gold" {
		t.Errorf("expected tier = %q (literal), got %v", "gold", got)
	}
	if got := res.Outputs["derived"]; got != "42" {
		t.Errorf("expected derived = %q (rendered from the computed field), got %v", "42", got)
	}
}

// The asymmetry pin: an override's *condition* matches against raw input
// only. The override editor is handed the segment's inputSchema and nothing
// else, so a condition testing a computed field must never match — even
// though the very same segment lets that field's value flow into an
// override's *output* template (TestEvalOverrides_Outputs above).
func TestEvalOverrides_ConditionCannotSeeComputedFields(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Computed: []model.ComputedField{
			{Name: "derived", Type: model.FieldTypeNumber, Formula: "base * 2"},
		},
		Overrides: []model.Rule{
			{
				RuleName:     "computed-condition",
				SuccessEvent: "should-not-match",
				Condition:    &model.Condition{Field: "derived", Operator: model.OpEq, Value: 42},
			},
		},
	}

	ctx := &EvalContext{Context: map[string]interface{}{"base": 21}}
	res, ok := EvalOverrides(seg, ctx)
	if ok {
		t.Errorf("expected the override's condition to not match a computed field, got %v", res)
	}
}
