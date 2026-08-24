package engine

import (
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
)

// countingStrategy records how many times it was asked to evaluate, so a test
// can tell "not in the output" apart from "never executed".
type countingStrategy struct {
	calls int
	value string
}

func (c *countingStrategy) Evaluate(_ *model.Segment, _ *strategy.EvalContext) (strategy.Result, bool) {
	c.calls++
	return strategy.Result{Segment: c.value, Reason: "counted"}, true
}

func graphEvaluator(extra map[string]strategy.Strategy) *Evaluator {
	strategies := map[string]strategy.Strategy{
		"static": &strategy.StaticStrategy{},
		"rule":   &strategy.RuleStrategy{},

		"checklist": &strategy.ChecklistStrategy{},
	}
	for k, v := range extra {
		strategies[k] = v
	}
	return NewEvaluator(strategies)
}

func staticLayer(name, value string, dependsOn ...string) model.Layer {
	return model.Layer{
		Name:      name,
		DependsOn: dependsOn,
		Segments: []model.Segment{{
			ID:       name + "-seg",
			Strategy: "static",
			Static:   &model.StaticConfig{Default: value},
		}},
	}
}

// checklistLayer builds a gate whose single check reports a problem when the
// field is not the required value.
func checklistLayer(name, rule, field, want string, dependsOn ...string) model.Layer {
	return model.Layer{
		Name:      name,
		DependsOn: dependsOn,
		Segments: []model.Segment{{
			ID:       name + "-seg",
			Strategy: model.StrategyChecklist,
			Rules: []model.Rule{{
				RuleName:     rule,
				ErrorMessage: rule + " failed",
				Condition:    &model.Condition{Field: field, Operator: model.OpNeq, Value: want},
			}},
		}},
	}
}

func evaluate(t *testing.T, e *Evaluator, snap *model.Snapshot, ctx map[string]interface{}, filter []string) *EvalResult {
	t.Helper()
	return e.Evaluate(snap, "subject", ctx, filter, nil, false, time.Now())
}

// Declaration order on disk must not matter — dependsOn decides execution.
func TestGraph_DependencyOrderIgnoresDeclarationOrder(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{
		{
			Name:      "downstream",
			DependsOn: []string{"upstream"},
			Segments: []model.Segment{{
				ID:       "s",
				Strategy: "rule",
				Rules: []model.Rule{{
					RuleName:     "sawUpstream",
					SuccessEvent: "saw-pro",
					Condition:    &model.Condition{Field: "layer:upstream", Operator: model.OpEq, Value: "pro"},
				}},
				Default: "missed",
			}},
		},
		staticLayer("upstream", "pro"),
	}}

	res := evaluate(t, graphEvaluator(nil), snap, nil, nil)
	if got := res.Layers["downstream"].Assignment.Segment; got != "saw-pro" {
		t.Errorf("downstream ran before its dependency: got %q", got)
	}
}

func TestGraph_DiamondDependency(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{
		staticLayer("root", "r"),
		staticLayer("left", "l", "root"),
		staticLayer("right", "x", "root"),
		{
			Name:      "join",
			DependsOn: []string{"left", "right"},
			Segments: []model.Segment{{
				ID:       "s",
				Strategy: "rule",
				Rules: []model.Rule{{
					RuleName:     "bothArrived",
					Operator:     model.CompositeAnd,
					SuccessEvent: "joined",
					Rules: []model.Rule{
						{RuleName: "l", Condition: &model.Condition{Field: "layer:left", Operator: model.OpEq, Value: "l"}},
						{RuleName: "r", Condition: &model.Condition{Field: "layer:right", Operator: model.OpEq, Value: "x"}},
					},
				}},
				Default: "incomplete",
			}},
		},
	}}

	res := evaluate(t, graphEvaluator(nil), snap, nil, nil)
	if got := res.Layers["join"].Assignment.Segment; got != "joined" {
		t.Errorf("expected both branches before the join, got %q", got)
	}
}

func TestGraph_CycleIsReported(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{
		staticLayer("a", "1", "b"),
		staticLayer("b", "2", "a"),
	}}

	res := evaluate(t, graphEvaluator(nil), snap, nil, nil)
	if len(res.Layers) != 0 {
		t.Errorf("a cyclic snapshot must not partially evaluate, got %v", res.Layers)
	}
	if len(res.Warnings) == 0 {
		t.Fatal("expected a warning describing the cycle")
	}
}

