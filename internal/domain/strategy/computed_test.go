package strategy

import (
	"math"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestComputedStrategy_ComputedFieldUsedInRule(t *testing.T) {
	seg := &model.Segment{
		Strategy: model.StrategyComputed,
		Computed: []model.ComputedField{
			{Name: "Adjusted", Type: "number", Formula: "Rating * 2"},
		},
		Rules: []model.Rule{
			{
				RuleName:     "high",
				SuccessEvent: "high-tier",
				Condition:   &model.Condition{Field: "Adjusted", Operator: "gt", Value: float64(10)},
			},
		},
		Default: "normal",
	}

	s := &ComputedStrategy{}

	// Rating=6 → Adjusted=12 → matches "gt 10"
	res, ok := s.Evaluate(seg, &EvalContext{
		SubjectKey: "user1",
		Context:    map[string]interface{}{"Rating": float64(6)},
	})
	if !ok || res.Segment != "high-tier" {
		t.Errorf("expected high-tier, got %v %v", res, ok)
	}

	// Rating=4 → Adjusted=8 → no rule match → default
	res, ok = s.Evaluate(seg, &EvalContext{
		SubjectKey: "user2",
		Context:    map[string]interface{}{"Rating": float64(4)},
	})
	if !ok || res.Segment != "normal" {
		t.Errorf("expected normal (default), got %v %v", res, ok)
	}
}

func TestComputedStrategy_FormulaOverwritesContext(t *testing.T) {
	seg := &model.Segment{
		Strategy: model.StrategyComputed,
		Computed: []model.ComputedField{
			{Name: "Score", Type: "number", Formula: "Base + Bonus"},
		},
		Rules: []model.Rule{
			{
				RuleName:     "winner",
				SuccessEvent: "winner",
				Condition:   &model.Condition{Field: "Score", Operator: "gte", Value: float64(100)},
			},
		},
	}

	s := &ComputedStrategy{}

	// Score from inputSchema would be 50, but expression computes Base+Bonus=120 → overwrites
	res, ok := s.Evaluate(seg, &EvalContext{
		SubjectKey: "u",
		Context:    map[string]interface{}{"Base": float64(80), "Bonus": float64(40), "Score": float64(50)},
	})
	if !ok || res.Segment != "winner" {
		t.Errorf("expression should overwrite context Score: got %v %v", res, ok)
	}
}

func TestComputedStrategy_ChainedFormulas(t *testing.T) {
	seg := &model.Segment{
		Strategy: model.StrategyComputed,
		Computed: []model.ComputedField{
			{Name: "Double", Type: "number", Formula: "X * 2"},
			{Name: "Quad", Type: "number", Formula: "Double * 2"}, // references previous
		},
		Rules: []model.Rule{
			{
				RuleName:     "quad",
				SuccessEvent: "quad",
				Condition:   &model.Condition{Field: "Quad", Operator: "eq", Value: float64(20)},
			},
		},
	}

	s := &ComputedStrategy{}

	res, ok := s.Evaluate(seg, &EvalContext{
		SubjectKey: "u",
		Context:    map[string]interface{}{"X": float64(5)},
	})
	if !ok || res.Segment != "quad" {
		t.Errorf("chained formula: expected quad, got %v %v", res, ok)
	}
}

func TestComputedStrategy_MathFunctions(t *testing.T) {
	s := &ComputedStrategy{}

	cases := []struct {
		name       string
		formula string
		ctx        map[string]interface{}
		wantApprox float64
		eps        float64
	}{
		{
			name:       "exp",
			formula: "exp(0.0)",
			ctx:        map[string]interface{}{},
			wantApprox: 1.0,
			eps:        1e-9,
		},
		{
			name:       "exp negative",
			formula: "exp(-1.0)",
			ctx:        map[string]interface{}{},
			wantApprox: 0.36787944,
			eps:        1e-6,
		},
		{
			name:       "ln",
			formula: "ln(X)",
			ctx:        map[string]interface{}{"X": math.E},
			wantApprox: 1.0,
			eps:        1e-9,
		},
		{
			name:       "pow",
			formula: "pow(X, 3.0)",
			ctx:        map[string]interface{}{"X": float64(2)},
			wantApprox: 8.0,
			eps:        1e-9,
		},
		{
			name:       "logistic sigmoid",
			formula: "1.0 / (1.0 + exp(-Z))",
			ctx:        map[string]interface{}{"Z": float64(0)},
			wantApprox: 0.5,
			eps:        1e-9,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seg := &model.Segment{
				Strategy: model.StrategyComputed,
				Computed: []model.ComputedField{
					{Name: "Result", Type: "number", Formula: tc.formula},
				},
				Rules: []model.Rule{
					{RuleName: "match", SuccessEvent: "ok",
						Condition: &model.Condition{Field: "Result", Operator: "gte", Value: float64(-1e18)}},
				},
			}
			res, ok := s.Evaluate(seg, &EvalContext{SubjectKey: "u", Context: tc.ctx})
			if !ok {
				t.Fatalf("evaluate failed")
			}
			got, ok2 := res.Computed["Result"].(float64)
			if !ok2 {
				t.Fatalf("Result not float64: %T %v", res.Computed["Result"], res.Computed["Result"])
			}
			if math.Abs(got-tc.wantApprox) > tc.eps {
				t.Errorf("expression %q: got %v, want ~%v (eps=%v)", tc.formula, got, tc.wantApprox, tc.eps)
			}
		})
	}
}

