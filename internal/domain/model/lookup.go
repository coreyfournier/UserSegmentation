package model

// LookupEntry is a single key/value pair in a lookup table. Key is used for
// matching; Value is an optional human-readable description of what the key means.
type LookupEntry struct {
	Key   interface{} `json:"key"`
	Value string      `json:"value,omitempty"`
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
}

// Keys returns the entry keys as a slice, for array-style membership checks.
func (t *LookupTable) Keys() []interface{} {
	keys := make([]interface{}, len(t.Entries))
	for i, e := range t.Entries {
		keys[i] = e.Key
	}
	return keys
}
