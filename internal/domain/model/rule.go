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
	RuleName string            `json:"ruleName"`
	Operator CompositeOperator `json:"operator,omitempty"`
	Enabled  *bool             `json:"enabled,omitempty"`
	// When gates this rule — and its whole subtree — on the evaluation context.
	// Absent means always applicable.
	//
	// A rule that does not apply contributes nothing: no failure is reported for
	// it, and it neither satisfies nor fails its parent. Putting the condition
	// here instead of repeating it as an And on every child is what lets one
	// predicate govern a whole block of checks.
	//
	// Enabled is the static form of the same idea; When is the data-dependent one.
	When         *Rule  `json:"when,omitempty"`
	SuccessEvent string `json:"successEvent,omitempty"`
	ErrorMessage string            `json:"errorMessage,omitempty"`
	Expression   *Expression       `json:"expression,omitempty"`
	Rules        []Rule            `json:"rules,omitempty"`
	// Messages are optional localized templates keyed by language code (e.g. "en").
	// Rendered with ${ ... } expr-lang interpolation when this rule wins.
	Messages map[string]string `json:"messages,omitempty"`
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
