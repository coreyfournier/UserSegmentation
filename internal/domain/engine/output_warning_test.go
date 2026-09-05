package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
)

// findWarning returns the first warning whose Field matches name, and whether
// one was found.
func findWarning(warnings []model.Warning, field string) (model.Warning, bool) {
	for _, w := range warnings {
		if w.Field == field {
			return w, true
		}
	}
	return model.Warning{}, false
}

func requiredOutputEvaluator() *Evaluator {
	return NewEvaluator(map[string]strategy.Strategy{
		"checklist": &strategy.ChecklistStrategy{},
		"rule":      &strategy.RuleStrategy{},
		"static":    &strategy.StaticStrategy{},
	})
}

// A required output field the reported finding never set produces a warning
// naming that field. The rule's Condition always matches and neither the rule
// nor the segment authors the "category" output, standing in for an
// expression that fails at runtime and drops the field: config validation
// (Task 5) cannot see this — it only proves an authoring path exists.
func TestRequiredOutput_MissingFromFindingWarns(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "checks",
				OutputSchema: model.OutputSchema{
					"category": model.OutputField{Type: model.FieldTypeString, Required: true},
				},
				Segments: []model.Segment{
					{
						ID:       "attendance",
						Strategy: model.StrategyChecklist,
						Rules: []model.Rule{
							{
								RuleName:     "no-days-worked",
								ErrorMessage: "no days worked",
								Condition:    &model.Condition{Field: "totalHours", Operator: model.OpEq, Value: 0},
							},
						},
					},
				},
			},
		},
	}

	e := requiredOutputEvaluator()
	result := e.Evaluate(snap, "user", map[string]interface{}{"totalHours": 0}, nil, nil, false, time.Now())

	lr, ok := result.Layers["checks"]
	if !ok {
		t.Fatal("expected the layer to report a result")
	}
	if len(lr.Failures) != 1 {
		t.Fatalf("expected one failure, got %d", len(lr.Failures))
	}
	if _, present := lr.Failures[0].Outputs["category"]; present {
		t.Fatalf("expected the finding to have no \"category\" output, got %v", lr.Failures[0].Outputs)
	}

	w, found := findWarning(result.Warnings, "category")
	if !found {
		t.Fatalf("expected a warning naming the missing required field %q, got %v", "category", result.Warnings)
	}
	if w.Segment != "attendance" {
		t.Errorf("expected warning segment %q, got %q", "attendance", w.Segment)
	}
}

// A required output field that is present produces no warning.
func TestRequiredOutput_PresentProducesNoWarning(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "checks",
				OutputSchema: model.OutputSchema{
					"category": model.OutputField{Type: model.FieldTypeString, Required: true},
				},
				Segments: []model.Segment{
					{
						ID:       "attendance",
						Strategy: model.StrategyChecklist,
						Rules: []model.Rule{
							{
								RuleName:     "no-days-worked",
								ErrorMessage: "no days worked",
								Condition:    &model.Condition{Field: "totalHours", Operator: model.OpEq, Value: 0},
								Outputs:      map[string]string{"category": "EmployeeAccountStatus"},
							},
						},
					},
				},
			},
		},
	}

	e := requiredOutputEvaluator()
	result := e.Evaluate(snap, "user", map[string]interface{}{"totalHours": 0}, nil, nil, false, time.Now())

	lr, ok := result.Layers["checks"]
	if !ok {
		t.Fatal("expected the layer to report a result")
	}
	if len(lr.Failures) != 1 {
		t.Fatalf("expected one failure, got %d", len(lr.Failures))
	}
	if got := lr.Failures[0].Outputs["category"]; got != "EmployeeAccountStatus" {
		t.Fatalf("expected category = EmployeeAccountStatus, got %v", got)
	}

	if _, found := findWarning(result.Warnings, "category"); found {
		t.Errorf("expected no warning for a present required field, got %v", result.Warnings)
	}
}

// An override winning no longer means a bare required-field warning by
// itself: overrides now resolve declared outputs too (Task 7), against the
// computed-enriched context. The warning instead fires when the override's
// own output *expression* fails at runtime and the field is dropped — here,
// a deliberately unparseable expression authored on the override.
//
// The underlying segment is a rule segment, not static: Finding 3 exempts
// static/percentage segments from required-output enforcement entirely
// (neither ever populates Result.Outputs), so this must be a strategy the
// exemption does not reach — otherwise the test would pass by accident,
// exercising the exemption instead of the override path it names.
func TestRequiredOutput_OverrideOutputExpressionFailsWarns(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "test",
				OutputSchema: model.OutputSchema{
					"category": model.OutputField{Type: model.FieldTypeString, Required: true},
				},
				Segments: []model.Segment{
					{
						ID:       "seg",
						Strategy: model.StrategyRule,
						Default:  "normal",
						Overrides: []model.Rule{
							{
								RuleName:     "vip-override",
								SuccessEvent: "override-val",
								Condition:    &model.Condition{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
								Outputs:      map[string]string{"category": "${amount *}"}, // unparseable template token
							},
						},
					},
				},
			},
		},
	}

	e := requiredOutputEvaluator()
	result := e.Evaluate(snap, "user", map[string]interface{}{"plan": "enterprise"}, nil, nil, false, time.Now())

	lr, ok := result.Layers["test"]
	if !ok {
		t.Fatal("expected the layer to report a result")
	}
	if lr.Assignment == nil || lr.Assignment.Strategy != "override" {
		t.Fatalf("expected the override to win, got %v", lr.Assignment)
	}
	if _, present := lr.Assignment.Outputs["category"]; present {
		t.Fatalf("expected the failed expression to drop the field, got %v", lr.Assignment.Outputs)
	}

	w, found := findWarning(result.Warnings, "category")
	if !found {
		t.Fatalf("expected a warning naming the missing required field %q, got %v", "category", result.Warnings)
	}
	if w.Segment != "seg" {
		t.Errorf("expected warning segment %q, got %q", "seg", w.Segment)
	}
}

