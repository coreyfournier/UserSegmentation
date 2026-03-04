package strategy

import "github.com/segmentation-service/segmentation/internal/domain/model"

// RuleStrategy evaluates composite rule trees. First matching rule's successEvent wins.
type RuleStrategy struct{}

func (s *RuleStrategy) Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	for i := range seg.Rules {
		r := &seg.Rules[i]
		if !r.IsEnabled() {
			continue
		}
		if evaluateRule(r, ctx.Context) {
			event := r.SuccessEvent
			if event == "" {
				event = r.RuleName
			}
			return Result{Segment: event, Reason: "rule:" + r.RuleName}, true
		}
	}
	if seg.Default != "" {
		return Result{Segment: seg.Default, Reason: "rule:default"}, true
	}
	return Result{}, false
}

// evaluateRule recursively evaluates a rule node.
func evaluateRule(r *model.Rule, ctx map[string]interface{}) bool {
	if !r.IsEnabled() {
		return false
	}
	if r.IsLeaf() {
		return EvalExpression(r.Expression, ctx)
	}
	// Composite rule
	switch r.Operator {
	case model.CompositeAnd:
		for i := range r.Rules {
			child := &r.Rules[i]
			if !child.IsEnabled() {
				continue
			}
			if !evaluateRule(child, ctx) {
				return false // short-circuit
			}
		}
		return true
	case model.CompositeOr:
		for i := range r.Rules {
			child := &r.Rules[i]
			if !child.IsEnabled() {
				continue
			}
			if evaluateRule(child, ctx) {
				return true // short-circuit
			}
		}
		return false
	default:
		return false
	}
}
