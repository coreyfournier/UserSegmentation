package strategy

import "github.com/segmentation-service/segmentation/internal/domain/model"

// EvalOverrides checks override rules before the primary strategy.
// Returns (result, true) if an override matched.
func EvalOverrides(overrides []model.Rule, ctx *EvalContext) (Result, bool) {
	for i := range overrides {
		r := &overrides[i]
		if !r.IsEnabled() {
			continue
		}
		if evaluateRule(r, ctx.Context) {
			event := r.SuccessEvent
			if event == "" {
				event = r.RuleName
			}
			return Result{Segment: event, Reason: "override:" + r.RuleName}, true
		}
	}
	return Result{}, false
}
