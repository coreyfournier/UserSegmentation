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
		model.Layer{Key: "b", DependsOn: []string{"missing"}},
	), `depends on unknown layer "missing"`)
}

func TestGraph_SelfDependency(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Key: "a", DependsOn: []string{"a"}},
	), "depends on itself")
}

func TestGraph_DuplicateLayerName(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Key: "a"},
		model.Layer{Key: "a"},
	), `duplicate layer key "a"`)
}

func TestGraph_DuplicateDependency(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Key: "a"},
		model.Layer{Key: "b", DependsOn: []string{"a", "a"}},
	), "duplicate dependency")
}

func TestGraph_DirectCycle(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Key: "a", DependsOn: []string{"b"}},
		model.Layer{Key: "b", DependsOn: []string{"a"}},
	), "cycle")
}

func TestGraph_IndirectCycle(t *testing.T) {
	expectError(t, graphSnapshot(
		model.Layer{Key: "a", DependsOn: []string{"c"}},
		model.Layer{Key: "b", DependsOn: []string{"a"}},
		model.Layer{Key: "c", DependsOn: []string{"b"}},
	), "cycle")
}

func TestGraph_AcyclicPasses(t *testing.T) {
	snap := graphSnapshot(
		model.Layer{Key: "root"},
		model.Layer{Key: "left", DependsOn: []string{"root"}},
		model.Layer{Key: "right", DependsOn: []string{"root"}},
		model.Layer{Key: "join", DependsOn: []string{"left", "right"}},
	)
	if err := ValidateSnapshot(snap); err != nil {
		t.Errorf("diamond should be valid, got: %v", err)
	}
}

// Assert segments carry expressions too, so they must get the same
// compile-time syntax checking — otherwise a config typo becomes a runtime
// unevaluable instead of a load failure.
func TestGraph_FormulasAreCompiled(t *testing.T) {
	seg := model.Segment{
		ID:       "bad",
		Strategy: model.StrategyChecklist,
		Computed: []model.ComputedField{{Name: "Broken", Type: model.FieldTypeNumber, Formula: "1 +"}},
	}
	expectError(t, graphSnapshot(model.Layer{Key: "gate", Segments: []model.Segment{seg}}), "formula")
}

// The When dispatch predicate is a rule tree and is checked like any other.
func TestGraph_WhenPredicateValidated(t *testing.T) {
	seg := model.Segment{
		ID:       "precision",
		Strategy: model.StrategyChecklist,
		When: &model.Rule{
			RuleName:  "isPrecision",
			Condition: &model.Condition{Field: "notInSchema", Operator: model.OpEq, Value: "Precision"},
		},
	}
	layer := model.Layer{
		Key:         "payroll",
		InputSchema: model.InputSchema{"productType": {Type: model.FieldTypeString}},
		Segments:    []model.Segment{seg},
	}
	expectError(t, graphSnapshot(layer), "not in inputSchema")
}
