package application

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestCreateLookup_AutoSlug(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	snap, err := uc.CreateLookup(model.LookupTable{
		Name: "Premium Zips", KeyType: model.FieldTypeString,
		Entries: []model.LookupEntry{{Key: "90210", Value: "Beverly Hills"}},
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if len(snap.Lookups) != 1 || snap.Lookups[0].ID != "premium-zips" {
		t.Fatalf("expected id premium-zips, got %v", snap.Lookups)
	}
}

func TestCreateLookup_SlugUniqueness(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	if _, err := uc.CreateLookup(model.LookupTable{Name: "Zips", KeyType: model.FieldTypeString}); err != nil {
		t.Fatal(err)
	}
	snap, err := uc.CreateLookup(model.LookupTable{Name: "Zips", KeyType: model.FieldTypeString})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Lookups[1].ID != "zips-2" {
		t.Fatalf("expected zips-2, got %q", snap.Lookups[1].ID)
	}
}

func TestUpdateLookup_PreservesIdAndKeyType(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	_, _ = uc.CreateLookup(model.LookupTable{Name: "Zips", KeyType: model.FieldTypeString})
	snap, err := uc.UpdateLookup("zips", model.LookupTable{
		ID: "hacked", Name: "Postal Codes", KeyType: model.FieldTypeNumber,
		Entries: []model.LookupEntry{{Key: "10001"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := snap.Lookups[0]
	if got.ID != "zips" || got.KeyType != model.FieldTypeString {
		t.Fatalf("id/keyType must be immutable, got %+v", got)
	}
	if got.Name != "Postal Codes" || len(got.Entries) != 1 {
		t.Fatalf("name/entries should update, got %+v", got)
	}
}

func TestDeleteLookup_BlockedWhenReferenced(t *testing.T) {
	uc, s, _ := newTestAdminUC()
	// Seed a snapshot with a lookup and a rule referencing it.
	s.Swap(&model.Snapshot{
		Version: 1,
		Lookups: []model.LookupTable{{ID: "zips", Name: "Zips", KeyType: model.FieldTypeString,
			Entries: []model.LookupEntry{{Key: "90210"}}}},
		Layers: []model.Layer{{Name: "l", Order: 1, Segments: []model.Segment{{
			ID: "seg", Strategy: "rule",
			InputSchema: model.InputSchema{"zip": {Type: model.FieldTypeString}},
			Rules: []model.Rule{{RuleName: "r",
				Expression: &model.Expression{Field: "zip", Operator: model.OpInLookup, Value: "zips"}}},
		}}}},
	})
	_, err := uc.DeleteLookup("zips")
	refErr, ok := err.(*LookupReferencedError)
	if !ok {
		t.Fatalf("expected LookupReferencedError, got %v", err)
	}
	if len(refErr.Refs) != 1 {
		t.Fatalf("expected 1 reference, got %v", refErr.Refs)
	}
}

func TestDeleteLookup_SucceedsWhenUnreferenced(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	_, _ = uc.CreateLookup(model.LookupTable{Name: "Zips", KeyType: model.FieldTypeString})
	snap, err := uc.DeleteLookup("zips")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if len(snap.Lookups) != 0 {
		t.Fatalf("expected lookup removed, got %v", snap.Lookups)
	}
}
