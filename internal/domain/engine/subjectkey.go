package engine

import (
	"fmt"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// resolveSubjectKey returns the subject key from the evaluation context and
// whether one was present.
//
// Non-string values are converted rather than rejected: the field's declared
// type is the author's to choose, and an employee id is as plausibly a number
// as a string. Worth knowing that the conversion is what gets hashed, so
// retyping the field re-buckets every subject in a percentage rollout — the
// same value under a different type is a different bucket.
//
// An empty string counts as absent. A static segment would otherwise fall
// through to its default and a percentage segment would hash "" — putting every
// such subject in one bucket, which is a wrong answer that looks like a right
// one.
func resolveSubjectKey(ctx map[string]interface{}) (string, bool) {
	v, ok := model.ResolveField(ctx, model.SubjectKeyField)
	if !ok || v == nil {
		return "", false
	}
	s := fmt.Sprint(v)
	if s == "" {
		return "", false
	}
	return s, true
}

// needsSubjectKey reports whether a segment's strategy reads the subject key.
// Only these two do; a rule or checklist segment never touches it.
func needsSubjectKey(seg *model.Segment) bool {
	return seg.Strategy == model.StrategyStatic || seg.Strategy == model.StrategyPercentage
}

// dropRequiredFieldWarning removes the generic "required field missing"
// warning for one segment and field, so a more specific warning about the same
// cause can replace rather than accompany it.
func dropRequiredFieldWarning(warnings []model.Warning, segID, field string) []model.Warning {
	out := warnings[:0:0]
	for _, w := range warnings {
		if w.Segment == segID && w.Field == field && w.Message == requiredFieldMissingMessage {
			continue
		}
		out = append(out, w)
	}
	return out
}
