package application

import (
	"fmt"
	"strings"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// Saved evaluation inputs, filed per layer. Named admin_test_fixture.go rather
// than admin_test.go because Go would treat the latter as a test file and
// refuse to build it into the package.

// sameTestSlot reports whether two tests occupy the same naming slot: the same
// layer, the same segment, and the same name ignoring case and surrounding
// space.
//
// Segment is part of the slot because two segments in one layer are two
// different things to test, and each wants its own "happy path". Before this,
// the second one saved took the first one's name — and, through the editor,
// its context with it.
func sameTestSlot(a, b model.SavedTest) bool {
	return a.Layer == b.Layer &&
		a.Segment == b.Segment &&
		strings.EqualFold(strings.TrimSpace(a.Name), strings.TrimSpace(b.Name))
}

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
	li := uc.findLayer(snap, t.Layer)
	if li < 0 {
		return nil, fmt.Errorf("layer %q not found", t.Layer)
	}
	if t.Segment != "" && !hasSegment(&snap.Layers[li], t.Segment) {
		return nil, fmt.Errorf("layer %q has no segment %q", t.Layer, t.Segment)
	}
	for _, existing := range snap.Tests {
		if sameTestSlot(existing, t) {
			return nil, fmt.Errorf("%s already has a test named %q", testSlotLabel(t), existing.Name)
		}
	}

	taken := make(map[string]bool, len(snap.Tests))
	for _, existing := range snap.Tests {
		taken[existing.ID] = true
	}
	// The segment is part of the id as well as of the uniqueness rule, so two
	// segments' identically named tests read as different things in the file
	// rather than as "name" and "name-2".
	base := slugify(t.Layer + "-" + t.Segment + "-" + t.Name)
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
// id, the layer and the segment it is filed under are all immutable: moving a
// test would silently change what it exercises, and is better expressed as
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

	// The slot is the stored test's layer and segment with the new name: a
	// rename may not collide, and neither field is movable (see below).
	slot := snap.Tests[idx]
	slot.Name = updated.Name
	for i, existing := range snap.Tests {
		if i == idx {
			continue
		}
		if sameTestSlot(existing, slot) {
			return nil, fmt.Errorf("%s already has a test named %q", testSlotLabel(slot), existing.Name)
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

// testSlotLabel names where a test is filed, for an error a person reads.
func testSlotLabel(t model.SavedTest) string {
	if t.Segment == "" {
		return fmt.Sprintf("layer %q", t.Layer)
	}
	return fmt.Sprintf("segment %q in layer %q", t.Segment, t.Layer)
}

// hasSegment reports whether the layer holds a segment with this id.
func hasSegment(l *model.Layer, id string) bool {
	for i := range l.Segments {
		if l.Segments[i].ID == id {
			return true
		}
	}
	return false
}
