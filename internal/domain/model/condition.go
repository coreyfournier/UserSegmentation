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
	// ValueField compares Field against another field's value instead of
	// against a literal — "hoursWorked >= minHours" rather than
	// "hoursWorked >= 20". It is resolved from the same context as Field, by
	// the same ResolveField, so a dotted path and a cross-layer "layer:x"
	// reference both work on the right-hand side exactly as they do on the
	// left.
	//
	// A separate field rather than a marker inside Value ("$minHours", say):
	// a value is an arbitrary JSON scalar, so any in-band marker is ambiguous
	// with a legitimate string and immediately needs an escape rule. Two
	// fields cost one exclusivity check, which validation makes once.
	//
	// Mutually exclusive with Value, and meaningless for the unary and lookup
	// operators — validation rejects both combinations rather than quietly
	// picking a winner.
	ValueField string `json:"valueField,omitempty"`
}

// ComparesToField reports whether this condition's right-hand side is another
// field rather than a literal.
func (c *Condition) ComparesToField() bool {
	return c != nil && c.ValueField != ""
}
