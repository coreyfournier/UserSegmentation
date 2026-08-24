package model

// Condition is a leaf-level test: field <operator> value.
//
// This is the "does it hold?" half of the vocabulary. The other half is a
// ComputedField, which answers "what is this value?" — the two used to share
// the name "expression", which made a rule's test and an expr-lang formula
// indistinguishable in both config and code.
//
// Value is omitted when empty so unary operators — which test the field itself
// and ignore it — do not persist a meaningless null. Only a nil interface is
// omitted, so a deliberate false or 0 is still written.
type Condition struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value,omitempty"`
}
