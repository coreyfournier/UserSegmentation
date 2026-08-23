package model

import "testing"

func TestResolveField_FlatKey(t *testing.T) {
	ctx := map[string]interface{}{"EarnedWages": 120.0}
	got, ok := ResolveField(ctx, "EarnedWages")
	if !ok || got != 120.0 {
		t.Fatalf("expected 120.0, got %v (ok=%v)", got, ok)
	}
}

func TestResolveField_NestedPath(t *testing.T) {
	ctx := map[string]interface{}{
		"company": map[string]interface{}{
			"payFrequency": "biweekly",
			"address":      map[string]interface{}{"state": "TX"},
		},
	}

	if got, ok := ResolveField(ctx, "company.payFrequency"); !ok || got != "biweekly" {
		t.Errorf("company.payFrequency: got %v (ok=%v)", got, ok)
	}
	if got, ok := ResolveField(ctx, "company.address.state"); !ok || got != "TX" {
		t.Errorf("company.address.state: got %v (ok=%v)", got, ok)
	}
}

// A key that literally contains a dot must win over path traversal, so configs
// written before nesting existed keep working unchanged.
func TestResolveField_FlatKeyBeatsPath(t *testing.T) {
	ctx := map[string]interface{}{
		"company.payFrequency": "flat-wins",
		"company":              map[string]interface{}{"payFrequency": "nested-loses"},
	}
	if got, _ := ResolveField(ctx, "company.payFrequency"); got != "flat-wins" {
		t.Errorf("expected the flat key to win, got %v", got)
	}
}

func TestResolveField_Missing(t *testing.T) {
	ctx := map[string]interface{}{
		"company": map[string]interface{}{"ein": "12-3456789"},
	}

	cases := []string{
		"company.payFrequency", // missing leaf
		"employee.hireDate",    // missing root
		"company.ein.extra",    // intermediate is not a map
		"nope",
	}
	for _, field := range cases {
		if _, ok := ResolveField(ctx, field); ok {
			t.Errorf("%s: expected absent", field)
		}
	}
}

func TestResolveField_StringMap(t *testing.T) {
	ctx := map[string]interface{}{
		"company": map[string]string{"productType": "Precision"},
	}
	if got, ok := ResolveField(ctx, "company.productType"); !ok || got != "Precision" {
		t.Errorf("got %v (ok=%v)", got, ok)
	}
}
