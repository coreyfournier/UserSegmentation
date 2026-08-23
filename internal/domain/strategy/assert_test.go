package strategy

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func assertCtx(ctx map[string]interface{}) *EvalContext {
	return &EvalContext{
		SubjectKey:      "company-1",
		Context:         ctx,
		DefaultLanguage: "en",
	}
}

func leaf(name, field string, value interface{}, msg string) model.Rule {
	return model.Rule{
		RuleName:     name,
		ErrorMessage: msg,
		Expression:   &model.Expression{Field: field, Operator: model.OpEq, Value: value},
	}
}

func failureNames(res Result) []string {
	out := make([]string, len(res.Failures))
	for i, f := range res.Failures {
		out[i] = f.Rule
	}
	return out
}

func TestAssert_Satisfied(t *testing.T) {
	seg := &model.Segment{
		ID:       "precision",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{
			leaf("hasEIN", "ein", "12-3456789", "EIN required"),
			leaf("hasFrequency", "payFrequency", "biweekly", "Frequency required"),
		},
	}
	ctx := assertCtx(map[string]interface{}{"ein": "12-3456789", "payFrequency": "biweekly"})

	res, ok := (&AssertStrategy{}).Evaluate(seg, ctx)
	if !ok {
		t.Fatal("assert should always produce a result")
	}
	if res.Status != model.StatusSatisfied {
		t.Errorf("expected satisfied, got %q", res.Status)
	}
	if len(res.Failures) != 0 {
		t.Errorf("expected no failures, got %v", failureNames(res))
	}
}

// The whole point of a gate: report every problem in one pass so the person
// fixing them does not make repeated round trips.
func TestAssert_CollectsEveryFailure(t *testing.T) {
	seg := &model.Segment{
		ID:       "precision",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{
			leaf("hasEIN", "ein", "12-3456789", "EIN required"),
			leaf("hasFrequency", "payFrequency", "biweekly", "Frequency required"),
			leaf("hasAnchorDate", "anchorDate", "2026-01-01", "Anchor date required"),
		},
	}
	ctx := assertCtx(map[string]interface{}{"ein": "12-3456789"})

	res, _ := (&AssertStrategy{}).Evaluate(seg, ctx)
	if res.Status != model.StatusViolated {
		t.Errorf("expected violated, got %q", res.Status)
	}
	got := failureNames(res)
	if len(got) != 2 {
		t.Fatalf("expected both remaining problems, got %v", got)
	}
	if got[0] != "hasFrequency" || got[1] != "hasAnchorDate" {
		t.Errorf("unexpected failures: %v", got)
	}
	if res.Failures[0].Message != "Frequency required" {
		t.Errorf("message not carried: %q", res.Failures[0].Message)
	}
}

// An And node reports each failing child, not just the first.
func TestAssert_AndCollectsAllChildren(t *testing.T) {
	seg := &model.Segment{
		ID:       "dates",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{{
			RuleName: "datesValid",
			Operator: model.CompositeAnd,
			Rules: []model.Rule{
				leaf("startSet", "start", "2026-01-01", "start"),
				leaf("endSet", "end", "2026-01-15", "end"),
				leaf("checkSet", "check", "2026-01-20", "check"),
			},
		}},
	}
	ctx := assertCtx(map[string]interface{}{"start": "2026-01-01"})

	res, _ := (&AssertStrategy{}).Evaluate(seg, ctx)
	if got := failureNames(res); len(got) != 2 || got[0] != "endSet" || got[1] != "checkSet" {
		t.Errorf("expected both failing children, got %v", got)
	}
}

