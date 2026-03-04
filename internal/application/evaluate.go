package application

import (
	"time"

	"github.com/segmentation-service/segmentation/internal/domain/engine"
	"github.com/segmentation-service/segmentation/internal/domain/ports"
)

// EvaluateUseCase handles single-user evaluation.
type EvaluateUseCase struct {
	store     ports.SegmentStore
	evaluator *engine.Evaluator
}

// NewEvaluateUseCase creates a new evaluate use case.
func NewEvaluateUseCase(store ports.SegmentStore, evaluator *engine.Evaluator) *EvaluateUseCase {
	return &EvaluateUseCase{store: store, evaluator: evaluator}
}

// Execute evaluates a single user.
func (uc *EvaluateUseCase) Execute(req EvaluateRequest) (*EvaluateResponse, error) {
	start := time.Now()
	now := start

	snap := uc.store.Get()
	if snap == nil {
		return nil, ErrNoConfig
	}

	ctx := req.Context
	if ctx == nil {
		ctx = make(map[string]interface{})
	}

	result := uc.evaluator.Evaluate(snap, req.SubjectKey, ctx, req.Layers, now)

	resp := &EvaluateResponse{
		SubjectKey:  req.SubjectKey,
		Layers:      make(map[string]LayerResultDTO, len(result.Layers)),
		EvaluatedAt: now.UTC().Format(time.RFC3339Nano),
		DurationUS:  time.Since(start).Microseconds(),
	}

	for name, a := range result.Layers {
		resp.Layers[name] = LayerResultDTO{
			Segment:  a.Segment,
			Strategy: a.Strategy,
			Reason:   a.Reason,
		}
	}

	for _, w := range result.Warnings {
		resp.Warnings = append(resp.Warnings, WarningDTO{
			Segment: w.Segment,
			Field:   w.Field,
			Message: w.Message,
		})
	}

	return resp, nil
}
