package strategy

import "github.com/segmentation-service/segmentation/internal/domain/model"

// EvalOverrides returns the first enabled override that matches.
//
// Conditions match against raw input only: the override editor offers the
// segment's inputSchema and nothing else, so computed fields are deliberately
// out of scope for matching. Output values are different — they may read
// computed fields, so they resolve against the enriched context. Enriching here
// cannot void the segment, because an override never runs in collect mode.
func EvalOverrides(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	for i := range seg.Overrides {
		r := &seg.Overrides[i]
		if !r.IsEnabled() {
			continue
		}
		if !evaluateRule(r, ctx.Context, ctx.Lookups) {
			continue
		}

		event := r.SuccessEvent
		if event == "" {
			event = r.RuleName
		}
		res := Result{Segment: event, Reason: "override:" + r.RuleName}
		applyMessages(&res, r.Messages, ctx)

		if len(seg.OutputSchema) > 0 {
			outCtx := ctx
			if len(seg.Computed) > 0 {
				enriched, _, _ := enrichWithComputed(seg.Computed, ctx.Context)
				derived := *ctx
				derived.Context = enriched
				outCtx = &derived
			}
			outputs, outErrs := evaluateOutputs(seg, r.Outputs, outCtx)
			res.Outputs = outputs
			res.RenderErrors = append(res.RenderErrors, outErrs...)
		}
		return res, true
	}
	return Result{}, false
}
