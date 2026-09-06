package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
	"github.com/segmentation-service/segmentation/internal/infrastructure/hash"
)

func subjectKeyEvaluator() *Evaluator {
	return NewEvaluator(map[string]strategy.Strategy{
		model.StrategyStatic:     &strategy.StaticStrategy{},
		model.StrategyRule:       &strategy.RuleStrategy{},
		model.StrategyPercentage: &strategy.PercentageStrategy{Hasher: &hash.FNV{}},
		model.StrategyChecklist:  &strategy.ChecklistStrategy{},
	})
}

func staticSnapshot() *model.Snapshot {
	return &model.Snapshot{Layers: []model.Layer{{
		Key:         "tier",
		InputSchema: model.InputSchema{model.SubjectKeyField: {Type: model.FieldTypeString}},
		Segments: []model.Segment{{
			ID:       "userTier",
			Strategy: model.StrategyStatic,
			Static:   &model.StaticConfig{Mappings: map[string]string{"emp-1": "platinum"}, Default: "standard"},
		}},
	}}}
}

// The subject key is an ordinary context field now.
func TestSubjectKey_ResolvedFromContext(t *testing.T) {
	res := subjectKeyEvaluator().Evaluate(
		staticSnapshot(),
		map[string]interface{}{model.SubjectKeyField: "emp-1"},
		nil, nil, false, time.Now(),
	)
	lr := res.Layers["tier"]
	if lr == nil || lr.Assignment == nil {
		t.Fatalf("expected an assignment, got %+v", lr)
	}
	if lr.Assignment.Segment != "platinum" {
		t.Errorf("expected platinum, got %q", lr.Assignment.Segment)
	}
}

// A non-string key is converted rather than rejected — the declared type is the
// author's choice.
func TestSubjectKey_NonStringIsConverted(t *testing.T) {
	snap := staticSnapshot()
	snap.Layers[0].InputSchema[model.SubjectKeyField] = model.SchemaField{Type: model.FieldTypeNumber}
	snap.Layers[0].Segments[0].Static.Mappings = map[string]string{"1001": "gold"}

	res := subjectKeyEvaluator().Evaluate(
		snap,
		// JSON numbers arrive as float64, which is what the conversion must handle.
		map[string]interface{}{model.SubjectKeyField: float64(1001)},
		nil, nil, false, time.Now(),
	)
	if got := res.Layers["tier"].Assignment.Segment; got != "gold" {
		t.Errorf("expected gold from a numeric subject key, got %q", got)
	}
}

// The whole point of the change: a layer with no static or percentage segment
// needs no subject key at all, and evaluating without one is unremarkable.
func TestSubjectKey_NotNeededByOtherStrategies(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{{
		Key:         "gate",
		InputSchema: model.InputSchema{"plan": {Type: model.FieldTypeString}},
		Segments: []model.Segment{{
			ID:       "check",
			Strategy: model.StrategyRule,
			Rules: []model.Rule{{
				RuleName:     "isPro",
				SuccessEvent: "pro",
				Condition:    &model.Condition{Field: "plan", Operator: model.OpEq, Value: "pro"},
			}},
		}},
	}}}

	res := subjectKeyEvaluator().Evaluate(
		snap,
		map[string]interface{}{"plan": "pro"},
		nil, nil, false, time.Now(),
	)
	lr := res.Layers["gate"]
	if lr.Assignment == nil || lr.Assignment.Segment != "pro" {
		t.Fatalf("expected pro, got %+v", lr.Assignment)
	}
	for _, w := range res.Warnings {
		if w.Field == model.SubjectKeyField {
			t.Errorf("a rule layer must not warn about the subject key: %+v", w)
		}
	}
}

// Absent, empty and nil all mean "no subject key". Each would otherwise be a
// silent wrong answer: static falls to its default, percentage buckets on "".
func TestSubjectKey_MissingIsUnevaluable(t *testing.T) {
	for name, ctx := range map[string]map[string]interface{}{
		"absent": {},
		"empty":  {model.SubjectKeyField: ""},
		"nil":    {model.SubjectKeyField: nil},
	} {
		t.Run(name, func(t *testing.T) {
			res := subjectKeyEvaluator().Evaluate(staticSnapshot(), ctx, nil, nil, false, time.Now())
			lr := res.Layers["tier"]
			if lr.Status != model.StatusUnevaluable {
				t.Errorf("expected %q, got %q", model.StatusUnevaluable, lr.Status)
			}
			if lr.Assignment != nil {
				t.Errorf("expected no assignment, got %+v", lr.Assignment)
			}
			var warned bool
			for _, w := range res.Warnings {
				if w.Field == model.SubjectKeyField && w.Segment == "userTier" {
					warned = true
				}
			}
			if !warned {
				t.Errorf("expected a warning naming the field, got %+v", res.Warnings)
			}
		})
	}
}

// Percentage is the more dangerous of the two: hashing "" is not an error, it
// just puts every subject in the same bucket.
func TestSubjectKey_MissingStopsPercentageBucketing(t *testing.T) {
	snap := &model.Snapshot{Layers: []model.Layer{{
		Key:         "rollout",
		InputSchema: model.InputSchema{model.SubjectKeyField: {Type: model.FieldTypeString}},
		Segments: []model.Segment{{
			ID:       "checkout",
			Strategy: model.StrategyPercentage,
			Percentage: &model.PercentageConfig{Salt: "v2", Buckets: []model.PercentageBucket{
				{Segment: "control", Weight: 50}, {Segment: "variant", Weight: 50},
			}},
		}},
	}}}

	res := subjectKeyEvaluator().Evaluate(snap, map[string]interface{}{}, nil, nil, false, time.Now())
	lr := res.Layers["rollout"]
	if lr.Status != model.StatusUnevaluable {
		t.Errorf("expected %q, got %q", model.StatusUnevaluable, lr.Status)
	}
	if lr.Assignment != nil {
		t.Errorf("expected no bucket to be assigned, got %+v", lr.Assignment)
	}
}

// The field is normally declared required, so the generic missing-field
// warning fires too. Only the specific one should survive: it names the
// strategy and says the segment did not run, which the generic one does not.
func TestSubjectKey_MissingWarnsOnceNotTwice(t *testing.T) {
	snap := staticSnapshot()
	snap.Layers[0].InputSchema[model.SubjectKeyField] = model.SchemaField{
		Type: model.FieldTypeString, Required: true,
	}

	res := subjectKeyEvaluator().Evaluate(snap, map[string]interface{}{}, nil, nil, false, time.Now())

	var forField []model.Warning
	for _, w := range res.Warnings {
		if w.Field == model.SubjectKeyField {
			forField = append(forField, w)
		}
	}
	if len(forField) != 1 {
		t.Fatalf("expected exactly one warning for the field, got %d: %+v", len(forField), forField)
	}
	if !strings.Contains(forField[0].Message, "static") {
		t.Errorf("expected the specific warning naming the strategy, got %q", forField[0].Message)
	}
}
