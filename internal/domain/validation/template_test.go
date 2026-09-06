package validation

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// ruleSnapshot builds a single-layer, single-segment snapshot with one
// top-level rule, for exercising template-token validation (errorMessage,
// messages) in isolation from everything else ValidateSnapshot checks.
func ruleSnapshot(schema model.InputSchema, computed []model.ComputedField, rule model.Rule) *model.Snapshot {
	return &model.Snapshot{
		Layers: []model.Layer{{
			Name:        "layer",
			InputSchema: schema,
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyRule,
				Computed: computed,
				Rules:    []model.Rule{rule},
			}},
		}},
	}
}

func expectValid(t *testing.T, snap *model.Snapshot) {
	t.Helper()
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("expected valid config, got: %v", err)
	}
}

// 1. A typoed token against a declared field is the silent typo this task
// closes: it must be rejected, naming both the rule and the unknown token.
func TestValidate_Template_TypoedTokenIsRejected(t *testing.T) {
	snap := ruleSnapshot(
		model.InputSchema{"name": {Type: model.FieldTypeString}},
		nil,
		model.Rule{
			RuleName:     "greet",
			Condition:    &model.Condition{Field: "name", Operator: model.OpEq, Value: "x"},
			ErrorMessage: "Hello ${nam}",
		},
	)
	err := ValidateSnapshot(snap)
	if err == nil {
		t.Fatal("expected an error for the typoed token")
	}
	if !strings.Contains(err.Error(), "greet") {
		t.Errorf("expected the rule to be named, got: %v", err)
	}
	if !strings.Contains(err.Error(), "nam") {
		t.Errorf("expected the unknown token to be named, got: %v", err)
	}
}

// 2. A dotted field is one flat schema key, not member access — this is the
// naming convention the config uses throughout (e.g. company.payFrequency),
// and it must validate exactly as ResolveField accepts it at runtime.
func TestValidate_Template_DottedFieldValidates(t *testing.T) {
	expectValid(t, ruleSnapshot(
		model.InputSchema{"company.payFrequency": {Type: model.FieldTypeString}},
		nil,
		model.Rule{
			RuleName:     "payFreq",
			Condition:    &model.Condition{Field: "company.payFrequency", Operator: model.OpEq, Value: "biweekly"},
			ErrorMessage: "Pay frequency ${company.payFrequency}",
		},
	))
}

// 3. A compound expression over a declared field still validates — only a
// literal flat-key match skips straight past expr; anything else compiles.
func TestValidate_Template_CompoundExpressionValidates(t *testing.T) {
	expectValid(t, ruleSnapshot(
		model.InputSchema{"totalHours": {Type: model.FieldTypeNumber}},
		nil,
		model.Rule{
			RuleName:     "hours",
			Condition:    &model.Condition{Field: "totalHours", Operator: model.OpGt, Value: 0},
			ErrorMessage: "${totalHours * 2}",
		},
	))
}

// 4. The false-rejection trap: an env-constrained compile rejects pow(2, 3)
// unless the runtime's own option set (strategy.ExprOptions) is passed too.
func TestValidate_Template_RegisteredMathFunctionValidates(t *testing.T) {
	expectValid(t, ruleSnapshot(
		model.InputSchema{"totalHours": {Type: model.FieldTypeNumber}},
		nil,
		model.Rule{
			RuleName:     "hours",
			Condition:    &model.Condition{Field: "totalHours", Operator: model.OpGt, Value: 0},
			ErrorMessage: "${pow(totalHours, 2)}",
		},
	))
}

// 5. A genuine syntax error is still rejected, declared field or not.
func TestValidate_Template_SyntaxErrorIsRejected(t *testing.T) {
	snap := ruleSnapshot(
		model.InputSchema{"amount": {Type: model.FieldTypeNumber}},
		nil,
		model.Rule{
			RuleName:     "amountCheck",
			Condition:    &model.Condition{Field: "amount", Operator: model.OpGt, Value: 0},
			ErrorMessage: "${amount *}",
		},
	)
	err := ValidateSnapshot(snap)
	if err == nil {
		t.Fatal("expected a syntax error")
	}
	if !strings.Contains(err.Error(), "amountCheck") {
		t.Errorf("expected the rule to be named, got: %v", err)
	}
}

// 6. A computed field is part of the effective schema, exactly like a rule
// condition reading one — a token naming it must validate too.
func TestValidate_Template_ComputedFieldTokenValidates(t *testing.T) {
	expectValid(t, ruleSnapshot(
		model.InputSchema{}, // non-nil: the layer does declare a (empty) schema
		[]model.ComputedField{{Name: "score", Type: model.FieldTypeNumber, Formula: "1 + 1"}},
		model.Rule{
			RuleName:     "scoreCheck",
			Condition:    &model.Condition{Field: "score", Operator: model.OpGt, Value: 0},
			ErrorMessage: "${score}",
		},
	))
}

// 7a. The same check applies to a string output field's authored value,
// wherever it is authored.
func TestValidate_Template_OutputFieldValueIsChecked(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name:         "layer",
			InputSchema:  model.InputSchema{"name": {Type: model.FieldTypeString}},
			OutputSchema: model.OutputSchema{"greeting": {Type: model.FieldTypeString}},
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyRule,
				Rules: []model.Rule{{
					RuleName:  "greet",
					Condition: &model.Condition{Field: "name", Operator: model.OpEq, Value: "x"},
					Outputs:   map[string]string{"greeting": "Hi ${nam}"},
				}},
			}},
		}},
	}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "greeting") || !strings.Contains(err.Error(), "nam") {
		t.Fatalf("expected the output field and unknown token to be named, got: %v", err)
	}

	// The correctly-spelled token validates.
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"greeting": "Hi ${name}"}
	expectValid(t, snap)
}

