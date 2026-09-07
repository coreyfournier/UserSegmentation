package application

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// twoSegmentLayer is the shape that exposed the overwrite: one layer holding
// two segments, which is what the tests panel shows from inside either one.
func twoSegmentLayer(t *testing.T) *AdminUseCase {
	t.Helper()
	uc, _, _ := newTestAdminUC()
	_, err := uc.CreateLayer(model.Layer{
		Key: "gates",
		Segments: []model.Segment{
			{ID: "stage1", Strategy: model.StrategyRule, Default: "no"},
			{ID: "stage2", Strategy: model.StrategyRule, Default: "no"},
		},
	})
	if err != nil {
		t.Fatalf("create layer: %v", err)
	}
	return uc
}

// The regression. Two segments in one layer each want a test called "happy
// path": before segments were part of the naming slot the second create was
// refused as a duplicate, and saving through the editor — which had the first
// segment's test selected, because the panel showed the whole layer's — wrote
// over it instead.
func TestCreateTest_SameNameInTwoSegments(t *testing.T) {
	uc := twoSegmentLayer(t)

	if _, err := uc.CreateTest(model.SavedTest{Layer: "gates", Segment: "stage1", Name: "happy path"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	snap, err := uc.CreateTest(model.SavedTest{Layer: "gates", Segment: "stage2", Name: "happy path"})
	if err != nil {
		t.Fatalf("the same name under a different segment must be allowed, got %v", err)
	}
	if len(snap.Tests) != 2 {
		t.Fatalf("expected both tests to exist, got %d", len(snap.Tests))
	}
	if snap.Tests[0].ID == snap.Tests[1].ID {
		t.Errorf("ids collided: %q", snap.Tests[0].ID)
	}
	if snap.Tests[1].ID != "gates-stage2-happy-path" {
		t.Errorf("the segment should be part of the id, got %q", snap.Tests[1].ID)
	}

	// Within one segment the name is still unique.
	_, err = uc.CreateTest(model.SavedTest{Layer: "gates", Segment: "stage1", Name: "Happy Path"})
	if err == nil || !strings.Contains(err.Error(), "already has a test named") {
		t.Errorf("expected a duplicate-name error within the segment, got %v", err)
	}
}

func TestCreateTest_RejectsUnknownSegment(t *testing.T) {
	uc := twoSegmentLayer(t)
	_, err := uc.CreateTest(model.SavedTest{Layer: "gates", Segment: "stage9", Name: "t"})
	if err == nil || !strings.Contains(err.Error(), "no segment") {
		t.Errorf("expected an unknown-segment error, got %v", err)
	}
}

// A rename may not take another test's name within the same segment, but the
// identical name in the sibling segment is not a collision.
func TestUpdateTest_RenameIsScopedToTheSegment(t *testing.T) {
	uc := twoSegmentLayer(t)
	if _, err := uc.CreateTest(model.SavedTest{Layer: "gates", Segment: "stage1", Name: "a"}); err != nil {
		t.Fatalf("create a: %v", err)
	}
	if _, err := uc.CreateTest(model.SavedTest{Layer: "gates", Segment: "stage1", Name: "b"}); err != nil {
		t.Fatalf("create b: %v", err)
	}
	if _, err := uc.CreateTest(model.SavedTest{Layer: "gates", Segment: "stage2", Name: "c"}); err != nil {
		t.Fatalf("create c: %v", err)
	}

	if _, err := uc.UpdateTest("gates-stage1-b", model.SavedTest{Name: "a"}); err == nil {
		t.Error("renaming onto a sibling's name in the same segment should be refused")
	}
	if _, err := uc.UpdateTest("gates-stage2-c", model.SavedTest{Name: "a"}); err != nil {
		t.Errorf("the same name in another segment is not a collision, got %v", err)
	}
}

// A segment rename carries its tests, in the same transaction. Leaving them
// behind would point them at a segment that no longer exists, which the
// snapshot validator refuses outright.
func TestUpdateSegment_RenameCarriesItsTests(t *testing.T) {
	uc := twoSegmentLayer(t)
	if _, err := uc.CreateTest(model.SavedTest{Layer: "gates", Segment: "stage2", Name: "t"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	snap, err := uc.UpdateSegment("gates", "stage2", model.Segment{
		ID: "stage2Gates", Strategy: model.StrategyRule, Default: "no",
	}, nil)
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if got := snap.Tests[0].Segment; got != "stage2Gates" {
		t.Errorf("test still filed under %q", got)
	}
}

// Deleting a segment takes its tests with it — they name inputs for something
// that no longer exists, and validation would reject the delete otherwise.
func TestDeleteSegment_RemovesItsTests(t *testing.T) {
	uc := twoSegmentLayer(t)
	for _, s := range []string{"stage1", "stage2"} {
		if _, err := uc.CreateTest(model.SavedTest{Layer: "gates", Segment: s, Name: "t"}); err != nil {
			t.Fatalf("create for %s: %v", s, err)
		}
	}

	snap, err := uc.DeleteSegment("gates", "stage2", nil)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(snap.Tests) != 1 || snap.Tests[0].Segment != "stage1" {
		t.Errorf("expected only stage1's test to remain, got %+v", snap.Tests)
	}
}

// A test written before segments existed keeps running the layer as a whole.
// Guessing which segment it meant would change what it exercises.
func TestSavedTest_LayerScopedRemainsValid(t *testing.T) {
	uc := twoSegmentLayer(t)
	snap, err := uc.CreateTest(model.SavedTest{Layer: "gates", Name: "legacy"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if snap.Tests[0].Segment != "" {
		t.Errorf("an unscoped test must stay unscoped, got %q", snap.Tests[0].Segment)
	}
	if snap.Tests[0].ID != "gates-legacy" {
		t.Errorf("unexpected id %q", snap.Tests[0].ID)
	}
}