// An override that *does* successfully author its required output produces
// no warning — pinning the other side of the Task 7 change: overrides are no
// longer a blanket source of missing-output warnings just for winning.
//
// The underlying segment is a rule segment, not static, for the same reason
// as TestRequiredOutput_OverrideOutputExpressionFailsWarns above: a static
// segment is now exempt from required-output enforcement (Finding 3), so
// this must exercise a strategy the exemption does not reach.
func TestRequiredOutput_OverrideAuthoredOutputNoWarning(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "test",
				OutputSchema: model.OutputSchema{
					"category": model.OutputField{Type: model.FieldTypeString, Required: true},
				},
				Segments: []model.Segment{
					{
						ID:       "seg",
						Strategy: model.StrategyRule,
						Default:  "normal",
						Overrides: []model.Rule{
							{
								RuleName:     "vip-override",
								SuccessEvent: "override-val",
								Condition:    &model.Condition{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
								Outputs:      map[string]string{"category": "vip"},
							},
						},
					},
				},
			},
		},
	}

	e := requiredOutputEvaluator()
	result := e.Evaluate(snap, "user", map[string]interface{}{"plan": "enterprise"}, nil, nil, false, time.Now())

	lr, ok := result.Layers["test"]
	if !ok {
		t.Fatal("expected the layer to report a result")
	}
	if lr.Assignment == nil || lr.Assignment.Strategy != "override" {
		t.Fatalf("expected the override to win, got %v", lr.Assignment)
	}
	if got := lr.Assignment.Outputs["category"]; got != "vip" {
		t.Fatalf("expected category = %q, got %v", "vip", got)
	}

	if _, found := findWarning(result.Warnings, "category"); found {
		t.Errorf("expected no warning when the override authored the required field, got %v", result.Warnings)
	}
}

// A satisfied checklist — the rule's condition never holds, so nothing fires —
// must not warn. Outputs for a checklist live per-Failure; with zero findings
// the per-finding loop simply never runs. Before the fix, CheckRequiredOutputs
// branched on len(failures) > 0 and fell through to checking a.Outputs[name]
// on the Assignment, which is structurally always empty for a checklist —
// producing a spurious warning on every healthy, satisfied evaluation.
func TestRequiredOutput_ChecklistSatisfiedNoWarning(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "checks",
				OutputSchema: model.OutputSchema{
					"category": model.OutputField{Type: model.FieldTypeString, Required: true},
				},
				Segments: []model.Segment{
					{
						ID:       "gates",
						Strategy: model.StrategyChecklist,
						Rules: []model.Rule{
							{
								RuleName:     "no-days-worked",
								ErrorMessage: "no days worked",
								Condition:    &model.Condition{Field: "totalHours", Operator: model.OpEq, Value: 0},
							},
						},
					},
				},
			},
		},
	}

	e := requiredOutputEvaluator()
	result := e.Evaluate(snap, "user", map[string]interface{}{"totalHours": 40}, nil, nil, false, time.Now())

	lr, ok := result.Layers["checks"]
	if !ok {
		t.Fatal("expected the layer to report a result")
	}
	if lr.Status != model.StatusSatisfied {
		t.Fatalf("expected status %q, got %q", model.StatusSatisfied, lr.Status)
	}
	if len(lr.Failures) != 0 {
		t.Fatalf("expected zero failures, got %d", len(lr.Failures))
	}

	if _, found := findWarning(result.Warnings, "category"); found {
		t.Errorf("expected no warning for a satisfied checklist, got %v", result.Warnings)
	}
}

