package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func lookups() map[string]model.LookupTable {
	return map[string]model.LookupTable{
		"premium-zips": {
			ID: "premium-zips", Name: "Premium Zips", KeyType: model.FieldTypeString,
			Entries: []model.LookupEntry{
				{Key: "90210", Value: "Beverly Hills"},
				{Key: "10001", Value: "NYC"},
			},
		},
		"vip-tiers": {
			ID: "vip-tiers", Name: "VIP Tiers", KeyType: model.FieldTypeNumber,
			Entries: []model.LookupEntry{{Key: 1.0}, {Key: 2.0}},
		},
	}
}

func TestInLookup_Match(t *testing.T) {
	expr := &model.Expression{Field: "zip", Operator: model.OpInLookup, Value: "premium-zips"}
	if !EvalExpression(expr, map[string]interface{}{"zip": "90210"}, lookups()) {
		t.Fatal("expected 90210 to be in premium-zips")
	}
}

func TestInLookup_NoMatch(t *testing.T) {
	expr := &model.Expression{Field: "zip", Operator: model.OpInLookup, Value: "premium-zips"}
	if EvalExpression(expr, map[string]interface{}{"zip": "00000"}, lookups()) {
		t.Fatal("expected 00000 not to be in premium-zips")
	}
}

func TestNotInLookup(t *testing.T) {
	expr := &model.Expression{Field: "zip", Operator: model.OpNotInLookup, Value: "premium-zips"}
	if !EvalExpression(expr, map[string]interface{}{"zip": "00000"}, lookups()) {
		t.Fatal("expected not_in_lookup true for 00000")
	}
	if EvalExpression(expr, map[string]interface{}{"zip": "90210"}, lookups()) {
		t.Fatal("expected not_in_lookup false for 90210")
	}
}

func TestInLookup_NumericCoercion(t *testing.T) {
	// Field is number 1 (float64), lookup keys are numeric — stringified compare matches.
	expr := &model.Expression{Field: "tier", Operator: model.OpInLookup, Value: "vip-tiers"}
	if !EvalExpression(expr, map[string]interface{}{"tier": 1.0}, lookups()) {
		t.Fatal("expected tier 1 to be in vip-tiers")
	}
}

func TestInLookup_MissingTable(t *testing.T) {
	expr := &model.Expression{Field: "zip", Operator: model.OpInLookup, Value: "does-not-exist"}
	if EvalExpression(expr, map[string]interface{}{"zip": "90210"}, lookups()) {
		t.Fatal("dangling table reference should yield false")
	}
}

func TestNotInLookup_MissingTable(t *testing.T) {
	// Dangling table: in_lookup is false, so not_in_lookup is true.
	expr := &model.Expression{Field: "zip", Operator: model.OpNotInLookup, Value: "does-not-exist"}
	if !EvalExpression(expr, map[string]interface{}{"zip": "90210"}, lookups()) {
		t.Fatal("not_in_lookup with dangling table should yield true")
	}
}
