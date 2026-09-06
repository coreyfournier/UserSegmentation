package application

import (
	"fmt"
	"strings"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// Saved evaluation inputs, filed per layer. Named admin_test_fixture.go rather
// than admin_test.go because Go would treat the latter as a test file and
// refuse to build it into the package.

// ListTests returns the saved tests, optionally narrowed to one layer.
func (uc *AdminUseCase) ListTests(layerKey string) []model.SavedTest {
	snap := uc.store.Get()
	if snap == nil {
		return nil
	}
	if layerKey == "" {
		return snap.Tests
	}
	out := make([]model.SavedTest, 0, len(snap.Tests))
	for _, t := range snap.Tests {
		if t.Layer == layerKey {
			out = append(out, t)
		}
	}
	return out
}

// CreateTest saves a new test, deriving an id by slugging its name within the
// layer. The id is the layer key and the slug together, so two layers can each
// hold a test named "missing subject key" without collision.
func (uc *AdminUseCase) CreateTest(t model.SavedTest) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	if strings.TrimSpace(t.Name) == "" {
		return nil, fmt.Errorf("test name is required")
	}
	if t.Layer == "" {
		return nil, fmt.Errorf("test layer is required")
	}

	snap := uc.cloneSnapshot()
	if uc.findLayer(snap, t.Layer) < 0 {
		return nil, fmt.Errorf("layer %q not found", t.Layer)
	}
	for _, existing := range snap.Tests {
		if existing.Layer == t.Layer && strings.EqualFold(strings.TrimSpace(existing.Name), strings.TrimSpace(t.Name)) {
			return nil, fmt.Errorf("layer %q already has a test named %q", t.Layer, existing.Name)
		}
	}

	taken := make(map[string]bool, len(snap.Tests))
	for _, existing := range snap.Tests {
		taken[existing.ID] = true
	}
	base := slugify(t.Layer + "-" + t.Name)
	if base == "" {
		return nil, fmt.Errorf("could not derive a valid id from name %q", t.Name)
	}
	t.ID = uniqueSlug(base, taken)
	if t.Context == nil {
		t.Context = map[string]interface{}{}
	}

	snap.Tests = append(snap.Tests, t)
	return uc.commitSnapshot(snap)
}

// UpdateTest replaces a saved test's name, context and rendering options. The
// id and the layer it is filed under are immutable: moving a test to another
// layer would silently change what it exercises, and is better expressed as
// deleting it and saving a new one where you meant.
func (uc *AdminUseCase) UpdateTest(id string, updated model.SavedTest) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	if strings.TrimSpace(updated.Name) == "" {
		return nil, fmt.Errorf("test name is required")
	}

	snap := uc.cloneSnapshot()
	idx := -1
	for i := range snap.Tests {
		if snap.Tests[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("test %q not found", id)
	}

	layer := snap.Tests[idx].Layer
	for i, existing := range snap.Tests {
		if i == idx {
			continue
		}
		if existing.Layer == layer && strings.EqualFold(strings.TrimSpace(existing.Name), strings.TrimSpace(updated.Name)) {
			return nil, fmt.Errorf("layer %q already has a test named %q", layer, existing.Name)
		}
	}

	snap.Tests[idx].Name = updated.Name
	snap.Tests[idx].Context = updated.Context
	snap.Tests[idx].Languages = updated.Languages
	snap.Tests[idx].RenderAll = updated.RenderAll
	if snap.Tests[idx].Context == nil {
		snap.Tests[idx].Context = map[string]interface{}{}
	}

	return uc.commitSnapshot(snap)
}

// DeleteTest removes a saved test. Nothing references a test, so unlike a layer
// or a lookup table there is nothing to check first.
func (uc *AdminUseCase) DeleteTest(id string) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	kept := snap.Tests[:0:0]
	found := false
	for _, t := range snap.Tests {
		if t.ID == id {
			found = true
			continue
		}
		kept = append(kept, t)
	}
	if !found {
		return nil, fmt.Errorf("test %q not found", id)
	}
	snap.Tests = kept
	return uc.commitSnapshot(snap)
}
