package strategy

import (
	"fmt"
	"strings"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// EvalCondition evaluates a leaf condition against the context. lookups
// provides the tables referenced by in_lookup / not_in_lookup operators.
func EvalCondition(cond *model.Condition, ctx map[string]interface{}, lookups map[string]model.LookupTable) bool {
	val, present := model.ResolveField(ctx, cond.Field)

	// Presence tests run first, because they are the only operators with an
	// answer for a field that is not in the context: absent is null. Checking
	// presence before them would make is_null false for a missing field, which
	// is backwards.
	if model.IsUnary(cond.Operator) {
		return evalUnary(cond.Operator, val, present)
	}

	if !present {
		return false
	}

	// The right-hand side may name another field rather than carry a literal.
	// It resolves through the same ResolveField as the left, so a dotted path
	// and a "layer:x" reference behave identically on both sides.
	expected := cond.Value
	if cond.ComparesToField() {
		other, ok := model.ResolveField(ctx, cond.ValueField)
		// An absent right-hand field fails the condition, mirroring the left:
		// there is nothing to compare against, and the alternative — treating
		// it as null and letting eq match another absent field — would make a
		// typo look like a passing rule.
		if !ok {
			return false
		}
		expected = other
	}
	return evalOp(cond.Operator, val, expected, lookups)
}

// evalUnary evaluates the operators that test the field itself. A field counts
// as null when it is absent from the context or explicitly null.
func evalUnary(op model.Operator, val interface{}, present bool) bool {
	isNull := !present || val == nil

	switch op {
	case model.OpIsNull:
		return isNull
	case model.OpIsNullOrEmpty:
		if isNull {
			return true
		}
		// Emptiness is the empty string exactly; whitespace is not trimmed, and
		// a zero or false value is not empty.
		s, ok := val.(string)
		return ok && s == ""
	default:
		return false
	}
}

func evalOp(op model.Operator, actual, expected interface{}, lookups map[string]model.LookupTable) bool {
	switch op {
	case model.OpEq:
		return compareEq(actual, expected)
	case model.OpNeq:
		return !compareEq(actual, expected)
	case model.OpGt:
		c, ok := compareNum(actual, expected)
		return ok && c > 0
	case model.OpGte:
		c, ok := compareNum(actual, expected)
		return ok && c >= 0
	case model.OpLt:
		c, ok := compareNum(actual, expected)
		return ok && c < 0
	case model.OpLte:
		c, ok := compareNum(actual, expected)
		return ok && c <= 0
	case model.OpIn:
		return evalIn(actual, expected)
	case model.OpNotIn:
		return !evalIn(actual, expected)
	case model.OpContains:
		return evalContains(actual, expected)
	case model.OpInLookup:
		return evalInLookup(actual, expected, lookups)
	case model.OpNotInLookup:
		return !evalInLookup(actual, expected, lookups)
	default:
		return false
	}
}

// evalInLookup treats the referenced lookup table's keys as an array and checks
// membership. expected is the table id. A missing/dangling table yields false.
func evalInLookup(actual, expected interface{}, lookups map[string]model.LookupTable) bool {
	id, ok := expected.(string)
	if !ok {
		return false
	}
	table, ok := lookups[id]
	if !ok {
		return false
	}
	return evalIn(actual, table.Keys())
}

func compareEq(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json_number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// json_number interface for json.Number compatibility
type json_number interface {
	Float64() (float64, error)
}

func compareNum(a, b interface{}) (int, bool) {
	fa, okA := toFloat64(a)
	fb, okB := toFloat64(b)
	if !okA || !okB {
		return 0, false
	}
	if fa < fb {
		return -1, true
	}
	if fa > fb {
		return 1, true
	}
	return 0, true
}

// evalIn checks if actual value is in the expected list.
func evalIn(actual, expected interface{}) bool {
	list, ok := toSlice(expected)
	if !ok {
		return false
	}
	actualStr := fmt.Sprintf("%v", actual)
	for _, item := range list {
		if fmt.Sprintf("%v", item) == actualStr {
			return true
		}
	}
	return false
}

// evalContains checks if a string contains a substring or an array contains an element.
func evalContains(actual, expected interface{}) bool {
	// String contains substring
	if s, ok := actual.(string); ok {
		if sub, ok := expected.(string); ok {
			return strings.Contains(s, sub)
		}
	}
	// Array contains element
	list, ok := toSlice(actual)
	if !ok {
		return false
	}
	expectedStr := fmt.Sprintf("%v", expected)
	for _, item := range list {
		if fmt.Sprintf("%v", item) == expectedStr {
			return true
		}
	}
	return false
}

func toSlice(v interface{}) ([]interface{}, bool) {
	switch s := v.(type) {
	case []interface{}:
		return s, true
	case []string:
		out := make([]interface{}, len(s))
		for i, item := range s {
			out[i] = item
		}
		return out, true
	default:
		return nil, false
	}
}
