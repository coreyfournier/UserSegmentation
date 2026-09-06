package model

import "time"

// Layer is an independent dimension of segmentation evaluated as a node in a
// dependency graph. Execution order is derived from DependsOn, not from any
// ordinal field.
type Layer struct {
	// Key is the layer's stable identity: the key it occupies in the
	// evaluation response, what DependsOn holds, what "layer:x" resolves, what
	// a request filters on, and how the admin API addresses it.
	//
	// It is constrained to a C# identifier — letters, digits and underscores,
	// not starting with a digit, and not a reserved word — because a consumer
	// generates types from the response, and a key like "CT Rule" or
	// "base-tier" cannot be a property name there.
	//
	// Changing it is allowed and is a deliberate act: every internal reference
	// is rewritten in the same transaction (AdminUseCase.UpdateLayer), but no
	// external consumer reading the old key can be reached from here.
	Key string `json:"key"`
	// Name is the friendly label, shown in the UI and emitted inside the
	// layer's result object. Optional, free-form, and with no uniqueness rule:
	// it identifies nothing, so two layers may share one. A layer without a
	// name is shown and reported by its key.
	Name string `json:"name,omitempty"`
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
