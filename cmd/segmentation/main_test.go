package main

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// fakeConfigSource returns a fixed snapshot (or error) from Load, standing in
// for the real file-backed ports.ConfigSource in these tests.
type fakeConfigSource struct {
	snap *model.Snapshot
	err  error
}

func (f *fakeConfigSource) Load() (*model.Snapshot, error) {
	return f.snap, f.err
}

// This is the regression guard for the startup path described in the branch
// review: a config carrying a segment-level inputSchema (the pre-migration
// shape — schemas now live only on the layer) used to boot clean because
// nothing between Load() and Swap() ever called validation.ValidateSnapshot.
// loadAndValidate is what closes that gap; this proves it actually rejects
// such a config rather than silently accepting it.
func TestLoadAndValidate_RejectsSegmentLevelInputSchema(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "tier",
			Segments: []model.Segment{{
				ID:                "seg",
				Strategy:          model.StrategyRule,
				LegacyInputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
			}},
		}},
	}

	_, _, err := loadAndValidate(&fakeConfigSource{snap: snap})
	if err == nil {
		t.Fatal("expected loadAndValidate to reject a segment-level inputSchema, got nil error")
	}
	if !strings.Contains(err.Error(), "config is invalid") || !strings.Contains(err.Error(), "inputSchema") {
		t.Fatalf("expected an invalid-config error naming inputSchema, got: %v", err)
	}
}

func TestLoadAndValidate_ValidConfigPasses(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name:        "tier",
			InputSchema: model.InputSchema{"country": {Type: model.FieldTypeString}},
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyRule,
				Rules: []model.Rule{{
					RuleName:  "check",
					Condition: &model.Condition{Field: "country", Operator: model.OpEq, Value: "US"},
				}},
			}},
		}},
	}

	got, warnings, err := loadAndValidate(&fakeConfigSource{snap: snap})
	if err != nil {
		t.Fatalf("expected a valid config to pass, got: %v", err)
	}
	if got != snap {
		t.Fatal("expected the loaded snapshot to be returned unchanged")
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings for a layer that declares an inputSchema, got: %v", warnings)
	}
}

// A layer with no inputSchema but a segment carrying rules should surface as
// a warning, not silently pass with nothing to see (Finding 3).
func TestLoadAndValidate_WarnsOnMissingInputSchemaWithRules(t *testing.T) {
	snap := &model.Snapshot{
		Layers: []model.Layer{{
			Name: "tier",
			Segments: []model.Segment{{
				ID:       "seg",
				Strategy: model.StrategyRule,
				Rules: []model.Rule{{
					RuleName:  "check",
					Condition: &model.Condition{Field: "whatever", Operator: model.OpEq, Value: "US"},
				}},
			}},
		}},
	}

	_, warnings, err := loadAndValidate(&fakeConfigSource{snap: snap})
	if err != nil {
		t.Fatalf("the escape hatch means this config is valid, got: %v", err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], `layer "tier"`) || !strings.Contains(warnings[0], "no inputSchema") {
		t.Fatalf("expected one warning naming the layer, got: %v", warnings)
	}
}

func TestLoadAndValidate_PropagatesLoadError(t *testing.T) {
	loadErr := &testLoadError{msg: "boom"}
	_, _, err := loadAndValidate(&fakeConfigSource{err: loadErr})
	if err == nil || !strings.Contains(err.Error(), "failed to load config") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected a load error wrapped with context, got: %v", err)
	}
}

type testLoadError struct{ msg string }

func (e *testLoadError) Error() string { return e.msg }
