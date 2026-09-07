package model

// SearchHit is one thing the query matched.
//
// Deliberately flat and self-describing rather than a nested layer/segment
// tree: a caller renders a result list without knowing the containment rules,
// and a store that pages results does not have to reassemble a hierarchy from
// a partial page.
type SearchHit struct {
	// Kind is "layer" or "segment".
	Kind string `json:"kind"`
	// Layer is the layer's name — the layer itself for a layer hit, the
	// containing layer for a segment hit.
	Layer string `json:"layer"`
	// Segment is the segment's id, empty for a layer hit.
	Segment string `json:"segment,omitempty"`
	// Field names what matched: "name", "id" or "strategy". A caller shows the
	// author why a row is in the list without re-running the match itself.
	Field string `json:"field"`
	// Value is the matched text, so a caller can highlight the term inside it.
	Value string `json:"value"`
}

// SearchResult is one query's answer.
type SearchResult struct {
	Query string      `json:"query"`
	Hits  []SearchHit `json:"hits"`
	// Truncated reports that the store stopped at the limit and more matches
	// exist. Meaningless for the in-memory store at today's config sizes, and
	// present precisely because it will not stay that way — a caller written
	// against this field keeps working when the store is a database.
	Truncated bool `json:"truncated"`
}
