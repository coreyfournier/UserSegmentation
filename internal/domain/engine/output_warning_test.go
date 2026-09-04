package engine

import (
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
				Segments: []model.Segment{
					{
						ID:       "attendance",
						Strategy: model.StrategyChecklist,
						OutputSchema: model.OutputSchema{
							"category": model.OutputField{Type: model.FieldTypeString, Required: true},
						},
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
				Segments: []model.Segment{
					{
						ID:       "attendance",
						Strategy: model.StrategyChecklist,
						OutputSchema: model.OutputSchema{
							"category": model.OutputField{Type: model.FieldTypeString, Required: true},
						},
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

// An override winning produces a warning: EvalOverrides resolves no segment
// and computes no outputs at all, so a required output field is absent from
// the assignment regardless of anything the primary strategy would have set.
func TestRequiredOutput_OverrideWinsWarns(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "test",
				Segments: []model.Segment{
					{
						ID:       "seg",
						Strategy: "static",
						Static:   &model.StaticConfig{Default: "normal"},
						OutputSchema: model.OutputSchema{
							"category": model.OutputField{Type: model.FieldTypeString, Required: true},
						},
						Overrides: []model.Rule{
							{
								RuleName:     "vip-override",
								SuccessEvent: "override-val",
								Condition:    &model.Condition{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
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

	w, found := findWarning(result.Warnings, "category")
	if !found {
		t.Fatalf("expected a warning naming the missing required field %q, got %v", "category", result.Warnings)
	}
	if w.Segment != "seg" {
		t.Errorf("expected warning segment %q, got %q", "seg", w.Segment)
	}
}
