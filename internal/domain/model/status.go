package model

// Strategy names recognised by the evaluator.
const (
	StrategyStatic     = "static"
	StrategyRule       = "rule"
	StrategyPercentage = "percentage"
	StrategyChecklist  = "checklist"
)

// KnownStrategies is the closed set the evaluator can dispatch to. An unknown
// name would otherwise be skipped silently and the segment would just never
// produce anything, so config validation rejects it.
var KnownStrategies = []string{
	StrategyStatic, StrategyRule, StrategyPercentage, StrategyChecklist,
}

// IsKnownStrategy reports whether the evaluator has an implementation for name.
func IsKnownStrategy(name string) bool {
	for _, s := range KnownStrategies {
		if s == name {
			return true
		}
	}
	return false
}

// LayerStatus reports the outcome of evaluating a layer.
//
// The checklist strategy owns the satisfied/violated vocabulary; every other
// strategy uses the neutral resolution vocabulary. Status is always emitted
// explicitly so a consumer never has to infer meaning from the failure count.
type LayerStatus string

const (
	// Checklist strategy.
	StatusSatisfied   LayerStatus = "satisfied"
	StatusViolated    LayerStatus = "violated"
	StatusUnevaluable LayerStatus = "unevaluable"

	// All other strategies.
	StatusResolved   LayerStatus = "resolved"
	StatusUnresolved LayerStatus = "unresolved"
	StatusSkipped    LayerStatus = "skipped"
)

// Failure is a single itemised problem reported by a checklist layer.
//
// RuleName is the stable identifier: descriptive enough to indicate what is
// wrong, and unique across the config (enforced by the ConfigSource). No
// separate code or field path is emitted — a rule may evaluate several fields
// together, so a field path would be present only sometimes, which consumers
// could not predict or explain.
type Failure struct {
	Rule    string `json:"rule"`
	Message string `json:"message"`
	// Segment is the id of the segment that reported this finding.
	//
	// Always set, for every finding of every checklist — a layer may run
	// several segments and merge their findings into one list, and a field
	// that appeared only in that case would be a property of the layer's
	// shape rather than of the finding. A consumer could not rely on it, and
	// adding a second segment to a layer would silently change the response
	// for findings that had nothing to do with the addition.
	Segment  string                 `json:"segment,omitempty"`
	Messages map[string]string      `json:"messages,omitempty"`
	Outputs  map[string]interface{} `json:"outputs,omitempty"`
}
