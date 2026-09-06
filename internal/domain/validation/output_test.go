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
			Key:          "diagnostics",
			OutputSchema: model.OutputSchema{"field": f},
			// "amount" is declared purely so the expression-syntax tests
			// below (which compile "amount * 2" etc.) exercise syntax, not
			// the load-time unknown-identifier check added alongside them —
			// see validateOutputExpressionSyntax.
			InputSchema: model.InputSchema{
				"x":      {Type: model.FieldTypeString},
				"amount": {Type: model.FieldTypeNumber},
			},
			Segments: []model.Segment{{
				ID:       "employee",
				Strategy: model.StrategyChecklist,
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

// A lookup-bound field's declared type must match the table's key type,
// mirroring validateLookupRef's own check for a rule condition.
func TestValidate_OutputLookupTypeMismatch(t *testing.T) {
	snap := snapWithOutputField(model.OutputField{
		Type:   model.FieldTypeString,
		Lookup: "vip-tiers",
	})
	snap.Lookups = []model.LookupTable{{
		ID: "vip-tiers", Name: "VIP Tiers", KeyType: model.FieldTypeNumber,
		Entries: []model.LookupEntry{{Key: 1.0}},
	}}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "does not match lookup") {
		t.Fatalf("expected a lookup type-mismatch error, got %v", err)
	}

	// Matching KeyType validates.
	snap = snapWithOutputField(model.OutputField{
		Type:   model.FieldTypeNumber,
		Lookup: "vip-tiers",
	})
	snap.Lookups = []model.LookupTable{{
		ID: "vip-tiers", Name: "VIP Tiers", KeyType: model.FieldTypeNumber,
		Entries: []model.LookupEntry{{Key: 1.0}},
	}}
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "1"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("matching lookup key type should validate, got %v", err)
	}
}

// An authored key absent from the schema can never do anything —
// evaluateOutputs iterates the schema, not what was authored — so it is
// rejected at load, naming both the key and the segment, at every authoring
// tier.
func TestValidate_UnknownOutputKeyIsRejected(t *testing.T) {
	// Segment-level typo.
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeString})
	snap.Layers[0].Segments[0].Outputs = map[string]string{"catgeory": "x"}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "catgeory") || !strings.Contains(err.Error(), "employee") {
		t.Fatalf("expected the unknown key and segment to be named, got %v", err)
	}

	// Rule-level typo.
	snap = snapWithOutputField(model.OutputField{Type: model.FieldTypeString})
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"catgeory": "x"}
	err = ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "catgeory") {
		t.Fatalf("expected the unknown rule-level key to be named, got %v", err)
	}

	// Override-level typo.
	snap = snapWithOutputField(model.OutputField{Type: model.FieldTypeString})
	seg := &snap.Layers[0].Segments[0]
	seg.Strategy = model.StrategyRule
	seg.Overrides = []model.Rule{{
		RuleName:  "vipBypass",
		Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
		Outputs:   map[string]string{"catgeory": "x"},
	}}
	err = ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "catgeory") || !strings.Contains(err.Error(), "vipBypass") {
		t.Fatalf("expected the unknown override-level key and override name, got %v", err)
	}

	// A disabled rule's typo is exempt, matching every other authoring check.
	snap = snapWithOutputField(model.OutputField{Type: model.FieldTypeString})
	disabled := false
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"catgeory": "x"}
	snap.Layers[0].Segments[0].Rules[0].Enabled = &disabled
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("disabled rule's unknown key should be exempt, got %v", err)
	}

	// A correctly-named key never errors.
	snap = snapWithOutputField(model.OutputField{Type: model.FieldTypeString})
	snap.Layers[0].Segments[0].Outputs = map[string]string{"field": "x"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("a declared key should not error, got %v", err)
	}
}

// static and percentage are exempt from every check this task adds — name and
// lookup-type — for the same reason they are exempt from Required: neither
// strategy ever populates Result.Outputs, so nothing they declare is binding.
// (Expression syntax checking is not among the checks these strategies are
// exempt from — it never was, even before eval mode was derived from type —
// so the authored value here must still be syntactically valid; it is
// deliberately not a real number to show that its *content* is unconstrained,
// just not its syntax.)
func TestValidate_OutputSchemaExemptOnStaticAndPercentage(t *testing.T) {
	for _, strat := range []string{model.StrategyStatic, model.StrategyPercentage} {
		snap := &model.Snapshot{
			Layers: []model.Layer{{
				Key: "tier",
				OutputSchema: model.OutputSchema{
					"field": model.OutputField{Type: model.FieldTypeNumber, Lookup: "vip-tiers"},
				},
				Segments: []model.Segment{{
					ID:       "seg",
					Strategy: strat,
					// An unknown key and a lookup type mismatch (field
					// declared number, table is string) — neither is
					// enforced for these strategies.
					Outputs: map[string]string{"field": "unparsed_value", "extraneous": "x"},
				}},
			}},
			Lookups: []model.LookupTable{{
				ID: "vip-tiers", Name: "VIP Tiers", KeyType: model.FieldTypeString,
				Entries: []model.LookupEntry{{Key: "gold"}},
			}},
		}
		if err := ValidateSnapshot(snap); err != nil {
			t.Fatalf("strategy %q: expected no error, got %v", strat, err)
		}
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
	// Any non-string type is expression mode now, so a number field exercises
	// the syntax check.
	exprField := model.OutputField{Type: model.FieldTypeNumber}

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

	// A string (template) field is never compiled, so the same broken text is
	// not a syntax error there.
	templateField := model.OutputField{Type: model.FieldTypeString}
	snap = snapWithOutputField(templateField)
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "amount *"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("template field should not be syntax-checked, got %v", err)
	}
}

