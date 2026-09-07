package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// cond builds a leaf condition comparing Field against another field.
func refCond(field string, op model.Operator, ref string) *model.Condition {
	return &model.Condition{Field: field, Operator: op, ValueField: ref}
}

func TestEvalCondition_ComparesAgainstAnotherField(t *testing.T) {
	ctx := map[string]interface{}{
		"hoursWorked": 40.0,
		"minHours":    20.0,
		"sameAsMin":   20.0,
		"tier":        "gold",
		"targetTier":  "gold",
		"otherTier":   "silver",
	}

	cases := []struct {
		name string
		cond *model.Condition
		want bool
	}{
		{"gte holds", refCond("hoursWorked", model.OpGte, "minHours"), true},
		{"lt does not", refCond("hoursWorked", model.OpLt, "minHours"), false},
		{"gt on equal values", refCond("sameAsMin", model.OpGt, "minHours"), false},
		{"gte on equal values", refCond("sameAsMin", model.OpGte, "minHours"), true},
		{"eq on strings", refCond("tier", model.OpEq, "targetTier"), true},
		{"eq on differing strings", refCond("tier", model.OpEq, "otherTier"), false},
		{"neq on differing strings", refCond("tier", model.OpNeq, "otherTier"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := EvalCondition(tc.cond, ctx, nil); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// An absent right-hand field fails the condition rather than comparing against
// nil. Comparing two absent fields with eq must not report a match — that is a
// typo passing itself off as a satisfied rule.
func TestEvalCondition_AbsentValueField(t *testing.T) {
	ctx := map[string]interface{}{"hoursWorked": 40.0}

	if EvalCondition(refCond("hoursWorked", model.OpGte, "minHours"), ctx, nil) {
		t.Error("gte against an absent field should not hold")
	}
	if EvalCondition(refCond("hoursWorked", model.OpNeq, "minHours"), ctx, nil) {
		t.Error("neq against an absent field should not hold either — there is nothing to compare")
	}
	if EvalCondition(refCond("missingLeft", model.OpEq, "alsoMissing"), ctx, nil) {
		t.Error("two absent fields must not compare equal")
	}
}

// The right-hand side resolves through the same ResolveField as the left, so a
// dotted path and a cross-layer key work identically on both sides.
func TestEvalCondition_ValueFieldResolution(t *testing.T) {
	ctx := map[string]interface{}{
		"company":     map[string]interface{}{"minHours": 30.0},
		"hoursWorked": 40.0,
		"layer:tier":  "gold",
		"tier":        "gold",
	}

	if !EvalCondition(refCond("hoursWorked", model.OpGt, "company.minHours"), ctx, nil) {
		t.Error("a dotted path should resolve on the right-hand side")
	}
	if !EvalCondition(refCond("tier", model.OpEq, "layer:tier"), ctx, nil) {
		t.Error("a cross-layer key should resolve on the right-hand side")
	}
}

// A literal still wins when no reference is set, and setting one takes over —
// the two never blend.
func TestEvalCondition_LiteralUnaffected(t *testing.T) {
	ctx := map[string]interface{}{"hoursWorked": 40.0, "minHours": 100.0}

	lit := &model.Condition{Field: "hoursWorked", Operator: model.OpGte, Value: 20.0}
	if !EvalCondition(lit, ctx, nil) {
		t.Error("a literal comparison should be unaffected by the reference path")
	}
	// The same condition pointed at a field instead resolves to 100, not 20.
	ref := &model.Condition{Field: "hoursWorked", Operator: model.OpGte, ValueField: "minHours"}
	if EvalCondition(ref, ctx, nil) {
		t.Error("the reference should be resolved, not the literal")
	}
}

// A unary operator ignores the right-hand side entirely, reference included:
// is_null asks about the left field and nothing else.
func TestEvalCondition_UnaryIgnoresValueField(t *testing.T) {
	ctx := map[string]interface{}{"other": 1.0}
	c := &model.Condition{Field: "missing", Operator: model.OpIsNull, ValueField: "other"}
	if !EvalCondition(c, ctx, nil) {
		t.Error("is_null should hold for an absent field regardless of valueField")
	}
}
