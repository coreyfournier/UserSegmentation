package strategy

import "github.com/segmentation-service/segmentation/internal/domain/model"

// EvalContext holds the subject key and merged context map (including cross-layer results).
type EvalContext struct {
	SubjectKey string
	Context    map[string]interface{}
	// Message rendering options, populated by the evaluator per layer.
	Languages       []string
	RenderAll       bool
	DefaultLanguage string
	// Lookups maps lookup table id to table, for in_lookup / not_in_lookup operators.
	Lookups map[string]model.LookupTable
	// CollectFailures inverts rule evaluation: every rule becomes an assertion
	// that must hold, the whole tree is walked without short-circuiting, and
	// each assertion that does not hold is itemised. Set internally by
	// AssertStrategy — it is not a config field.
	CollectFailures bool
}

// Result is the outcome of a strategy evaluation.
type Result struct {
	Segment      string
	Reason       string
	Expressions  map[string]interface{}
	Messages     map[string]string
	RenderErrors []RenderError
	// Failures is populated only in collect mode.
	Failures []model.Failure
	// Status is set by AssertStrategy; other strategies leave it empty and the
	// evaluator supplies the neutral resolution vocabulary.
	Status model.LayerStatus
}

// Strategy evaluates a segment definition against the given context.
type Strategy interface {
	Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool)
}
