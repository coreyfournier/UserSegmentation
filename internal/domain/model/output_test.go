package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// An object field exists for expression-mode outputs that return a map. It must
// never be usable in a condition, so no operator may accept it.
func TestFieldTypeObject_NotUsableInConditions(t *testing.T) {
	for op := range OperatorTypes {
		if OperatorSupportsType(op, FieldTypeObject) {
			t.Fatalf("operator %q must not accept the object type", op)
		}
	}
}

func TestOutputFieldDropsEval(t *testing.T) {
	// A stale "eval" key must be captured, not dropped. Silently ignoring it
	// would turn an expression-mode string value into a template, changing
	// what it emits with no error anywhere.
	var f OutputField
	if err := json.Unmarshal([]byte(`{"type":"string","eval":"expression"}`), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f.Type != FieldTypeString {
		t.Errorf("type = %v", f.Type)
	}
	if f.LegacyEval == "" {
		t.Error("legacy eval was dropped instead of captured")
	}

	// A current field round-trips with no eval key at all.
	b, err := json.Marshal(OutputField{Type: FieldTypeNumber, Required: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "eval") {
		t.Errorf("eval must not be emitted: %s", b)
	}
}

func TestOutputField_IsTemplate(t *testing.T) {
	if !(OutputField{Type: FieldTypeString}).IsTemplate() {
		t.Error("string field should be a template")
	}
	if (OutputField{Type: FieldTypeNumber}).IsTemplate() {
		t.Error("number field should not be a template")
	}
	if (OutputField{Type: FieldTypeObject}).IsTemplate() {
		t.Error("object field should not be a template")
	}
}
