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

	result := uc.evaluator.Evaluate(snap, req.SubjectKey, ctx, req.Layers, req.Languages, req.RenderAll, now)

	resp := &EvaluateResponse{
		SubjectKey:  req.SubjectKey,
		Layers:      make(map[string]LayerResultDTO, len(result.Layers)),
		EvaluatedAt: now.UTC().Format(time.RFC3339Nano),
		DurationUS:  time.Since(start).Microseconds(),
	}

	for key, lr := range result.Layers {
		dto := LayerResultDTO{Name: lr.Name, Status: string(lr.Status)}
		if a := lr.Assignment; a != nil {
			dto.Segment = a.Segment
			dto.Strategy = a.Strategy
			dto.Reason = a.Reason
			dto.Computed = a.Computed
			dto.Messages = a.Messages
			dto.Outputs = a.Outputs
		}
		for _, f := range lr.Failures {
			dto.Failures = append(dto.Failures, FailureDTO{
				Rule:     f.Rule,
				Message:  f.Message,
				Messages: f.Messages,
				Outputs:  f.Outputs,
			})
		}
		resp.Layers[key] = dto
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
