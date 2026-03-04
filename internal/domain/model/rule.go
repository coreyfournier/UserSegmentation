package model

// CompositeOperator represents And/Or grouping.
type CompositeOperator string

const (
	CompositeAnd CompositeOperator = "And"
	CompositeOr  CompositeOperator = "Or"
)

// Rule is a node in the composite rule tree.
// A leaf rule has an Expression; a composite rule has an Operator and nested Rules.
type Rule struct {
	RuleName     string            `json:"ruleName"`
	Operator     CompositeOperator `json:"operator,omitempty"`
	Enabled      *bool             `json:"enabled,omitempty"`
	SuccessEvent string            `json:"successEvent,omitempty"`
	ErrorMessage string            `json:"errorMessage,omitempty"`
	Expression   *Expression       `json:"expression,omitempty"`
	Rules        []Rule            `json:"rules,omitempty"`
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
	return r.Expression != nil
}
