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
	adminUC *application.AdminUseCase,
	store ports.SegmentStore,
) *http.Server {
	mux := http.NewServeMux()

	mux.Handle("POST /v1/evaluate", &EvaluateHandler{uc: evaluateUC})
	mux.Handle("POST /v1/evaluate/batch", &BatchHandler{uc: batchUC})
	mux.Handle("GET /v1/segments", &SegmentsHandler{store: store})
	mux.Handle("GET /v1/health", &HealthHandler{store: store})
	mux.Handle("POST /v1/reload", &ReloadHandler{uc: reloadUC})

	// Admin CRUD routes
	admin := NewAdminHandler(adminUC)
	mux.HandleFunc("GET /v1/admin/layers", admin.ListLayers)
	mux.HandleFunc("POST /v1/admin/layers", admin.CreateLayer)
	mux.HandleFunc("PUT /v1/admin/layers/{name}", admin.UpdateLayer)
	mux.HandleFunc("DELETE /v1/admin/layers/{name}", admin.DeleteLayer)
	mux.HandleFunc("GET /v1/admin/layers/{name}/segments", admin.ListSegments)
	mux.HandleFunc("POST /v1/admin/layers/{name}/segments", admin.CreateSegment)
	mux.HandleFunc("PUT /v1/admin/layers/{name}/segments/{id}", admin.UpdateSegment)
	mux.HandleFunc("DELETE /v1/admin/layers/{name}/segments/{id}", admin.DeleteSegment)
	mux.HandleFunc("POST /v1/admin/import", admin.ImportSnapshot)
	mux.HandleFunc("GET /v1/admin/export", admin.ExportSnapshot)

	handler := CORS(PanicRecovery(Timing(Logging(mux))))

	return &http.Server{
		Addr:    addr,
		Handler: handler,
	}
}
