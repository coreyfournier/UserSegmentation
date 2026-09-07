package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// A segment whose rule and default both emit a fee, and whose fee is a
// computed field — the shape that motivated DefaultOutputs. Before it, the
// default could only take the segment-wide fallback, so a fee that differed
// when nothing matched had nowhere to live.
func feeSegment() *model.Segment {
	return &model.Segment{
		ID:       "fees",
		Strategy: model.StrategyRule,
		Computed: []model.ComputedField{
			{Name: "TransferFee", Type: model.FieldTypeNumber, Formula: "amount * 0.02"},
		},
		Rules: []model.Rule{{
			RuleName:     "waived",
			SuccessEvent: "no-fee",
			Condition:    &model.Condition{Field: "plan", Operator: model.OpEq, Value: "pro"},
			Outputs:      map[string]string{"TransferFee": "0"},
		}},
		Default:        "standard-fee",
		DefaultOutputs: map[string]string{"TransferFee": "TransferFee"},
	}
}

func feeCtx(plan string, amount float64) *EvalContext {
	return &EvalContext{
		Context: map[string]interface{}{"plan": plan, "amount": amount},
		OutputSchema: model.OutputSchema{
			"TransferFee": {Type: model.FieldTypeNumber},
		},
	}
}

func TestDefaultOutputs_AuthoredOnTheDefaultPath(t *testing.T) {
	s := &RuleStrategy{}

	res, ok := s.Evaluate(feeSegment(), feeCtx("basic", 100))
	if !ok {
		t.Fatal("expected the default to resolve")
	}
	if res.Segment != "standard-fee" {
		t.Fatalf("expected the default segment, got %q", res.Segment)
	}
	// The default's own expression, reading the computed field.
	if got := numeric(t, res.Outputs["TransferFee"]); got != 2.0 {
		t.Errorf("expected the computed fee of 2, got %v", got)
	}
}

// The rule path is unaffected: its own value still wins.
func TestDefaultOutputs_RuleValueStillWins(t *testing.T) {
	s := &RuleStrategy{}

	res, ok := s.Evaluate(feeSegment(), feeCtx("pro", 100))
	if !ok {
		t.Fatal("expected the rule to match")
	}
	if res.Segment != "no-fee" {
		t.Fatalf("expected the rule's event, got %q", res.Segment)
	}
	if got := numeric(t, res.Outputs["TransferFee"]); got != 0.0 {
		t.Errorf("expected the rule's waived fee of 0, got %v", got)
	}
}

// A default value overrides the segment-wide fallback for the default path
// only, exactly as a rule's own value does for that rule.
func TestDefaultOutputs_BeatTheSegmentFallback(t *testing.T) {
	seg := feeSegment()
	seg.Outputs = map[string]string{"TransferFee": "99"}

	res, _ := (&RuleStrategy{}).Evaluate(seg, feeCtx("basic", 100))
	if got := numeric(t, res.Outputs["TransferFee"]); got != 2.0 {
		t.Errorf("the default's own value must win over the fallback, got %v", got)
	}
}

// With no default values authored, the segment-wide fallback still applies —
// the behaviour that existed before this field.
func TestDefaultOutputs_FallbackStillAppliesWhenUnset(t *testing.T) {
	seg := feeSegment()
	seg.DefaultOutputs = nil
	seg.Outputs = map[string]string{"TransferFee": "7"}

	res, _ := (&RuleStrategy{}).Evaluate(seg, feeCtx("basic", 100))
	if got := numeric(t, res.Outputs["TransferFee"]); got != 7.0 {
		t.Errorf("expected the segment fallback of 7, got %v", got)
	}
}

// A checklist returns from collectViolations before the default branch, so
// values authored there are never read. Config may carry them; nothing emits.
func TestDefaultOutputs_NotReadByAChecklist(t *testing.T) {
	seg := feeSegment()
	seg.Strategy = model.StrategyChecklist

	ctx := feeCtx("basic", 100)
	ctx.CollectFailures = true

	res, ok := (&RuleStrategy{}).Evaluate(seg, ctx)
	if !ok {
		t.Fatal("expected the checklist to report")
	}
	if res.Segment != "" {
		t.Errorf("a checklist resolves no segment value, got %q", res.Segment)
	}
	if len(res.Outputs) != 0 {
		t.Errorf("a checklist emits outputs per failure, not on the result: %+v", res.Outputs)
	}
}

// numeric compares an emitted output to a number without caring which numeric
// type expr produced. A literal 0 in an expression comes back as an int and a
// computed value as a float64 — a real difference on the wire, but not what
// these tests are about.
func numeric(t *testing.T, v interface{}) float64 {
	t.Helper()
	switch n := v.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case float64:
		return n
	default:
		t.Fatalf("expected a number, got %v (%T)", v, v)
		return 0
	}
}
