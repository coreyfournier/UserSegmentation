package validation

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func graphSnapshot(layers ...model.Layer) *model.Snapshot {
	return &model.Snapshot{Layers: layers}
}

func expectError(t *testing.T, snap *model.Snapshot, want string) {
	t.Helper()
	err := ValidateSnapshot(snap)
	if err == nil {
		t.Fatalf("expected an error containing %q", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("expected %q, got: %v", want, err)
	}
}

func TestGraph_UnknownDependency(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Name: "b", DependsOn: []string{"missing"}},
	), `depends on unknown layer "missing"`)
}

func TestGraph_SelfDependency(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Name: "a", DependsOn: []string{"a"}},
	), "depends on itself")
}

func TestGraph_DuplicateLayerName(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Name: "a"},
		model.Layer{Name: "a"},
	), `duplicate layer name "a"`)
}

func TestGraph_DuplicateDependency(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Name: "a"},
		model.Layer{Name: "b", DependsOn: []string{"a", "a"}},
	), "duplicate dependency")
}

func TestGraph_DirectCycle(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Name: "a", DependsOn: []string{"b"}},
		model.Layer{Name: "b", DependsOn: []string{"a"}},
	), "cycle")
}

func TestGraph_IndirectCycle(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Name: "a", DependsOn: []string{"c"}},
		model.Layer{Name: "b", DependsOn: []string{"a"}},
		model.Layer{Name: "c", DependsOn: []string{"b"}},
	), "cycle")
}

func TestGraph_AcyclicPasses(t *testing.T) {
	snap := graphSnapshot(
		model.Layer{Name: "root"},
		model.Layer{Name: "left", DependsOn: []string{"root"}},
		model.Layer{Name: "right", DependsOn: []string{"root"}},
		model.Layer{Name: "join", DependsOn: []string{"left", "right"}},
	)
	if err := ValidateSnapshot(snap); err != nil {
		t.Errorf("diamond should be valid, got: %v", err)
	}
}

// Assert segments carry expressions too, so they must get the same
// compile-time syntax checking — otherwise a config typo becomes a runtime
// unevaluable instead of a load failure.
func TestGraph_AssertExpressionsAreCompiled(t *testing.T) {
	seg := model.Segment{
		ID:          "bad",
		Strategy:    model.StrategyChecklist,
		Computed: []model.ComputedField{{Name: "Broken", Type: model.FieldTypeNumber, Formula: "1 +"}},
	}
	expectError(t, graphSnapshot(model.Layer{Name: "gate", Segments: []model.Segment{seg}}), "expression")
}

// The When dispatch predicate is a rule tree and is checked like any other.
func TestGraph_WhenPredicateValidated(t *testing.T) {
	seg := model.Segment{
		ID:          "precision",
		Strategy:    model.StrategyChecklist,
		InputSchema: model.InputSchema{"productType": {Type: model.FieldTypeString}},
		When: &model.Rule{
			RuleName:   "isPrecision",
			Condition: &model.Condition{Field: "notInSchema", Operator: model.OpEq, Value: "Precision"},
		},
	}
	expectError(t, graphSnapshot(model.Layer{Name: "payroll", Segments: []model.Segment{seg}}), "not in inputSchema")
}
