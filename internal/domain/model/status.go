package model

// Strategy names recognised by the evaluator.
const (
	StrategyStatic     = "static"
	StrategyRule       = "rule"
	StrategyPercentage = "percentage"
	StrategyExpression = "expression"
	StrategyAssert     = "assert"
)

// LayerStatus reports the outcome of evaluating a layer.
//
// The assert strategy owns the assertion vocabulary; every other strategy uses
// the neutral resolution vocabulary. Status is always emitted explicitly so a
// consumer never has to infer meaning from the length of the failure list.
type LayerStatus string

const (
	// Assert strategy.
	StatusSatisfied   LayerStatus = "satisfied"
	StatusViolated    LayerStatus = "violated"
	StatusUnevaluable LayerStatus = "unevaluable"

	// All other strategies.
	StatusResolved   LayerStatus = "resolved"
	StatusUnresolved LayerStatus = "unresolved"
	StatusSkipped    LayerStatus = "skipped"
)

// Failure is a single itemised problem reported by an assert layer.
//
// RuleName is the stable identifier: descriptive enough to indicate what is
// wrong, and unique across the config (enforced by the ConfigSource). No
// separate code or field path is emitted — a rule may evaluate several fields
// together, so a field path would be present only sometimes, which consumers
// could not predict or explain.
type Failure struct {
	Rule     string            `json:"rule"`
	Message  string            `json:"message"`
	Messages map[string]string `json:"messages,omitempty"`
}
