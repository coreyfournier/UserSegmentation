package model

// EvalMode says how an output field's authored value becomes a value.
//
//	literal     the value is a constant
//	template    the value is text with ${ ... } tokens, rendered to a string
//	expression  the value is one whole expr-lang expression, keeping its type
type EvalMode string

const (
	EvalLiteral    EvalMode = "literal"
	EvalTemplate   EvalMode = "template"
	EvalExpression EvalMode = "expression"
)

// OutputField declares one field a segment emits with each reported item.
//
// Lookup names a table whose keys are the field's permitted values. A
// lookup-bound field is emitted as {key, value} — plus order when the table sets
// EmitOrder — so a consumer can sort and display without reading the table.
//
// Required is the caller's contract, and it is checked at two different points.
// At snapshot load it is an error for a required field to be unauthored, so an
// author cannot silently drop a field a consumer depends on (Task 5). At
// evaluation it is a warning for a required field to be absent from what was
// actually emitted, because config validity cannot guarantee runtime presence —
// an expression can fail, and an override emits no outputs at all (Task 6). It
// deliberately mirrors SchemaField.Required in declaration, but note the runtime
// halves are the only halves that behave alike: an input field's required-ness
// cannot be checked at load, because config does not know the caller's context.
type OutputField struct {
	Type     FieldType `json:"type"`
	Eval     EvalMode  `json:"eval,omitempty"`
	Lookup   string    `json:"lookup,omitempty"`
	Required bool      `json:"required,omitempty"`
}

// EvalMode returns the declared mode, defaulting to literal.
func (f OutputField) EvalMode() EvalMode {
	if f.Eval == "" {
		return EvalLiteral
	}
	return f.Eval
}

// OutputSchema maps output field names to their declarations.
type OutputSchema map[string]OutputField
