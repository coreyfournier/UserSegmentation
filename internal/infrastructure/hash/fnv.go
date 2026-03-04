package hash

import (
	hash_fnv "hash/fnv"
)

// FNV implements ports.Hasher using FNV-1a.
type FNV struct{}

// Bucket returns a deterministic bucket index [0, 100) for the given user key and salt.
func (f *FNV) Bucket(userKey, salt string) int {
	h := hash_fnv.New32a()
	_, _ = h.Write([]byte(salt + ":" + userKey))
	return int(h.Sum32() % 100)
}
