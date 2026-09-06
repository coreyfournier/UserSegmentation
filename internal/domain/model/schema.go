package model

// SchemaField describes one expected field in the evaluation context.
type SchemaField struct {
	Type     FieldType `json:"type"`
	Required bool      `json:"required"`
	// Lookup names a table whose keys are this field's permitted values,
	// mirroring OutputField.Lookup. It declares the field's domain: the
	// condition editor offers the table's keys instead of a free-text box, so
	// an author picks a value that exists rather than typing one that might.
	//
	// It is a declaration, not enforcement. Nothing checks at evaluation that
	// an incoming value is one of the table's keys — the same deliberate
	// no-guarding trade made everywhere else lookups are used. What is checked
	// at load is that the table exists and that its key type agrees with this
	// field's, because a binding that cannot agree is a mistake rather than a
	// risk an author is choosing to take.
	Lookup string `json:"lookup,omitempty"`
}

// InputSchema maps field names to their schema definitions.
type InputSchema map[string]SchemaField