func TestComputedStrategy_LogisticChain(t *testing.T) {
	// Models a full logistic scoring chain: Z → P → segment decision.
	seg := &model.Segment{
		Strategy: model.StrategyComputed,
		Computed: []model.ComputedField{
			{Name: "Z", Type: "number", Formula: "W0 + (W1 * S1) + (W2 * S2)"},
			{Name: "P", Type: "number", Formula: "1.0 / (1.0 + exp(-Z))"},
		},
		Rules: []model.Rule{
			{
				RuleName:     "high-risk",
				SuccessEvent: "decline",
				Condition:   &model.Condition{Field: "P", Operator: "gt", Value: float64(0.5)},
			},
		},
		Default: "approve",
	}

	s := &ComputedStrategy{}

	// W0=-3, W1=2, S1=1 (strong positive signal), W2=0, S2=0 → Z=-1, P≈0.27 → approve
	res, ok := s.Evaluate(seg, &EvalContext{
		SubjectKey: "u1",
		Context: map[string]interface{}{
			"W0": float64(-3), "W1": float64(2), "S1": float64(1),
			"W2": float64(0), "S2": float64(0),
		},
	})
	if !ok || res.Segment != "approve" {
		t.Errorf("low Z: expected approve, got %v", res.Segment)
	}

	// W0=0, W1=2, S1=1, W2=1, S2=0.6 → Z=2.6, P≈0.93 → decline
	res, ok = s.Evaluate(seg, &EvalContext{
		SubjectKey: "u2",
		Context: map[string]interface{}{
			"W0": float64(0), "W1": float64(2), "S1": float64(1),
			"W2": float64(1), "S2": float64(0.6),
		},
	})
	if !ok || res.Segment != "decline" {
		t.Errorf("high Z: expected decline, got %v", res.Segment)
	}

	// Verify P is in (0,1)
	p, ok2 := res.Computed["P"].(float64)
	if !ok2 || p < 0 || p > 1 {
		t.Errorf("P should be in (0,1), got %v", p)
	}
}

// employee is a helper that builds the map shape that JSON unmarshalling produces.
func employee(id int, state string, spend float64) map[string]interface{} {
	return map[string]interface{}{
		"Id":                   float64(id),
		"State":                state,
		"TransferSpendThisMonth": spend,
	}
}

