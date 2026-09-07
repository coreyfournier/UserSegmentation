package model

type Operator string

const (
	OpEq       Operator = "eq"
	OpNeq      Operator = "neq"
	OpGt       Operator = "gt"
	OpGte      Operator = "gte"
	OpLt       Operator = "lt"
	OpLte      Operator = "lte"
	OpIn         Operator = "in"
	OpNotIn      Operator = "not_in"
	OpContains   Operator = "contains"
	OpInLookup   Operator = "in_lookup"
	OpNotInLookup Operator = "not_in_lookup"
	// Presence tests. These take no value and are the only operators evaluated
	// when the field is absent from the context — a field that is not there at
	// all is null.
	OpIsNull        Operator = "is_null"
	OpIsNullOrEmpty Operator = "is_null_or_empty"
)

type FieldType string

const (
	FieldTypeString  FieldType = "string"
	FieldTypeNumber  FieldType = "number"
	FieldTypeBoolean FieldType = "boolean"
	FieldTypeArray   FieldType = "array"
	// FieldTypeObject is for expression-mode output fields that return a map.
	// It is deliberately absent from OperatorTypes so it can never appear in a
	// condition.
	FieldTypeObject FieldType = "object"
)

// OperatorTypes maps each operator to the field types it supports.
var OperatorTypes = map[Operator][]FieldType{
	OpEq:       {FieldTypeString, FieldTypeNumber, FieldTypeBoolean},
	OpNeq:      {FieldTypeString, FieldTypeNumber, FieldTypeBoolean},
	OpGt:       {FieldTypeNumber},
	OpGte:      {FieldTypeNumber},
	OpLt:       {FieldTypeNumber},
	OpLte:      {FieldTypeNumber},
	OpIn:          {FieldTypeString, FieldTypeNumber},
	OpNotIn:       {FieldTypeString, FieldTypeNumber},
	OpContains:    {FieldTypeArray, FieldTypeString},
	OpInLookup:    {FieldTypeString, FieldTypeNumber},
	OpNotInLookup: {FieldTypeString, FieldTypeNumber},
	// Any optional field of any type can be null.
	OpIsNull: {FieldTypeString, FieldTypeNumber, FieldTypeBoolean, FieldTypeArray},
	// Emptiness here means the empty string, so this is a string test.
	OpIsNullOrEmpty: {FieldTypeString},
}

// unaryOperators test the field itself rather than comparing it to a value.
var unaryOperators = map[Operator]bool{
	OpIsNull:        true,
	OpIsNullOrEmpty: true,
}

// IsUnary reports whether an operator takes no value.
//
// Two consequences: Expression.Value is ignored, and the operator is evaluated
// even when the field is missing from the context. Every other operator has
// nothing to compare an absent field against and is false.
func IsUnary(op Operator) bool {
	return unaryOperators[op]
}

func ValidOperator(op Operator) bool {
	_, ok := OperatorTypes[op]
	return ok
}

func OperatorSupportsType(op Operator, ft FieldType) bool {
	types, ok := OperatorTypes[op]
	if !ok {
		return false
	}
	for _, t := range types {
		if t == ft {
			return true
		}
	}
	return false
}
