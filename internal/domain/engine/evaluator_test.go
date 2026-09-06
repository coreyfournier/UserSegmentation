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
				Key: "baseTier",
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
	result := e.Evaluate(snap, map[string]interface{}{"subjectKey": "vip"}, nil, nil, false, time.Now())
	if result.Layers["baseTier"] == nil || result.Layers["baseTier"].Assignment.Segment != "platinum" {
		t.Errorf("expected platinum, got %v", result.Layers["baseTier"])
	}

	result = e.Evaluate(snap, map[string]interface{}{"subjectKey": "other"}, nil, nil, false, time.Now())
	if result.Layers["baseTier"] == nil || result.Layers["baseTier"].Assignment.Segment != "standard" {
		t.Errorf("expected standard, got %v", result.Layers["baseTier"])
	}
}

func TestEvaluator_CrossLayerDependency(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Key: "baseTier",
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
				Key: "promotions",
				Segments: []model.Segment{
					{
						ID:       "promo",
						Strategy: "rule",
						Rules: []model.Rule{
							{
								RuleName:     "pro-promo",
								SuccessEvent: "special-offer",
								Condition:    &model.Condition{Field: "layer:baseTier", Operator: model.OpEq, Value: "pro"},
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
	result := e.Evaluate(snap, map[string]interface{}{"subjectKey": "vip"}, nil, nil, false, time.Now())
	if result.Layers["promotions"] == nil || result.Layers["promotions"].Assignment.Segment != "special-offer" {
		t.Errorf("expected special-offer, got %v", result.Layers["promotions"])
	}

	// Non-VIP gets free tier, promo defaults to none
	result = e.Evaluate(snap, map[string]interface{}{"subjectKey": "other"}, nil, nil, false, time.Now())
	if result.Layers["promotions"] == nil || result.Layers["promotions"].Assignment.Segment != "none" {
		t.Errorf("expected none, got %v", result.Layers["promotions"])
	}
}

func TestEvaluator_PromotionTimeGating(t *testing.T) {
	future := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Key: "promos",
				Segments: []model.Segment{
					{
						ID:       "future-promo",
						Strategy: "rule",
						Promotion: &model.Promotion{
							EffectiveFrom: &future,
						},
						Rules: []model.Rule{
							{RuleName: "always", SuccessEvent: "promo", Condition: &model.Condition{Field: "x", Operator: model.OpEq, Value: "y"}},
						},
						Default: "none",
					},
				},
			},
		},
	}

	e := newTestEvaluator(0)

	// Now is before effective_from, segment should be skipped. The layer still
	// reports a status — it resolved to nothing rather than being absent.
	result := e.Evaluate(snap, map[string]interface{}{"subjectKey": "user", "x": "y"}, nil, nil, false, time.Now())
	lr, ok := result.Layers["promos"]
	if !ok {
		t.Fatal("expected the layer to report a status")
	}
	if lr.Assignment != nil {
		t.Errorf("expected no assignment for future promo, got %v", lr.Assignment)
	}
	if lr.Status != model.StatusUnresolved {
		t.Errorf("expected status %q, got %q", model.StatusUnresolved, lr.Status)
	}

	// Now is after effective_from (use past as effective_from)
	snap.Layers[0].Segments[0].Promotion.EffectiveFrom = &past
	result = e.Evaluate(snap, map[string]interface{}{"subjectKey": "user", "x": "y"}, nil, nil, false, time.Now())
	if result.Layers["promos"] == nil || result.Layers["promos"].Assignment.Segment != "promo" {
		t.Errorf("expected promo, got %v", result.Layers["promos"])
	}
}

func TestEvaluator_LayerFilter(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{Key: "a", Segments: []model.Segment{{ID: "s", Strategy: "static", Static: &model.StaticConfig{Default: "a-val"}}}},
			{Key: "b", Segments: []model.Segment{{ID: "s", Strategy: "static", Static: &model.StaticConfig{Default: "b-val"}}}},
		},
	}

	e := newTestEvaluator(0)
	result := e.Evaluate(snap, map[string]interface{}{"subjectKey": "user"}, []string{"b"}, nil, false, time.Now())
	if _, ok := result.Layers["a"]; ok {
		t.Error("expected layer 'a' to be filtered out")
	}
	if result.Layers["b"] == nil || result.Layers["b"].Assignment.Segment != "b-val" {
		t.Errorf("expected b-val, got %v", result.Layers["b"])
	}
}

func TestEvaluator_OverrideTakesPriority(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{
			{
				Key: "test",
				Segments: []model.Segment{
					{
						ID:       "seg",
						Strategy: "static",
						Static:   &model.StaticConfig{Default: "normal"},
						Overrides: []model.Rule{
							{
								RuleName:     "vip-override",
								SuccessEvent: "override-val",
								Condition:    &model.Condition{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
							},
						},
					},
				},
			},
		},
	}

	e := newTestEvaluator(0)

	// Override matches
	result := e.Evaluate(snap, map[string]interface{}{"subjectKey": "user", "plan": "enterprise"}, nil, nil, false, time.Now())
	if result.Layers["test"] == nil || result.Layers["test"].Assignment.Segment != "override-val" {
		t.Errorf("expected override-val, got %v", result.Layers["test"])
	}
	if result.Layers["test"].Assignment.Strategy != "override" {
		t.Errorf("expected strategy override, got %s", result.Layers["test"].Assignment.Strategy)
	}

	// Override doesn't match, falls through to static
	result = e.Evaluate(snap, map[string]interface{}{"subjectKey": "user", "plan": "free"}, nil, nil, false, time.Now())
	if result.Layers["test"] == nil || result.Layers["test"].Assignment.Segment != "normal" {
		t.Errorf("expected normal, got %v", result.Layers["test"])
	}
}
