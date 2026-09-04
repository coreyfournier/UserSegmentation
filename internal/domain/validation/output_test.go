package validation

import (
	"strings"
	"testing"

	"github.com/expr-lang/expr"
	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func snapWithOutputField(f model.OutputField) *model.Snapshot {
	return &model.Snapshot{
		Layers: []model.Layer{{
			Name: "diagnostics",
			Segments: []model.Segment{{
				ID:           "employee",
				Strategy:     model.StrategyChecklist,
				OutputSchema: model.OutputSchema{"field": f},
				InputSchema:  model.InputSchema{"x": {Type: model.FieldTypeString}},
				Rules: []model.Rule{{
					RuleName:  "someCheck",
					Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
				}},
			}},
		}},
	}
}

func TestValidate_OutputLookupMustExist(t *testing.T) {
	err := ValidateSnapshot(snapWithOutputField(model.OutputField{
		Type:   model.FieldTypeString,
		Lookup: "no-such-table",
	}))
	if err == nil || !strings.Contains(err.Error(), "no-such-table") {
		t.Fatalf("expected an unknown-lookup error, got %v", err)
	}
}

func TestValidate_ObjectTypeRequiresExpressionMode(t *testing.T) {
	err := ValidateSnapshot(snapWithOutputField(model.OutputField{
		Type: model.FieldTypeObject,
	}))
	if err == nil || !strings.Contains(err.Error(), "object") {
		t.Fatalf("expected an object-type error, got %v", err)
	}

	if err := ValidateSnapshot(snapWithOutputField(model.OutputField{
		Type: model.FieldTypeObject,
		Eval: model.EvalExpression,
	})); err != nil {
		t.Fatalf("object with expression mode should be valid, got %v", err)
	}
}

func TestValidate_RequiredOutputMustBeAuthored(t *testing.T) {
	required := model.OutputField{Type: model.FieldTypeString, Required: true}

	// The rule in the fixture authors nothing, so a required field is an error.
	err := ValidateSnapshot(snapWithOutputField(required))
	if err == nil || !strings.Contains(err.Error(), "someCheck") {
		t.Fatalf("expected the unauthored rule to be named, got %v", err)
	}

	// A value on the rule satisfies it.
	snap := snapWithOutputField(required)
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "x"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("rule-level value should satisfy, got %v", err)
	}

	// So does one segment-level value, for every rule at once.
	snap = snapWithOutputField(required)
	snap.Layers[0].Segments[0].Outputs = map[string]string{"field": "x"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("segment-level value should satisfy, got %v", err)
	}

	// A disabled rule is exempt: a work-in-progress item must not wedge a save.
	snap = snapWithOutputField(required)
	disabled := false
	snap.Layers[0].Segments[0].Rules[0].Enabled = &disabled
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("disabled rule should be exempt, got %v", err)
	}

	// An optional field is never required to be authored.
	if err := ValidateSnapshot(snapWithOutputField(model.OutputField{
		Type: model.FieldTypeString,
	})); err != nil {
		t.Fatalf("optional field should not be gated, got %v", err)
	}
}

// An enabled override on a rule segment carries the same required-output
// obligation as a top-level rule: it can fire and replace the strategy result
// entirely, so an unauthored required field is a load-time error just as it
// would be for a rule.
func TestValidate_RequiredOutputMustBeAuthoredOnOverride(t *testing.T) {
	required := model.OutputField{Type: model.FieldTypeString, Required: true}

	snap := snapWithOutputField(required)
	seg := &snap.Layers[0].Segments[0]
	seg.Strategy = model.StrategyRule
	seg.Rules[0].Outputs = map[string]string{"field": "x"}
	seg.Overrides = []model.Rule{{
		RuleName:  "vipBypass",
		Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
	}}

	// The override authors nothing, so it is an error even though the rule
	// itself satisfies the field.
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "vipBypass") {
		t.Fatalf("expected the unauthored override to be named, got %v", err)
	}

	// A value on the override satisfies it.
	snap.Layers[0].Segments[0].Overrides[0].Outputs = map[string]string{"field": "x"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("override-level value should satisfy, got %v", err)
	}

	// A disabled override is exempt, exactly like a disabled rule.
	snap = snapWithOutputField(required)
	seg = &snap.Layers[0].Segments[0]
	seg.Strategy = model.StrategyRule
	seg.Rules[0].Outputs = map[string]string{"field": "x"}
	disabled := false
	seg.Overrides = []model.Rule{{
		RuleName:  "vipBypass",
		Enabled:   &disabled,
		Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
	}}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("disabled override should be exempt, got %v", err)
	}

	// A segment-level value covers the override too, for the same reason it
	// covers every rule at once.
	snap = snapWithOutputField(required)
	seg = &snap.Layers[0].Segments[0]
	seg.Strategy = model.StrategyRule
	seg.Rules[0].Outputs = map[string]string{"field": "x"}
	seg.Outputs = map[string]string{"field": "x"}
	seg.Overrides = []model.Rule{{
		RuleName:  "vipBypass",
		Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
	}}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("segment-level value should satisfy the override too, got %v", err)
	}
}

