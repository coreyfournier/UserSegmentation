package application

import (
	"encoding/json"
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
		Name: "extra",
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

// Finding 5: the wire format has zero test coverage — nothing in this package
// or internal/infrastructure/http references Outputs. This runs a checklist
// segment exercising all three eval modes (literal, template, expression) plus
// a lookup-bound field with EmitOrder, and a rule segment reporting its own
// output directly on the layer result, then marshals the response to JSON and
// asserts the actual wire shape: the "outputs" key name on both FailureDTO and
// LayerResultDTO, and the {key, value, order} shape of the lookup-bound field.
func TestEvaluateUseCase_OutputsWireFormat(t *testing.T) {
	memStore := store.NewMemory()
	strategies := map[string]strategy.Strategy{
		"checklist": &strategy.ChecklistStrategy{},
		"rule":      &strategy.RuleStrategy{},
	}
	evaluator := engine.NewEvaluator(strategies)
	uc := NewEvaluateUseCase(memStore, evaluator)

	snap := &model.Snapshot{
		Version: 1,
		Lookups: []model.LookupTable{
			{
				ID:        "diagnosis-type",
				Name:      "Diagnosis Type",
				KeyType:   model.FieldTypeString,
				EmitOrder: true,
				Entries: []model.LookupEntry{
					{Key: "LowHours", Value: "Hours abnormally low", Order: 30},
				},
			},
		},
		Layers: []model.Layer{
			{
				Name: "diagnostics",
				Segments: []model.Segment{
					{
						ID:       "attendance",
						Strategy: model.StrategyChecklist,
						OutputSchema: model.OutputSchema{
							"category":      {Type: model.FieldTypeString}, // literal
							"description":   {Type: model.FieldTypeString, Eval: model.EvalTemplate},
							"signals":       {Type: model.FieldTypeObject, Eval: model.EvalExpression},
							"diagnosisType": {Type: model.FieldTypeString, Lookup: "diagnosis-type"},
						},
						Outputs: map[string]string{"category": "EmployeeAccountStatus"},
						Rules: []model.Rule{{
							RuleName:     "lowHours",
							ErrorMessage: "Hours look low.",
							Condition:    &model.Condition{Field: "totalHours", Operator: model.OpLt, Value: 2},
							Outputs: map[string]string{
								"description":   "${ totalHours } hours over ${ daysElapsed } days",
								"signals":       "{ TotalHours: totalHours }",
								"diagnosisType": "LowHours",
							},
						}},
					},
				},
			},
			{
				Name: "tier",
				Segments: []model.Segment{
					{
						ID:       "vip",
						Strategy: model.StrategyRule,
						OutputSchema: model.OutputSchema{
							"tier": {Type: model.FieldTypeString},
						},
						Rules: []model.Rule{{
							RuleName:     "matches",
							SuccessEvent: "vip-segment",
							Condition:    &model.Condition{Field: "plan", Operator: model.OpEq, Value: "enterprise"},
							Outputs:      map[string]string{"tier": "gold"},
						}},
					},
				},
			},
		},
	}
	memStore.Swap(snap)

	resp, err := uc.Execute(EvaluateRequest{
		SubjectKey: "user1",
		Context: map[string]interface{}{
			"totalHours":  1.5,
			"daysElapsed": 3,
			"plan":        "enterprise",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var wire map[string]interface{}
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	layers, ok := wire["layers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a \"layers\" object, got %v", wire["layers"])
	}

	// The checklist layer itself carries no outputs: they live per-failure.
	diag, ok := layers["diagnostics"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a \"diagnostics\" layer, got %v", layers)
	}
	if _, present := diag["outputs"]; present {
		t.Errorf("checklist layer result should carry no top-level \"outputs\" key, got %v", diag["outputs"])
	}
	failures, ok := diag["failures"].([]interface{})
	if !ok || len(failures) != 1 {
		t.Fatalf("expected one failure under \"failures\", got %v", diag["failures"])
	}
	failure, ok := failures[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a failure object, got %v", failures[0])
	}
	failureOutputs, ok := failure["outputs"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected the failure to carry an \"outputs\" key, got %v", failure)
	}

	if failureOutputs["category"] != "EmployeeAccountStatus" {
		t.Errorf("category (literal) = %v", failureOutputs["category"])
	}
	if failureOutputs["description"] != "1.5 hours over 3 days" {
		t.Errorf("description (template) = %v", failureOutputs["description"])
	}
	signals, ok := failureOutputs["signals"].(map[string]interface{})
	if !ok || signals["TotalHours"] != float64(1.5) {
		t.Errorf("signals (expression) = %v", failureOutputs["signals"])
	}
	diagnosisType, ok := failureOutputs["diagnosisType"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected diagnosisType to be enriched to a {key, value, order} object, got %v", failureOutputs["diagnosisType"])
	}
	if diagnosisType["key"] != "LowHours" || diagnosisType["value"] != "Hours abnormally low" || diagnosisType["order"] != float64(30) {
		t.Errorf("lookup enrichment {key, value, order} = %v", diagnosisType)
	}

	// The rule layer reports its output directly on the layer result.
	tier, ok := layers["tier"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a \"tier\" layer, got %v", layers)
	}
	tierOutputs, ok := tier["outputs"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected the layer result to carry an \"outputs\" key, got %v", tier)
	}
	if tierOutputs["tier"] != "gold" {
		t.Errorf("tier = %v", tierOutputs["tier"])
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
