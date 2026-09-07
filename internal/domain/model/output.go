package model

// OutputField declares one field a segment emits with each reported item.
//
// How the authored value becomes a value is derived from Type, not declared:
// a string field is a template, every other type is an expression. Those were
// once a separate axis, which made pairings like {object, template} writable
// and therefore something validation had to reject. Deriving it makes them
// unwritable.
//
// The derivation loses nothing. A template token is a full expression, so a
// string can still be computed — ${ company.name + " Inc" } — and a template
// with no tokens renders to itself, so constants still work. For a non-string,
// a bare 3 or true is already a valid expression. It also removes a trap: in
// expression mode an unquoted Critical is an identifier lookup that yields nil
// with no error, which is exactly why string constants used to need a literal
// mode.
//
// One thing is given up: a string constant cannot contain ${…}, because
// renderTemplate has no escape. Message templates have always had this
// limitation.
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
	Type   FieldType `json:"type"`
	Lookup string    `json:"lookup,omitempty"`
	// Required is the caller's contract: an error at snapshot load if no
	// authoring path supplies it, a warning at evaluation if it is absent
	// anyway.
	Required bool `json:"required,omitempty"`
	// LegacyEval exists only to catch config written when the mode was
	// declared. It carries no behaviour; validation rejects any field where it
	// is set. Without it the decoder would drop the key, and a string field
	// that said eval:"expression" would silently start being rendered as a
	// template — its value quietly changing meaning. Delete once no config in
	// flight carries it.
	LegacyEval string `json:"eval,omitempty"`
}

// IsTemplate reports whether this field's authored value is a ${…} template
// rather than an expression. The single place the derivation lives.
func (f OutputField) IsTemplate() bool { return f.Type == FieldTypeString }

// OutputSchema maps output field names to their declarations.
type OutputSchema map[string]OutputField