// A failing Or reports the Or itself. Listing each branch would tell the
// resolver to set all three fields when any one would have done.
func TestAssert_OrReportsNodeNotBranches(t *testing.T) {
	seg := &model.Segment{
		ID:       "contact",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{{
			RuleName:     "hasAnyContactMethod",
			Operator:     model.CompositeOr,
			ErrorMessage: "Provide at least one of email, phone, or mailing address.",
			Rules: []model.Rule{
				leaf("hasEmail", "email", "a@b.com", "email"),
				leaf("hasPhone", "phone", "555", "phone"),
				leaf("hasAddress", "address", "1 Main St", "address"),
			},
		}},
	}
	ctx := assertCtx(map[string]interface{}{})

	res, _ := (&AssertStrategy{}).Evaluate(seg, ctx)
	got := failureNames(res)
	if len(got) != 1 || got[0] != "hasAnyContactMethod" {
		t.Fatalf("expected a single failure for the Or node, got %v", got)
	}
	if !strings.Contains(res.Failures[0].Message, "at least one") {
		t.Errorf("expected the Or node's own message, got %q", res.Failures[0].Message)
	}
}

func TestAssert_OrSatisfiedByOneBranch(t *testing.T) {
	seg := &model.Segment{
		ID:       "contact",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{{
			RuleName: "hasAnyContactMethod",
			Operator: model.CompositeOr,
			Rules: []model.Rule{
				leaf("hasEmail", "email", "a@b.com", "email"),
				leaf("hasPhone", "phone", "555", "phone"),
			},
		}},
	}
	ctx := assertCtx(map[string]interface{}{"phone": "555"})

	res, _ := (&AssertStrategy{}).Evaluate(seg, ctx)
	if res.Status != model.StatusSatisfied {
		t.Errorf("one satisfied branch should satisfy the Or, got %q (%v)", res.Status, failureNames(res))
	}
}

func TestAssert_NestedAndUnderOr(t *testing.T) {
	// (start AND end) OR waived — neither branch holds, so the Or is reported.
	seg := &model.Segment{
		ID:       "dates",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{{
			RuleName:     "datesOrWaiver",
			Operator:     model.CompositeOr,
			ErrorMessage: "Supply both pay period dates, or record a waiver.",
			Rules: []model.Rule{
				{
					RuleName: "bothDates",
					Operator: model.CompositeAnd,
					Rules: []model.Rule{
						leaf("startSet", "start", "2026-01-01", "start"),
						leaf("endSet", "end", "2026-01-15", "end"),
					},
				},
				leaf("waived", "waiver", true, "waiver"),
			},
		}},
	}
	ctx := assertCtx(map[string]interface{}{"start": "2026-01-01"})

	res, _ := (&AssertStrategy{}).Evaluate(seg, ctx)
	if got := failureNames(res); len(got) != 1 || got[0] != "datesOrWaiver" {
		t.Errorf("expected only the Or node, got %v", got)
	}
}

func TestAssert_DisabledRuleIgnored(t *testing.T) {
	off := false
	seg := &model.Segment{
		ID:       "gate",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{
			{
				RuleName:   "retired",
				Enabled:    &off,
				Expression: &model.Expression{Field: "nope", Operator: model.OpEq, Value: "x"},
			},
		},
	}

	res, _ := (&AssertStrategy{}).Evaluate(seg, assertCtx(map[string]interface{}{}))
	if res.Status != model.StatusSatisfied {
		t.Errorf("a disabled rule must not fail the gate, got %q", res.Status)
	}
}

