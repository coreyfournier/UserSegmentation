package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestPanicRecovery_NoPanic(t *testing.T) {
	handler := PanicRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestPanicRecovery_WithPanic(t *testing.T) {
	handler := PanicRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "internal server error") {
		t.Errorf("expected error message, got %s", w.Body.String())
	}
}

func TestTiming_SetsHeader(t *testing.T) {
	handler := Timing(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	timing := w.Header().Get("Server-Timing")
	if timing == "" {
		t.Error("expected Server-Timing header")
	}
	if !strings.HasPrefix(timing, "total;dur=") {
		t.Errorf("unexpected Server-Timing format: %s", timing)
	}
}

func TestCORS_SetsHeaders(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS origin header")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("expected CORS methods header")
	}
	if w.Header().Get("Access-Control-Allow-Headers") == "" {
		t.Error("expected CORS headers header")
	}
}

func TestCORS_OptionsPreflight(t *testing.T) {
	called := false
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	req := httptest.NewRequest("OPTIONS", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204 for OPTIONS, got %d", w.Code)
	}
	if called {
		t.Error("next handler should not be called for OPTIONS")
	}
}

func TestLogging_PassesThrough(t *testing.T) {
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

func TestStatusWriter_CapturesCode(t *testing.T) {
	handler := Logging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
	if !strings.Contains(w.Body.String(), `"key":"value"`) {
		t.Errorf("unexpected body: %s", w.Body.String())
	}
}

// --- Segments Handler edge case ---

type nilStore struct{}

func (s *nilStore) Get() *model.Snapshot    { return nil }
func (s *nilStore) Swap(_ *model.Snapshot) {}

func TestSegmentsHandler_NoConfig(t *testing.T) {
	handler := &SegmentsHandler{store: &nilStore{}}
	req := httptest.NewRequest("GET", "/v1/segments", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

// --- Evaluate Handler edge cases ---

func TestEvaluateHandler_InvalidJSON(t *testing.T) {
	handler := &EvaluateHandler{uc: nil}
	req := httptest.NewRequest("POST", "/v1/evaluate", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// --- Batch Handler edge cases ---

func TestBatchHandler_InvalidJSON(t *testing.T) {
	handler := &BatchHandler{uc: nil}
	req := httptest.NewRequest("POST", "/v1/evaluate/batch", strings.NewReader("{invalid"))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestBatchHandler_EmptySubjects(t *testing.T) {
	handler := &BatchHandler{uc: nil}
	req := httptest.NewRequest("POST", "/v1/evaluate/batch", strings.NewReader(`{"subjects":[]}`))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
