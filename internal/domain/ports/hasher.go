package ports

// Hasher computes a deterministic bucket index for percentage-based segmentation.
type Hasher interface {
	Bucket(userKey, salt string) int
}
