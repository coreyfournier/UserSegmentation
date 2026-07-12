package strategy

import (
	"strings"
	"testing"
)

func TestRenderMessages_BareVariable(t *testing.T) {
	raw := map[string]string{"en": "Hi ${Name}!"}
	res := RenderMessages(raw, map[string]interface{}{"Name": "Bob"}, []string{"en"}, false, "en")
	if got := res.Rendered["en"]; got != "Hi Bob!" {
		t.Fatalf("got %q, want %q", got, "Hi Bob!")
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
}

func TestRenderMessages_Expression(t *testing.T) {
	raw := map[string]string{"en": "Status: ${CTTotal > 30 ? 'free' : 'paid'}"}
	res := RenderMessages(raw, map[string]interface{}{"CTTotal": 35.0}, []string{"en"}, false, "en")
	if got := res.Rendered["en"]; got != "Status: free" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderMessages_NumericStringify(t *testing.T) {
	raw := map[string]string{"en": "Fee ${TransferFee}"}
	res := RenderMessages(raw, map[string]interface{}{"TransferFee": 4.0}, []string{"en"}, false, "en")
	if got := res.Rendered["en"]; got != "Fee 4" {
		t.Fatalf("got %q, want %q", got, "Fee 4")
	}
}

func TestRenderMessages_FallbackToDefaultLanguage(t *testing.T) {
	raw := map[string]string{"en": "Hello"}
	res := RenderMessages(raw, nil, []string{"es"}, false, "en")
	// Caller asked for "es"; en is the fallback content, keyed under the request.
	if got := res.Rendered["es"]; got != "Hello" {
		t.Fatalf("got %q, want fallback %q", got, "Hello")
	}
}

func TestRenderMessages_MissingNoFallbackOmits(t *testing.T) {
	raw := map[string]string{"en": "Hello"}
	res := RenderMessages(raw, nil, []string{"es"}, false, "fr") // fr not present either
	if _, ok := res.Rendered["es"]; ok {
		t.Fatalf("expected es to be omitted, got %v", res.Rendered)
	}
}

func TestRenderMessages_RenderAll(t *testing.T) {
	raw := map[string]string{"en": "Hi", "es": "Hola"}
	res := RenderMessages(raw, nil, nil, true, "en")
	if len(res.Rendered) != 2 || res.Rendered["en"] != "Hi" || res.Rendered["es"] != "Hola" {
		t.Fatalf("renderAll got %v", res.Rendered)
	}
}

func TestRenderMessages_ErrorKeepsRawTokenAndReports(t *testing.T) {
	raw := map[string]string{"en": "Val ${1 +}"} // invalid expr -> compile error
	res := RenderMessages(raw, nil, []string{"en"}, false, "en")
	if got := res.Rendered["en"]; !strings.Contains(got, "${1 +}") {
		t.Fatalf("expected raw token preserved, got %q", got)
	}
	if len(res.Errors) != 1 || res.Errors[0].Language != "en" {
		t.Fatalf("expected 1 render error for en, got %v", res.Errors)
	}
}

func TestRenderMessages_MissingVariableRendersEmpty(t *testing.T) {
	raw := map[string]string{"en": "Hi[${Missing}]"}
	res := RenderMessages(raw, map[string]interface{}{}, []string{"en"}, false, "en")
	if got := res.Rendered["en"]; got != "Hi[]" {
		t.Fatalf("got %q, want %q", got, "Hi[]")
	}
	if len(res.Errors) != 0 {
		t.Fatalf("missing var should not be a render error: %v", res.Errors)
	}
}

func TestRenderMessages_EmptyRawReturnsNil(t *testing.T) {
	res := RenderMessages(nil, nil, []string{"en"}, false, "en")
	if len(res.Rendered) != 0 || len(res.Errors) != 0 {
		t.Fatalf("expected empty result, got %v", res)
	}
}
