package strategy

import "github.com/segmentation-service/segmentation/internal/domain/model"

// ChecklistStrategy runs a list of checks and reports every one that fires.
//
// Each rule states a condition that describes a problem: when the condition
// holds, the rule's message is reported. A rule therefore means the same thing
// here as under first-match evaluation — it fires on a match — which is what
// keeps identical config from meaning opposite things across strategies.
//
// It is the third link in an existing delegation chain:
//
//	ChecklistStrategy  ->  RuleStrategy
//	   sets                  derives computed fields, then reports every rule
//	   CollectFailures        that matches instead of stopping at the first
//	   computes Status
//
// so it inherits computed fields for free, and rule evaluation itself stays in
// one place.
type ChecklistStrategy struct {
	rules RuleStrategy
}

func (s *ChecklistStrategy) Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	collecting := *ctx
	collecting.CollectFailures = true

	res, ok := s.rules.Evaluate(seg, &collecting)
	if !ok {
		return res, false
	}

	// RuleStrategy already reports unevaluable when a formula failed at
	// runtime; only the satisfied/violated distinction is left.
	if res.Status == "" {
		if len(res.Failures) > 0 {
			res.Status = model.StatusViolated
		} else {
			res.Status = model.StatusSatisfied
		}
	}

	// A checklist resolves no segment value, so nothing is injected into the
	// context as "layer:<name>". Dependent layers gate on status instead.
	res.Segment = ""
	return res, true
}
