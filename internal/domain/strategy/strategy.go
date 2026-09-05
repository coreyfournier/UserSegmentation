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
	// OutputSchema is the layer's — the only place it is declared. Strategies
	// never look it up themselves.
	OutputSchema model.OutputSchema
	// CollectFailures reports every rule that matches instead of stopping at
	// the first. A rule still fires on a match; only what happens then differs.
	// Set internally by ChecklistStrategy — it is not a config field.
	CollectFailures bool
}

// Result is the outcome of a strategy evaluation.
type Result struct {
	Segment      string
	Reason       string
	Computed     map[string]interface{}
	Messages     map[string]string
	RenderErrors []RenderError
	// Outputs are the declared output fields resolved for the reported item —
	// the winning rule under first-match, or the segment default.
	Outputs map[string]interface{}
	// Failures is populated only in collect mode.
	Failures []model.Failure
	// Status is set by ChecklistStrategy; other strategies leave it empty and the
	// evaluator supplies the neutral resolution vocabulary.
	Status model.LayerStatus
}

// Strategy evaluates a segment definition against the given context.
type Strategy interface {
	Evaluate(seg *model.Segment, ctx *EvalContext) (Result, bool)
}
