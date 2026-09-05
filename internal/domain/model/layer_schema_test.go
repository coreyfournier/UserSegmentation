package model

import (
	"encoding/json"
	"testing"
)

func TestLayerCarriesSchemas(t *testing.T) {
	in := Layer{
		Name:         "company-identity",
		InputSchema:  InputSchema{"company.ein": {Type: FieldTypeString, Required: true}},
		OutputSchema: OutputSchema{"severity": {Type: FieldTypeString}},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Layer
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.InputSchema["company.ein"].Required {
		t.Errorf("input schema lost: %s", b)
	}
	if out.OutputSchema["severity"].Type != FieldTypeString {
		t.Errorf("output schema lost: %s", b)
	}
}

func TestSegmentCapturesLegacySchemas(t *testing.T) {
	// A schema left on a segment must be captured, not silently dropped — a
	// dropped input schema turns rule-field validation off without saying so.
	var seg Segment
	err := json.Unmarshal([]byte(`{"id":"s","strategy":"rule",
		"inputSchema":{"age":{"type":"number"}},
		"outputSchema":{"severity":{"type":"string"}}}`), &seg)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(seg.LegacyInputSchema) != 1 {
		t.Errorf("legacy input schema not captured: %+v", seg)
	}
	if len(seg.LegacyOutputSchema) != 1 {
		t.Errorf("legacy output schema not captured: %+v", seg)
	}
}
