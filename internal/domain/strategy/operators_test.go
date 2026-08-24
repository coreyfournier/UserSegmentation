package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestEvalCondition_Eq(t *testing.T) {
	ctx := map[string]interface{}{"country": "US"}
	cond := &model.Condition{Field: "country", Operator: model.OpEq, Value: "US"}
	if !EvalCondition(cond, ctx, nil) {
		t.Error("expected eq to match")
	}
	cond.Value = "CA"
	if EvalCondition(cond, ctx, nil) {
		t.Error("expected eq not to match")
	}
}

func TestEvalCondition_Neq(t *testing.T) {
	ctx := map[string]interface{}{"country": "US"}
	cond := &model.Condition{Field: "country", Operator: model.OpNeq, Value: "CA"}
	if !EvalCondition(cond, ctx, nil) {
		t.Error("expected neq to match")
	}
}

func TestEvalCondition_Numeric(t *testing.T) {
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
		cond := &model.Condition{Field: "age", Operator: tt.op, Value: tt.val}
		got := EvalCondition(cond, ctx, nil)
		if got != tt.want {
			t.Errorf("op=%s val=%v: got %v, want %v", tt.op, tt.val, got, tt.want)
		}
	}
}

func TestEvalCondition_In(t *testing.T) {
	ctx := map[string]interface{}{"country": "US"}
	cond := &model.Condition{Field: "country", Operator: model.OpIn, Value: []interface{}{"US", "CA"}}
	if !EvalCondition(cond, ctx, nil) {
		t.Error("expected in to match")
	}
	cond.Value = []interface{}{"UK", "DE"}
	if EvalCondition(cond, ctx, nil) {
		t.Error("expected in not to match")
	}
}

func TestEvalCondition_Contains_String(t *testing.T) {
	ctx := map[string]interface{}{"email": "user@example.com"}
	cond := &model.Condition{Field: "email", Operator: model.OpContains, Value: "example"}
	if !EvalCondition(cond, ctx, nil) {
		t.Error("expected contains to match substring")
	}
}

func TestEvalCondition_Contains_Array(t *testing.T) {
	ctx := map[string]interface{}{"tags": []interface{}{"beta", "vip"}}
	cond := &model.Condition{Field: "tags", Operator: model.OpContains, Value: "beta"}
	if !EvalCondition(cond, ctx, nil) {
		t.Error("expected contains to match array element")
	}
	cond.Value = "alpha"
	if EvalCondition(cond, ctx, nil) {
		t.Error("expected contains not to match missing element")
	}
}

func TestEvalCondition_MissingField(t *testing.T) {
	ctx := map[string]interface{}{}
	cond := &model.Condition{Field: "missing", Operator: model.OpEq, Value: "x"}
	if EvalCondition(cond, ctx, nil) {
		t.Error("expected missing field to not match")
	}
}
