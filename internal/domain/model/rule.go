package model

// CompositeOperator represents And/Or grouping.
type CompositeOperator string

const (
	CompositeAnd CompositeOperator = "And"
	CompositeOr  CompositeOperator = "Or"
)

// Rule is a node in the composite rule tree.
// A leaf rule has a Condition; a composite rule has an Operator and nested Rules.
type Rule struct {
	RuleName     string            `json:"ruleName"`
	Operator     CompositeOperator `json:"operator,omitempty"`
	Enabled      *bool             `json:"enabled,omitempty"`
	SuccessEvent string            `json:"successEvent,omitempty"`
	ErrorMessage string            `json:"errorMessage,omitempty"`
	Condition    *Condition        `json:"condition,omitempty"`
	Rules        []Rule            `json:"rules,omitempty"`
	// Messages are optional localized templates keyed by language code (e.g. "en").
	// Rendered with ${ ... } expr-lang interpolation when this rule wins.
	Messages map[string]string `json:"messages,omitempty"`
	// Outputs are this item's authored values for the segment's output schema,
	// keyed by field name. Only a top-level rule reports, so only a top-level
	// rule's Outputs are read.
	Outputs map[string]string `json:"outputs,omitempty"`
}

// IsEnabled returns true if the rule is enabled (defaults to true if nil).
func (r *Rule) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

// IsLeaf returns true when this rule has an expression (no nested rules).
func (r *Rule) IsLeaf() bool {
	return r.Condition != nil
}
