package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func evalField(op model.Operator, field string, ctx map[string]interface{}) bool {
	return EvalExpression(&model.Expression{Field: field, Operator: op}, ctx, nil)
}

// A field that is not in the context at all is null. This is the case the
// operator exists for, and it only works because presence tests are evaluated
// before the missing-field short circuit.
func TestIsNull_AbsentField(t *testing.T) {
	if !evalField(model.OpIsNull, "defaultPayRate", map[string]interface{}{}) {
		t.Error("an absent field should be null")
	}
	if !evalField(model.OpIsNull, "company.defaultPayRate", map[string]interface{}{
		"company": map[string]interface{}{"ein": "12-3456789"},
	}) {
		t.Error("an absent nested field should be null")
	}
}

func TestIsNull_ExplicitNull(t *testing.T) {
	if !evalField(model.OpIsNull, "payRate", map[string]interface{}{"payRate": nil}) {
		t.Error("an explicit null should be null")
	}
}

func TestIsNull_PresentValuesAreNotNull(t *testing.T) {
	cases := map[string]interface{}{
		"empty string": "",
		"string":       "biweekly",
		"zero":         0.0,
		"number":       25.5,
		"false":        false,
		"true":         true,
		"empty array":  []interface{}{},
		"array":        []interface{}{"a"},
	}
	for name, v := range cases {
		if evalField(model.OpIsNull, "f", map[string]interface{}{"f": v}) {
			t.Errorf("%s should not be null", name)
		}
	}
}

func TestIsNullOrEmpty(t *testing.T) {
	cases := []struct {
		name string
		ctx  map[string]interface{}
		want bool
	}{
		{"absent", map[string]interface{}{}, true},
		{"explicit null", map[string]interface{}{"f": nil}, true},
		{"empty string", map[string]interface{}{"f": ""}, true},
		{"non-empty string", map[string]interface{}{"f": "x"}, false},
		// Whitespace is not trimmed and non-strings are never "empty".
		{"whitespace", map[string]interface{}{"f": " "}, false},
		{"zero", map[string]interface{}{"f": 0.0}, false},
		{"false", map[string]interface{}{"f": false}, false},
	}
	for _, tc := range cases {
		if got := evalField(model.OpIsNullOrEmpty, "f", tc.ctx); got != tc.want {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.want, got)
		}
	}
}

// Value is ignored, so a stale one left over from another operator cannot
// change the outcome.
func TestUnaryOperators_IgnoreValue(t *testing.T) {
	expr := &model.Expression{Field: "f", Operator: model.OpIsNull, Value: "leftover"}
	if !EvalExpression(expr, map[string]interface{}{}, nil) {
		t.Error("is_null should ignore a stale value")
	}
}

// Comparison operators keep their existing behaviour on an absent field: false,
// which is what makes "must be set" assertions like neq "" work.
func TestComparisonOperators_StillFalseWhenAbsent(t *testing.T) {
	for _, op := range []model.Operator{model.OpEq, model.OpNeq, model.OpGt, model.OpIn, model.OpContains} {
		expr := &model.Expression{Field: "missing", Operator: op, Value: ""}
		if EvalExpression(expr, map[string]interface{}{}, nil) {
			t.Errorf("%s should be false for an absent field", op)
		}
	}
}

func TestIsUnary(t *testing.T) {
	unary := []model.Operator{model.OpIsNull, model.OpIsNullOrEmpty}
	for _, op := range unary {
		if !model.IsUnary(op) {
			t.Errorf("%s should be unary", op)
		}
	}
	for _, op := range []model.Operator{model.OpEq, model.OpIn, model.OpInLookup} {
		if model.IsUnary(op) {
			t.Errorf("%s should not be unary", op)
		}
	}
}

// Type gating: is_null applies to every type, is_null_or_empty to strings.
func TestNullOperators_TypeSupport(t *testing.T) {
	all := []model.FieldType{model.FieldTypeString, model.FieldTypeNumber, model.FieldTypeBoolean, model.FieldTypeArray}
	for _, ft := range all {
		if !model.OperatorSupportsType(model.OpIsNull, ft) {
			t.Errorf("is_null should support %s", ft)
		}
	}
	if !model.OperatorSupportsType(model.OpIsNullOrEmpty, model.FieldTypeString) {
		t.Error("is_null_or_empty should support string")
	}
	for _, ft := range []model.FieldType{model.FieldTypeNumber, model.FieldTypeBoolean, model.FieldTypeArray} {
		if model.OperatorSupportsType(model.OpIsNullOrEmpty, ft) {
			t.Errorf("is_null_or_empty should not support %s", ft)
		}
	}
}

// The presence operators are the natural way to state a checklist condition:
// the problem is that the value is not there.
func TestNullOperators_AsChecklistConditions(t *testing.T) {
	seg := &model.Segment{
		ID:       "company",
		Strategy: model.StrategyChecklist,
		Rules: []model.Rule{{
			RuleName:     "contactEmailMissing",
			ErrorMessage: "Contact email is required for deduction emails.",
			Expression:   &model.Expression{Field: "EmailContact", Operator: model.OpIsNullOrEmpty},
		}},
	}

	fire, _ := (&ChecklistStrategy{}).Evaluate(seg, checkCtx(map[string]interface{}{"EmailContact": ""}))
	if fire.Status != model.StatusViolated {
		t.Errorf("an empty contact email should fire, got %q", fire.Status)
	}

	quiet, _ := (&ChecklistStrategy{}).Evaluate(seg, checkCtx(map[string]interface{}{"EmailContact": "a@b.com"}))
	if quiet.Status != model.StatusSatisfied {
		t.Errorf("a supplied contact email should not fire, got %q", quiet.Status)
	}
}
