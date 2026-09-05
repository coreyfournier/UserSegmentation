package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// These tests pin the type-derived eval mode end to end through
// evaluateOutputs: a string field is a template, every other type is an
// expression. See model.OutputField.IsTemplate.

// A string field renders its value as a template — both a plain constant with
// no tokens and one with a ${} token that pulls from context.
func TestEvaluateOutputs_StringFieldRendersAsTemplate(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Outputs:  map[string]string{"label": "steady", "greeting": "Hello, ${ name }"},
	}
	ctx := &EvalContext{
		Context: map[string]interface{}{"name": "Ada"},
		OutputSchema: model.OutputSchema{
			"label":    {Type: model.FieldTypeString},
			"greeting": {Type: model.FieldTypeString},
		},
	}

	out, errs := evaluateOutputs(seg, nil, ctx)
	if len(errs) != 0 {
		t.Fatalf("expected no render errors, got %v", errs)
	}
	if out["label"] != "steady" {
		t.Errorf("label = %v, want the constant unchanged", out["label"])
	}
	if out["greeting"] != "Hello, Ada" {
		t.Errorf("greeting = %v, want the token rendered", out["greeting"])
	}
}

// A number field evaluates its value as an expression — both a bare literal
// and one computed from context.
func TestEvaluateOutputs_NumberFieldEvaluatesAsExpression(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Outputs:  map[string]string{"rank": "3", "total": "a + b"},
	}
	ctx := &EvalContext{
		Context: map[string]interface{}{"a": 2, "b": 5},
		OutputSchema: model.OutputSchema{
			"rank":  {Type: model.FieldTypeNumber},
			"total": {Type: model.FieldTypeNumber},
		},
	}

	out, errs := evaluateOutputs(seg, nil, ctx)
	if len(errs) != 0 {
		t.Fatalf("expected no render errors, got %v", errs)
	}
	if out["rank"] != 3 {
		t.Errorf("rank = %v (%T), want 3", out["rank"], out["rank"])
	}
	if out["total"] != 7 {
		t.Errorf("total = %v (%T), want 7", out["total"], out["total"])
	}
}

// A boolean field evaluates as an expression too, yielding the Go bool rather
// than the literal string "true".
func TestEvaluateOutputs_BooleanFieldYieldsBoolNotString(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Outputs:  map[string]string{"active": "true"},
	}
	ctx := &EvalContext{
		OutputSchema: model.OutputSchema{"active": {Type: model.FieldTypeBoolean}},
	}

	out, errs := evaluateOutputs(seg, nil, ctx)
	if len(errs) != 0 {
		t.Fatalf("expected no render errors, got %v", errs)
	}
	got, isBool := out["active"].(bool)
	if !isBool {
		t.Fatalf("expected active to be a bool, got %T (%v)", out["active"], out["active"])
	}
	if got != true {
		t.Errorf("active = %v, want true", got)
	}
}

// An object field's map expression yields a Go map, not a string.
func TestEvaluateOutputs_ObjectFieldYieldsMap(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Outputs:  map[string]string{"signals": "{ Rank: rank }"},
	}
	ctx := &EvalContext{
		Context:      map[string]interface{}{"rank": 3},
		OutputSchema: model.OutputSchema{"signals": {Type: model.FieldTypeObject}},
	}

	out, errs := evaluateOutputs(seg, nil, ctx)
	if len(errs) != 0 {
		t.Fatalf("expected no render errors, got %v", errs)
	}
	sig, isMap := out["signals"].(map[string]interface{})
	if !isMap {
		t.Fatalf("expected signals to be a map, got %T", out["signals"])
	}
	if sig["Rank"] != 3 {
		t.Errorf("signals = %v", sig)
	}
}

// A failing value — template or expression — still degrades: the error is
// recorded, the field is omitted entirely (never half-rendered), and the item
// is kept, with unaffected fields still emitted.
func TestEvaluateOutputs_FailingValueDegradesFieldOnly(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Outputs: map[string]string{
			"greeting": "Hello, ${ name + }", // broken template token
			"total":    "a +",                // broken expression
			"label":    "steady",             // unaffected
		},
	}
	ctx := &EvalContext{
		Context: map[string]interface{}{"name": "Ada", "a": 2},
		OutputSchema: model.OutputSchema{
			"greeting": {Type: model.FieldTypeString},
			"total":    {Type: model.FieldTypeNumber},
			"label":    {Type: model.FieldTypeString},
		},
	}

	out, errs := evaluateOutputs(seg, nil, ctx)
	if len(errs) != 2 {
		t.Fatalf("expected two render errors, got %v", errs)
	}
	if _, present := out["greeting"]; present {
		t.Error("broken template should not emit a value, half-rendered or otherwise")
	}
	if _, present := out["total"]; present {
		t.Error("broken expression should not emit a value")
	}
	if out["label"] != "steady" {
		t.Errorf("label = %v, unaffected fields should still be emitted", out["label"])
	}
}
