package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
)

func newMessageEvaluator() *Evaluator {
	return NewEvaluator(map[string]strategy.Strategy{
		"static":     &strategy.StaticStrategy{},
		"rule":       &strategy.RuleStrategy{},
		"expression": &strategy.ExpressionStrategy{},
	})
}

func TestEvaluator_RuleMessageRendered(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "l", Order: 1,
			Segments: []model.Segment{{
				ID: "s", Strategy: "rule",
				Rules: []model.Rule{{
					RuleName: "hit", SuccessEvent: "won",
					Expression: &model.Expression{Field: "plan", Operator: model.OpEq, Value: "pro"},
					Messages:   map[string]string{"en": "Hi ${Name}", "es": "Hola ${Name}"},
				}},
				Default: "none",
			}},
		}},
	}
	e := newMessageEvaluator()
	res := e.Evaluate(snap, "u", map[string]interface{}{"plan": "pro", "Name": "Bob"}, nil, []string{"es"}, false, time.Now())
	a := res.Layers["l"]
	if a == nil || a.Segment != "won" {
		t.Fatalf("expected won, got %v", a)
	}
	if a.Messages["es"] != "Hola Bob" {
		t.Fatalf("got messages %v", a.Messages)
	}
	if _, ok := a.Messages["en"]; ok {
		t.Fatalf("only requested es should render, got %v", a.Messages)
	}
}

func TestEvaluator_OverrideMessageRendered(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "l", Order: 1,
			Segments: []model.Segment{{
				ID: "s", Strategy: "static",
				Static: &model.StaticConfig{Default: "normal"},
				Overrides: []model.Rule{{
					RuleName: "ov", SuccessEvent: "override-val",
					Expression: &model.Expression{Field: "plan", Operator: model.OpEq, Value: "ent"},
					Messages:   map[string]string{"en": "Override for ${plan}"},
				}},
			}},
		}},
	}
	e := newMessageEvaluator()
	res := e.Evaluate(snap, "u", map[string]interface{}{"plan": "ent"}, nil, []string{"en"}, false, time.Now())
	a := res.Layers["l"]
	if a == nil || a.Strategy != "override" || a.Messages["en"] != "Override for ent" {
		t.Fatalf("got %v (messages %v)", a, a.Messages)
	}
}

func TestEvaluator_DefaultMessageRendered(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "l", Order: 1,
			Segments: []model.Segment{{
				ID: "s", Strategy: "rule",
				Rules: []model.Rule{{
					RuleName: "hit", SuccessEvent: "won",
					Expression: &model.Expression{Field: "plan", Operator: model.OpEq, Value: "pro"},
				}},
				Default:         "none",
				DefaultMessages: map[string]string{"en": "No match, sorry"},
			}},
		}},
	}
	e := newMessageEvaluator()
	res := e.Evaluate(snap, "u", map[string]interface{}{"plan": "free"}, nil, []string{"en"}, false, time.Now())
	a := res.Layers["l"]
	if a == nil || a.Segment != "none" || a.Messages["en"] != "No match, sorry" {
		t.Fatalf("got %v (messages %v)", a, a.Messages)
	}
}

func TestEvaluator_MessageUsesComputedExpressionField(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "l", Order: 1,
			Segments: []model.Segment{{
				ID: "s", Strategy: "expression",
				Expressions: []model.ExpressionDef{
					{Name: "TransferFee", Type: "number", Expression: "CTTotal > 30 ? 0 : 4"},
				},
				Rules: []model.Rule{{
					RuleName: "partial", SuccessEvent: "fee-partial",
					Expression: &model.Expression{Field: "CTTotal", Operator: model.OpLte, Value: 30},
					Messages:   map[string]string{"en": "You pay ${TransferFee}"},
				}},
				Default: "fee-standard",
			}},
		}},
	}
	e := newMessageEvaluator()
	res := e.Evaluate(snap, "u", map[string]interface{}{"CTTotal": 20.0}, nil, []string{"en"}, false, time.Now())
	a := res.Layers["l"]
	if a == nil || a.Messages["en"] != "You pay 4" {
		t.Fatalf("expected computed field in message, got %v (messages %v)", a, a.Messages)
	}
}

