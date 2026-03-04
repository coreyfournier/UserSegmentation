package model

// SchemaField describes one expected field in the evaluation context.
type SchemaField struct {
	Type     FieldType `json:"type"`
	Required bool      `json:"required"`
}

// InputSchema maps field names to their schema definitions.
type InputSchema map[string]SchemaField
