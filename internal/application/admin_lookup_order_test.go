package application

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestCreateLookup_InfersOrderFromPosition(t *testing.T) {
	uc, _, _ := newTestAdminUC()

	snap, err := uc.CreateLookup(model.LookupTable{
		Name:    "severity",
		KeyType: model.FieldTypeString,
		Entries: []model.LookupEntry{
			{Key: "Critical", Value: "Blocks access"},
			{Key: "Warning", Value: "Needs attention"},
		},
	})
	if err != nil {
		t.Fatalf("CreateLookup: %v", err)
	}

	tbl := snap.Lookups[len(snap.Lookups)-1]
	if tbl.Entries[0].Order != 0 || tbl.Entries[1].Order != 1 {
		t.Fatalf("expected inferred orders 0,1 got %d,%d",
			tbl.Entries[0].Order, tbl.Entries[1].Order)
	}
}

func TestUpdateLookup_CustomOrderIsPreserved(t *testing.T) {
	uc, _, _ := newTestAdminUC()

	created, err := uc.CreateLookup(model.LookupTable{
		Name:    "diagnosis-type",
		KeyType: model.FieldTypeString,
		Entries: []model.LookupEntry{{Key: "A"}, {Key: "B"}},
	})
	if err != nil {
		t.Fatalf("CreateLookup: %v", err)
	}
	id := created.Lookups[len(created.Lookups)-1].ID

	snap, err := uc.UpdateLookup(id, model.LookupTable{
		Name:        "diagnosis-type",
		Description: "interleaves with severity: odds here, evens there",
		CustomOrder: true,
		Entries: []model.LookupEntry{
			{Key: "A", Order: 1},
			{Key: "B", Order: 5},
		},
	})
	if err != nil {
		t.Fatalf("UpdateLookup: %v", err)
	}

	var tbl model.LookupTable
	for _, l := range snap.Lookups {
		if l.ID == id {
			tbl = l
		}
	}
	if tbl.Entries[0].Order != 1 || tbl.Entries[1].Order != 5 {
		t.Fatalf("custom orders were overwritten: %d,%d",
			tbl.Entries[0].Order, tbl.Entries[1].Order)
	}
	if tbl.Description == "" {
		t.Fatal("description was not persisted")
	}
}

// Two tables may not share a name. Only the id is a reference, so duplicates
// work as far as the engine is concerned — but the author picks from a dropdown
// showing the name, and uniqueSlug quietly makes the ids differ, so two entries
// reading "severity" are indistinguishable there.
func TestCreateLookup_RejectsDuplicateName(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	base := model.LookupTable{Name: "severity", KeyType: model.FieldTypeString,
		Entries: []model.LookupEntry{{Key: "Critical"}}}
	if _, err := uc.CreateLookup(base); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := uc.CreateLookup(base); err == nil {
		t.Fatal("a second lookup with the same name must be rejected")
	}
	// Case and surrounding space must not be a way around it.
	if _, err := uc.CreateLookup(model.LookupTable{Name: "  Severity ", KeyType: model.FieldTypeString}); err == nil {
		t.Fatal("a differently-cased duplicate must also be rejected")
	}
	if _, err := uc.CreateLookup(model.LookupTable{Name: "category", KeyType: model.FieldTypeString}); err != nil {
		t.Fatalf("a genuinely new name must still be accepted: %v", err)
	}
}

func TestUpdateLookup_NameClashRules(t *testing.T) {
	uc, _, _ := newTestAdminUC()
	a, err := uc.CreateLookup(model.LookupTable{Name: "severity", KeyType: model.FieldTypeString})
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	idA := a.Lookups[len(a.Lookups)-1].ID
	if _, err := uc.CreateLookup(model.LookupTable{Name: "category", KeyType: model.FieldTypeString}); err != nil {
		t.Fatalf("create b: %v", err)
	}
	// Re-saving a table under its own name is not a clash with itself.
	if _, err := uc.UpdateLookup(idA, model.LookupTable{Name: "severity"}); err != nil {
		t.Fatalf("re-saving under its own name must be allowed: %v", err)
	}
	// Renaming onto another table's name is.
	if _, err := uc.UpdateLookup(idA, model.LookupTable{Name: "category"}); err == nil {
		t.Fatal("renaming onto an existing name must be rejected")
	}
}
