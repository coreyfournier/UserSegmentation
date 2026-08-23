package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func isPrecision() *model.Rule {
	return &model.Rule{
		RuleName:   "isPrecision",
		Expression: &model.Expression{Field: "productType", Operator: model.OpEq, Value: "Precision"},
	}
}

// The motivating case: one condition governs a whole block of checks, and each
// check inside it is still reported on its own.
func precisionSegment() *model.Segment {
	return &model.Segment{
		ID:       "company",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{
			leaf("companyHasFederalEIN", "ein", "12-3456789", "Federal EIN is required."),
			{
				RuleName: "precisionPayrollRequirements",
				Operator: model.CompositeAnd,
				When:     isPrecision(),
				Rules: []model.Rule{
					leaf("hasDefaultPayRate", "defaultPayRate", 25.0, "A default pay rate is required."),
					leaf("hasAnchorDate", "anchorDate", "2026-01-01", "An anchor date is required."),
					leaf("hasPayFrequency", "payFrequency", "biweekly", "A pay frequency is required."),
				},
			},
		},
	}
}

func TestWhen_GatedBlockAppliesAndItemisesEachCheck(t *testing.T) {
	ctx := assertCtx(map[string]interface{}{
		"productType": "Precision",
		"ein":         "12-3456789",
		// All three Precision-only fields are wrong or absent.
	})

	res, _ := (&AssertStrategy{}).Evaluate(precisionSegment(), ctx)
	if res.Status != model.StatusViolated {
		t.Fatalf("expected violated, got %q", res.Status)
	}

	got := failureNames(res)
	want := []string{"hasDefaultPayRate", "hasAnchorDate", "hasPayFrequency"}
	if len(got) != len(want) {
		t.Fatalf("expected each gated check reported separately, got %v", got)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("failure %d: expected %q, got %q", i, name, got[i])
		}
	}
	if res.Failures[0].Message != "A default pay rate is required." {
		t.Errorf("gated children keep their own messages, got %q", res.Failures[0].Message)
	}
}

// The whole block is silent for a company the condition does not select — no
// failures, and no "not applicable" state to interpret.
func TestWhen_GatedBlockContributesNothingWhenNotApplicable(t *testing.T) {
	ctx := assertCtx(map[string]interface{}{
		"productType": "Express",
		"ein":         "12-3456789",
		// None of the Precision fields are set, and none should be checked.
	})

	res, _ := (&AssertStrategy{}).Evaluate(precisionSegment(), ctx)
	if res.Status != model.StatusSatisfied {
		t.Fatalf("expected satisfied for Express, got %q with %v", res.Status, failureNames(res))
	}
}

// The ungated checks in the same segment still run.
func TestWhen_UngatedChecksStillApply(t *testing.T) {
	ctx := assertCtx(map[string]interface{}{"productType": "Express", "ein": ""})

	res, _ := (&AssertStrategy{}).Evaluate(precisionSegment(), ctx)
	if got := failureNames(res); len(got) != 1 || got[0] != "companyHasFederalEIN" {
		t.Errorf("expected only the ungated check to fail, got %v", got)
	}
}

func TestWhen_OnALeaf(t *testing.T) {
	seg := &model.Segment{
		ID:       "company",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{
			{
				RuleName:     "hasStateTaxId",
				When:         &model.Rule{RuleName: "inCA", Expression: &model.Expression{Field: "state", Operator: model.OpEq, Value: "CA"}},
				ErrorMessage: "A California tax ID is required.",
				Expression:   &model.Expression{Field: "stateTaxId", Operator: model.OpNeq, Value: ""},
			},
		},
	}

	inCA, _ := (&AssertStrategy{}).Evaluate(seg, assertCtx(map[string]interface{}{"state": "CA", "stateTaxId": ""}))
	if got := failureNames(inCA); len(got) != 1 {
		t.Errorf("CA company should be checked, got %v", got)
	}

	elsewhere, _ := (&AssertStrategy{}).Evaluate(seg, assertCtx(map[string]interface{}{"state": "TX", "stateTaxId": ""}))
	if elsewhere.Status != model.StatusSatisfied {
		t.Errorf("non-CA company should not be checked, got %q %v", elsewhere.Status, failureNames(elsewhere))
	}
}