func TestValidate_RequiredOutputWithDefaultNeedsSegmentValue(t *testing.T) {
	// The default branch reads no rule values, so a rule-level value cannot
	// cover it — only a segment-level one can.
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeString, Required: true})
	seg := &snap.Layers[0].Segments[0]
	seg.Strategy = model.StrategyRule
	seg.Default = "fallback"
	seg.Rules[0].Outputs = map[string]string{"field": "x"}

	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "default") {
		t.Fatalf("expected a default-path error, got %v", err)
	}

	seg.Outputs = map[string]string{"field": "x"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("segment-level value should satisfy the default path, got %v", err)
	}
}

func TestValidate_ChecklistCannotDeclareOverrides(t *testing.T) {
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeString})
	seg := &snap.Layers[0].Segments[0] // the fixture's strategy is checklist
	seg.Overrides = []model.Rule{{
		RuleName:  "vipBypass",
		Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
	}}

	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "override") {
		t.Fatalf("expected an override-on-checklist error, got %v", err)
	}

	// The same overrides on a rule segment are legitimate.
	seg.Strategy = model.StrategyRule
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("overrides on a rule segment should be valid, got %v", err)
	}
}

func TestExprCompile_AcceptsRegisteredMathFunctions(t *testing.T) {
	if _, err := expr.Compile("pow(2, 3)"); err != nil {
		t.Fatalf("bare Compile rejects a registered math function: %v", err)
	}
}

func TestValidate_OutputExpressionSyntaxIsChecked(t *testing.T) {
	exprField := model.OutputField{Type: model.FieldTypeString, Eval: model.EvalExpression}

	// A broken expression authored on the rule is caught and the field is named.
	snap := snapWithOutputField(exprField)
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "amount *"}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "field") {
		t.Fatalf("expected a rule-level expression syntax error naming the field, got %v", err)
	}

	// The same broken expression authored on the segment is also caught.
	snap = snapWithOutputField(exprField)
	snap.Layers[0].Segments[0].Outputs = map[string]string{"field": "amount *"}
	err = ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "field") {
		t.Fatalf("expected a segment-level expression syntax error naming the field, got %v", err)
	}

	// A valid expression, in either place, produces no error.
	snap = snapWithOutputField(exprField)
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "amount * 2"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("valid rule-level expression should not error, got %v", err)
	}

	snap = snapWithOutputField(exprField)
	snap.Layers[0].Segments[0].Outputs = map[string]string{"field": "amount * 2"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("valid segment-level expression should not error, got %v", err)
	}

	// A literal-mode field (Eval left empty) is never compiled, so the same
	// broken text is not a syntax error there.
	literalField := model.OutputField{Type: model.FieldTypeString}
	snap = snapWithOutputField(literalField)
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "amount *"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("literal-mode field should not be syntax-checked, got %v", err)
	}
}

func TestValidate_DisabledRuleExpressionSyntaxIsExempt(t *testing.T) {
	// A disabled rule's broken expression must not wedge the save — it is
	// only reported once the rule is re-enabled, matching requiredOutputErrors.
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeString, Eval: model.EvalExpression})
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "amount *"}
	disabled := false
	snap.Layers[0].Segments[0].Rules[0].Enabled = &disabled

	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("disabled rule's broken expression should be exempt, got %v", err)
	}
}

func TestValidate_ChecklistDefaultIsNotGated(t *testing.T) {
	// The fixture's strategy is checklist, which delegates to RuleStrategy but
	// returns before the default branch runs — a stray Default is inert there,
	// so it must not force a segment-level output value.
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeString, Required: true})
	seg := &snap.Layers[0].Segments[0]
	seg.Default = "fallback"
	seg.Rules[0].Outputs = map[string]string{"field": "x"}

	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("a checklist's stray default should not be gated, got %v", err)
	}
}

// Finding 1: the load-time gate must test the same thing the runtime does —
// non-emptiness, not mere key presence. A bare empty string authored at
// segment level used to satisfy the early return in requiredOutputErrors
// (`_, ok := seg.Outputs[name]`) and silently exempt every rule in the
// segment, while evaluateOutputs (strategy/output.go) treats that same value
// as unauthored (`!ok || raw == ""`). The two checks must agree.
func TestValidate_RequiredOutputEmptySegmentValueDoesNotSatisfy(t *testing.T) {
	required := model.OutputField{Type: model.FieldTypeString, Required: true}

	snap := snapWithOutputField(required)
	seg := &snap.Layers[0].Segments[0]
	seg.Strategy = model.StrategyRule
	seg.Outputs = map[string]string{"field": ""}

	err := ValidateSnapshot(snap)
	if err == nil {
		t.Fatal("expected an empty segment-level value to NOT satisfy a required field")
	}
	if !strings.Contains(err.Error(), "someCheck") {
		t.Fatalf("expected the unauthored rule to still be named, got %v", err)
	}
}