// An unevaluable checklist — a computed field's formula fails at runtime, so
// collectViolations never runs and reports zero findings — must not warn
// either, for the same reason: no record came back short because nothing was
// reported at all.
func TestRequiredOutput_ChecklistUnevaluableNoWarning(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "checks",
				OutputSchema: model.OutputSchema{
					"category": model.OutputField{Type: model.FieldTypeString, Required: true},
				},
				Segments: []model.Segment{
					{
						ID:       "gates",
						Strategy: model.StrategyChecklist,
						Computed: []model.ComputedField{
							{Name: "utilization", Type: model.FieldTypeNumber, Formula: "advanceTaken / advanceLimit"},
						},
						Rules: []model.Rule{
							{
								RuleName:     "over-limit",
								ErrorMessage: "over limit",
								Condition:    &model.Condition{Field: "utilization", Operator: model.OpGt, Value: 1},
							},
						},
					},
				},
			},
		},
	}

	e := requiredOutputEvaluator()
	result := e.Evaluate(snap, "user", map[string]interface{}{}, nil, nil, false, time.Now())

	lr, ok := result.Layers["checks"]
	if !ok {
		t.Fatal("expected the layer to report a result")
	}
	if lr.Status != model.StatusUnevaluable {
		t.Fatalf("expected status %q, got %q", model.StatusUnevaluable, lr.Status)
	}
	if len(lr.Failures) != 0 {
		t.Fatalf("expected zero failures, got %d", len(lr.Failures))
	}

	if _, found := findWarning(result.Warnings, "category"); found {
		t.Errorf("expected no warning for an unevaluable checklist, got %v", result.Warnings)
	}
}

// Finding 4: an output field that fails to resolve must name itself in the
// warning, distinct from a message-template render error. Before the fix,
// every RenderError — output or message alike — was formatted as "message
// render error in %q", pointing a caller at message templates that were
// never involved when it was actually an output field that vanished.
func TestRequiredOutput_FailedExpressionNamesTheField(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "test",
				OutputSchema: model.OutputSchema{
					"diagnosis": model.OutputField{Type: model.FieldTypeString},
				},
				Segments: []model.Segment{
					{
						ID:       "seg",
						Strategy: model.StrategyRule,
						Rules: []model.Rule{
							{
								RuleName:     "matches",
								SuccessEvent: "matched",
								Condition:    &model.Condition{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
								Outputs:      map[string]string{"diagnosis": "${amount *}"}, // unparseable template token
							},
						},
					},
				},
			},
		},
	}

	e := requiredOutputEvaluator()
	result := e.Evaluate(snap, "user", map[string]interface{}{"plan": "enterprise"}, nil, nil, false, time.Now())

	lr, ok := result.Layers["test"]
	if !ok {
		t.Fatal("expected the layer to report a result")
	}
	if _, present := lr.Assignment.Outputs["diagnosis"]; present {
		t.Fatalf("expected the failed expression to drop the field, got %v", lr.Assignment.Outputs)
	}

	w, found := findWarning(result.Warnings, "diagnosis")
	if !found {
		t.Fatalf("expected a warning naming the failed output field %q, got %v", "diagnosis", result.Warnings)
	}
	if !strings.Contains(w.Message, `output "diagnosis" failed to resolve`) {
		t.Fatalf("expected the warning to name the output field distinctly from a message render error, got %q", w.Message)
	}
	if strings.Contains(w.Message, "message render error") {
		t.Fatalf("an output failure must not be reported as a message render error, got %q", w.Message)
	}
}

// Three or more required output fields absent from a reported finding must
// produce warnings in deterministic, alphabetical order — not whatever order
// Go's randomized map iteration happens to yield. The field names are chosen
// so their alphabetical order ("alpha", "mike", "zulu") differs from their
// declaration order below; without sort.Strings in CheckRequiredOutputs this
// test would be flaky, failing whenever map iteration did not land on
// alphabetical order by chance.
func TestRequiredOutput_MultipleMissingFieldsAreSortedDeterministically(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "checks",
				OutputSchema: model.OutputSchema{
					"zulu":  model.OutputField{Type: model.FieldTypeString, Required: true},
					"alpha": model.OutputField{Type: model.FieldTypeString, Required: true},
					"mike":  model.OutputField{Type: model.FieldTypeString, Required: true},
				},
				Segments: []model.Segment{
					{
						ID:       "gates",
						Strategy: model.StrategyChecklist,
						Rules: []model.Rule{
							{
								RuleName:     "no-days-worked",
								ErrorMessage: "no days worked",
								Condition:    &model.Condition{Field: "totalHours", Operator: model.OpEq, Value: 0},
							},
						},
					},
				},
			},
		},
	}

	e := requiredOutputEvaluator()
	result := e.Evaluate(snap, "user", map[string]interface{}{"totalHours": 0}, nil, nil, false, time.Now())

	lr, ok := result.Layers["checks"]
	if !ok {
		t.Fatal("expected the layer to report a result")
	}
	if len(lr.Failures) != 1 {
		t.Fatalf("expected one failure, got %d", len(lr.Failures))
	}

	var gotFields []string
	for _, w := range result.Warnings {
		if w.Segment == "gates" {
			gotFields = append(gotFields, w.Field)
		}
	}
	want := []string{"alpha", "mike", "zulu"}
	if len(gotFields) != len(want) {
		t.Fatalf("expected %d warnings, got %d: %v", len(want), len(gotFields), gotFields)
	}
	for i := range want {
		if gotFields[i] != want[i] {
			t.Fatalf("expected warnings in alphabetical order %v, got %v", want, gotFields)
		}
	}
}
