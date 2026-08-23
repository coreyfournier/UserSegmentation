package config

import (
	"path/filepath"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/validation"
)

// The shipped config must load and validate after the order -> dependsOn
// migration. This is the regression guard on the migration itself: the edges
// were derived from real layer: references, not synthesized from the old
// ordinal, so only the three genuine dependencies exist.
func TestShippedConfig_LoadsAndValidates(t *testing.T) {
	path := filepath.Join("..", "..", "..", "config", "segments.json")

	snap, err := NewFileSource(path).Load()
	if err != nil {
		t.Fatalf("shipped config failed to load: %v", err)
	}
	if err := validation.ValidateSnapshot(snap); err != nil {
		t.Fatalf("shipped config failed validation: %v", err)
	}

	want := map[string][]string{
		// Migrated from the removed `order` field: derived from real layer:
		// references, so only these three edges exist.
		"experiments": {"base-tier"},
		"promotions":  {"base-tier"},
		"features":    {"base-tier"},

		// The progressive readiness gates.
		"company-payroll-setup": {"company-identity"},
		"employee-readiness":    {"company-payroll-setup"},
	}

	for _, layer := range snap.Layers {
		expected, shouldHaveDeps := want[layer.Name]
		switch {
		case !shouldHaveDeps && len(layer.DependsOn) > 0:
			t.Errorf("layer %q should have no dependencies, got %v", layer.Name, layer.DependsOn)
		case shouldHaveDeps && len(layer.DependsOn) != len(expected):
			t.Errorf("layer %q: expected %v, got %v", layer.Name, expected, layer.DependsOn)
		case shouldHaveDeps:
			for i, dep := range expected {
				if layer.DependsOn[i] != dep {
					t.Errorf("layer %q: expected %v, got %v", layer.Name, expected, layer.DependsOn)
					break
				}
			}
		}
	}
}
