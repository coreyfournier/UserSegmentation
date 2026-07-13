package model

import "time"

// Layer is an independent dimension of segmentation evaluated in order.
type Layer struct {
	Name     string    `json:"name"`
	Order    int       `json:"order"`
	Segments []Segment `json:"segments"`
	// DefaultLanguage is the fallback locale for message rendering when a
	// requested language has no message on the winning rule. Empty means "en".
	DefaultLanguage string `json:"defaultLanguage,omitempty"`
}

// Snapshot is an immutable, pre-validated configuration loaded atomically.
type Snapshot struct {
	Version      int           `json:"version"`
	LastModified *time.Time    `json:"last_modified,omitempty"`
	Layers       []Layer       `json:"layers"`
	Lookups      []LookupTable `json:"lookups,omitempty"`
}
