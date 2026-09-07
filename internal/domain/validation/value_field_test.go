package validation

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// refSnapshot builds a one-rule snapshot whose condition is supplied by the
// caller, over a schema with one field of each type it needs.
func refSnapshot(cond *model.Condition, deps ...string) *model.Snapshot {
	return &model.Snapshot{
		Layers: []model.Layer{
			{
				Key:       "gate",
				DependsOn: deps,
				InputSchema: model.InputSchema{
					"hoursWorked": {Type: model.FieldTypeNumber},
					"minHours":    {Type: model.FieldTypeNumber},
					"tier":        {Type: model.FieldTypeString},
					"tags":        {Type: model.FieldTypeArray},
				},
				Segments: []model.Segment{{
					ID:       "s",
					Strategy: model.StrategyRule,
					Rules:    []model.Rule{{RuleName: "r", Condition: cond}},
				}},
			},
		},
	}
}

func TestValidate_ValueFieldAccepted(t *testing.T) {
	snap := refSnapshot(&model.Condition{
		Field: "hoursWorked", Operator: model.OpGte, ValueField: "minHours",
	})
	if err := ValidateSnapshot(snap); err != nil {
		t.Errorf("comparing two declared number fields should be valid, got: %v", err)
	}
}

func TestValidate_ValueFieldRejections(t *testing.T) {
	cases := []struct {
		name string
		cond *model.Condition
		want string
	}{
		{
			// The engine would resolve nothing and the rule would read false
			// forever, which is indistinguishable from "did not match".
			name: "undeclared reference",
			cond: &model.Condition{Field: "hoursWorked", Operator: model.OpGte, ValueField: "minHrs"},
			want: "valueField \"minHrs\" not in inputSchema",
		},
		{
			name: "both sides authored",
			cond: &model.Condition{Field: "hoursWorked", Operator: model.OpGte, Value: 20, ValueField: "minHours"},
			want: "sets both value and valueField",
		},
		{
			// number vs string can never compare equal, so the rule is dead.
			name: "type mismatch",
			cond: &model.Condition{Field: "hoursWorked", Operator: model.OpEq, ValueField: "tier"},
			want: "can never match",
		},
		{
			name: "unary operator",
			cond: &model.Condition{Field: "tier", Operator: model.OpIsNull, ValueField: "minHours"},
			want: "takes no value",
		},
		{
			name: "lookup operator",
			cond: &model.Condition{Field: "tier", Operator: model.OpInLookup, ValueField: "minHours"},
			want: "compares against a lookup table id",
		},
		{
			name: "compared to itself",
			cond: &model.Condition{Field: "hoursWorked", Operator: model.OpGte, ValueField: "hoursWorked"},
			want: "against itself",
		},
		{
			name: "undeclared cross-layer reference",
			cond: &model.Condition{Field: "tier", Operator: model.OpEq, ValueField: "layer:other"},
			want: "not declared in dependsOn",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSnapshot(refSnapshot(tc.cond))
			if err == nil {
				t.Fatalf("expected an error mentioning %q, got none", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected an error mentioning %q, got: %v", tc.want, err)
			}
		})
	}
}

// A declared dependency makes the cross-layer reference legal, and its type is
// deliberately unchecked — a layer result is not a schema field.
func TestValidate_ValueFieldCrossLayerWithDependency(t *testing.T) {
	snap := refSnapshot(
		&model.Condition{Field: "tier", Operator: model.OpEq, ValueField: "layer:other"},
		"other",
	)
	snap.Layers = append(snap.Layers, model.Layer{Key: "other", Segments: []model.Segment{}})
	if err := ValidateSnapshot(snap); err != nil {
		t.Errorf("a declared cross-layer reference should be valid, got: %v", err)
	}
}

// A list operator's right-hand side is the list itself, so it must be an
// array — not the left field's own type, which is what a naive equality rule
// would demand.
func TestValidate_ValueFieldListOperators(t *testing.T) {
	ok := refSnapshot(&model.Condition{Field: "tier", Operator: model.OpIn, ValueField: "tags"})
	if err := ValidateSnapshot(ok); err != nil {
		t.Errorf("in against an array field should be valid, got: %v", err)
	}

	bad := refSnapshot(&model.Condition{Field: "tier", Operator: model.OpIn, ValueField: "hoursWorked"})
	err := ValidateSnapshot(bad)
	if err == nil || !strings.Contains(err.Error(), "needs a list") {
		t.Errorf("in against a number field should be rejected, got: %v", err)
	}
}

// contains over an array compares against one element, and an array's element
// type is not declared anywhere — so there is nothing to check, and demanding
// the two sides agree would reject a correct rule.
func TestValidate_ValueFieldContainsOverArray(t *testing.T) {
	snap := refSnapshot(&model.Condition{Field: "tags", Operator: model.OpContains, ValueField: "tier"})
	if err := ValidateSnapshot(snap); err != nil {
		t.Errorf("contains over an array should not demand type agreement, got: %v", err)
	}
}
