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

	results := make([]EvaluateResponse, len(req.Users))
	var wg sync.WaitGroup
	wg.Add(len(req.Users))

	for i, user := range req.Users {
		go func(idx int, u EvaluateRequest) {
			defer wg.Done()
			resp, err := uc.evaluateUC.Execute(u)
			if err != nil {
				results[idx] = EvaluateResponse{
					UserKey: u.UserKey,
					Layers:  make(map[string]LayerResultDTO),
				}
				return
			}
			results[idx] = *resp
		}(i, user)
	}

	wg.Wait()

	return &BatchEvaluateResponse{
		Results:    results,
		DurationUS: time.Since(start).Microseconds(),
	}, nil
}
