package engine

import (
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// The layer is the only place input/output schemas are declared now — no
// segment carries either field. These tests pin evaluateLayer reading
// layer.InputSchema / layer.OutputSchema directly, with no per-segment
// resolution step.

// A checklist segment emits the fields declared by its layer's output schema
// on each Failure.Outputs.
func TestLayerSchema_ChecklistFailureCarriesLayerOutputs(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "checks",
				OutputSchema: model.OutputSchema{
					"category": model.OutputField{Type: model.FieldTypeString},
				},
				Segments: []model.Segment{
					{
						ID:       "attendance",
						Strategy: model.StrategyChecklist,
						Outputs:  map[string]string{"category": "EmployeeAccountStatus"},
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
	if got := lr.Failures[0].Outputs["category"]; got != "EmployeeAccountStatus" {
		t.Fatalf("expected category = EmployeeAccountStatus from the layer's output schema, got %v", got)
	}
}

// A required layer output field, absent because its expression failed at
// runtime, produces the evaluation warning naming it.
func TestLayerSchema_RequiredOutputFieldMissingWarns(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "test",
				OutputSchema: model.OutputSchema{
					"diagnosis": model.OutputField{Type: model.FieldTypeString, Required: true},
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
		t.Fatalf("expected a warning naming the missing required output field %q, got %v", "diagnosis", result.Warnings)
	}
	if w.Segment != "seg" {
		t.Errorf("expected warning segment %q, got %q", "seg", w.Segment)
	}
}

// Required input fields on the layer produce the missing-field warning.
func TestLayerSchema_RequiredInputFieldMissingWarns(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "checks",
				InputSchema: model.InputSchema{
					"accountId": model.SchemaField{Type: model.FieldTypeString, Required: true},
				},
				Segments: []model.Segment{
					{
						ID:       "seg",
						Strategy: model.StrategyStatic,
						Static:   &model.StaticConfig{Default: "resolved"},
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
	if lr.Assignment == nil || lr.Assignment.Segment != "resolved" {
		t.Fatalf("expected the segment to still resolve, got %v", lr.Assignment)
	}

	w, found := findWarning(lr.Warnings, "accountId")
	if !found {
		t.Fatalf("expected a warning naming the missing required input field %q, got %v", "accountId", lr.Warnings)
	}
	if w.Message != "required field missing from context" {
		t.Errorf("unexpected warning message: %q", w.Message)
	}
}

// The duplicate case. CheckRequiredFields runs for every segment that passes
// its `when` dispatch, and the segment loop only exits once a strategy
// succeeds — so a rule segment that matches nothing falls through (no rule
// fires and there is no default) and the next segment is checked too,
// against the same layer schema. Both segments are missing the same required
// field, so without de-duplication the warning would be reported twice.
func TestLayerSchema_DuplicateRequiredInputWarningCollapsedOnce(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name: "checks",
				InputSchema: model.InputSchema{
					"accountId": model.SchemaField{Type: model.FieldTypeString, Required: true},
				},
				Segments: []model.Segment{
					{
						// First segment: a rule segment that matches nothing and has
						// no default, so it falls through without resolving.
						ID:       "first",
						Strategy: model.StrategyRule,
						Rules: []model.Rule{
							{
								RuleName:  "never-matches",
								Condition: &model.Condition{Field: "plan", Operator: model.OpEq, Value: "nope"},
							},
						},
					},
					{
						// Second segment: resolves, so the layer loop stops here.
						ID:       "second",
						Strategy: model.StrategyStatic,
						Static:   &model.StaticConfig{Default: "resolved"},
					},
				},
			},
		},
	}

	e := requiredOutputEvaluator()
	result := e.Evaluate(snap, "user", map[string]interface{}{"plan": "other"}, nil, nil, false, time.Now())

	lr, ok := result.Layers["checks"]
	if !ok {
		t.Fatal("expected the layer to report a result")
	}
	if lr.Assignment == nil || lr.Assignment.Segment != "resolved" {
		t.Fatalf("expected the second segment to resolve, got %v", lr.Assignment)
	}

	var matches []model.Warning
	for _, w := range lr.Warnings {
		if w.Field == "accountId" {
			matches = append(matches, w)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("expected the missing required field to be reported exactly once, got %d: %v", len(matches), matches)
	}
}
