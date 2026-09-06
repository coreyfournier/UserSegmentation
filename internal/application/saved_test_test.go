package application

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func testWithLayers(t *testing.T) (*AdminUseCase, *AdminUseCase) {
	t.Helper()
	uc, _, _ := newTestAdminUC()
	if _, err := uc.CreateLayer(model.Layer{Key: "gate"}); err != nil {
		t.Fatalf("create layer: %v", err)
	}
	return uc, uc
}

func TestCreateTest_DerivesIdAndPersists(t *testing.T) {
	uc, _ := testWithLayers(t)

	snap, err := uc.CreateTest(model.SavedTest{
		Layer:   "gate",
		Name:    "Missing subject key",
		Context: map[string]interface{}{"plan": "pro"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(snap.Tests) != 1 {
		t.Fatalf("expected one test, got %d", len(snap.Tests))
	}
	got := snap.Tests[0]
	if got.ID != "gate-missing-subject-key" {
		t.Errorf("unexpected id %q", got.ID)
	}
	if got.Context["plan"] != "pro" {
		t.Errorf("context not persisted: %+v", got.Context)
	}
}

// Names are unique per layer, not globally: the same name is a reasonable
// label for a test of two different layers.
func TestCreateTest_NameUniquePerLayerOnly(t *testing.T) {
	uc, _ := testWithLayers(t)
	if _, err := uc.CreateLayer(model.Layer{Key: "other"}); err != nil {
		t.Fatalf("create layer: %v", err)
	}

	if _, err := uc.CreateTest(model.SavedTest{Layer: "gate", Name: "Empty context"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := uc.CreateTest(model.SavedTest{Layer: "other", Name: "Empty context"}); err != nil {
		t.Errorf("the same name under a different layer must be allowed, got %v", err)
	}
	_, err := uc.CreateTest(model.SavedTest{Layer: "gate", Name: "empty context"})
	if err == nil || !strings.Contains(err.Error(), "already has a test named") {
		t.Errorf("expected a duplicate-name error (case-insensitive), got %v", err)
	}
}

func TestCreateTest_RejectsUnknownLayer(t *testing.T) {
	uc, _ := testWithLayers(t)
	_, err := uc.CreateTest(model.SavedTest{Layer: "nope", Name: "x"})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected an unknown-layer error, got %v", err)
	}
}

// A test whose whole point is an absent required field has to be storable.
func TestCreateTest_ContextIsNotValidatedAgainstTheSchema(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	if _, err := uc.CreateLayer(model.Layer{
		Key:         "strict",
		InputSchema: model.InputSchema{"age": {Type: model.FieldTypeNumber, Required: true}},
	}); err != nil {
		t.Fatalf("create layer: %v", err)
	}
	if _, err := uc.CreateTest(model.SavedTest{
		Layer: "strict", Name: "no age at all", Context: map[string]interface{}{},
	}); err != nil {
		t.Errorf("a test omitting a required field must be storable, got %v", err)
	}
}

func TestListTests_FiltersByLayer(t *testing.T) {
	uc, _ := testWithLayers(t)
	if _, err := uc.CreateLayer(model.Layer{Key: "other"}); err != nil {
		t.Fatalf("create layer: %v", err)
	}
	_, _ = uc.CreateTest(model.SavedTest{Layer: "gate", Name: "a"})
	_, _ = uc.CreateTest(model.SavedTest{Layer: "other", Name: "b"})

	if all := uc.ListTests(""); len(all) != 2 {
		t.Errorf("expected both tests unfiltered, got %d", len(all))
	}
	only := uc.ListTests("gate")
	if len(only) != 1 || only[0].Layer != "gate" {
		t.Errorf("expected only the gate test, got %+v", only)
	}
}

func TestUpdateTest_ChangesContentNotIdentity(t *testing.T) {
	uc, _ := testWithLayers(t)
	snap, err := uc.CreateTest(model.SavedTest{Layer: "gate", Name: "first"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	id := snap.Tests[0].ID

	snap, err = uc.UpdateTest(id, model.SavedTest{
		// Layer and ID here are deliberately wrong; both must be ignored.
		ID: "hacked", Layer: "elsewhere",
		Name:    "renamed",
		Context: map[string]interface{}{"x": 1.0},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	got := snap.Tests[0]
	if got.ID != id {
		t.Errorf("id must be immutable, got %q", got.ID)
	}
	if got.Layer != "gate" {
		t.Errorf("layer must be immutable, got %q", got.Layer)
	}
	if got.Name != "renamed" || got.Context["x"] != 1.0 {
		t.Errorf("content not updated: %+v", got)
	}
}

func TestDeleteTest(t *testing.T) {
	uc, _ := testWithLayers(t)
	snap, _ := uc.CreateTest(model.SavedTest{Layer: "gate", Name: "doomed"})
	id := snap.Tests[0].ID

	snap, err := uc.DeleteTest(id)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(snap.Tests) != 0 {
		t.Errorf("expected no tests, got %+v", snap.Tests)
	}
	if _, err := uc.DeleteTest(id); err == nil {
		t.Error("expected deleting a missing test to error")
	}
}

// A test is filed under a layer key, so a key change has to take it along —
// otherwise it names a layer that no longer exists and validation rejects the
// whole update.
func TestUpdateLayer_KeyChangeMovesItsTests(t *testing.T) {
	uc, _ := testWithLayers(t)
	if _, err := uc.CreateTest(model.SavedTest{Layer: "gate", Name: "keeps up"}); err != nil {
		t.Fatalf("create test: %v", err)
	}

	snap, err := uc.UpdateLayer("gate", model.Layer{Key: "gateway"})
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if snap.Tests[0].Layer != "gateway" {
		t.Errorf("expected the test to follow the key, got %q", snap.Tests[0].Layer)
	}

	// And the preview reports it, so the author is told before saving.
	refs, err := uc.PreviewRekey("gateway", "gatewayV2")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	var sawTest bool
	for _, r := range refs {
		if r.Where == "test" {
			sawTest = true
		}
	}
	if !sawTest {
		t.Errorf("expected the preview to mention the test, got %+v", refs)
	}
}

// Deleting a layer takes its tests with it. Leaving them would name a layer
// that no longer exists, and the delete would fail validation on data the
// author did not think they were touching.
func TestDeleteLayer_RemovesItsTests(t *testing.T) {
	uc, _ := testWithLayers(t)
	if _, err := uc.CreateLayer(model.Layer{Key: "survivor"}); err != nil {
		t.Fatalf("create layer: %v", err)
	}
	_, _ = uc.CreateTest(model.SavedTest{Layer: "gate", Name: "goes away"})
	_, _ = uc.CreateTest(model.SavedTest{Layer: "survivor", Name: "stays"})

	snap, err := uc.DeleteLayer("gate")
	if err != nil {
		t.Fatalf("delete layer: %v", err)
	}
	if len(snap.Tests) != 1 || snap.Tests[0].Layer != "survivor" {
		t.Errorf("expected only the survivor's test to remain, got %+v", snap.Tests)
	}
}
