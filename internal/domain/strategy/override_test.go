package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestEvalOverrides_Match(t *testing.T) {
	overrides := []model.Rule{
		{
			RuleName:     "vip-override",
			SuccessEvent: "vip-segment",
			Expression:   &model.Expression{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
		},
	}

	ctx := &EvalContext{Context: map[string]interface{}{"plan": "enterprise"}}
	res, ok := EvalOverrides(overrides, ctx)
	if !ok || res.Segment != "vip-segment" {
		t.Errorf("expected vip-segment, got %v %v", res, ok)
	}
}

func TestEvalOverrides_NoMatch(t *testing.T) {
	overrides := []model.Rule{
		{
			RuleName:     "vip-override",
			SuccessEvent: "vip-segment",
			Expression:   &model.Expression{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
		},
	}

	ctx := &EvalContext{Context: map[string]interface{}{"plan": "free"}}
	_, ok := EvalOverrides(overrides, ctx)
	if ok {
		t.Error("expected no match")
	}
}

func TestEvalOverrides_Disabled(t *testing.T) {
	overrides := []model.Rule{
		{
			RuleName:     "disabled",
			Enabled:      boolPtr(false),
			SuccessEvent: "should-skip",
			Expression:   &model.Expression{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
		},
	}

	ctx := &EvalContext{Context: map[string]interface{}{"plan": "enterprise"}}
	_, ok := EvalOverrides(overrides, ctx)
	if ok {
		t.Error("expected disabled override to be skipped")
	}
}
