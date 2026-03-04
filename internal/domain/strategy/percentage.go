package strategy

import (
	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/ports"
)

// PercentageStrategy assigns segments based on deterministic hashing.
type PercentageStrategy struct {
	Hasher ports.Hasher
}

func (s *PercentageStrategy) Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	if seg.Percentage == nil || len(seg.Percentage.Buckets) == 0 {
		return Result{}, false
	}
	bucket := s.Hasher.Bucket(ctx.UserKey, seg.Percentage.Salt)
	cumulative := 0
	for _, b := range seg.Percentage.Buckets {
		cumulative += b.Weight
		if bucket < cumulative {
			return Result{
				Segment: b.Segment,
				Reason:  "percentage:" + seg.Percentage.Salt,
			}, true
		}
	}
	// Fallback to last bucket if weights don't sum to 100
	last := seg.Percentage.Buckets[len(seg.Percentage.Buckets)-1]
	return Result{Segment: last.Segment, Reason: "percentage:" + seg.Percentage.Salt}, true
}