func TestValidate_DisabledRuleExpressionSyntaxIsExempt(t *testing.T) {
	// A disabled rule's broken expression must not wedge the save — it is
	// only reported once the rule is re-enabled, matching requiredOutputErrors.
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeNumber})
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
	exprField := model.OutputField{Type: model.FieldTypeNumber}

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
				Key:          "tier",
				OutputSchema: model.OutputSchema{"field": required},
				Segments: []model.Segment{{
					ID:       "seg",
					Strategy: strat,
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
	schema := model.OutputSchema{
		"category": model.OutputField{Type: model.FieldTypeString, Required: true},
	}
	for _, strat := range []string{model.StrategyStatic, model.StrategyPercentage} {
		seg := &model.Segment{
			ID:       "seg",
			Strategy: strat,
		}
		a := &model.Assignment{Segment: "whatever", Strategy: strat} // Outputs deliberately absent

		if got := CheckRequiredOutputs(seg, schema, a, nil); len(got) != 0 {
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
	}
	schema := model.OutputSchema{
		"category": model.OutputField{Type: model.FieldTypeString, Required: true},
	}
	a := &model.Assignment{Segment: "vip", Strategy: "override"} // Outputs absent: the override's value failed to resolve

	got := CheckRequiredOutputs(seg, schema, a, nil)
	if len(got) != 1 {
		t.Fatalf("expected one warning for the missing required field on the override, got %v", got)
	}
	if got[0].Field != "category" {
		t.Errorf("expected the warning to name %q, got %q", "category", got[0].Field)
	}
}

// snapWithInputField mirrors snapWithOutputField for the input side: one layer
// whose schema declares a single bound field, with a segment present so the
// layer is not trivially empty.
func snapWithInputField(f model.SchemaField) *model.Snapshot {
	return &model.Snapshot{
		Layers: []model.Layer{{
			Key:         "diagnostics",
			InputSchema: model.InputSchema{"tier": f},
			Segments: []model.Segment{{
				ID:       "employee",
				Strategy: model.StrategyChecklist,
				Rules: []model.Rule{{
					RuleName:  "someCheck",
					Condition: &model.Condition{Field: "tier", Operator: model.OpIsNull},
				}},
			}},
		}},
	}
}

// An input field's lookup binding is checked exactly as an output field's is:
// the table has to exist.
func TestValidate_InputLookupMustExist(t *testing.T) {
	err := ValidateSnapshot(snapWithInputField(model.SchemaField{
		Type:   model.FieldTypeString,
		Lookup: "no-such-table",
	}))
	if err == nil || !strings.Contains(err.Error(), `input "tier": lookup "no-such-table" does not exist`) {
		t.Fatalf("expected an unknown-lookup error, got %v", err)
	}
}

func TestValidate_InputLookupTypeMismatch(t *testing.T) {
	snap := snapWithInputField(model.SchemaField{
		Type:   model.FieldTypeString,
		Lookup: "vip-tiers",
	})
	snap.Lookups = []model.LookupTable{{
		ID: "vip-tiers", Name: "VIP Tiers", KeyType: model.FieldTypeNumber,
		Entries: []model.LookupEntry{{Key: 1.0}},
	}}
	if err := ValidateSnapshot(snap); err == nil || !strings.Contains(err.Error(), `input "tier": field type "string" does not match lookup`) {
		t.Fatalf("expected a lookup type-mismatch error, got %v", err)
	}

	// Agreeing types validate. The condition is retyped alongside the field so
	// the operator check does not fail for an unrelated reason.
	snap.Layers[0].InputSchema["tier"] = model.SchemaField{Type: model.FieldTypeNumber, Lookup: "vip-tiers"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("expected a matching key type to validate, got %v", err)
	}
}

// An unbound field is the common case and must stay untouched by the new check.
func TestValidate_InputWithoutLookupIsUnaffected(t *testing.T) {
	if err := ValidateSnapshot(snapWithInputField(model.SchemaField{Type: model.FieldTypeString})); err != nil {
		t.Fatalf("expected an unbound input field to validate, got %v", err)
	}
}
