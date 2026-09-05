package strategy

import "testing"

// TestRenderTemplate_DeclaredField covers the ordering that fixes the shipped
// config bug at config/segments.json:572: a "${...}" token must resolve
// through model.ResolveField before falling back to expr, so a flat dotted
// key (a declared field, not member access) renders instead of failing.
func TestRenderTemplate_DeclaredField(t *testing.T) {
	t.Run("flat dotted key renders and reports no bad token", func(t *testing.T) {
		env := map[string]interface{}{"company.ein": "12-3456789"}
		got, bad := renderTemplate("EIN ${company.ein}", env)
		if got != "EIN 12-3456789" {
			t.Fatalf("got %q, want %q", got, "EIN 12-3456789")
		}
		if len(bad) != 0 {
			t.Fatalf("expected no bad tokens, got %v", bad)
		}
	})

	t.Run("nested map still walks", func(t *testing.T) {
		env := map[string]interface{}{
			"nested": map[string]interface{}{"ein": "99-9"},
		}
		got, bad := renderTemplate("EIN ${nested.ein}", env)
		if got != "EIN 99-9" {
			t.Fatalf("got %q, want %q", got, "EIN 99-9")
		}
		if len(bad) != 0 {
			t.Fatalf("expected no bad tokens, got %v", bad)
		}
	})

	t.Run("plain key still resolves", func(t *testing.T) {
		env := map[string]interface{}{"name": "Ada"}
		got, bad := renderTemplate("Hello ${name}", env)
		if got != "Hello Ada" {
			t.Fatalf("got %q, want %q", got, "Hello Ada")
		}
		if len(bad) != 0 {
			t.Fatalf("expected no bad tokens, got %v", bad)
		}
	})

	t.Run("compound expression falls through to expr", func(t *testing.T) {
		env := map[string]interface{}{"totalHours": 3}
		got, bad := renderTemplate("Total ${totalHours * 2}", env)
		if got != "Total 6" {
			t.Fatalf("got %q, want %q", got, "Total 6")
		}
		if len(bad) != 0 {
			t.Fatalf("expected no bad tokens, got %v", bad)
		}
	})

	t.Run("syntax error still reports a bad token", func(t *testing.T) {
		env := map[string]interface{}{"amount": 5}
		got, bad := renderTemplate("Bad ${amount *}", env)
		if len(bad) != 1 {
			t.Fatalf("expected 1 bad token, got %v", bad)
		}
		if got != "Bad ${amount *}" {
			t.Fatalf("got %q, want raw token preserved", got)
		}
	})

	t.Run("declared field holding nil renders empty without a bad token", func(t *testing.T) {
		env := map[string]interface{}{"missing": nil}
		got, bad := renderTemplate("Val[${missing}]", env)
		if got != "Val[]" {
			t.Fatalf("got %q, want %q", got, "Val[]")
		}
		if len(bad) != 0 {
			t.Fatalf("expected no bad tokens, got %v", bad)
		}
	})
}
