package application

import (
	"sync"
	"time"
)

// BatchEvaluateUseCase handles multi-user batch evaluation.
type BatchEvaluateUseCase struct {
	evaluateUC *EvaluateUseCase
}

// NewBatchEvaluateUseCase creates a new batch evaluate use case.
func NewBatchEvaluateUseCase(evaluateUC *EvaluateUseCase) *BatchEvaluateUseCase {
	return &BatchEvaluateUseCase{evaluateUC: evaluateUC}
}

// Execute evaluates multiple users in parallel.
func (uc *BatchEvaluateUseCase) Execute(req BatchEvaluateRequest) (*BatchEvaluateResponse, error) {
	start := time.Now()

	results := make([]EvaluateResponse, len(req.Subjects))
	var wg sync.WaitGroup
	wg.Add(len(req.Subjects))

	for i, subj := range req.Subjects {
		go func(idx int, u EvaluateRequest) {
			defer wg.Done()
			resp, err := uc.evaluateUC.Execute(u)
			if err != nil {
				results[idx] = EvaluateResponse{
					SubjectKey: u.SubjectKey,
					Layers:     make(map[string]LayerResultDTO),
				}
				return
			}
			results[idx] = *resp
		}(i, subj)
	}

	wg.Wait()

	return &BatchEvaluateResponse{
		Results:    results,
		DurationUS: time.Since(start).Microseconds(),
	}, nil
}
