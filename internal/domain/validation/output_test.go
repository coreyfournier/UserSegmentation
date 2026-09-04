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
