package model

// Expression is a leaf-level condition: field <operator> value.
//
// Value is omitted when empty so unary operators — which test the field itself
// and ignore it — do not persist a meaningless null. Only a nil interface is
// omitted, so a deliberate false or 0 is still written.
type Expression struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value,omitempty"`
}