// A violated gate blocks everything downstream of it, transitively.
func TestGraph_SkipPropagatesFromViolatedGate(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{
		checklistLayer("identity", "hasEIN", "ein", "12-3456789"),
		checklistLayer("payroll", "hasFrequency", "payFrequency", "biweekly", "identity"),
		checklistLayer("tax", "hasTaxId", "taxId", "T-1", "payroll"),
	}}

	res := evaluate(t, graphEvaluator(nil), snap, map[string]interface{}{
		// EIN is wrong, but everything downstream is present and valid.
		"ein": "wrong", "payFrequency": "biweekly", "taxId": "T-1",
	}, nil)

	if got := res.Layers["identity"].Status; got != model.StatusViolated {
		t.Errorf("identity: expected violated, got %q", got)
	}
	for _, name := range []string{"payroll", "tax"} {
		lr := res.Layers[name]
		if lr.Status != model.StatusUnevaluable {
			t.Errorf("%s: expected unevaluable, got %q", name, lr.Status)
		}
		if len(lr.Failures) != 0 {
			t.Errorf("%s: a skipped gate must report no failures, got %v", name, lr.Failures)
		}
	}
}

// A satisfied gate lets the next one run.
func TestGraph_SatisfiedGateReleasesDependents(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{
		checklistLayer("identity", "hasEIN", "ein", "12-3456789"),
		checklistLayer("payroll", "hasFrequency", "payFrequency", "biweekly", "identity"),
	}}

	res := evaluate(t, graphEvaluator(nil), snap, map[string]interface{}{
		"ein": "12-3456789", "payFrequency": "monthly",
	}, nil)

	if got := res.Layers["identity"].Status; got != model.StatusSatisfied {
		t.Fatalf("identity: expected satisfied, got %q", got)
	}
	lr := res.Layers["payroll"]
	if lr.Status != model.StatusViolated {
		t.Errorf("payroll: expected violated, got %q", lr.Status)
	}
	if len(lr.Failures) != 1 || lr.Failures[0].Rule != "hasFrequency" {
		t.Errorf("payroll: expected the itemised failure, got %v", lr.Failures)
	}
}

// An unresolved non-assert layer skips its dependents, using the neutral
// vocabulary rather than the assertion one.
func TestGraph_UnresolvedLayerSkipsDependents(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{
		{
			Name: "upstream",
			Segments: []model.Segment{{
				ID:       "s",
				Strategy: "rule",
				Rules: []model.Rule{{
					RuleName:  "never",
					Condition: &model.Condition{Field: "nope", Operator: model.OpEq, Value: "x"},
				}},
				// No default, so nothing resolves.
			}},
		},
		staticLayer("downstream", "d", "upstream"),
	}}

	res := evaluate(t, graphEvaluator(nil), snap, nil, nil)
	if got := res.Layers["upstream"].Status; got != model.StatusUnresolved {
		t.Errorf("upstream: expected unresolved, got %q", got)
	}
	if got := res.Layers["downstream"].Status; got != model.StatusSkipped {
		t.Errorf("downstream: expected skipped, got %q", got)
	}
	if res.Layers["downstream"].Assignment != nil {
		t.Error("a skipped layer must not produce an assignment")
	}
}

// An assert layer resolves no segment value, so nothing is injected under
// "layer:<name>" — dependents gate on status instead.
func TestGraph_ChecklistInjectsNoContextValue(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{
		checklistLayer("gate", "hasEIN", "ein", "12-3456789"),
		{
			Name:      "downstream",
			DependsOn: []string{"gate"},
			Segments: []model.Segment{{
				ID:       "s",
				Strategy: "rule",
				Rules: []model.Rule{{
					RuleName:     "readsGate",
					SuccessEvent: "read-something",
					Condition:    &model.Condition{Field: "layer:gate", Operator: model.OpEq, Value: "satisfied"},
				}},
				Default: "nothing-injected",
			}},
		},
	}}

	res := evaluate(t, graphEvaluator(nil), snap, map[string]interface{}{"ein": "12-3456789"}, nil)
	if got := res.Layers["downstream"].Assignment.Segment; got != "nothing-injected" {
		t.Errorf("assert should inject no value, got %q", got)
	}
}

// Filtering evaluates the requested layers plus their transitive dependencies,
// and nothing else.
func TestGraph_FilterEvaluatesDependencyClosureOnly(t *testing.T) {
	unrelated := &countingStrategy{value: "u"}
	snap := &model.Snapshot{Layers: []model.Layer{
		staticLayer("upstream", "pro"),
		{
			Name:      "wanted",
			DependsOn: []string{"upstream"},
			Segments: []model.Segment{{
				ID:       "s",
				Strategy: "rule",
				Rules: []model.Rule{{
					RuleName:     "sawUpstream",
					SuccessEvent: "saw-pro",
					Condition:    &model.Condition{Field: "layer:upstream", Operator: model.OpEq, Value: "pro"},
				}},
				Default: "missed",
			}},
		},
		{Name: "unrelated", Segments: []model.Segment{{ID: "s", Strategy: "counting"}}},
	}}

	e := graphEvaluator(map[string]strategy.Strategy{"counting": unrelated})
	res := evaluate(t, e, snap, nil, []string{"wanted"})

	if len(res.Layers) != 1 {
		t.Errorf("only the requested layer should be output, got %v", res.Layers)
	}
	if got := res.Layers["wanted"].Assignment.Segment; got != "saw-pro" {
		t.Errorf("transitive dependency did not run: got %q", got)
	}
	if unrelated.calls != 0 {
		t.Errorf("a layer outside the closure must not execute, ran %d times", unrelated.calls)
	}

	// Unfiltered, it does run.
	evaluate(t, e, snap, nil, nil)
	if unrelated.calls != 1 {
		t.Errorf("expected the unrelated layer to run when unfiltered, ran %d times", unrelated.calls)
	}
}

