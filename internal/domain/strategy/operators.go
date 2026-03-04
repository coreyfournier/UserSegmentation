package strategy

import (
	"fmt"
	"strings"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// EvalExpression evaluates a leaf expression against the context.
func EvalExpression(expr *model.Expression, ctx map[string]interface{}) bool {
	val, ok := ctx[expr.Field]
	if !ok {
		return false
	}
	return evalOp(expr.Operator, val, expr.Value)
}

func evalOp(op model.Operator, actual, expected interface{}) bool {
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
	case model.OpContains:
		return evalContains(actual, expected)
	default:
		return false
	}
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
