package http

import (
	"net/http"

	"github.com/segmentation-service/segmentation/internal/application"
	"github.com/segmentation-service/segmentation/internal/domain/ports"
)

// NewServer creates an HTTP server with all routes configured.
func NewServer(
	addr string,
	evaluateUC *application.EvaluateUseCase,
	batchUC *application.BatchEvaluateUseCase,
	reloadUC *application.ReloadUseCase,
	store ports.SegmentStore,
) *http.Server {
	mux := http.NewServeMux()

	mux.Handle("POST /v1/evaluate", &EvaluateHandler{uc: evaluateUC})
	mux.Handle("POST /v1/evaluate/batch", &BatchHandler{uc: batchUC})
	mux.Handle("GET /v1/segments", &SegmentsHandler{store: store})
	mux.Handle("GET /v1/health", &HealthHandler{store: store})
	mux.Handle("POST /v1/reload", &ReloadHandler{uc: reloadUC})

	handler := PanicRecovery(Timing(Logging(mux)))

	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}
}