func TestComputedStrategy_CTFeeOverride(t *testing.T) {
	// Mirrors the three scenarios from the C#/Lua POC:
	//   Scenario A: CT total = 40  → fee-waived  (fee = 0,  exceeds 30)
	//   Scenario B: CT total = 25  → fee-standard (fee = 4,  total+4 ≤ 30)
	//   Scenario C: CT total = 28  → fee-partial  (fee = 2,  total+4 > 30)
	seg := &model.Segment{
		Strategy: model.StrategyComputed,
		Computed: []model.ComputedField{
			{
				Name:       "CTTotal",
				Type:       "number",
				Formula: `sum(map(filter(Employees, {.State == "CT"}), {.TransferSpendThisMonth}))`,
			},
			{
				Name:       "TransferFee",
				Type:       "number",
				Formula: "CTTotal > 30.0 ? 0.0 : (CTTotal + 4.0 > 30.0 ? 30.0 - CTTotal : 4.0)",
			},
		},
		Rules: []model.Rule{
			{
				RuleName:     "fee-waived",
				SuccessEvent: "fee-waived",
				Condition:   &model.Condition{Field: "CTTotal", Operator: "gt", Value: float64(30)},
			},
			{
				RuleName:     "fee-partial",
				SuccessEvent: "fee-partial",
				Condition:   &model.Condition{Field: "CTTotal", Operator: "gt", Value: float64(26)},
			},
		},
		Default: "fee-standard",
	}

	s := &ComputedStrategy{}

	type want struct {
		segment    string
		ctTotal    float64
		transferFee float64
	}

	cases := []struct {
		name      string
		employees []interface{}
		want      want
	}{
		{
			name: "Scenario A — CT spend exceeds 30",
			employees: []interface{}{
				employee(1234, "CT", 10),
				employee(1232, "MD", 15),
				employee(1888, "CT", 30),
			},
			want: want{segment: "fee-waived", ctTotal: 40, transferFee: 0},
		},
		{
			name: "Scenario B — CT spend under threshold",
			employees: []interface{}{
				employee(1234, "CT", 10),
				employee(1232, "MD", 50),
				employee(1888, "CT", 15),
			},
			want: want{segment: "fee-standard", ctTotal: 25, transferFee: 4},
		},
		{
			name: "Scenario C — CT spend will hit 30 with fee",
			employees: []interface{}{
				employee(1234, "CT", 20),
				employee(1232, "MD", 50),
				employee(1888, "CT", 8),
			},
			want: want{segment: "fee-partial", ctTotal: 28, transferFee: 2},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := map[string]interface{}{"Employees": tc.employees}
			res, ok := s.Evaluate(seg, &EvalContext{SubjectKey: "batch", Context: ctx})
			if !ok {
				t.Fatalf("evaluate returned not-ok")
			}
			if res.Segment != tc.want.segment {
				t.Errorf("segment: got %q, want %q", res.Segment, tc.want.segment)
			}
			ctTotal, _ := res.Computed["CTTotal"].(float64)
			fee, _ := res.Computed["TransferFee"].(float64)
			if ctTotal != tc.want.ctTotal {
				t.Errorf("CTTotal: got %v, want %v", ctTotal, tc.want.ctTotal)
			}
			if fee != tc.want.transferFee {
				t.Errorf("TransferFee: got %v, want %v", fee, tc.want.transferFee)
			}
		})
	}
}

func TestComputedStrategy_BadFormulaSkipped(t *testing.T) {
	seg := &model.Segment{
		Strategy: model.StrategyComputed,
		Computed: []model.ComputedField{
			{Name: "Bad", Type: "number", Formula: "!!!invalid!!!"}, // compile error
			{Name: "Good", Type: "number", Formula: "X + 1"},
		},
		Rules: []model.Rule{
			{
				RuleName:     "ok",
				SuccessEvent: "ok",
				Condition:   &model.Condition{Field: "Good", Operator: "eq", Value: float64(6)},
			},
		},
	}

	s := &ComputedStrategy{}

	res, ok := s.Evaluate(seg, &EvalContext{
		SubjectKey: "u",
		Context:    map[string]interface{}{"X": float64(5)},
	})
	if !ok || res.Segment != "ok" {
		t.Errorf("bad expression should be skipped: got %v %v", res, ok)
	}
}
