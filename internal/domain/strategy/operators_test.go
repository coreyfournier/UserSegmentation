package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestEvalExpression_Eq(t *testing.T) {
	ctx := map[string]interface{}{"country": "US"}
	expr := &model.Expression{Field: "country", Operator: model.OpEq, Value: "US"}
	if !EvalExpression(expr, ctx) {
		t.Error("expected eq to match")
	}
	expr.Value = "CA"
	if EvalExpression(expr, ctx) {
		t.Error("expected eq not to match")
	}
}

func TestEvalExpression_Neq(t *testing.T) {
	ctx := map[string]interface{}{"country": "US"}
	expr := &model.Expression{Field: "country", Operator: model.OpNeq, Value: "CA"}
	if !EvalExpression(expr, ctx) {
		t.Error("expected neq to match")
	}
}

func TestEvalExpression_Numeric(t *testing.T) {
	ctx := map[string]interface{}{"age": float64(25)}

	tests := []struct {
		op   model.Operator
		val  interface{}
		want bool
	}{
		{model.OpGt, float64(18), true},
		{model.OpGt, float64(25), false},
		{model.OpGte, float64(25), true},
		{model.OpLt, float64(30), true},
		{model.OpLt, float64(25), false},
		{model.OpLte, float64(25), true},
	}

	for _, tt := range tests {
		expr := &model.Expression{Field: "age", Operator: tt.op, Value: tt.val}
		got := EvalExpression(expr, ctx)
		if got != tt.want {
			t.Errorf("op=%s val=%v: got %v, want %v", tt.op, tt.val, got, tt.want)
		}
	}
}

func TestEvalExpression_In(t *testing.T) {
	ctx := map[string]interface{}{"country": "US"}
	expr := &model.Expression{Field: "country", Operator: model.OpIn, Value: []interface{}{"US", "CA"}}
	if !EvalExpression(expr, ctx) {
		t.Error("expected in to match")
	}
	expr.Value = []interface{}{"UK", "DE"}
	if EvalExpression(expr, ctx) {
		t.Error("expected in not to match")
	}
}

func TestEvalExpression_Contains_String(t *testing.T) {
	ctx := map[string]interface{}{"email": "user@example.com"}
	expr := &model.Expression{Field: "email", Operator: model.OpContains, Value: "example"}
	if !EvalExpression(expr, ctx) {
		t.Error("expected contains to match substring")
	}
}

func TestEvalExpression_Contains_Array(t *testing.T) {
	ctx := map[string]interface{}{"tags": []interface{}{"beta", "vip"}}
	expr := &model.Expression{Field: "tags", Operator: model.OpContains, Value: "beta"}
	if !EvalExpression(expr, ctx) {
		t.Error("expected contains to match array element")
	}
	expr.Value = "alpha"
	if EvalExpression(expr, ctx) {
		t.Error("expected contains not to match missing element")
	}
}

func TestEvalExpression_MissingField(t *testing.T) {
	ctx := map[string]interface{}{}
	expr := &model.Expression{Field: "missing", Operator: model.OpEq, Value: "x"}
	if EvalExpression(expr, ctx) {
		t.Error("expected missing field to not match")
	}
}
