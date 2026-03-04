package strategy

import "github.com/segmentation-service/segmentation/internal/domain/model"

// EvalContext holds the subject key and merged context map (including cross-layer results).
type EvalContext struct {
	SubjectKey string
	Context    map[string]interface{}
}

// Result is the outcome of a strategy evaluation.
type Result struct {
	Segment string
	Reason  string
}

// Strategy evaluates a segment definition against the given context.
type Strategy interface {
	Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool)
}
