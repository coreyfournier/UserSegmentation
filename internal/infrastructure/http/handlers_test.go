package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/segmentation-service/segmentation/internal/application"
	"github.com/segmentation-service/segmentation/internal/domain/engine"
	"github.com/segmentation-service/segmentation/internal/domain/model"
	"github.com/segmentation-service/segmentation/internal/domain/strategy"
	"github.com/segmentation-service/segmentation/internal/infrastructure/store"
)

type testHasher struct{}

func (h *testHasher) Bucket(_, _ string) int { return 0 } // implements ports.Hasher

func setupTestServer() (*http.ServeMux, *store.Memory) {
	memStore := store.NewMemory()
	snap := &model.Snapshot{
		Version: 1,
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
	memStore.Swap(snap)

	strategies := map[string]strategy.Strategy{
		"static":     &strategy.StaticStrategy{},
		"rule":       &strategy.RuleStrategy{},
		"percentage": &strategy.PercentageStrategy{Hasher: &testHasher{}},
	}
	evaluator := engine.NewEvaluator(strategies)
	evaluateUC := application.NewEvaluateUseCase(memStore, evaluator)
	batchUC := application.NewBatchEvaluateUseCase(evaluateUC)

	mux := http.NewServeMux()
	mux.Handle("POST /v1/evaluate", &EvaluateHandler{uc: evaluateUC})
	mux.Handle("POST /v1/evaluate/batch", &BatchHandler{uc: batchUC})
	mux.Handle("GET /v1/segments", &SegmentsHandler{store: memStore})
	mux.Handle("GET /v1/health", &HealthHandler{store: memStore})

	return mux, memStore
}

func TestEvaluateHandler_Success(t *testing.T) {
	mux, _ := setupTestServer()

	body := `{"subject_key":"vip","context":{}}`
	req := httptest.NewRequest("POST", "/v1/evaluate", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp application.EvaluateResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Layers["base-tier"].Segment != "platinum" {
		t.Errorf("expected platinum, got %s", resp.Layers["base-tier"].Segment)
	}
}

func TestEvaluateHandler_MissingSubjectKey(t *testing.T) {
	mux, _ := setupTestServer()

	body := `{"context":{}}`
	req := httptest.NewRequest("POST", "/v1/evaluate", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBatchHandler_Success(t *testing.T) {
	mux, _ := setupTestServer()

	body := `{"subjects":[{"subject_key":"vip","context":{}},{"subject_key":"other","context":{}}]}`
	req := httptest.NewRequest("POST", "/v1/evaluate/batch", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp application.BatchEvaluateResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(resp.Results))
	}
}

func TestHealthHandler(t *testing.T) {
	mux, _ := setupTestServer()

	req := httptest.NewRequest("GET", "/v1/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "healthy" {
		t.Errorf("expected healthy, got %v", resp["status"])
	}
}

func TestSegmentsHandler(t *testing.T) {
	mux, _ := setupTestServer()

	req := httptest.NewRequest("GET", "/v1/segments", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestHealthHandler_NoConfig(t *testing.T) {
	emptyStore := store.NewMemory()
	mux := http.NewServeMux()
	mux.Handle("GET /v1/health", &HealthHandler{store: emptyStore})

	req := httptest.NewRequest("GET", "/v1/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "degraded" {
		t.Errorf("expected degraded, got %v", resp["status"])
	}
}
