package model

import "testing"

// An object field exists for expression-mode outputs that return a map. It must
// never be usable in a condition, so no operator may accept it.
func TestFieldTypeObject_NotUsableInConditions(t *testing.T) {
	for op := range OperatorTypes {
		if OperatorSupportsType(op, FieldTypeObject) {
			t.Fatalf("operator %q must not accept the object type", op)
		}
	}
}

func TestOutputField_EvalDefaultsToLiteral(t *testing.T) {
	f := OutputField{Type: FieldTypeString}
	if f.EvalMode() != EvalLiteral {
		t.Fatalf("expected literal by default, got %q", f.EvalMode())
	}
	f.Eval = EvalExpression
	if f.EvalMode() != EvalExpression {
		t.Fatalf("expected expression, got %q", f.EvalMode())
	}
}
