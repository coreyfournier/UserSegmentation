package model

type Operator string

const (
	OpEq       Operator = "eq"
	OpNeq      Operator = "neq"
	OpGt       Operator = "gt"
	OpGte      Operator = "gte"
	OpLt       Operator = "lt"
	OpLte      Operator = "lte"
	OpIn       Operator = "in"
	OpContains Operator = "contains"
)

type FieldType string

const (
	FieldTypeString  FieldType = "string"
	FieldTypeNumber  FieldType = "number"
	FieldTypeBoolean FieldType = "boolean"
	FieldTypeArray   FieldType = "array"
)

// OperatorTypes maps each operator to the field types it supports.
var OperatorTypes = map[Operator][]FieldType{
	OpEq:       {FieldTypeString, FieldTypeNumber, FieldTypeBoolean},
	OpNeq:      {FieldTypeString, FieldTypeNumber, FieldTypeBoolean},
	OpGt:       {FieldTypeNumber},
	OpGte:      {FieldTypeNumber},
	OpLt:       {FieldTypeNumber},
	OpLte:      {FieldTypeNumber},
	OpIn:       {FieldTypeString, FieldTypeNumber},
	OpContains: {FieldTypeArray, FieldTypeString},
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
