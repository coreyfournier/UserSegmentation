package model

// ComputedField is a named value derived from the context before rules run,
// evaluated with expr-lang.
//
// This is the "what is this value?" half of the vocabulary; a Condition is the
// "does it hold?" half. Formula holds the expr-lang source — naming it that
// rather than "expression" avoids the previous expression.expression.
type ComputedField struct {
	Name    string    `json:"name"`
	Type    FieldType `json:"type"`
	Formula string    `json:"formula"`
}
