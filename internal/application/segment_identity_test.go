package application

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// The segment's id used to be pinned to whatever the request was addressed to,
// which made it uneditable through the API at all.
func TestUpdateSegment_RenamesTheID(t *testing.T) {
	uc, _, _ := newTestAdminUC()

	snap, err := uc.UpdateSegment("baseLayer", "seg1", model.Segment{
		ID:       "renamed",
		Name:     "Renamed segment",
		Strategy: model.StrategyStatic,
		Static:   &model.StaticConfig{Mappings: map[string]string{}, Default: "x"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := snap.Layers[0].Segments[0]
	if got.ID != "renamed" {
		t.Errorf("expected the id to change, got %q", got.ID)
	}
	if got.Name != "Renamed segment" {
		t.Errorf("expected the friendly name to persist, got %q", got.Name)
	}
}

// An omitted id keeps the one being addressed, so a caller updating only the
// body need not restate it.
func TestUpdateSegment_OmittedIDIsPreserved(t *testing.T) {
	uc, _, _ := newTestAdminUC()

	snap, err := uc.UpdateSegment("baseLayer", "seg1", model.Segment{
		Strategy: model.StrategyStatic,
		Static:   &model.StaticConfig{Mappings: map[string]string{}, Default: "x"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := snap.Layers[0].Segments[0].ID; got != "seg1" {
		t.Errorf("expected seg1 to be preserved, got %q", got)
	}
}

// Renaming onto a sibling's id is refused: findSegment would only ever reach
// the first of the two, leaving the other uneditable but still evaluated.
func TestUpdateSegment_RenameOntoASiblingIsRefused(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	if _, err := uc.CreateSegment("baseLayer", model.Segment{
		ID:       "seg2",
		Strategy: model.StrategyStatic,
		Static:   &model.StaticConfig{Mappings: map[string]string{}, Default: "y"},
	}, nil); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := uc.UpdateSegment("baseLayer", "seg1", model.Segment{
		ID:       "seg2",
		Strategy: model.StrategyStatic,
		Static:   &model.StaticConfig{Mappings: map[string]string{}, Default: "x"},
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected a duplicate-id error, got %v", err)
	}
}

// Renaming to its own id is not a collision with itself.
func TestUpdateSegment_RenameToTheSameIDIsFine(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	if _, err := uc.UpdateSegment("baseLayer", "seg1", model.Segment{
		ID:       "seg1",
		Strategy: model.StrategyStatic,
		Static:   &model.StaticConfig{Mappings: map[string]string{}, Default: "x"},
	}, nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
