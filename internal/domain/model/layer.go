package model

// Layer is an independent dimension of segmentation evaluated in order.
type Layer struct {
	Name     string    `json:"name"`
	Order    int       `json:"order"`
	Segments []Segment `json:"segments"`
}

// Snapshot is an immutable, pre-validated configuration loaded atomically.
type Snapshot struct {
	Version int     `json:"version"`
	Layers  []Layer `json:"layers"`
}
