package strategy

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func checkCtx(ctx map[string]interface{}) *EvalContext {
	return &EvalContext{
		SubjectKey:      "company-1",
		Context:         ctx,
		DefaultLanguage: "en",
	}
}

// check builds a checklist item: a condition describing a problem, plus the
// message reported when it holds.
func check(name, field string, op model.Operator, value interface{}, msg string) model.Rule {
	return model.Rule{
		RuleName:     name,
		ErrorMessage: msg,
		Expression:   &model.Expression{Field: field, Operator: op, Value: value},
	}
}

func missing(name, field, msg string) model.Rule {
	return model.Rule{
		RuleName:     name,
		ErrorMessage: msg,
		Expression:   &model.Expression{Field: field, Operator: model.OpIsNullOrEmpty},
	}
}

func failureNames(res Result) []string {
	out := make([]string, len(res.Failures))
	for i, f := range res.Failures {
		out[i] = f.Rule
	}
	return out
}

func run(seg *model.Segment, ctx map[string]interface{}) Result {
	res, _ := (&ChecklistStrategy{}).Evaluate(seg, checkCtx(ctx))
	return res
}

func companyChecklist() *model.Segment {
	return &model.Segment{
		ID:       "company",
		Strategy: model.StrategyChecklist,
		Rules: []model.Rule{
			missing("companyMissingFederalEIN", "ein", "Federal EIN is required."),
			missing("companyMissingLegalName", "legalName", "Legal name is required."),
		},
	}
}

// Nothing wrong means nothing reported.
func TestChecklist_Satisfied(t *testing.T) {
	res := run(companyChecklist(), map[string]interface{}{"ein": "12-3456789", "legalName": "Acme"})
	if res.Status != model.StatusSatisfied {
		t.Errorf("expected satisfied, got %q with %v", res.Status, failureNames(res))
	}
	if len(res.Failures) != 0 {
		t.Errorf("expected no failures, got %v", failureNames(res))
	}
}

// The point of a checklist: every problem is reported in one pass, so the
// person fixing them does not make repeated round trips.
func TestChecklist_ReportsEveryProblemAtOnce(t *testing.T) {
	res := run(companyChecklist(), map[string]interface{}{})
	if res.Status != model.StatusViolated {
		t.Fatalf("expected violated, got %q", res.Status)
	}
	got := failureNames(res)
	if len(got) != 2 || got[0] != "companyMissingFederalEIN" || got[1] != "companyMissingLegalName" {
		t.Errorf("expected both problems, got %v", got)
	}
	if res.Failures[0].Message != "Federal EIN is required." {
		t.Errorf("message not carried: %q", res.Failures[0].Message)
	}
}

// A rule fires on a match here exactly as it does under first-match evaluation.
// The same expression must not mean opposite things across strategies.
func TestChecklist_FiresOnMatchLikeRuleStrategy(t *testing.T) {
	expr := &model.Expression{Field: "ein", Operator: model.OpIsNullOrEmpty}
	ctx := map[string]interface{}{"ein": ""}

	// Under first-match the rule matches and wins.
	ruleSeg := &model.Segment{
		ID:       "s",
		Strategy: model.StrategyRule,
		Rules:    []model.Rule{{RuleName: "einEmpty", SuccessEvent: "matched", Expression: expr}},
	}
	first, ok := (&RuleStrategy{}).Evaluate(ruleSeg, checkCtx(ctx))
	if !ok || first.Segment != "matched" {
		t.Fatalf("first-match: expected the rule to match, got %q", first.Segment)
	}

	// The identical rule under a checklist reports, rather than resolving.
	listSeg := &model.Segment{
		ID:       "s",
		Strategy: model.StrategyChecklist,
		Rules:    []model.Rule{{RuleName: "einEmpty", ErrorMessage: "empty", Expression: expr}},
	}
	if got := failureNames(run(listSeg, ctx)); len(got) != 1 || got[0] != "einEmpty" {
		t.Errorf("checklist: expected the same rule to fire, got %v", got)
	}
}

