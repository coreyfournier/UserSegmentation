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