// An inapplicable child neither fails an And nor satisfies an Or — the same
// treatment a disabled rule gets.
func TestWhen_InapplicableChildDoesNotDecideItsParent(t *testing.T) {
	ctx := map[string]interface{}{"productType": "Express", "always": "yes"}

	and := &model.Rule{
		RuleName: "group",
		Operator: model.CompositeAnd,
		Rules: []model.Rule{
			{RuleName: "gated", When: isPrecision(), Expression: &model.Expression{Field: "missing", Operator: model.OpEq, Value: "x"}},
			{RuleName: "plain", Expression: &model.Expression{Field: "always", Operator: model.OpEq, Value: "yes"}},
		},
	}
	if !evaluateRule(and, ctx, nil) {
		t.Error("an inapplicable child must not fail an And")
	}

	or := &model.Rule{
		RuleName: "group",
		Operator: model.CompositeOr,
		Rules: []model.Rule{
			{RuleName: "gated", When: isPrecision(), Expression: &model.Expression{Field: "always", Operator: model.OpEq, Value: "yes"}},
		},
	}
	if evaluateRule(or, ctx, nil) {
		t.Error("an inapplicable branch must not satisfy an Or")
	}
}

// Segmentation is unaffected except that a gated rule simply does not match.
func TestWhen_InFirstMatchStrategy(t *testing.T) {
	seg := &model.Segment{
		ID:       "tier",
		Strategy: model.StrategyRule,
		Rules: []model.Rule{
			{
				RuleName:     "precisionTier",
				When:         isPrecision(),
				SuccessEvent: "precision",
				Expression:   &model.Expression{Field: "plan", Operator: model.OpEq, Value: "premium"},
			},
			{
				RuleName:     "standardTier",
				SuccessEvent: "standard",
				Expression:   &model.Expression{Field: "plan", Operator: model.OpEq, Value: "premium"},
			},
		},
	}

	precision, _ := (&RuleStrategy{}).Evaluate(seg, assertCtx(map[string]interface{}{"productType": "Precision", "plan": "premium"}))
	if precision.Segment != "precision" {
		t.Errorf("expected the gated rule to win, got %q", precision.Segment)
	}

	express, _ := (&RuleStrategy{}).Evaluate(seg, assertCtx(map[string]interface{}{"productType": "Express", "plan": "premium"}))
	if express.Segment != "standard" {
		t.Errorf("expected the gated rule to be passed over, got %q", express.Segment)
	}
}

// Nesting: a gate inside a gate.
func TestWhen_NestedGates(t *testing.T) {
	seg := &model.Segment{
		ID:       "company",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{{
			RuleName: "precisionBlock",
			Operator: model.CompositeAnd,
			When:     isPrecision(),
			Rules: []model.Rule{
				{
					RuleName: "californiaBlock",
					Operator: model.CompositeAnd,
					When:     &model.Rule{RuleName: "inCA", Expression: &model.Expression{Field: "state", Operator: model.OpEq, Value: "CA"}},
					Rules: []model.Rule{
						leaf("hasCaTaxId", "caTaxId", "CA-1", "CA tax ID required."),
					},
				},
				leaf("hasAnchorDate", "anchorDate", "2026-01-01", "Anchor date required."),
			},
		}},
	}

	cases := []struct {
		name  string
		ctx   map[string]interface{}
		fails []string
	}{
		{"precision in CA", map[string]interface{}{"productType": "Precision", "state": "CA"}, []string{"hasCaTaxId", "hasAnchorDate"}},
		{"precision elsewhere", map[string]interface{}{"productType": "Precision", "state": "TX"}, []string{"hasAnchorDate"}},
		{"express in CA", map[string]interface{}{"productType": "Express", "state": "CA"}, nil},
	}

	for _, tc := range cases {
		res, _ := (&AssertStrategy{}).Evaluate(seg, assertCtx(tc.ctx))
		got := failureNames(res)
		if len(got) != len(tc.fails) {
			t.Errorf("%s: expected %v, got %v", tc.name, tc.fails, got)
			continue
		}
		for i, name := range tc.fails {
			if got[i] != name {
				t.Errorf("%s: expected %v, got %v", tc.name, tc.fails, got)
				break
			}
		}
	}
}