// A group is one item: And/Or build its condition, and the whole item reports
// once with its own message.
func TestChecklist_AndGroupIsOneItem(t *testing.T) {
	seg := &model.Segment{
		ID:       "employee",
		Strategy: model.StrategyChecklist,
		Rules: []model.Rule{{
			// "at least one contact method" inverts to "all of them absent".
			RuleName:     "employeeMissingAllContactMethods",
			Operator:     model.CompositeAnd,
			ErrorMessage: "Provide at least one contact method: email or phone.",
			Rules: []model.Rule{
				missing("contactEmailAbsent", "email", ""),
				missing("contactPhoneAbsent", "phone", ""),
			},
		}},
	}

	both := run(seg, map[string]interface{}{})
	got := failureNames(both)
	if len(got) != 1 || got[0] != "employeeMissingAllContactMethods" {
		t.Fatalf("expected one item for the group, got %v", got)
	}
	if !strings.Contains(both.Failures[0].Message, "at least one") {
		t.Errorf("expected the group's own message, got %q", both.Failures[0].Message)
	}

	// One contact method present means the condition no longer holds.
	if res := run(seg, map[string]interface{}{"phone": "555"}); res.Status != model.StatusSatisfied {
		t.Errorf("expected satisfied with a phone on file, got %q %v", res.Status, failureNames(res))
	}
}

func TestChecklist_OrGroupIsOneItem(t *testing.T) {
	seg := &model.Segment{
		ID:       "payroll",
		Strategy: model.StrategyChecklist,
		Rules: []model.Rule{{
			RuleName:     "precisionMissingDefaultPayRate",
			Operator:     model.CompositeOr,
			ErrorMessage: "A default pay rate is required.",
			Rules: []model.Rule{
				check("rateAbsent", "rate", model.OpIsNull, nil, ""),
				check("rateNotPositive", "rate", model.OpLte, 0, ""),
			},
		}},
	}

	for _, tc := range []struct {
		name string
		ctx  map[string]interface{}
		want model.LayerStatus
	}{
		{"absent", map[string]interface{}{}, model.StatusViolated},
		{"zero", map[string]interface{}{"rate": 0.0}, model.StatusViolated},
		{"negative", map[string]interface{}{"rate": -1.0}, model.StatusViolated},
		{"positive", map[string]interface{}{"rate": 25.0}, model.StatusSatisfied},
	} {
		res := run(seg, tc.ctx)
		if res.Status != tc.want {
			t.Errorf("%s: expected %q, got %q %v", tc.name, tc.want, res.Status, failureNames(res))
		}
		if res.Status == model.StatusViolated && len(res.Failures) != 1 {
			t.Errorf("%s: a group reports once, got %v", tc.name, failureNames(res))
		}
	}
}

func TestChecklist_DisabledRuleIgnored(t *testing.T) {
	off := false
	seg := &model.Segment{
		ID:       "c",
		Strategy: model.StrategyChecklist,
		Rules: []model.Rule{{
			RuleName:   "retired",
			Enabled:    &off,
			Expression: &model.Expression{Field: "anything", Operator: model.OpIsNull},
		}},
	}
	if res := run(seg, map[string]interface{}{}); res.Status != model.StatusSatisfied {
		t.Errorf("a disabled check must not fire, got %q", res.Status)
	}
}

// Checklist composes ExpressionStrategy, so computed fields are available to
// the conditions and to their messages.
func TestChecklist_InheritsComputedFields(t *testing.T) {
	seg := &model.Segment{
		ID:       "limits",
		Strategy: model.StrategyChecklist,
		Expressions: []model.ExpressionDef{
			{Name: "MaxAllowed", Type: model.FieldTypeNumber, Expression: "EarnedWages * 0.5"},
		},
		Rules: []model.Rule{{
			RuleName:     "advanceLimitBelowFloor",
			ErrorMessage: "Advance limit of ${MaxAllowed} is below the required floor of 60.",
			Expression:   &model.Expression{Field: "MaxAllowed", Operator: model.OpLt, Value: 60},
		}},
	}

	res := run(seg, map[string]interface{}{"EarnedWages": 100.0})
	if res.Expressions["MaxAllowed"] != 50.0 {
		t.Fatalf("computed field missing: %v", res.Expressions)
	}
	if len(res.Failures) != 1 {
		t.Fatalf("expected the floor check to fire, got %v", failureNames(res))
	}
	if !strings.Contains(res.Failures[0].Message, "50") {
		t.Errorf("computed value should interpolate, got %q", res.Failures[0].Message)
	}

	if res := run(seg, map[string]interface{}{"EarnedWages": 200.0}); res.Status != model.StatusSatisfied {
		t.Errorf("expected satisfied when the limit clears the floor, got %q", res.Status)
	}
}

