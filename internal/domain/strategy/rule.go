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
			res := Result{Segment: event, Reason: "rule:" + r.RuleName}
			applyMessages(&res, r.Messages, ctx)
			return res, true
		}
	}
	if seg.Default != "" {
		res := Result{Segment: seg.Default, Reason: "rule:default"}
		applyMessages(&res, seg.DefaultMessages, ctx)
		return res, true
	}
	return Result{}, false
}

// applyMessages renders the raw localized templates against the eval context and
// attaches the rendered messages and any render errors to res.
func applyMessages(res *Result, raw map[string]string, ctx *EvalContext) {
	if len(raw) == 0 {
		return
	}
	rr := RenderMessages(raw, ctx.Context, ctx.Languages, ctx.RenderAll, ctx.DefaultLanguage)
	if len(rr.Rendered) > 0 {
		res.Messages = rr.Rendered
	}
	res.RenderErrors = append(res.RenderErrors, rr.Errors...)
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
