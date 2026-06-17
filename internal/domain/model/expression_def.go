package model

// ExpressionDef defines a computed field evaluated via expr-lang before rule evaluation.
type ExpressionDef struct {
	Name       string    `json:"name"`
	Type       FieldType `json:"type"`
	Expression string    `json:"expression"`
}
