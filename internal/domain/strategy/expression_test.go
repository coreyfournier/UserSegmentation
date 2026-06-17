package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestExpressionStrategy_ComputedFieldUsedInRule(t *testing.T) {
	seg := &model.Segment{
		Strategy: "expression",
		Expressions: []model.ExpressionDef{
			{Name: "Adjusted", Type: "number", Expression: "Rating * 2"},
		},
		Rules: []model.Rule{
			{
				RuleName:     "high",
				SuccessEvent: "high-tier",
				Expression:   &model.Expression{Field: "Adjusted", Operator: "gt", Value: float64(10)},
			},
		},
		Default: "normal",
	}

	s := &ExpressionStrategy{}

	// Rating=6 → Adjusted=12 → matches "gt 10"
	res, ok := s.Evaluate(seg, &EvalContext{
		SubjectKey: "user1",
		Context:    map[string]interface{}{"Rating": float64(6)},
	})
	if !ok || res.Segment != "high-tier" {
		t.Errorf("expected high-tier, got %v %v", res, ok)
	}

	// Rating=4 → Adjusted=8 → no rule match → default
	res, ok = s.Evaluate(seg, &EvalContext{
		SubjectKey: "user2",
		Context:    map[string]interface{}{"Rating": float64(4)},
	})
	if !ok || res.Segment != "normal" {
		t.Errorf("expected normal (default), got %v %v", res, ok)
	}
}

func TestExpressionStrategy_ExpressionOverwritesContext(t *testing.T) {
	seg := &model.Segment{
		Strategy: "expression",
		Expressions: []model.ExpressionDef{
			{Name: "Score", Type: "number", Expression: "Base + Bonus"},
		},
		Rules: []model.Rule{
			{
				RuleName:     "winner",
				SuccessEvent: "winner",
				Expression:   &model.Expression{Field: "Score", Operator: "gte", Value: float64(100)},
			},
		},
	}

	s := &ExpressionStrategy{}

	// Score from inputSchema would be 50, but expression computes Base+Bonus=120 → overwrites
	res, ok := s.Evaluate(seg, &EvalContext{
		SubjectKey: "u",
		Context:    map[string]interface{}{"Base": float64(80), "Bonus": float64(40), "Score": float64(50)},
	})
	if !ok || res.Segment != "winner" {
		t.Errorf("expression should overwrite context Score: got %v %v", res, ok)
	}
}

func TestExpressionStrategy_ChainedExpressions(t *testing.T) {
	seg := &model.Segment{
		Strategy: "expression",
		Expressions: []model.ExpressionDef{
			{Name: "Double", Type: "number", Expression: "X * 2"},
			{Name: "Quad", Type: "number", Expression: "Double * 2"}, // references previous
		},
		Rules: []model.Rule{
			{
				RuleName:     "quad",
				SuccessEvent: "quad",
				Expression:   &model.Expression{Field: "Quad", Operator: "eq", Value: float64(20)},
			},
		},
	}

	s := &ExpressionStrategy{}

	res, ok := s.Evaluate(seg, &EvalContext{
		SubjectKey: "u",
		Context:    map[string]interface{}{"X": float64(5)},
	})
	if !ok || res.Segment != "quad" {
		t.Errorf("chained expression: expected quad, got %v %v", res, ok)
	}
}

func TestExpressionStrategy_BadExpressionSkipped(t *testing.T) {
	seg := &model.Segment{
		Strategy: "expression",
		Expressions: []model.ExpressionDef{
			{Name: "Bad", Type: "number", Expression: "!!!invalid!!!"}, // compile error
			{Name: "Good", Type: "number", Expression: "X + 1"},
		},
		Rules: []model.Rule{
			{
				RuleName:     "ok",
				SuccessEvent: "ok",
				Expression:   &model.Expression{Field: "Good", Operator: "eq", Value: float64(6)},
			},
		},
	}

	s := &ExpressionStrategy{}

	res, ok := s.Evaluate(seg, &EvalContext{
		SubjectKey: "u",
		Context:    map[string]interface{}{"X": float64(5)},
	})
	if !ok || res.Segment != "ok" {
		t.Errorf("bad expression should be skipped: got %v %v", res, ok)
	}
}
