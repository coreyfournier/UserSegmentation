package validation

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func snapWithLookup(t model.LookupTable, seg model.Segment) *model.Snapshot {
	return &model.Snapshot{
		Lookups: []model.LookupTable{t},
		Layers:  []model.Layer{{Name: "l", Order: 1, Segments: []model.Segment{seg}}},
	}
}

func zipRuleSegment(op model.Operator, tableID string) model.Segment {
	return model.Segment{
		ID: "s", Strategy: "rule",
		InputSchema: model.InputSchema{"zip": {Type: model.FieldTypeString}},
		Rules: []model.Rule{{
			RuleName:   "r",
			Expression: &model.Expression{Field: "zip", Operator: op, Value: tableID},
		}},
	}
}

func TestValidate_LookupValidReference(t *testing.T) {
	tbl := model.LookupTable{ID: "premium-zips", Name: "Premium", KeyType: model.FieldTypeString,
		Entries: []model.LookupEntry{{Key: "90210"}}}
	if e := ValidateSnapshot(snapWithLookup(tbl, zipRuleSegment(model.OpInLookup, "premium-zips"))); e != nil {
		t.Fatalf("expected valid, got %v", e)
	}
}

func TestValidate_LookupDanglingReference(t *testing.T) {
	tbl := model.LookupTable{ID: "premium-zips", Name: "Premium", KeyType: model.FieldTypeString}
	e := ValidateSnapshot(snapWithLookup(tbl, zipRuleSegment(model.OpInLookup, "nope")))
	if e == nil || !strings.Contains(e.Error(), "unknown lookup table") {
		t.Fatalf("expected dangling reference error, got %v", e)
	}
}

func TestValidate_LookupTypeMismatch(t *testing.T) {
	// Table is number-keyed but field zip is string.
	tbl := model.LookupTable{ID: "nums", Name: "Nums", KeyType: model.FieldTypeNumber,
		Entries: []model.LookupEntry{{Key: 1.0}}}
	e := ValidateSnapshot(snapWithLookup(tbl, zipRuleSegment(model.OpInLookup, "nums")))
	if e == nil || !strings.Contains(e.Error(), "does not match lookup") {
		t.Fatalf("expected type mismatch error, got %v", e)
	}
}

func TestValidate_LookupBadKeyType(t *testing.T) {
	tbl := model.LookupTable{ID: "bad", Name: "Bad", KeyType: model.FieldTypeArray}
	e := ValidateSnapshot(&model.Snapshot{Lookups: []model.LookupTable{tbl}})
	if e == nil || !strings.Contains(e.Error(), "keyType") {
		t.Fatalf("expected keyType error, got %v", e)
	}
}

func TestValidate_LookupEntryKeyTypeMismatch(t *testing.T) {
	tbl := model.LookupTable{ID: "nums", Name: "Nums", KeyType: model.FieldTypeNumber,
		Entries: []model.LookupEntry{{Key: "not-a-number"}}}
	e := ValidateSnapshot(&model.Snapshot{Lookups: []model.LookupTable{tbl}})
	if e == nil || !strings.Contains(e.Error(), "does not match keyType") {
		t.Fatalf("expected entry key type error, got %v", e)
	}
}

func TestValidate_LookupBadSlugAndDuplicate(t *testing.T) {
	snap := &model.Snapshot{Lookups: []model.LookupTable{
		{ID: "Bad Id", Name: "x", KeyType: model.FieldTypeString},
		{ID: "dup", Name: "a", KeyType: model.FieldTypeString},
		{ID: "dup", Name: "b", KeyType: model.FieldTypeString},
	}}
	e := ValidateSnapshot(snap)
	if e == nil || !strings.Contains(e.Error(), "slug") || !strings.Contains(e.Error(), "duplicate id") {
		t.Fatalf("expected slug + duplicate errors, got %v", e)
	}
}