func TestEvaluator_MessageFallbackToLayerDefaultLanguage(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "l", Order: 1, DefaultLanguage: "en",
			Segments: []model.Segment{{
				ID: "s", Strategy: "rule",
				Rules: []model.Rule{{
					RuleName: "hit", SuccessEvent: "won",
					Expression: &model.Expression{Field: "plan", Operator: model.OpEq, Value: "pro"},
					Messages:   map[string]string{"en": "English only"},
				}},
				Default: "none",
			}},
		}},
	}
	e := newMessageEvaluator()
	res := e.Evaluate(snap, "u", map[string]interface{}{"plan": "pro"}, nil, []string{"es"}, false, time.Now())
	a := res.Layers["l"]
	if a == nil || a.Messages["es"] != "English only" {
		t.Fatalf("expected fallback to en content under es, got %v", a.Messages)
	}
}

func TestEvaluator_MessageRenderErrorProducesWarning(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "l", Order: 1,
			Segments: []model.Segment{{
				ID: "s", Strategy: "rule",
				Rules: []model.Rule{{
					RuleName: "hit", SuccessEvent: "won",
					Expression: &model.Expression{Field: "plan", Operator: model.OpEq, Value: "pro"},
					Messages:   map[string]string{"en": "Bad ${1 +}"},
				}},
				Default: "none",
			}},
		}},
	}
	e := newMessageEvaluator()
	res := e.Evaluate(snap, "u", map[string]interface{}{"plan": "pro"}, nil, []string{"en"}, false, time.Now())
	a := res.Layers["l"]
	if a == nil || !strings.Contains(a.Messages["en"], "${1 +}") {
		t.Fatalf("expected raw token preserved, got %v", a.Messages)
	}
	found := false
	for _, w := range res.Warnings {
		if w.Segment == "s" && w.Field == "en" && strings.Contains(w.Message, "render error") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a render-error warning, got %v", res.Warnings)
	}
}

func TestEvaluator_NoMessagesWhenNoLanguagesRequested(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "l", Order: 1,
			Segments: []model.Segment{{
				ID: "s", Strategy: "rule",
				Rules: []model.Rule{{
					RuleName: "hit", SuccessEvent: "won",
					Expression: &model.Expression{Field: "plan", Operator: model.OpEq, Value: "pro"},
					Messages:   map[string]string{"en": "Hi"},
				}},
				Default: "none",
			}},
		}},
	}
	e := newMessageEvaluator()
	res := e.Evaluate(snap, "u", map[string]interface{}{"plan": "pro"}, nil, nil, false, time.Now())
	if a := res.Layers["l"]; a == nil || len(a.Messages) != 0 {
		t.Fatalf("expected no messages without languages, got %v", a.Messages)
	}
}

func TestEvaluator_RenderAllReturnsAllLocales(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "l", Order: 1,
			Segments: []model.Segment{{
				ID: "s", Strategy: "rule",
				Rules: []model.Rule{{
					RuleName: "hit", SuccessEvent: "won",
					Expression: &model.Expression{Field: "plan", Operator: model.OpEq, Value: "pro"},
					Messages:   map[string]string{"en": "Hi", "es": "Hola"},
				}},
				Default: "none",
			}},
		}},
	}
	e := newMessageEvaluator()
	res := e.Evaluate(snap, "u", map[string]interface{}{"plan": "pro"}, nil, nil, true, time.Now())
	a := res.Layers["l"]
	if a == nil || len(a.Messages) != 2 {
		t.Fatalf("expected renderAll to return both locales, got %v", a.Messages)
	}
}