// A segment whose When predicate is false is passed over entirely: it produces
// no output and is not a reported state.
func TestGraph_WhenDispatchSelectsSegment(t *testing.T) {
	typeSegment := func(id, productType, rule, field, want string) model.Segment {
		return model.Segment{
			ID:       id,
			When:     &model.Rule{RuleName: "is" + id, Condition: &model.Condition{Field: "productType", Operator: model.OpEq, Value: productType}},
			Strategy: model.StrategyChecklist,
			Rules: []model.Rule{{
				RuleName:     rule,
				ErrorMessage: rule + " failed",
				Condition:    &model.Condition{Field: field, Operator: model.OpNeq, Value: want},
			}},
		}
	}

	snap := &model.Snapshot{Layers: []model.Layer{{
		Name: "payroll",
		Segments: []model.Segment{
			typeSegment("precision", "Precision", "precisionAnchorDateWrong", "anchorDate", "2026-01-01"),
			typeSegment("express", "Express", "expressEinWrong", "ein", "12-3456789"),
		},
	}}}

	e := graphEvaluator(nil)

	// An Express company must not be judged by Precision's rules.
	res := evaluate(t, e, snap, map[string]interface{}{
		"productType": "Express", "ein": "12-3456789",
	}, nil)
	lr := res.Layers["payroll"]
	if lr.Status != model.StatusSatisfied {
		t.Errorf("express: expected satisfied, got %q %v", lr.Status, lr.Failures)
	}

	// The Precision segment applies to a Precision company, and fails.
	// A comparison fires on a wrong value; absence needs a presence operator,
	// which TestChecklist_ComparisonsDoNotFireOnAbsentFields covers.
	res = evaluate(t, e, snap, map[string]interface{}{
		"productType": "Precision", "ein": "12-3456789", "anchorDate": "1999-01-01",
	}, nil)
	lr = res.Layers["payroll"]
	if lr.Status != model.StatusViolated {
		t.Fatalf("precision: expected violated, got %q", lr.Status)
	}
	if len(lr.Failures) != 1 || lr.Failures[0].Rule != "precisionAnchorDateWrong" {
		t.Errorf("precision: unexpected failures %v", lr.Failures)
	}

	// A type no segment claims runs no checks, so nothing was found wrong. It
	// must not report unevaluable — that would block readiness for every
	// subject a conditional layer simply does not cover.
	res = evaluate(t, e, snap, map[string]interface{}{"productType": "TimeAndAttendance"}, nil)
	lr = res.Layers["payroll"]
	if lr.Status != model.StatusSatisfied {
		t.Errorf("unmatched type: expected satisfied, got %q", lr.Status)
	}
	if len(lr.Failures) != 0 {
		t.Errorf("unmatched type: expected no failures, got %v", lr.Failures)
	}
}

// Nested context: an employee sent alongside its parent company is one
// document, and rules address it by path.
func TestGraph_NestedEntityContext(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{{
		Name: "employee-readiness",
		Segments: []model.Segment{{
			ID:       "all",
			Strategy: model.StrategyChecklist,
			Rules: []model.Rule{
				{
					RuleName:     "employeeMissingHireDate",
					ErrorMessage: "Hire date is required.",
					Condition:    &model.Condition{Field: "employee.hireDate", Operator: model.OpIsNullOrEmpty},
				},
				{
					RuleName:     "parentCompanyNotPrecision",
					ErrorMessage: "Parent company must be Precision.",
					Condition:    &model.Condition{Field: "company.productType", Operator: model.OpNeq, Value: "Precision"},
				},
			},
		}},
	}}}

	res := evaluate(t, graphEvaluator(nil), snap, map[string]interface{}{
		"employee": map[string]interface{}{"hireDate": ""},
		"company":  map[string]interface{}{"productType": "Precision"},
	}, nil)

	lr := res.Layers["employee-readiness"]
	if lr.Status != model.StatusViolated {
		t.Fatalf("expected violated, got %q", lr.Status)
	}
	if len(lr.Failures) != 1 || lr.Failures[0].Rule != "employeeMissingHireDate" {
		t.Errorf("expected only the hire-date failure, got %v", lr.Failures)
	}
}
