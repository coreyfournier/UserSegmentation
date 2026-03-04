package strategy

import "github.com/segmentation-service/segmentation/internal/domain/model"

// StaticStrategy assigns segments via direct subject key lookup.
type StaticStrategy struct{}

func (s *StaticStrategy) Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	if seg.Static == nil {
		return Result{}, false
	}
	if mapped, ok := seg.Static.Mappings[ctx.SubjectKey]; ok {
		return Result{Segment: mapped, Reason: "static:mapping"}, true
	}
	if seg.Static.Default != "" {
		return Result{Segment: seg.Static.Default, Reason: "static:default"}, true
	}
	return Result{}, false
}
