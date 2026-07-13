package validation

import (
	"fmt"
	"regexp"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// validateLookups validates lookup tables and populates index with id->table.
func validateLookups(tables []model.LookupTable, index map[string]model.LookupTable) []string {
	var errs []string
	seen := make(map[string]bool, len(tables))
	for _, t := range tables {
		if t.ID == "" {
			errs = append(errs, "lookup table: id is required")
			continue
		}
		if !slugRe.MatchString(t.ID) {
			errs = append(errs, fmt.Sprintf("lookup %q: id must be a slug (lowercase alphanumeric and hyphens)", t.ID))
		}
		if seen[t.ID] {
			errs = append(errs, fmt.Sprintf("lookup %q: duplicate id", t.ID))
		}
		seen[t.ID] = true
		index[t.ID] = t

		if t.Name == "" {
			errs = append(errs, fmt.Sprintf("lookup %q: name is required", t.ID))
		}
		if !validKeyType(t.KeyType) {
			errs = append(errs, fmt.Sprintf("lookup %q: keyType %q must be string, number, or boolean", t.ID, t.KeyType))
			continue // entry checks need a valid keyType
		}
		for i, e := range t.Entries {
			if e.Key == nil {
				errs = append(errs, fmt.Sprintf("lookup %q entry %d: key is required", t.ID, i))
				continue
			}
			if !keyMatchesType(e.Key, t.KeyType) {
				errs = append(errs, fmt.Sprintf("lookup %q entry %d: key %v does not match keyType %q", t.ID, i, e.Key, t.KeyType))
			}
		}
	}
	return errs
}

func validKeyType(ft model.FieldType) bool {
	return ft == model.FieldTypeString || ft == model.FieldTypeNumber
}

// keyMatchesType reports whether a JSON-decoded key value matches the declared type.
func keyMatchesType(key interface{}, ft model.FieldType) bool {
	switch ft {
	case model.FieldTypeString:
		_, ok := key.(string)
		return ok
	case model.FieldTypeNumber:
		switch key.(type) {
		case float64, float32, int, int64:
			return true
		default:
			return false
		}
	default:
		return false
	}
}
