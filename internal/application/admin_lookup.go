package application

import (
	"fmt"
	"strings"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// normalizeOrder stamps list position onto each entry unless the table authors
// its own numbers. The result is always persisted, so no store has to infer it.
func normalizeOrder(t *model.LookupTable) {
	if t.CustomOrder {
		return
	}
	for i := range t.Entries {
		t.Entries[i].Order = i
	}
}

// LookupReferencedError indicates a lookup table cannot be deleted because rules
// still reference it.
type LookupReferencedError struct {
	ID   string
	Refs []string
}

func (e *LookupReferencedError) Error() string {
	return fmt.Sprintf("lookup %q is referenced by %s", e.ID, strings.Join(e.Refs, ", "))
}

// ListLookups returns the configured lookup tables.
func (uc *AdminUseCase) ListLookups() []model.LookupTable {
	snap := uc.store.Get()
	if snap == nil {
		return nil
	}
	return snap.Lookups
}

// CreateLookup adds a lookup table, deriving an immutable id by slugging the name.
func (uc *AdminUseCase) CreateLookup(table model.LookupTable) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	if strings.TrimSpace(table.Name) == "" {
		return nil, fmt.Errorf("lookup name is required")
	}
	snap := uc.cloneSnapshot()

	taken := make(map[string]bool, len(snap.Lookups))
	for _, t := range snap.Lookups {
		taken[t.ID] = true
	}
	table.ID = uniqueSlug(slugify(table.Name), taken)
	if table.ID == "" {
		return nil, fmt.Errorf("could not derive a valid id from name %q", table.Name)
	}
	if table.Entries == nil {
		table.Entries = []model.LookupEntry{}
	}
	normalizeOrder(&table)
	snap.Lookups = append(snap.Lookups, table)
	return uc.commitSnapshot(snap)
}

// UpdateLookup updates a lookup table's display name, description, ordering
// flags (emitOrder, customOrder), and entries. The id and keyType are
// immutable and preserved from the existing table.
func (uc *AdminUseCase) UpdateLookup(id string, updated model.LookupTable) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	idx := -1
	for i := range snap.Lookups {
		if snap.Lookups[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("lookup %q not found", id)
	}
	if strings.TrimSpace(updated.Name) == "" {
		return nil, fmt.Errorf("lookup name is required")
	}
	entries := updated.Entries
	if entries == nil {
		entries = []model.LookupEntry{}
	}
	// Preserve immutable id and keyType.
	snap.Lookups[idx].Name = updated.Name
	snap.Lookups[idx].Description = updated.Description
	snap.Lookups[idx].EmitOrder = updated.EmitOrder
	snap.Lookups[idx].CustomOrder = updated.CustomOrder
	snap.Lookups[idx].Entries = entries
	normalizeOrder(&snap.Lookups[idx])
	return uc.commitSnapshot(snap)
}

// DeleteLookup removes a lookup table, refusing if any rule references it.
func (uc *AdminUseCase) DeleteLookup(id string) (*model.Snapshot, error) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	snap := uc.cloneSnapshot()
	idx := -1
	for i := range snap.Lookups {
		if snap.Lookups[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil, fmt.Errorf("lookup %q not found", id)
	}
	if refs := lookupReferences(snap, id); len(refs) > 0 {
		return nil, &LookupReferencedError{ID: id, Refs: refs}
	}
	snap.Lookups = append(snap.Lookups[:idx], snap.Lookups[idx+1:]...)
	return uc.commitSnapshot(snap)
}

// lookupReferences returns human-readable locations of rules referencing the id.
func lookupReferences(snap *model.Snapshot, id string) []string {
	var refs []string
	for _, layer := range snap.Layers {
		for _, seg := range layer.Segments {
			for i := range seg.Rules {
				refs = append(refs, findLookupRefs(&seg.Rules[i], id, seg.ID)...)
			}
			for i := range seg.Overrides {
				refs = append(refs, findLookupRefs(&seg.Overrides[i], id, seg.ID)...)
			}
		}
	}
	return refs
}

func findLookupRefs(r *model.Rule, id, segID string) []string {
	var refs []string
	if r.Condition != nil {
		op := r.Condition.Operator
		if op == model.OpInLookup || op == model.OpNotInLookup {
			if v, ok := r.Condition.Value.(string); ok && v == id {
				refs = append(refs, fmt.Sprintf("segment %q rule %q", segID, r.RuleName))
			}
		}
	}
	for i := range r.Rules {
		refs = append(refs, findLookupRefs(&r.Rules[i], id, segID)...)
	}
	return refs
}

// slugify converts a display name into a lowercase hyphenated slug.
func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevHyphen := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		case r == ' ' || r == '-' || r == '_':
			if b.Len() > 0 && !prevHyphen {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// uniqueSlug returns base, or base-2/base-3/... if already taken.
func uniqueSlug(base string, taken map[string]bool) string {
	if base == "" {
		return ""
	}
	if !taken[base] {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !taken[candidate] {
			return candidate
		}
	}
}