// Finding 2: override expression values must be syntax-checked at load, just
// like segment-level and rule-level values — validateOutputExpressionSyntax
// used to walk only seg.Outputs and seg.Rules, leaving an override's broken
// expression to fail at every evaluation instead of at load.
func TestValidate_OverrideOutputExpressionSyntaxIsChecked(t *testing.T) {
	exprField := model.OutputField{Type: model.FieldTypeString, Eval: model.EvalExpression}

	snap := snapWithOutputField(exprField)
	seg := &snap.Layers[0].Segments[0]
	seg.Strategy = model.StrategyRule
	seg.Overrides = []model.Rule{{
		RuleName:  "vipBypass",
		Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
		Outputs:   map[string]string{"field": "amount *"}, // unparseable
	}}

	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "vipBypass") || !strings.Contains(err.Error(), "field") {
		t.Fatalf("expected an override-level expression syntax error naming the override and field, got %v", err)
	}

	// A valid expression on the override produces no error.
	snap = snapWithOutputField(exprField)
	seg = &snap.Layers[0].Segments[0]
	seg.Strategy = model.StrategyRule
	seg.Overrides = []model.Rule{{
		RuleName:  "vipBypass",
		Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
		Outputs:   map[string]string{"field": "amount * 2"},
	}}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("valid override-level expression should not error, got %v", err)
	}

	// A disabled override's broken expression is exempt, matching the rule
	// and segment-level exemptions.
	snap = snapWithOutputField(exprField)
	seg = &snap.Layers[0].Segments[0]
	seg.Strategy = model.StrategyRule
	disabled := false
	seg.Overrides = []model.Rule{{
		RuleName:  "vipBypass",
		Enabled:   &disabled,
		Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
		Outputs:   map[string]string{"field": "amount *"},
	}}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("disabled override's broken expression should be exempt, got %v", err)
	}
}

// Finding 3 (load half): Required is unsatisfiable on static/percentage
// segments — neither strategy ever populates Result.Outputs — so config
// validation must not reject them for an unauthored required field.
func TestValidate_RequiredOutputExemptOnStaticAndPercentage(t *testing.T) {
	required := model.OutputField{Type: model.FieldTypeString, Required: true}

	for _, strat := range []string{model.StrategyStatic, model.StrategyPercentage} {
		snap := &model.Snapshot{
			Layers: []model.Layer{{
				Name: "tier",
				Segments: []model.Segment{{
					ID:           "seg",
					Strategy:     strat,
					OutputSchema: model.OutputSchema{"field": required},
				}},
			}},
		}
		if err := ValidateSnapshot(snap); err != nil {
			t.Fatalf("strategy %q: expected no error for an unauthored required field, got %v", strat, err)
		}
	}
}

// Finding 3 (runtime half): CheckRequiredOutputs must not warn on a static or
// percentage segment's own assignment, for the same reason the load-time gate
// is exempt — neither strategy ever populates Outputs, so the warning would
// fire on every single evaluation with no way to silence it.
func TestCheckRequiredOutputs_ExemptOnStaticAndPercentageAssignment(t *testing.T) {
	for _, strat := range []string{model.StrategyStatic, model.StrategyPercentage} {
		seg := &model.Segment{
			ID:       "seg",
			Strategy: strat,
			OutputSchema: model.OutputSchema{
				"category": model.OutputField{Type: model.FieldTypeString, Required: true},
			},
		}
		a := &model.Assignment{Segment: "whatever", Strategy: strat} // Outputs deliberately absent

		if got := CheckRequiredOutputs(seg, a, nil); len(got) != 0 {
			t.Fatalf("strategy %q: expected no warnings, got %v", strat, got)
		}
	}
}

// Finding 3 (runtime half, the exception): an override that fires on a
// static/percentage segment DOES resolve declared outputs (EvalOverrides
// calls evaluateOutputs regardless of the segment's own strategy), so it
// carries the same required-output obligation as any other override — the
// exemption above must not swallow this case.
func TestCheckRequiredOutputs_OverrideOnStaticSegmentStillChecked(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyStatic,
		OutputSchema: model.OutputSchema{
			"category": model.OutputField{Type: model.FieldTypeString, Required: true},
		},
	}
	a := &model.Assignment{Segment: "vip", Strategy: "override"} // Outputs absent: the override's value failed to resolve

	got := CheckRequiredOutputs(seg, a, nil)
	if len(got) != 1 {
		t.Fatalf("expected one warning for the missing required field on the override, got %v", got)
	}
	if got[0].Field != "category" {
		t.Errorf("expected the warning to name %q, got %q", "category", got[0].Field)
	}
}
