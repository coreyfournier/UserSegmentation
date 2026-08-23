package strategy

import "github.com/segmentation-service/segmentation/internal/domain/model"

// AssertStrategy validates an entity against a set of assertions, itemising
// every one that does not hold rather than stopping at the first. It is the
// strategy a validation gate is built from.
//
// It is the third link in an existing delegation chain:
//
//	AssertStrategy  ->  ExpressionStrategy  ->  RuleStrategy
//	   sets                enriches with          collects failures
//	   CollectFailures      computed fields       instead of short-circuiting
//	   computes Status
//
// so it inherits expr-lang computed fields for free, and the whole-tree walk
// lives in one place instead of being duplicated per strategy.
type AssertStrategy struct {
	expressions ExpressionStrategy
}

func (s *AssertStrategy) Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	collecting := *ctx
	collecting.CollectFailures = true

	res, ok := s.expressions.Evaluate(seg, &collecting)
	if !ok {
		return res, false
	}

	// ExpressionStrategy already reports unevaluable when a computed field
	// failed at runtime; only the satisfied/violated distinction is left.
	if res.Status == "" {
		if len(res.Failures) > 0 {
			res.Status = model.StatusViolated
		} else {
			res.Status = model.StatusSatisfied
		}
	}

	// An assert segment resolves no segment value, so nothing is injected into
	// the context as "layer:<name>". Dependent layers gate on status instead.
	res.Segment = ""
	return res, true
}
