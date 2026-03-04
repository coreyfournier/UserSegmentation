package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func boolPtr(b bool) *bool { return &b }

func TestRuleStrategy_SimpleLeaf(t *testing.T) {
	seg := &model.Segment{
		Strategy: "rule",
		Rules: []model.Rule{
			{
				RuleName:     "us-check",
				SuccessEvent: "us-segment",
				Expression:   &model.Expression{Field: "country", Operator: model.OpEq, Value: "US"},
			},
		},
		Default: "other",
	}
	s := &RuleStrategy{}

	res, ok := s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"country": "US"}})
	if !ok || res.Segment != "us-segment" {
		t.Errorf("got %v %v, want us-segment", res, ok)
	}

	res, ok = s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"country": "UK"}})
	if !ok || res.Segment != "other" {
		t.Errorf("got %v %v, want other (default)", res, ok)
	}
}

func TestRuleStrategy_CompositeAnd(t *testing.T) {
	seg := &model.Segment{
		Strategy: "rule",
		Rules: []model.Rule{
			{
				RuleName:     "premium",
				Operator:     model.CompositeAnd,
				SuccessEvent: "premium",
				Rules: []model.Rule{
					{RuleName: "age", Expression: &model.Expression{Field: "age", Operator: model.OpGte, Value: float64(18)}},
					{RuleName: "country", Expression: &model.Expression{Field: "country", Operator: model.OpEq, Value: "US"}},
				},
			},
		},
	}
	s := &RuleStrategy{}

	// Both conditions met
	res, ok := s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"age": float64(25), "country": "US"}})
	if !ok || res.Segment != "premium" {
		t.Errorf("expected premium, got %v %v", res, ok)
	}

	// Age fails
	_, ok = s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"age": float64(16), "country": "US"}})
	if ok {
		t.Error("expected no match when age < 18")
	}
}

func TestRuleStrategy_CompositeOr(t *testing.T) {
	seg := &model.Segment{
		Strategy: "rule",
		Rules: []model.Rule{
			{
				RuleName:     "eligible",
				Operator:     model.CompositeOr,
				SuccessEvent: "eligible",
				Rules: []model.Rule{
					{RuleName: "us", Expression: &model.Expression{Field: "country", Operator: model.OpEq, Value: "US"}},
					{RuleName: "high-spend", Expression: &model.Expression{Field: "total_spend", Operator: model.OpGte, Value: float64(5000)}},
				},
			},
		},
	}
	s := &RuleStrategy{}

	// First condition met
	res, ok := s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"country": "US", "total_spend": float64(100)}})
	if !ok || res.Segment != "eligible" {
		t.Errorf("expected eligible via country, got %v %v", res, ok)
	}

	// Second condition met
	res, ok = s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"country": "UK", "total_spend": float64(6000)}})
	if !ok || res.Segment != "eligible" {
		t.Errorf("expected eligible via spend, got %v %v", res, ok)
	}

	// Neither
	_, ok = s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"country": "UK", "total_spend": float64(100)}})
	if ok {
		t.Error("expected no match")
	}
}

func TestRuleStrategy_NestedAndOr(t *testing.T) {
	// age >= 18 AND (country == "US" OR total_spend >= 5000)
	seg := &model.Segment{
		Strategy: "rule",
		Rules: []model.Rule{
			{
				RuleName:     "premium-eligible",
				Operator:     model.CompositeAnd,
				SuccessEvent: "premium",
				Rules: []model.Rule{
					{RuleName: "age-check", Expression: &model.Expression{Field: "age", Operator: model.OpGte, Value: float64(18)}},
					{
						RuleName: "region-or-spend",
						Operator: model.CompositeOr,
						Rules: []model.Rule{
							{RuleName: "us-user", Expression: &model.Expression{Field: "country", Operator: model.OpEq, Value: "US"}},
							{RuleName: "high-spender", Expression: &model.Expression{Field: "total_spend", Operator: model.OpGte, Value: float64(5000)}},
						},
					},
				},
			},
		},
	}
	s := &RuleStrategy{}

	// age 25, US — match
	res, ok := s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"age": float64(25), "country": "US", "total_spend": float64(100)}})
	if !ok || res.Segment != "premium" {
		t.Errorf("expected premium, got %v %v", res, ok)
	}

	// age 25, UK, high spend — match
	res, ok = s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"age": float64(25), "country": "UK", "total_spend": float64(6000)}})
	if !ok || res.Segment != "premium" {
		t.Errorf("expected premium via spend, got %v %v", res, ok)
	}

	// age 16, US — no match (age fails)
	_, ok = s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"age": float64(16), "country": "US", "total_spend": float64(6000)}})
	if ok {
		t.Error("expected no match for underage")
	}
}

func TestRuleStrategy_DisabledRule(t *testing.T) {
	seg := &model.Segment{
		Strategy: "rule",
		Rules: []model.Rule{
			{
				RuleName:     "disabled-rule",
				Enabled:      boolPtr(false),
				SuccessEvent: "should-not-match",
				Expression:   &model.Expression{Field: "country", Operator: model.OpEq, Value: "US"},
			},
		},
		Default: "fallback",
	}
	s := &RuleStrategy{}
	res, ok := s.Evaluate(seg, &EvalContext{Context: map[string]interface{}{"country": "US"}})
	if !ok || res.Segment != "fallback" {
		t.Errorf("expected fallback for disabled rule, got %v %v", res, ok)
	}
}