// Assert composes ExpressionStrategy, so computed fields are available to both
// the assertions and their messages.
func TestAssert_InheritsComputedFields(t *testing.T) {
	seg := &model.Segment{
		ID:       "limits",
		Strategy: model.StrategyAssert,
		Expressions: []model.ExpressionDef{
			{Name: "MaxAllowed", Type: model.FieldTypeNumber, Expression: "EarnedWages * 0.5"},
		},
		Rules: []model.Rule{{
			// MaxAllowed computes to 50, so this assertion reads the computed
			// value numerically and fails against the floor of 60.
			RuleName:     "limitMeetsFloor",
			ErrorMessage: "Advance limit of ${MaxAllowed} is below the required floor of 60.",
			Expression:   &model.Expression{Field: "MaxAllowed", Operator: model.OpGte, Value: 60},
		}},
	}
	ctx := assertCtx(map[string]interface{}{"EarnedWages": 100.0})

	res, _ := (&AssertStrategy{}).Evaluate(seg, ctx)
	if res.Expressions["MaxAllowed"] != 50.0 {
		t.Fatalf("computed field missing: %v", res.Expressions)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("expected the floor assertion to fail, got %v", failureNames(res))
	}
	if !strings.Contains(res.Failures[0].Message, "50") {
		t.Errorf("computed value should interpolate into the message, got %q", res.Failures[0].Message)
	}

	// And it passes when the computed value clears the floor.
	ctx = assertCtx(map[string]interface{}{"EarnedWages": 200.0})
	if res, _ := (&AssertStrategy{}).Evaluate(seg, ctx); res.Status != model.StatusSatisfied {
		t.Errorf("expected satisfied when MaxAllowed is 100, got %q %v", res.Status, failureNames(res))
	}
}

// Regression: ExpressionStrategy used to rebuild EvalContext field by field and
// drop Lookups, so in_lookup silently evaluated false inside expression-bearing
// segments while still passing config validation.
func TestAssert_LookupsReachRules(t *testing.T) {
	lookups := map[string]model.LookupTable{
		"frequencies": {
			ID:      "frequencies",
			KeyType: model.FieldTypeString,
			Entries: []model.LookupEntry{{Key: "biweekly"}, {Key: "semimonthly"}},
		},
	}
	seg := &model.Segment{
		ID:          "frequency",
		Strategy:    model.StrategyAssert,
		Expressions: []model.ExpressionDef{{Name: "Unused", Type: model.FieldTypeNumber, Expression: "1"}},
		Rules: []model.Rule{{
			RuleName:     "frequencySupported",
			ErrorMessage: "unsupported",
			Expression:   &model.Expression{Field: "payFrequency", Operator: model.OpInLookup, Value: "frequencies"},
		}},
	}

	ctx := assertCtx(map[string]interface{}{"payFrequency": "biweekly"})
	ctx.Lookups = lookups

	res, _ := (&AssertStrategy{}).Evaluate(seg, ctx)
	if res.Status != model.StatusSatisfied {
		t.Errorf("lookup table did not reach the rule: %q %v", res.Status, failureNames(res))
	}
}

// A computed field that fails at runtime makes the gate unevaluable. Falling
// through would report the rules that consumed it as violations, telling the
// resolver a value is wrong when it could not in fact be computed.
func TestAssert_ExpressionRuntimeFailureIsUnevaluable(t *testing.T) {
	seg := &model.Segment{
		ID:       "limits",
		Strategy: model.StrategyAssert,
		Expressions: []model.ExpressionDef{
			{Name: "Ratio", Type: model.FieldTypeNumber, Expression: "Missing + 1"},
		},
		Rules: []model.Rule{{
			RuleName:     "ratioWithinBounds",
			ErrorMessage: "ratio too high",
			Expression:   &model.Expression{Field: "Ratio", Operator: model.OpLte, Value: 1},
		}},
	}

	res, _ := (&AssertStrategy{}).Evaluate(seg, assertCtx(map[string]interface{}{}))
	if res.Status != model.StatusUnevaluable {
		t.Errorf("expected unevaluable, got %q", res.Status)
	}
	if len(res.Failures) != 0 {
		t.Errorf("an unevaluable gate must not report violations, got %v", failureNames(res))
	}
}

