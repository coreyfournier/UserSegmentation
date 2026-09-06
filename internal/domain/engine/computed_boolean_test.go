package engine

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/validation"
)

// A computed boolean tested for equality by a rule: a field whose formula is a
// comparison ("10 >= 1"), declared boolean, read by a condition using eq.
//
// The whole path is exercised — load-time validation and then evaluation —
// because the two check different things and a config can pass one and fail
// the other.
func computedBooleanSnapshot(computedType model.FieldType, value interface{}) *model.Snapshot {
	return &model.Snapshot{Layers: []model.Layer{{
		Key:         "flags",
		InputSchema: model.InputSchema{"amount": {Type: model.FieldTypeNumber}},
		Segments: []model.Segment{{
			ID:       "check",
			Strategy: model.StrategyRule,
			Computed: []model.ComputedField{
				{Name: "IsBig", Type: computedType, Formula: "10 >= 1"},
			},
			Rules: []model.Rule{{
				RuleName:     "big",
				SuccessEvent: "is-big",
				Condition:    &model.Condition{Field: "IsBig", Operator: model.OpEq, Value: value},
			}},
			Default: "not-big",
		}},
	}}}
}

func evaluateComputedBoolean(t *testing.T, snap *model.Snapshot) *LayerResult {
	t.Helper()
	res := subjectKeyEvaluator().Evaluate(
		snap,
		map[string]interface{}{"amount": 5.0},
		nil, nil, false, time.Now(),
	)
	return res.Layers["flags"]
}

func TestComputedBoolean_EqualityRule(t *testing.T) {
	snap := computedBooleanSnapshot(model.FieldTypeBoolean, true)

	if err := validation.ValidateSnapshot(snap); err != nil {
		t.Fatalf("expected the config to be valid, got: %v", err)
	}

	lr := evaluateComputedBoolean(t, snap)
	if lr.Status != model.StatusResolved {
		t.Fatalf("expected %q, got %q", model.StatusResolved, lr.Status)
	}
	if lr.Assignment == nil || lr.Assignment.Segment != "is-big" {
		t.Fatalf("expected the rule to match, got %+v", lr.Assignment)
	}
	// The formula's own value, reported alongside the assignment.
	if got := lr.Assignment.Computed["IsBig"]; got != true {
		t.Errorf("expected the computed field to be true, got %v (%T)", got, got)
	}
}

// The condition value as JSON decodes it — the shape the admin API and the
// config file actually deliver, rather than a Go literal a test wrote.
func TestComputedBoolean_FromJSON(t *testing.T) {
	var seg model.Segment
	raw := `{"id":"check","strategy":"rule",
      "computed":[{"name":"IsBig","type":"boolean","formula":"10 >= 1"}],
      "rules":[{"ruleName":"big","successEvent":"is-big",
        "condition":{"field":"IsBig","operator":"eq","value":true}}],
      "default":"not-big"}`
	if err := json.Unmarshal([]byte(raw), &seg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	snap := &model.Snapshot{Layers: []model.Layer{{
		Key:         "flags",
		InputSchema: model.InputSchema{"amount": {Type: model.FieldTypeNumber}},
		Segments:    []model.Segment{seg},
	}}}
	if err := validation.ValidateSnapshot(snap); err != nil {
		t.Fatalf("expected the decoded config to be valid, got: %v", err)
	}
	if lr := evaluateComputedBoolean(t, snap); lr.Assignment == nil || lr.Assignment.Segment != "is-big" {
		t.Fatalf("expected the rule to match, got %+v", lr.Assignment)
	}
}

// The type dropdown defaults to number, so a boolean formula declared number is
// an easy mistake. It is accepted: the declared type drives which operators the
// editor offers and what validation checks, not what expr returns, and nothing
// coerces the value at evaluation.
func TestComputedBoolean_DeclaredTypeIsNotEnforcedAtRuntime(t *testing.T) {
	snap := computedBooleanSnapshot(model.FieldTypeNumber, true)
	if err := validation.ValidateSnapshot(snap); err != nil {
		t.Fatalf("expected a mismatched declared type to still validate, got: %v", err)
	}
	lr := evaluateComputedBoolean(t, snap)
	if lr.Assignment == nil || lr.Assignment.Segment != "is-big" {
		t.Fatalf("expected the rule to match anyway, got %+v", lr.Assignment)
	}
	if got := lr.Assignment.Computed["IsBig"]; got != true {
		t.Errorf("expected the bool the formula produced, got %v (%T)", got, got)
	}
}

// Comparison against a value the caller supplied as a string still matches:
// EvalCondition compares by string form, so "true" and true agree. Recorded
// because it is leniency worth knowing about rather than an obvious property.
func TestComputedBoolean_StringValueStillMatches(t *testing.T) {
	snap := computedBooleanSnapshot(model.FieldTypeBoolean, "true")
	if err := validation.ValidateSnapshot(snap); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if lr := evaluateComputedBoolean(t, snap); lr.Assignment == nil || lr.Assignment.Segment != "is-big" {
		t.Fatalf("expected a string \"true\" to match the boolean, got %+v", lr.Assignment)
	}
}

// The neighbouring mistake that does fail, and the likeliest source of an
// error on a computed boolean: reaching for a comparison operator because the
// formula contains one. A boolean supports eq and neq, not gte.
func TestComputedBoolean_ComparisonOperatorIsRejected(t *testing.T) {
	snap := computedBooleanSnapshot(model.FieldTypeBoolean, true)
	snap.Layers[0].Segments[0].Rules[0].Condition.Operator = model.OpGte

	err := validation.ValidateSnapshot(snap)
	if err == nil {
		t.Fatal("expected gte on a boolean field to be rejected")
	}
	if !strings.Contains(err.Error(), "gte") || !strings.Contains(err.Error(), "boolean") {
		t.Errorf("expected an error naming the operator and the type, got: %v", err)
	}
}
