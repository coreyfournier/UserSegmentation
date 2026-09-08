package engine

import (
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
)

// twoSegmentSnapshot mirrors the shape that prompted OnlySegments: one layer,
// two segments with the same `when`, so the first always resolves and the
// second can never be reached.
func twoSegmentSnapshot() *model.Snapshot {
	when := &model.Rule{
		RuleName:  "applies",
		Condition: &model.Condition{Field: "productType", Operator: model.OpEq, Value: "T&A"},
	}
	seg := func(id, event string) model.Segment {
		return model.Segment{
			ID:       id,
			Strategy: model.StrategyRule,
			When:     when,
			Rules: []model.Rule{{
				RuleName:     id + "-rule",
				SuccessEvent: event,
				Condition:    &model.Condition{Field: "age", Operator: model.OpGte, Value: 18},
			}},
		}
	}
	return &model.Snapshot{Layers: []model.Layer{{
		Key: "gates",
		InputSchema: model.InputSchema{
			"productType": {Type: model.FieldTypeString},
			"age":         {Type: model.FieldTypeNumber},
		},
		Segments: []model.Segment{seg("stage1", "first"), seg("stage2", "second")},
	}}}
}

func TestOnlySegments_ReachesASegmentBehindAnother(t *testing.T) {
	snap := twoSegmentSnapshot()
	ctx := map[string]interface{}{"productType": "T&A", "age": 30.0}
	ev := segFilterEvaluator()

	// Without the restriction the first segment wins, every time — which is
	// the behaviour that made the second one untestable.
	unfiltered := ev.Evaluate(snap, ctx, []string{"gates"}, nil, false, time.Now())
	if got := unfiltered.Layers["gates"].Assignment.Segment; got != "first" {
		t.Fatalf("expected the first segment to resolve, got %q", got)
	}

	// Restricted to the second, it is the only one considered.
	filtered := ev.Evaluate(snap, ctx, []string{"gates"}, nil, false, time.Now(), OnlySegments([]string{"stage2"}))
	if got := filtered.Layers["gates"].Assignment.Segment; got != "second" {
		t.Errorf("expected the second segment to resolve, got %q", got)
	}
}

// An empty list is no restriction, so a caller may pass whatever it has.
func TestOnlySegments_EmptyIsNoRestriction(t *testing.T) {
	snap := twoSegmentSnapshot()
	ctx := map[string]interface{}{"productType": "T&A", "age": 30.0}
	ev := segFilterEvaluator()

	for _, ids := range [][]string{nil, {}} {
		res := ev.Evaluate(snap, ctx, []string{"gates"}, nil, false, time.Now(), OnlySegments(ids))
		if res.Layers["gates"].Assignment == nil || res.Layers["gates"].Assignment.Segment != "first" {
			t.Errorf("OnlySegments(%v) should not restrict anything", ids)
		}
	}
}

// A layer evaluated only because something depends on it must run in full.
// Narrowing it would change what the layer under test resolves against, and
// the answer would be about a configuration that does not exist.
func TestOnlySegments_DependenciesRunInFull(t *testing.T) {
	snap := twoSegmentSnapshot()
	snap.Layers = append(snap.Layers, model.Layer{
		Key:         "downstream",
		DependsOn:   []string{"gates"},
		InputSchema: model.InputSchema{"age": {Type: model.FieldTypeNumber}},
		Segments: []model.Segment{{
			ID:       "only",
			Strategy: model.StrategyRule,
			Rules: []model.Rule{{
				RuleName:     "gated",
				SuccessEvent: "downstream-ran",
				Condition:    &model.Condition{Field: "layer:gates", Operator: model.OpEq, Value: "first"},
			}},
		}},
	})
	ctx := map[string]interface{}{"productType": "T&A", "age": 30.0}

	// The filter names the requested layer's own segment. The dependency has
	// no segment by that name, so if the filter reached it, it would evaluate
	// nothing, inject no "layer:gates", and the downstream rule could not
	// match. Downstream resolving is the proof that it did not.
	res := segFilterEvaluator().Evaluate(
		snap, ctx, []string{"downstream"}, nil, false, time.Now(), OnlySegments([]string{"only"}))

	lr := res.Layers["downstream"]
	if lr == nil || lr.Assignment == nil {
		t.Fatalf("downstream did not resolve: %+v", lr)
	}
	if lr.Assignment.Segment != "downstream-ran" {
		t.Errorf("dependency was narrowed by the filter: got %q", lr.Assignment.Segment)
	}
	if _, reported := res.Layers["gates"]; reported {
		t.Error("a dependency should not appear in the output when it was not requested")
	}
}

// segFilterEvaluator is the evaluator these tests run against. Separate from
// evaluator_test.go's newTestEvaluator, which takes a fixed hash bucket these
// cases have no use for.
func segFilterEvaluator() *Evaluator {
	return NewEvaluator(map[string]strategy.Strategy{
		model.StrategyRule:      &strategy.RuleStrategy{},
		model.StrategyChecklist: &strategy.ChecklistStrategy{},
	})
}