// A failure's message is its payload, so it is always populated — including for
// a rule that carries only localized messages and a caller that asked for no
// particular language. Previously such a failure came back with no text at all.
func TestAssert_MessageAlwaysPopulated(t *testing.T) {
	seg := &model.Segment{
		ID:       "gate",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{
			{
				RuleName:   "localizedOnly",
				Messages:   map[string]string{"en": "Localized only.", "es": "Solo localizado."},
				Expression: &model.Expression{Field: "ein", Operator: model.OpNeq, Value: ""},
			},
			{
				RuleName:     "plainWins",
				ErrorMessage: "Plain wins.",
				Messages:     map[string]string{"en": "Localized loses."},
				Expression:   &model.Expression{Field: "ein", Operator: model.OpNeq, Value: ""},
			},
		},
	}

	// No languages requested: the localized map is not rendered, but message is.
	res, _ := (&AssertStrategy{}).Evaluate(seg, assertCtx(map[string]interface{}{"ein": ""}))
	if len(res.Failures) != 2 {
		t.Fatalf("expected both assertions to fail, got %v", failureNames(res))
	}
	if res.Failures[0].Message != "Localized only." {
		t.Errorf("expected the default-language fallback, got %q", res.Failures[0].Message)
	}
	if res.Failures[0].Messages != nil {
		t.Errorf("localized rendering stays opt-in, got %v", res.Failures[0].Messages)
	}
	if res.Failures[1].Message != "Plain wins." {
		t.Errorf("errorMessage should win over the localized map, got %q", res.Failures[1].Message)
	}
}

// The fallback honours the layer's default language rather than assuming English.
func TestAssert_MessageFallbackUsesLayerDefaultLanguage(t *testing.T) {
	seg := &model.Segment{
		ID:       "gate",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{{
			RuleName:   "localizedOnly",
			Messages:   map[string]string{"en": "English.", "es": "Español."},
			Expression: &model.Expression{Field: "ein", Operator: model.OpNeq, Value: ""},
		}},
	}

	ctx := assertCtx(map[string]interface{}{"ein": ""})
	ctx.DefaultLanguage = "es"

	res, _ := (&AssertStrategy{}).Evaluate(seg, ctx)
	if res.Failures[0].Message != "Español." {
		t.Errorf("expected the layer default language, got %q", res.Failures[0].Message)
	}
}

// A rule with no message at all still reports, identified by its rule name.
func TestAssert_FailureWithoutAnyMessage(t *testing.T) {
	seg := &model.Segment{
		ID:       "gate",
		Strategy: model.StrategyAssert,
		Rules: []model.Rule{{
			RuleName:   "noText",
			Expression: &model.Expression{Field: "ein", Operator: model.OpNeq, Value: ""},
		}},
	}

	res, _ := (&AssertStrategy{}).Evaluate(seg, assertCtx(map[string]interface{}{"ein": ""}))
	if len(res.Failures) != 1 || res.Failures[0].Rule != "noText" {
		t.Fatalf("expected the failure to still be reported, got %v", failureNames(res))
	}
	if res.Failures[0].Message != "" {
		t.Errorf("nothing to fall back to, expected empty, got %q", res.Failures[0].Message)
	}
}

// Regression guard on the segmentation hot path: without collection, rule
// evaluation still stops at the first match and reports no failures.
func TestRuleStrategy_StillShortCircuits(t *testing.T) {
	seg := &model.Segment{
		ID:       "tier",
		Strategy: model.StrategyRule,
		Rules: []model.Rule{
			leaf("first", "country", "US", ""),
			leaf("second", "country", "CA", ""),
		},
		Default: "other",
	}
	seg.Rules[0].SuccessEvent = "us-tier"

	res, ok := (&RuleStrategy{}).Evaluate(seg, assertCtx(map[string]interface{}{"country": "US"}))
	if !ok || res.Segment != "us-tier" {
		t.Fatalf("expected first-match-wins, got %q (ok=%v)", res.Segment, ok)
	}
	if len(res.Failures) != 0 {
		t.Errorf("default mode must not collect failures, got %v", failureNames(res))
	}
	if res.Status != "" {
		t.Errorf("default mode must not set an assert status, got %q", res.Status)
	}
}