// 7b. The same check applies to a messages entry, per language.
func TestValidate_Template_MessagesEntryIsChecked(t *testing.T) {
	snap := ruleSnapshot(
		model.InputSchema{"name": {Type: model.FieldTypeString}},
		nil,
		model.Rule{
			RuleName:  "greet",
			Condition: &model.Condition{Field: "name", Operator: model.OpEq, Value: "x"},
			Messages:  map[string]string{"en": "Hello ${nam}"},
		},
	)
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "greet") || !strings.Contains(err.Error(), "nam") {
		t.Fatalf("expected the rule and unknown token to be named, got: %v", err)
	}

	// The correctly-spelled token, in the same place, validates.
	snap.Layers[0].Segments[0].Rules[0].Messages = map[string]string{"en": "Hello ${name}"}
	expectValid(t, snap)
}

// 7c. defaultMessages is checked too.
func TestValidate_Template_DefaultMessagesIsChecked(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name:        "layer",
			InputSchema: model.InputSchema{"name": {Type: model.FieldTypeString}},
			Segments: []model.Segment{{
				ID:              "seg",
				Strategy:        model.StrategyRule,
				Default:         "fallback",
				DefaultMessages: map[string]string{"en": "Hello ${nam}"},
			}},
		}},
	}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "nam") {
		t.Fatalf("expected the unknown token in defaultMessages to be named, got: %v", err)
	}

	snap.Layers[0].Segments[0].DefaultMessages = map[string]string{"en": "Hello ${name}"}
	expectValid(t, snap)
}

// 8. The escape hatch: a layer with no inputSchema has nothing to check a
// token against, so every token is accepted rather than every token being
// rejected. A Computed field is added so the rule tree is still walked (the
// segment does not hit the separate, total "nothing to validate" skip),
// proving the token check itself — not just the surrounding skip — treats a
// nil schema as "accept everything."
func TestValidate_Template_NoInputSchemaAcceptsEveryToken(t *testing.T) {
	expectValid(t, ruleSnapshot(
		nil, // no inputSchema at all
		[]model.ComputedField{{Name: "score", Type: model.FieldTypeNumber, Formula: "1 + 1"}},
		model.Rule{
			RuleName:     "scoreCheck",
			Condition:    &model.Condition{Field: "score", Operator: model.OpGt, Value: 0},
			ErrorMessage: "${totallyUndeclared}",
		},
	))
}

// 9. A token inside a nested And/Or rule's errorMessage is still checked —
// the walk must recurse rather than stop at the top-level rule.
func TestValidate_Template_NestedRuleErrorMessageIsChecked(t *testing.T) {
	snap := ruleSnapshot(
		model.InputSchema{"name": {Type: model.FieldTypeString}},
		nil,
		model.Rule{
			RuleName: "top",
			Operator: model.CompositeAnd,
			Rules: []model.Rule{
				{
					RuleName:  "innerOk",
					Condition: &model.Condition{Field: "name", Operator: model.OpEq, Value: "x"},
				},
				{
					RuleName:     "innerBad",
					Condition:    &model.Condition{Field: "name", Operator: model.OpEq, Value: "y"},
					ErrorMessage: "Hello ${nam}",
				},
			},
		},
	)
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "innerBad") || !strings.Contains(err.Error(), "nam") {
		t.Fatalf("expected the nested rule and unknown token to be named, got: %v", err)
	}
}

// Step 4: the expression check (non-string output fields) is upgraded to the
// same env-constrained compile, so a typo is caught at load instead of
// resolving to nil at runtime.
func TestValidate_OutputExpressionRejectsUnknownIdentifier(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name:         "layer",
			InputSchema:  model.InputSchema{"MaxAllowed": {Type: model.FieldTypeNumber}},
			OutputSchema: model.OutputSchema{"cap": {Type: model.FieldTypeNumber}},
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyRule,
				Rules: []model.Rule{{
					RuleName:  "capRule",
					Condition: &model.Condition{Field: "MaxAllowed", Operator: model.OpGt, Value: 0},
					Outputs:   map[string]string{"cap": "MaxAllowd"}, // typo: should be MaxAllowed
				}},
			}},
		}},
	}
	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "cap") {
		t.Fatalf("expected the output field to be named in an unknown-identifier error, got: %v", err)
	}

	// The correctly-spelled identifier validates.
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"cap": "MaxAllowed"}
	expectValid(t, snap)
}

// A cross-layer reference in a template must validate. The evaluator injects
// "layer:<name>" as a flat context key and ResolveField finds it before expr is
// consulted, so the runtime renders it — measured: "Your tier is pro." with no
// warnings. Rejecting it here on a colon parse error would block config that
// works, which is worse than the gap this validation closes.
func TestValidate_TemplateAcceptsLayerReference(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{
		{Name: "base-tier", InputSchema: model.InputSchema{"plan": {Type: model.FieldTypeString}},
			Segments: []model.Segment{{ID: "t", Strategy: model.StrategyRule,
				Rules: []model.Rule{{RuleName: "r", SuccessEvent: "pro",
					Condition: &model.Condition{Field: "plan", Operator: model.OpEq, Value: "pro"}}}}}},
		{Name: "promos", DependsOn: []string{"base-tier"},
			InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
			Segments: []model.Segment{{ID: "p", Strategy: model.StrategyChecklist,
				Rules: []model.Rule{{RuleName: "n", ErrorMessage: "Tier is ${layer:base-tier}.",
					Condition: &model.Condition{Field: "country", Operator: model.OpEq, Value: "US"}}}}}},
	}}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("a layer reference in a template must validate, got: %v", err)
	}
}
