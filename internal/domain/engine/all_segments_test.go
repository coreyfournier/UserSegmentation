package engine

import (
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// checklistLayer builds a layer of checklist segments that share a `when`, so
// every one of them applies to the same subject — the shape that prompted this
// behaviour.
func gatesChecklistLayer(firstMatchOnly bool) *model.Layer {
	when := &model.Rule{
		RuleName:  "applies",
		Condition: &model.Condition{Field: "productType", Operator: model.OpEq, Value: "T&A"},
	}
	check := func(id, rule string, threshold int) model.Segment {
		return model.Segment{
			ID:       id,
			Strategy: model.StrategyChecklist,
			When:     when,
			Rules: []model.Rule{{
				RuleName:     rule,
				ErrorMessage: rule + " fired",
				Condition:    &model.Condition{Field: "age", Operator: model.OpLt, Value: threshold},
			}},
		}
	}
	return &model.Layer{
		Key:            "gates",
		FirstMatchOnly: firstMatchOnly,
		InputSchema: model.InputSchema{
			"productType": {Type: model.FieldTypeString},
			"age":         {Type: model.FieldTypeNumber},
		},
		Segments: []model.Segment{check("stage1", "S1", 18), check("stage2", "S2", 21)},
	}
}

func gatesResult(t *testing.T, layer *model.Layer, ctx map[string]interface{}) *LayerResult {
	t.Helper()
	snap := &model.Snapshot{Layers: []model.Layer{*layer}}
	res := segFilterEvaluator().Evaluate(snap, ctx, []string{"gates"}, nil, false, time.Now())
	lr := res.Layers["gates"]
	if lr == nil {
		t.Fatal("no result for the layer")
	}
	return lr
}

// The default: every applicable checklist segment runs and their findings
// merge. A checklist resolves no value, so there is nothing for a second one
// to conflict with.
func TestAllSegments_ChecklistsMerge(t *testing.T) {
	lr := gatesResult(t, gatesChecklistLayer(false), map[string]interface{}{
		"productType": "T&A", "age": 19.0,
	})

	if lr.Status != model.StatusViolated {
		t.Errorf("status = %q, want violated", lr.Status)
	}
	if len(lr.Failures) != 1 {
		t.Fatalf("expected stage2's finding, got %+v", lr.Failures)
	}
	if lr.Failures[0].Rule != "S2" || lr.Failures[0].Segment != "stage2" {
		t.Errorf("unexpected finding %+v", lr.Failures[0])
	}
	// Both segments ran, so the reason names both.
	if lr.Assignment == nil || lr.Assignment.Reason != "stage1 + stage2" {
		t.Errorf("reason = %+v, want both contributors", lr.Assignment)
	}
}

// Both reporting: the findings appear in evaluation order, each stamped with
// the segment that produced it.
func TestAllSegments_BothReport(t *testing.T) {
	lr := gatesResult(t, gatesChecklistLayer(false), map[string]interface{}{
		"productType": "T&A", "age": 15.0,
	})

	if len(lr.Failures) != 2 {
		t.Fatalf("expected both findings, got %+v", lr.Failures)
	}
	if lr.Failures[0].Segment != "stage1" || lr.Failures[1].Segment != "stage2" {
		t.Errorf("findings out of order or unstamped: %+v", lr.Failures)
	}
}

// Opting out restores the old behaviour exactly: the first applicable segment
// answers and the rest never run.
func TestAllSegments_FirstMatchOnlyStopsAtOne(t *testing.T) {
	lr := gatesResult(t, gatesChecklistLayer(true), map[string]interface{}{
		"productType": "T&A", "age": 15.0,
	})

	if len(lr.Failures) != 1 || lr.Failures[0].Rule != "S1" {
		t.Fatalf("expected only stage1's finding, got %+v", lr.Failures)
	}
	// One contributor, so nothing is stamped and the reason is unchanged.
	if lr.Failures[0].Segment != "" {
		t.Errorf("a single-segment result should carry no segment stamp: %+v", lr.Failures[0])
	}
	if lr.Assignment.Reason != "checklist:stage1" {
		t.Errorf("reason = %q, want the single-segment form", lr.Assignment.Reason)
	}
}

// A layer whose one segment reports looks exactly as it always did: no stamp,
// no joined reason. This is what keeps every existing config's response stable.
func TestAllSegments_SingleSegmentUnchanged(t *testing.T) {
	layer := gatesChecklistLayer(false)
	layer.Segments = layer.Segments[:1]
	lr := gatesResult(t, layer, map[string]interface{}{"productType": "T&A", "age": 15.0})

	if len(lr.Failures) != 1 || lr.Failures[0].Segment != "" {
		t.Errorf("unexpected findings %+v", lr.Failures)
	}
	if lr.Assignment.Reason != "checklist:stage1" {
		t.Errorf("reason = %q", lr.Assignment.Reason)
	}
}

// `when` still gates each segment on its own: a segment that does not apply
// contributes nothing, and the others carry on.
func TestAllSegments_WhenStillGatesIndividually(t *testing.T) {
	layer := gatesChecklistLayer(false)
	layer.Segments[0].When = &model.Rule{
		RuleName:  "never",
		Condition: &model.Condition{Field: "productType", Operator: model.OpEq, Value: "other"},
	}
	lr := gatesResult(t, layer, map[string]interface{}{"productType": "T&A", "age": 15.0})

	if len(lr.Failures) != 1 || lr.Failures[0].Rule != "S2" {
		t.Errorf("only stage2 applies, got %+v", lr.Failures)
	}
	if lr.Failures[0].Segment != "" {
		t.Error("one contributor needs no stamp")
	}
}

// A value-resolving segment ends the layer regardless of the default, because
// the layer has exactly one answer and a second value has nowhere to go. This
// is what keeps transferFee's promo-then-standard arrangement working.
func TestAllSegments_ValueResolvingStillFirstWins(t *testing.T) {
	seg := func(id, event string) model.Segment {
		return model.Segment{
			ID:       id,
			Strategy: model.StrategyRule,
			Rules: []model.Rule{{
				RuleName:     id + "-r",
				SuccessEvent: event,
				Condition:    &model.Condition{Field: "age", Operator: model.OpGte, Value: 18},
			}},
		}
	}
	layer := &model.Layer{
		Key:         "fee",
		InputSchema: model.InputSchema{"age": {Type: model.FieldTypeNumber}},
		Segments:    []model.Segment{seg("promo", "promo-fee"), seg("standard", "standard-fee")},
	}
	snap := &model.Snapshot{Layers: []model.Layer{*layer}}
	res := segFilterEvaluator().Evaluate(snap, map[string]interface{}{"age": 30.0},
		[]string{"fee"}, nil, false, time.Now())

	if got := res.Layers["fee"].Assignment.Segment; got != "promo-fee" {
		t.Errorf("the first resolving segment must win, got %q", got)
	}
}

// Merged status takes the worst, and unevaluable outranks violated: a partial
// list of findings must not be reported as though it were the whole answer.
func TestMergeStatus_WorstWins(t *testing.T) {
	cases := []struct {
		have, next, want model.LayerStatus
	}{
		{model.StatusSatisfied, model.StatusViolated, model.StatusViolated},
		{model.StatusViolated, model.StatusSatisfied, model.StatusViolated},
		{model.StatusViolated, model.StatusUnevaluable, model.StatusUnevaluable},
		{model.StatusUnevaluable, model.StatusViolated, model.StatusUnevaluable},
		{model.StatusSatisfied, model.StatusSatisfied, model.StatusSatisfied},
	}
	for _, tc := range cases {
		if got := mergeStatus(tc.have, tc.next, false); got != tc.want {
			t.Errorf("mergeStatus(%q, %q) = %q, want %q", tc.have, tc.next, got, tc.want)
		}
	}
	// The first contributor's status stands, whatever it is.
	if got := mergeStatus(model.StatusViolated, model.StatusSatisfied, true); got != model.StatusSatisfied {
		t.Errorf("the first contributor should set the status, got %q", got)
	}
}
