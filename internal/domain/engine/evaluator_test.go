package engine

import (
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
)

type mockHasher struct{ bucket int }

func (m *mockHasher) Bucket(_, _ string) int { return m.bucket }

func newTestEvaluator(bucket int) *Evaluator {
	return NewEvaluator(map[string]strategy.Strategy{
		"static":     &strategy.StaticStrategy{},
		"rule":       &strategy.RuleStrategy{},
		"percentage": &strategy.PercentageStrategy{Hasher: &mockHasher{bucket: bucket}},
	})
}

func TestEvaluator_StaticLayer(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name:  "base-tier",
				Order: 1,
				Segments: []model.Segment{
					{
						ID:       "tier",
						Strategy: "static",
						Static: &model.StaticConfig{
							Mappings: map[string]string{"vip": "platinum"},
							Default:  "standard",
						},
					},
				},
			},
		},
	}

	e := newTestEvaluator(0)
	result := e.Evaluate(snap, "vip", map[string]interface{}{}, nil, time.Now())
	if result.Layers["base-tier"] == nil || result.Layers["base-tier"].Segment != "platinum" {
		t.Errorf("expected platinum, got %v", result.Layers["base-tier"])
	}

	result = e.Evaluate(snap, "other", map[string]interface{}{}, nil, time.Now())
	if result.Layers["base-tier"] == nil || result.Layers["base-tier"].Segment != "standard" {
		t.Errorf("expected standard, got %v", result.Layers["base-tier"])
	}
}

func TestEvaluator_CrossLayerDependency(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name:  "base-tier",
				Order: 1,
				Segments: []model.Segment{
					{
						ID:       "tier",
						Strategy: "static",
						Static: &model.StaticConfig{
							Mappings: map[string]string{"vip": "pro"},
							Default:  "free",
						},
					},
				},
			},
			{
				Name:  "promotions",
				Order: 2,
				Segments: []model.Segment{
					{
						ID:       "promo",
						Strategy: "rule",
						Rules: []model.Rule{
							{
								RuleName:     "pro-promo",
								SuccessEvent: "special-offer",
								Expression:   &model.Expression{Field: "layer:base-tier", Operator: model.OpEq, Value: "pro"},
							},
						},
						Default: "none",
					},
				},
			},
		},
	}

	e := newTestEvaluator(0)

	// VIP user gets pro tier, then promo matches
	result := e.Evaluate(snap, "vip", map[string]interface{}{}, nil, time.Now())
	if result.Layers["promotions"] == nil || result.Layers["promotions"].Segment != "special-offer" {
		t.Errorf("expected special-offer, got %v", result.Layers["promotions"])
	}

	// Non-VIP gets free tier, promo defaults to none
	result = e.Evaluate(snap, "other", map[string]interface{}{}, nil, time.Now())
	if result.Layers["promotions"] == nil || result.Layers["promotions"].Segment != "none" {
		t.Errorf("expected none, got %v", result.Layers["promotions"])
	}
}

func TestEvaluator_PromotionTimeGating(t *testing.T) {
	future := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name:  "promos",
				Order: 1,
				Segments: []model.Segment{
					{
						ID:       "future-promo",
						Strategy: "rule",
						Promotion: &model.Promotion{
							EffectiveFrom: &future,
						},
						Rules: []model.Rule{
							{RuleName: "always", SuccessEvent: "promo", Expression: &model.Expression{Field: "x", Operator: model.OpEq, Value: "y"}},
						},
						Default: "none",
					},
				},
			},
		},
	}

	e := newTestEvaluator(0)

	// Now is before effective_from, segment should be skipped
	result := e.Evaluate(snap, "user", map[string]interface{}{"x": "y"}, nil, time.Now())
	if a, ok := result.Layers["promos"]; ok {
		t.Errorf("expected no assignment for future promo, got %v", a)
	}

	// Now is after effective_from (use past as effective_from)
	snap.Layers[0].Segments[0].Promotion.EffectiveFrom = &past
	result = e.Evaluate(snap, "user", map[string]interface{}{"x": "y"}, nil, time.Now())
	if result.Layers["promos"] == nil || result.Layers["promos"].Segment != "promo" {
		t.Errorf("expected promo, got %v", result.Layers["promos"])
	}
}

func TestEvaluator_LayerFilter(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{Name: "a", Order: 1, Segments: []model.Segment{{ID: "s", Strategy: "static", Static: &model.StaticConfig{Default: "a-val"}}}},
			{Name: "b", Order: 2, Segments: []model.Segment{{ID: "s", Strategy: "static", Static: &model.StaticConfig{Default: "b-val"}}}},
		},
	}

	e := newTestEvaluator(0)
	result := e.Evaluate(snap, "user", nil, []string{"b"}, time.Now())
	if _, ok := result.Layers["a"]; ok {
		t.Error("expected layer 'a' to be filtered out")
	}
	if result.Layers["b"] == nil || result.Layers["b"].Segment != "b-val" {
		t.Errorf("expected b-val, got %v", result.Layers["b"])
	}
}

func TestEvaluator_OverrideTakesPriority(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Name:  "test",
				Order: 1,
				Segments: []model.Segment{
					{
						ID:       "seg",
						Strategy: "static",
						Static:   &model.StaticConfig{Default: "normal"},
						Overrides: []model.Rule{
							{
								RuleName:     "vip-override",
								SuccessEvent: "override-val",
								Expression:   &model.Expression{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
							},
						},
					},
				},
			},
		},
	}

	e := newTestEvaluator(0)

	// Override matches
	result := e.Evaluate(snap, "user", map[string]interface{}{"plan": "enterprise"}, nil, time.Now())
	if result.Layers["test"] == nil || result.Layers["test"].Segment != "override-val" {
		t.Errorf("expected override-val, got %v", result.Layers["test"])
	}
	if result.Layers["test"].Strategy != "override" {
		t.Errorf("expected strategy override, got %s", result.Layers["test"].Strategy)
	}

	// Override doesn't match, falls through to static
	result = e.Evaluate(snap, "user", map[string]interface{}{"plan": "free"}, nil, time.Now())
	if result.Layers["test"] == nil || result.Layers["test"].Segment != "normal" {
		t.Errorf("expected normal, got %v", result.Layers["test"])
	}
}
