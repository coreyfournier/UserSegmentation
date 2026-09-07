package model

// LookupEntry is a single key/value pair in a lookup table. Key is used for
// matching and is the stable identifier a consumer may reference in code; Value
// is the human-readable label and is free to change. Order is the entry's
// position in the table's ordering — always persisted, even when inferred from
// list position, because a relational store cannot reorder rows cheaply.
type LookupEntry struct {
	Key   interface{} `json:"key"`
	Value string      `json:"value,omitempty"`
	Order int         `json:"order"`
}

// LookupTable is a centralized, named set of typed keys referenced by rules via
// the in_lookup / not_in_lookup operators.
//
// ID is an immutable internal identifier (auto-slugged from Name at creation)
// used by rule references; Name is a mutable display name. KeyType is immutable
// after creation.
type LookupTable struct {
	ID      string        `json:"id"`
	Name    string        `json:"name"`
	KeyType FieldType     `json:"keyType"`
	Entries []LookupEntry `json:"entries"`
	// Description is the author's note on how the table is meant to be used,
	// including any cross-table ordering scheme. Ordering invariants are
	// documented here rather than validated.
	Description string `json:"description,omitempty"`
	// EmitOrder includes each entry's Order in the evaluation response.
	EmitOrder bool `json:"emitOrder,omitempty"`
	// CustomOrder means the numbers are hand-authored rather than inferred from
	// list position. Gaps are the mechanism for interleaving several tables into
	// one ordering, so nothing checks contiguity or uniqueness.
	CustomOrder bool `json:"customOrder,omitempty"`
}

// Keys returns the entry keys as a slice, for array-style membership checks.
func (t *LookupTable) Keys() []interface{} {
	keys := make([]interface{}, len(t.Entries))
	for i, e := range t.Entries {
		keys[i] = e.Key
	}
	return keys
}