// Regression: ExpressionStrategy used to rebuild EvalContext field by field and
// drop Lookups, so lookup operators silently evaluated false.
func TestChecklist_LookupsReachRules(t *testing.T) {
	ctx := checkCtx(map[string]interface{}{"payFrequency": "weekly"})
	ctx.Lookups = map[string]model.LookupTable{
		"frequencies": {
			ID:      "frequencies",
			KeyType: model.FieldTypeString,
			Entries: []model.LookupEntry{{Key: "biweekly"}, {Key: "semimonthly"}},
		},
	}
	seg := &model.Segment{
		ID:          "frequency",
		Strategy:    model.StrategyChecklist,
		Expressions: []model.ExpressionDef{{Name: "Unused", Type: model.FieldTypeNumber, Expression: "1"}},
		Rules: []model.Rule{{
			RuleName:     "frequencyUnsupported",
			ErrorMessage: "unsupported",
			Expression:   &model.Expression{Field: "payFrequency", Operator: model.OpNotInLookup, Value: "frequencies"},
		}},
	}

	res, _ := (&ChecklistStrategy{}).Evaluate(seg, ctx)
	if got := failureNames(res); len(got) != 1 {
		t.Errorf("lookup table did not reach the rule: %q %v", res.Status, got)
	}
}

// A computed field that fails at runtime makes the list unevaluable. Falling
// through would report checks that consumed it as real problems.
func TestChecklist_ExpressionRuntimeFailureIsUnevaluable(t *testing.T) {
	seg := &model.Segment{
		ID:       "limits",
		Strategy: model.StrategyChecklist,
		Expressions: []model.ExpressionDef{
			{Name: "Ratio", Type: model.FieldTypeNumber, Expression: "Missing + 1"},
		},
		Rules: []model.Rule{{
			RuleName:     "ratioTooHigh",
			ErrorMessage: "ratio too high",
			Expression:   &model.Expression{Field: "Ratio", Operator: model.OpGt, Value: 1},
		}},
	}

	res := run(seg, map[string]interface{}{})
	if res.Status != model.StatusUnevaluable {
		t.Errorf("expected unevaluable, got %q", res.Status)
	}
	if len(res.Failures) != 0 {
		t.Errorf("an unevaluable list must report nothing, got %v", failureNames(res))
	}
}

