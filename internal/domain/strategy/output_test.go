package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func outputSeg() *model.Segment {
	return &model.Segment{
		ID:       "employee-account",
		Strategy: model.StrategyChecklist,
		Outputs:  map[string]string{"category": "EmployeeAccountStatus"},
		Rules: []model.Rule{{
			RuleName:     "timesheetHoursAbnormallyLow",
			ErrorMessage: "Hours look low.",
			Outputs: map[string]string{
				"diagnosisType": "TimesheetHoursAbnormallyLow",
				"description":   "${ totalHours } hours over ${ daysElapsed } days",
				"signals":       "{ TotalHours: totalHours, DaysElapsed: daysElapsed }",
			},
			Condition: &model.Condition{
				Field:    "totalHours",
				Operator: model.OpLt,
				Value:    2,
			},
		}},
	}
}

func outputCtx() *EvalContext {
	return &EvalContext{
		Context: map[string]interface{}{"totalHours": 1.5, "daysElapsed": 3},
		OutputSchema: model.OutputSchema{
			"category":      {Type: model.FieldTypeString},
			"diagnosisType": {Type: model.FieldTypeString, Lookup: "diagnosis-type"},
			"description":   {Type: model.FieldTypeString},
			"signals":       {Type: model.FieldTypeObject},
		},
		Lookups: map[string]model.LookupTable{
			"diagnosis-type": {
				ID:        "diagnosis-type",
				KeyType:   model.FieldTypeString,
				EmitOrder: true,
				Entries: []model.LookupEntry{
					{Key: "TimesheetHoursAbnormallyLow", Value: "Hours abnormally low", Order: 30},
				},
			},
		},
		CollectFailures: true,
	}
}

func TestChecklist_FailureCarriesOutputs(t *testing.T) {
	var s ChecklistStrategy
	res, ok := s.Evaluate(outputSeg(), outputCtx())
	if !ok || len(res.Failures) != 1 {
		t.Fatalf("expected one failure, got ok=%v n=%d", ok, len(res.Failures))
	}
	out := res.Failures[0].Outputs

	// literal from segment level, because the item does not set it
	if out["category"] != "EmployeeAccountStatus" {
		t.Errorf("category = %v", out["category"])
	}

	// template renders to a string
	if out["description"] != "1.5 hours over 3 days" {
		t.Errorf("description = %v", out["description"])
	}

	// lookup-bound field is enriched with value and order
	lk, isMap := out["diagnosisType"].(map[string]interface{})
	if !isMap {
		t.Fatalf("diagnosisType should be a map, got %T", out["diagnosisType"])
	}
	if lk["key"] != "TimesheetHoursAbnormallyLow" || lk["value"] != "Hours abnormally low" || lk["order"] != 30 {
		t.Errorf("lookup enrichment = %v", lk)
	}

	// expression keeps its type
	sig, isMap := out["signals"].(map[string]interface{})
	if !isMap {
		t.Fatalf("signals should be a map, got %T", out["signals"])
	}
	if sig["TotalHours"] != 1.5 {
		t.Errorf("signals = %v", sig)
	}
}

// A broken expression drops its own field and records an error. The finding
// still reports — evidence failing must not take the diagnosis with it.
func TestChecklist_BadOutputExpressionDegrades(t *testing.T) {
	seg := outputSeg()
	seg.Rules[0].Outputs["signals"] = "totalHours +"

	var s ChecklistStrategy
	res, _ := s.Evaluate(seg, outputCtx())
	if len(res.Failures) != 1 {
		t.Fatalf("expected the failure to survive, got %d", len(res.Failures))
	}
	if _, present := res.Failures[0].Outputs["signals"]; present {
		t.Error("broken expression should not emit a value")
	}
	if len(res.RenderErrors) == 0 {
		t.Error("expected a render error to be recorded")
	}
	if res.Failures[0].Outputs["category"] != "EmployeeAccountStatus" {
		t.Error("other fields should still be emitted")
	}
}

// A broken template token drops its own field and records an error, even
// though renderTemplate itself returns a partially-rendered string with the
// bad token left in literally. The finding still reports.
func TestChecklist_BadOutputTemplateDegrades(t *testing.T) {
	seg := outputSeg()
	seg.Rules[0].Outputs["description"] = "${ totalHours } hours over ${ daysElapsed + }"

	var s ChecklistStrategy
	res, _ := s.Evaluate(seg, outputCtx())
	if len(res.Failures) != 1 {
		t.Fatalf("expected the failure to survive, got %d", len(res.Failures))
	}
	if _, present := res.Failures[0].Outputs["description"]; present {
		t.Error("broken template should not emit a value")
	}
	if len(res.RenderErrors) == 0 {
		t.Error("expected a render error to be recorded")
	}
	if res.Failures[0].Outputs["category"] != "EmployeeAccountStatus" {
		t.Error("other fields should still be emitted")
	}
}

