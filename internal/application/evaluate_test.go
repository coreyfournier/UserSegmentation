package application

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/engine"
	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
	"github.com/segmentation-service/segmentation/internal/infrastructure/store"
)

type fixedHasher struct{ bucket int }

func (h *fixedHasher) Bucket(_, _ string) int { return h.bucket }

func newTestEvaluateUC() (*EvaluateUseCase, *store.Memory) {
	memStore := store.NewMemory()
	strategies := map[string]strategy.Strategy{
		"static":     &strategy.StaticStrategy{},
		"rule":       &strategy.RuleStrategy{},
		"percentage": &strategy.PercentageStrategy{Hasher: &fixedHasher{bucket: 10}},
	}
	evaluator := engine.NewEvaluator(strategies)
	uc := NewEvaluateUseCase(memStore, evaluator)
	return uc, memStore
}

func testSnapshot() *model.Snapshot {
	return &model.Snapshot{
		Version: 1,
		Layers: []model.Layer{
			{
				Name:  "tier",
				Order: 1,
				Segments: []model.Segment{
					{
						ID:       "lookup",
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
}

// --- EvaluateUseCase ---

func TestEvaluateUseCase_Success(t *testing.T) {
	uc, s := newTestEvaluateUC()
	s.Swap(testSnapshot())

	resp, err := uc.Execute(EvaluateRequest{
		SubjectKey: "vip",
		Context:    map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.SubjectKey != "vip" {
		t.Errorf("expected subject_key=vip, got %s", resp.SubjectKey)
	}
	if resp.Layers["tier"].Segment != "platinum" {
		t.Errorf("expected platinum, got %s", resp.Layers["tier"].Segment)
	}
	if resp.DurationUS < 0 {
		t.Error("expected non-negative duration")
	}
	if resp.EvaluatedAt == "" {
		t.Error("expected evaluated_at to be set")
	}
}

func TestEvaluateUseCase_DefaultSegment(t *testing.T) {
	uc, s := newTestEvaluateUC()
	s.Swap(testSnapshot())

	resp, err := uc.Execute(EvaluateRequest{
		SubjectKey: "unknown",
		Context:    map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Layers["tier"].Segment != "standard" {
		t.Errorf("expected standard, got %s", resp.Layers["tier"].Segment)
	}
}

func TestEvaluateUseCase_NoConfig(t *testing.T) {
	uc, _ := newTestEvaluateUC()
	_, err := uc.Execute(EvaluateRequest{SubjectKey: "x"})
	if err != ErrNoConfig {
		t.Errorf("expected ErrNoConfig, got %v", err)
	}
}

func TestEvaluateUseCase_NilContext(t *testing.T) {
	uc, s := newTestEvaluateUC()
	s.Swap(testSnapshot())

	resp, err := uc.Execute(EvaluateRequest{SubjectKey: "vip"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Layers["tier"].Segment != "platinum" {
		t.Errorf("expected platinum, got %s", resp.Layers["tier"].Segment)
	}
}

func TestEvaluateUseCase_LayerFilter(t *testing.T) {
	uc, s := newTestEvaluateUC()
	snap := testSnapshot()
	snap.Layers = append(snap.Layers, model.Layer{
		Name: "extra", Order: 2,
		Segments: []model.Segment{{ID: "s1", Strategy: "static", Static: &model.StaticConfig{Default: "x"}}},
	})
	s.Swap(snap)

	resp, err := uc.Execute(EvaluateRequest{
		SubjectKey: "vip",
		Context:    map[string]interface{}{},
		Layers:     []string{"tier"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := resp.Layers["extra"]; ok {
		t.Error("extra layer should be filtered out")
	}
	if resp.Layers["tier"].Segment != "platinum" {
		t.Errorf("expected platinum, got %s", resp.Layers["tier"].Segment)
	}
}

// --- BatchEvaluateUseCase ---

func TestBatchEvaluateUseCase_Success(t *testing.T) {
	uc, s := newTestEvaluateUC()
	s.Swap(testSnapshot())
	batchUC := NewBatchEvaluateUseCase(uc)

	resp, err := batchUC.Execute(BatchEvaluateRequest{
		Subjects: []EvaluateRequest{
			{SubjectKey: "vip", Context: map[string]interface{}{}},
			{SubjectKey: "other", Context: map[string]interface{}{}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(resp.Results))
	}
	if resp.Results[0].Layers["tier"].Segment != "platinum" {
		t.Errorf("expected platinum for vip")
	}
	if resp.Results[1].Layers["tier"].Segment != "standard" {
		t.Errorf("expected standard for other")
	}
}

func TestBatchEvaluateUseCase_EmptySubjects(t *testing.T) {
	uc, s := newTestEvaluateUC()
	s.Swap(testSnapshot())
	batchUC := NewBatchEvaluateUseCase(uc)

	resp, err := batchUC.Execute(BatchEvaluateRequest{Subjects: []EvaluateRequest{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(resp.Results))
	}
}

func TestBatchEvaluateUseCase_NoConfig(t *testing.T) {
	uc, _ := newTestEvaluateUC()
	batchUC := NewBatchEvaluateUseCase(uc)

	resp, err := batchUC.Execute(BatchEvaluateRequest{
		Subjects: []EvaluateRequest{{SubjectKey: "x"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should get an empty-layer result, not a crash
	if resp.Results[0].SubjectKey != "x" {
		t.Errorf("expected subject_key=x, got %s", resp.Results[0].SubjectKey)
	}
	if len(resp.Results[0].Layers) != 0 {
		t.Errorf("expected empty layers on error, got %d", len(resp.Results[0].Layers))
	}
}

func TestBatchEvaluateUseCase_PreservesOrder(t *testing.T) {
	uc, s := newTestEvaluateUC()
	s.Swap(testSnapshot())
	batchUC := NewBatchEvaluateUseCase(uc)

	keys := []string{"a", "b", "c", "d", "e"}
	reqs := make([]EvaluateRequest, len(keys))
	for i, k := range keys {
		reqs[i] = EvaluateRequest{SubjectKey: k, Context: map[string]interface{}{}}
	}

	resp, err := batchUC.Execute(BatchEvaluateRequest{Subjects: reqs})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, k := range keys {
		if resp.Results[i].SubjectKey != k {
			t.Errorf("result[%d] subject_key = %s, want %s", i, resp.Results[i].SubjectKey, k)
		}
	}
}