// The message is a failure's payload, so it is always populated — including for
// a rule carrying only localized messages when no language was requested.
func TestChecklist_MessageAlwaysPopulated(t *testing.T) {
	seg := &model.Segment{
		ID:       "c",
		Strategy: model.StrategyChecklist,
		Rules: []model.Rule{
			{
				RuleName:   "localizedOnly",
				Messages:   map[string]string{"en": "Localized only.", "es": "Solo localizado."},
				Expression: &model.Expression{Field: "ein", Operator: model.OpIsNullOrEmpty},
			},
			{
				RuleName:     "plainWins",
				ErrorMessage: "Plain wins.",
				Messages:     map[string]string{"en": "Localized loses."},
				Expression:   &model.Expression{Field: "ein", Operator: model.OpIsNullOrEmpty},
			},
		},
	}

	res := run(seg, map[string]interface{}{"ein": ""})
	if len(res.Failures) != 2 {
		t.Fatalf("expected both checks to fire, got %v", failureNames(res))
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

func TestChecklist_MessageFallbackUsesLayerDefaultLanguage(t *testing.T) {
	seg := &model.Segment{
		ID:       "c",
		Strategy: model.StrategyChecklist,
		Rules: []model.Rule{{
			RuleName:   "localizedOnly",
			Messages:   map[string]string{"en": "English.", "es": "Español."},
			Expression: &model.Expression{Field: "ein", Operator: model.OpIsNullOrEmpty},
		}},
	}

	ctx := checkCtx(map[string]interface{}{"ein": ""})
	ctx.DefaultLanguage = "es"

	res, _ := (&ChecklistStrategy{}).Evaluate(seg, ctx)
	if res.Failures[0].Message != "Español." {
		t.Errorf("expected the layer default language, got %q", res.Failures[0].Message)
	}
}

func TestChecklist_FailureWithoutAnyMessage(t *testing.T) {
	seg := &model.Segment{
		ID:       "c",
		Strategy: model.StrategyChecklist,
		Rules: []model.Rule{{
			RuleName:   "noText",
			Expression: &model.Expression{Field: "ein", Operator: model.OpIsNullOrEmpty},
		}},
	}

	res := run(seg, map[string]interface{}{"ein": ""})
	if len(res.Failures) != 1 || res.Failures[0].Rule != "noText" {
		t.Fatalf("expected the failure to still be reported, got %v", failureNames(res))
	}
	if res.Failures[0].Message != "" {
		t.Errorf("nothing to fall back to, expected empty, got %q", res.Failures[0].Message)
	}
}

// A trap worth pinning down: a comparison has nothing to compare an absent
// field against, so it does not fire. When absence is also a problem, say so
// explicitly with a presence operator.
func TestChecklist_ComparisonsDoNotFireOnAbsentFields(t *testing.T) {
	comparison := &model.Segment{
		ID:       "c",
		Strategy: model.StrategyChecklist,
		Rules: []model.Rule{
			check("rateNotPositive", "rate", model.OpLte, 0, "rate must be positive"),
		},
	}
	if res := run(comparison, map[string]interface{}{}); res.Status != model.StatusSatisfied {
		t.Errorf("a comparison alone should not fire on an absent field, got %q", res.Status)
	}

	// Or the two conditions together, which is how the shipped config states it.
	withAbsence := &model.Segment{
		ID:       "c",
		Strategy: model.StrategyChecklist,
		Rules: []model.Rule{{
			RuleName:     "rateMissingOrNotPositive",
			Operator:     model.CompositeOr,
			ErrorMessage: "A positive rate is required.",
			Rules: []model.Rule{
				check("rateAbsent", "rate", model.OpIsNull, nil, ""),
				check("rateNotPositive", "rate", model.OpLte, 0, ""),
			},
		}},
	}
	if res := run(withAbsence, map[string]interface{}{}); res.Status != model.StatusViolated {
		t.Errorf("pairing with is_null should catch absence, got %q", res.Status)
	}
}

// Regression guard on the segmentation hot path: without collection, rule
// evaluation still stops at the first match and reports no failures.
func TestRuleStrategy_StillShortCircuits(t *testing.T) {
	seg := &model.Segment{
		ID:       "tier",
		Strategy: model.StrategyRule,
		Rules: []model.Rule{
			{RuleName: "first", SuccessEvent: "us-tier", Expression: &model.Expression{Field: "country", Operator: model.OpEq, Value: "US"}},
			{RuleName: "second", SuccessEvent: "ca-tier", Expression: &model.Expression{Field: "country", Operator: model.OpEq, Value: "CA"}},
		},
		Default: "other",
	}

	res, ok := (&RuleStrategy{}).Evaluate(seg, checkCtx(map[string]interface{}{"country": "US"}))
	if !ok || res.Segment != "us-tier" {
		t.Fatalf("expected first-match-wins, got %q (ok=%v)", res.Segment, ok)
	}
	if len(res.Failures) != 0 {
		t.Errorf("default mode must not collect failures, got %v", failureNames(res))
	}
	if res.Status != "" {
		t.Errorf("default mode must not set a checklist status, got %q", res.Status)
	}
}