// When a rule and its segment both declare the same output field, the rule's
// value wins: item-level authorship is more specific than the segment default.
func TestChecklist_ItemOutputOverridesSegmentOutput(t *testing.T) {
	seg := outputSeg()
	seg.Rules[0].Outputs["category"] = "OverriddenByRule"

	var s ChecklistStrategy
	res, ok := s.Evaluate(seg, outputCtx())
	if !ok || len(res.Failures) != 1 {
		t.Fatalf("expected one failure, got ok=%v n=%d", ok, len(res.Failures))
	}
	if got := res.Failures[0].Outputs["category"]; got != "OverriddenByRule" {
		t.Errorf("category = %v, want rule value to win over segment value", got)
	}
}

// An unknown key passes through bare. Lookup membership is documented, not
// enforced at evaluation time.
func TestEnrichLookupValue_UnknownKeyPassesThrough(t *testing.T) {
	got := enrichLookupValue("diagnosis-type", "NotInTable", outputCtx())
	if got != "NotInTable" {
		t.Fatalf("expected the bare key, got %v", got)
	}
}

// Same for boolean.
func TestEvaluateOutputs_LiteralBooleanEmitsBoolean(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Outputs:  map[string]string{"active": "true"},
	}
	out, errs := evaluateOutputs(seg, nil, &EvalContext{OutputSchema: model.OutputSchema{"active": {Type: model.FieldTypeBoolean}}})
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

// A literal string is unaffected: it stays a plain Go string, not wrapped or
// re-typed.
func TestEvaluateOutputs_LiteralStringEmitsString(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Outputs:  map[string]string{"category": "EmployeeAccountStatus"},
	}
	out, errs := evaluateOutputs(seg, nil, &EvalContext{OutputSchema: model.OutputSchema{"category": {Type: model.FieldTypeString}}})
	if len(errs) != 0 {
		t.Fatalf("expected no render errors, got %v", errs)
	}
	if got, isString := out["category"].(string); !isString || got != "EmployeeAccountStatus" {
		t.Errorf("category = %v (%T), want the string EmployeeAccountStatus", out["category"], out["category"])
	}
}

// An expression that resolves to nothing — an unknown identifier, most often
// a typo in a field name — is not a compile error: expr.Run returns (nil,
// nil). The field must be omitted (not emitted as a null that would satisfy a
// Required check), exactly one RenderError must name it, and a sibling field
// that resolves fine must still be emitted.
func TestEvaluateOutputs_UnboundIdentifierDegrades(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Outputs: map[string]string{
			"shortfall": "MaxAllowd", // typo: MaxAllowed
			"severity":  "Critical",
		},
	}
	ctx := &EvalContext{
		Context: map[string]interface{}{"MaxAllowed": 40, "actualHours": 45},
		OutputSchema: model.OutputSchema{
			"shortfall": {Type: model.FieldTypeNumber, Required: true},
			"severity":  {Type: model.FieldTypeString},
		},
	}
	out, errs := evaluateOutputs(seg, nil, ctx)

	if _, present := out["shortfall"]; present {
		t.Errorf("shortfall should be omitted, got %v", out["shortfall"])
	}
	if len(errs) != 1 {
		t.Fatalf("expected exactly one render error, got %d: %v", len(errs), errs)
	}
	if errs[0].Field != "shortfall" {
		t.Errorf("render error field = %q, want %q", errs[0].Field, "shortfall")
	}
	if got, isString := out["severity"].(string); !isString || got != "Critical" {
		t.Errorf("severity = %v (%T), want the sibling field to still be emitted", out["severity"], out["severity"])
	}
}

// A lookup-bound field with a numeric KeyType now compares against the
// coerced numeric value instead of the raw authored string, so a match that
// used to fall through as a bare key now enriches correctly.
func TestEvaluateOutputs_NumericLookupKeyNowMatches(t *testing.T) {
	seg := &model.Segment{
		ID:       "seg",
		Strategy: model.StrategyRule,
		Outputs:  map[string]string{"tier": "1"},
	}
	ctx := &EvalContext{
		OutputSchema: model.OutputSchema{
			"tier": {Type: model.FieldTypeNumber, Lookup: "vip-tiers"},
		},
		Lookups: map[string]model.LookupTable{
			"vip-tiers": {
				ID: "vip-tiers", KeyType: model.FieldTypeNumber,
				Entries: []model.LookupEntry{{Key: 1.0, Value: "Gold", Order: 1}},
			},
		},
	}
	out, errs := evaluateOutputs(seg, nil, ctx)
	if len(errs) != 0 {
		t.Fatalf("expected no render errors, got %v", errs)
	}
	enriched, isMap := out["tier"].(map[string]interface{})
	if !isMap {
		t.Fatalf("expected the numeric key to now match and enrich, got %T (%v)", out["tier"], out["tier"])
	}
	if enriched["value"] != "Gold" {
		t.Errorf("enriched value = %v, want Gold", enriched["value"])
	}
}
