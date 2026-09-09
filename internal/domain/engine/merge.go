package engine

import (
	"strings"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// tagFailures stamps each finding with the segment that reported it.
//
// Every finding carries it, not only those from a layer that ran several
// segments. It was conditional at first, to keep a single-segment layer's
// response byte-identical to what it had always been — but that made the field
// a property of the layer's shape rather than of the finding, so a consumer
// could not rely on it and adding a second segment silently changed the
// response of every finding in the layer.
func tagFailures(failures []model.Failure, segID string) []model.Failure {
	if len(failures) == 0 {
		return nil
	}
	out := make([]model.Failure, len(failures))
	copy(out, failures)
	for i := range out {
		out[i].Segment = segID
	}
	return out
}

// finishLayer completes a layer result by naming every contributing segment in
// the reason, where more than one contributed.
//
// The single-contributor reason is left as the strategy wrote it
// ("checklist:stage1"), which already names the segment — rewriting it to the
// bare id would drop the strategy the reason exists to report.
func finishLayer(lr *LayerResult, contributed []string) *LayerResult {
	if len(contributed) > 1 && lr.Assignment != nil {
		lr.Assignment.Reason = strings.Join(contributed, " + ")
	}
	return dedupRequiredFieldWarnings(lr)
}

// mergeStatus combines the status of another contributing segment with what the
// layer has so far.
//
// first says this is the first contributor, whose status stands as-is —
// there is nothing to be worse than yet.
//
// Otherwise the worst wins, and "worst" puts unevaluable above violated:
// violated with a partial list of findings invites acting on it as though it
// were the whole truth, while unevaluable says plainly that part of the answer
// is missing. The findings that were collected are still reported either way.
func mergeStatus(have, next model.LayerStatus, first bool) model.LayerStatus {
	if first || have == "" {
		return next
	}
	if statusRank(next) > statusRank(have) {
		return next
	}
	return have
}

func statusRank(s model.LayerStatus) int {
	switch s {
	case model.StatusUnevaluable:
		return 3
	case model.StatusViolated:
		return 2
	case model.StatusSatisfied:
		return 1
	default:
		return 0
	}
}

// mergeAssignment folds another segment's assignment into the layer's.
//
// The first one stands: its strategy and reason describe how the layer began
// answering, and finishLayer rewrites the reason to name every contributor.
// Computed values accumulate, first writer winning a collision — two segments
// deriving the same name are describing the same subject, and silently
// preferring the later one would make the reported value depend on segment
// order for no stated reason.
func mergeAssignment(have, next *model.Assignment) *model.Assignment {
	if have == nil {
		return next
	}
	if next == nil {
		return have
	}
	for k, v := range next.Computed {
		if have.Computed == nil {
			have.Computed = map[string]interface{}{}
		}
		if _, exists := have.Computed[k]; !exists {
			have.Computed[k] = v
		}
	}
	return have
}
