package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// Exclusive findings are a rule segment: first match wins and emits its outputs.
func TestRule_WinnerCarriesOutputs(t *testing.T) {
	seg := &model.Segment{
		ID:       "balance-availability",
		Strategy: model.StrategyRule,
		Outputs:  map[string]string{"severity": "Warning"},
		Rules: []model.Rule{
			{
				RuleName: "noDaysWorkedYet",
				Outputs:  map[string]string{"title": "No days worked in ${ cycleName }"},
				Condition: &model.Condition{
					Field: "calcMode", Operator: model.OpEq, Value: "SalaryNonClocking",
				},
			},
			{
				RuleName: "noPayDataAvailable",
				Outputs:  map[string]string{"title": "No pay data"},
				Condition: &model.Condition{
					Field: "calcMode", Operator: model.OpEq, Value: "NetPayCalculator",
				},
			},
		},
	}
	ctx := &EvalContext{
		Context: map[string]interface{}{
			"calcMode":  "SalaryNonClocking",
			"cycleName": "March",
		},
		OutputSchema: model.OutputSchema{
			"severity": {Type: model.FieldTypeString},
			"title":    {Type: model.FieldTypeString},
		},
	}

	var s RuleStrategy
	res, ok := s.Evaluate(seg, ctx)
	if !ok {
		t.Fatal("expected the segment to resolve")
	}
	if res.Outputs["severity"] != "Warning" {
		t.Errorf("severity = %v", res.Outputs["severity"])
	}
	if res.Outputs["title"] != "No days worked in March" {
		t.Errorf("title = %v", res.Outputs["title"])
	}
}

// Falling through to the default emits the segment-level values only.
func TestRule_DefaultCarriesSegmentOutputs(t *testing.T) {
	seg := &model.Segment{
		ID:       "balance-availability",
		Strategy: model.StrategyRule,
		Outputs:  map[string]string{"severity": "Info"},
		Default:  "none",
		Rules: []model.Rule{{
			RuleName:  "never",
			Condition: &model.Condition{Field: "calcMode", Operator: model.OpEq, Value: "nope"},
		}},
	}
	ctx := &EvalContext{
		Context:      map[string]interface{}{"calcMode": "other"},
		OutputSchema: model.OutputSchema{"severity": {Type: model.FieldTypeString}},
	}

	var s RuleStrategy
	res, ok := s.Evaluate(seg, ctx)
	if !ok || res.Segment != "none" {
		t.Fatalf("expected the default, got ok=%v segment=%q", ok, res.Segment)
	}
	if res.Outputs["severity"] != "Info" {
		t.Errorf("severity = %v", res.Outputs["severity"])
	}
}
