package config

import (
	"path/filepath"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
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
		"experiments": {"baseTier"},
		"promotions":  {"baseTier"},
		"features":    {"baseTier"},

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

// TestShippedConfig_CompanyPayrollSetupSchemaIsUnioned locks in the one
// non-mechanical decision in the segment->layer schema migration:
// company-payroll-setup holds two segments (precision, express) that each
// declared a different inputSchema before schemas moved to the layer. The
// layer's schema is their union, and a field is required if it was required
// on either segment — so six of the seven fields end up required and only
// company.defaultPayRate (optional on precision, absent from express) stays
// optional. If this ever collapses back to an intersection, or a required
// flag gets dropped, this test catches it.
func TestShippedConfig_CompanyPayrollSetupSchemaIsUnioned(t *testing.T) {
	path := filepath.Join("..", "..", "..", "config", "segments.json")

	snap, err := NewFileSource(path).Load()
	if err != nil {
		t.Fatalf("shipped config failed to load: %v", err)
	}

	var layer *model.Layer
	for i := range snap.Layers {
		if snap.Layers[i].Name == "company-payroll-setup" {
			layer = &snap.Layers[i]
			break
		}
	}
	if layer == nil {
		t.Fatal("company-payroll-setup layer not found")
	}

	// No segment should carry a schema of its own anymore; the layer is the
	// only place it is declared.
	for _, seg := range layer.Segments {
		if len(seg.LegacyInputSchema) > 0 {
			t.Errorf("segment %q still declares its own inputSchema", seg.ID)
		}
	}

	wantRequired := map[string]bool{
		"company.anchorDate":     true,
		"company.checkDate":      true,
		"company.defaultPayRate": false,
		"company.fundingAccount": true,
		"company.payFrequency":   true,
		"company.periodEnd":      true,
		"company.productType":    true,
	}

	if len(layer.InputSchema) != len(wantRequired) {
		t.Fatalf("expected %d unioned fields, got %d: %+v", len(wantRequired), len(layer.InputSchema), layer.InputSchema)
	}
	for field, wantReq := range wantRequired {
		sf, ok := layer.InputSchema[field]
		if !ok {
			t.Errorf("expected field %q in unioned schema, missing", field)
			continue
		}
		if sf.Required != wantReq {
			t.Errorf("field %q: expected required=%v, got %v", field, wantReq, sf.Required)
		}
	}
}
