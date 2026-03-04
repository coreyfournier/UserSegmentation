package model

import (
	"testing"
	"time"
)

// --- Promotion.IsActive ---

func TestPromotion_IsActive_NilPromotion(t *testing.T) {
	var p *Promotion
	if !p.IsActive(time.Now()) {
		t.Error("nil promotion should always be active")
	}
}

func TestPromotion_IsActive_EmptyWindow(t *testing.T) {
	p := &Promotion{}
	if !p.IsActive(time.Now()) {
		t.Error("empty window should be active")
	}
}

func TestPromotion_IsActive_BeforeFrom(t *testing.T) {
	from := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &Promotion{EffectiveFrom: &from}
	if p.IsActive(from.Add(-time.Hour)) {
		t.Error("should be inactive before effective_from")
	}
}

func TestPromotion_IsActive_AfterUntil(t *testing.T) {
	until := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &Promotion{EffectiveUntil: &until}
	if p.IsActive(until.Add(time.Hour)) {
		t.Error("should be inactive after effective_until")
	}
}

func TestPromotion_IsActive_WithinWindow(t *testing.T) {
	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &Promotion{EffectiveFrom: &from, EffectiveUntil: &until}
	now := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	if !p.IsActive(now) {
		t.Error("should be active within window")
	}
}

func TestPromotion_IsActive_OnlyFrom(t *testing.T) {
	from := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &Promotion{EffectiveFrom: &from}
	if !p.IsActive(from.Add(time.Hour)) {
		t.Error("should be active after effective_from with no until")
	}
}

func TestPromotion_IsActive_OnlyUntil(t *testing.T) {
	until := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	p := &Promotion{EffectiveUntil: &until}
	if !p.IsActive(until.Add(-time.Hour)) {
		t.Error("should be active before effective_until with no from")
	}
}

// --- Rule.IsEnabled ---

func TestRule_IsEnabled_NilField(t *testing.T) {
	r := &Rule{RuleName: "test"}
	if !r.IsEnabled() {
		t.Error("nil Enabled should default to true")
	}
}

func TestRule_IsEnabled_True(t *testing.T) {
	enabled := true
	r := &Rule{RuleName: "test", Enabled: &enabled}
	if !r.IsEnabled() {
		t.Error("expected enabled")
	}
}

func TestRule_IsEnabled_False(t *testing.T) {
	disabled := false
	r := &Rule{RuleName: "test", Enabled: &disabled}
	if r.IsEnabled() {
		t.Error("expected disabled")
	}
}

// --- Rule.IsLeaf ---

func TestRule_IsLeaf_WithExpression(t *testing.T) {
	r := &Rule{
		RuleName:   "leaf",
		Expression: &Expression{Field: "x", Operator: OpEq, Value: "y"},
	}
	if !r.IsLeaf() {
		t.Error("rule with expression should be leaf")
	}
}

func TestRule_IsLeaf_WithoutExpression(t *testing.T) {
	r := &Rule{
		RuleName: "composite",
		Operator: CompositeAnd,
		Rules:    []Rule{{RuleName: "child"}},
	}
	if r.IsLeaf() {
		t.Error("rule without expression should not be leaf")
	}
}

// --- ValidOperator ---

func TestValidOperator(t *testing.T) {
	valid := []Operator{OpEq, OpNeq, OpGt, OpGte, OpLt, OpLte, OpIn, OpContains}
	for _, op := range valid {
		if !ValidOperator(op) {
			t.Errorf("expected %q to be valid", op)
		}
	}
	if ValidOperator("nope") {
		t.Error("expected unknown operator to be invalid")
	}
}

// --- OperatorSupportsType ---

func TestOperatorSupportsType(t *testing.T) {
	tests := []struct {
		op   Operator
		ft   FieldType
		want bool
	}{
		{OpEq, FieldTypeString, true},
		{OpEq, FieldTypeNumber, true},
		{OpEq, FieldTypeBoolean, true},
		{OpEq, FieldTypeArray, false},
		{OpGt, FieldTypeNumber, true},
		{OpGt, FieldTypeString, false},
		{OpIn, FieldTypeString, true},
		{OpIn, FieldTypeArray, false},
		{OpContains, FieldTypeArray, true},
		{OpContains, FieldTypeString, true},
		{OpContains, FieldTypeNumber, false},
	}
	for _, tt := range tests {
		got := OperatorSupportsType(tt.op, tt.ft)
		if got != tt.want {
			t.Errorf("OperatorSupportsType(%q, %q) = %v, want %v", tt.op, tt.ft, got, tt.want)
		}
	}
}

func TestOperatorSupportsType_InvalidOperator(t *testing.T) {
	if OperatorSupportsType("nope", FieldTypeString) {
		t.Error("invalid operator should not support any type")
	}
}
