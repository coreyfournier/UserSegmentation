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
