package model

import "time"

// Layer is an independent dimension of segmentation evaluated as a node in a
// dependency graph. Execution order is derived from DependsOn, not from any
// ordinal field.
type Layer struct {
	Name string `json:"name"`
	// DependsOn names the layers that must resolve before this one runs. A rule
	// referencing "layer:x" must declare x here. If any dependency does not
	// resolve, this layer is skipped rather than evaluated against absent context.
	DependsOn []string  `json:"dependsOn,omitempty"`
	Segments  []Segment `json:"segments"`
	// DefaultLanguage is the fallback locale for message rendering when a
	// requested language has no message on the winning rule. Empty means "en".
	DefaultLanguage string `json:"defaultLanguage,omitempty"`
	// InputSchema declares the fields every segment in this layer reads. It is
	// the only place an input schema is declared — segments do not have one.
	//
	// A layer that declares none, whose segments declare no computed fields,
	// skips rule-field validation entirely. That escape hatch is deliberate and
	// several shipped layers rely on it.
	InputSchema InputSchema `json:"inputSchema,omitempty"`
	// OutputSchema declares the record every segment in this layer emits with
	// each reported item. Like InputSchema, it is the only place it is declared.
	OutputSchema OutputSchema `json:"outputSchema,omitempty"`
}

// Snapshot is an immutable, pre-validated configuration loaded atomically.
type Snapshot struct {
	Version      int           `json:"version"`
	LastModified *time.Time    `json:"last_modified,omitempty"`
	Layers       []Layer       `json:"layers"`
	Lookups      []LookupTable `json:"lookups,omitempty"`
}
