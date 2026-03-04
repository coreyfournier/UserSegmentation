package model

// Expression is a leaf-level condition: field <operator> value.
type Expression struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value"`
}
