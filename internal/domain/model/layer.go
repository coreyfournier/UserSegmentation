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
	// Revision is the optimistic-concurrency token: it advances on every write
	// that touches this layer, its segments included. The layer is the
	// aggregate — it owns its segments and the schemas they share — so one
	// revision guards the whole of it. That is what makes a schema change safe
	// to guard at all: dropping an output field changes what every segment
	// must author, so a segment save that predates it has to be refused.
	//
	// An integer rather than the timestamp below, because two writes inside
	// one clock tick share a timestamp and would compare equal. Every store
	// has a mechanism for this shape of check.
	// Always serialised, omitempty included, because a concurrency token is
	// meaningful at zero: a layer loaded from config that predates this field
	// starts at 0, and a client has to send that rather than guess whether an
	// absent field means zero or means "do not check".
	Revision int `json:"revision"`
	// UpdatedAt is when that revision was written. For display: it answers
	// "how stale is what I am looking at", which a bare counter cannot.
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
	// FirstMatchOnly stops the layer at the first segment that applies, instead
	// of running every applicable one.
	//
	// The default is to run them all, and the default is the safe direction:
	// forgetting to set a flag then produces more reporting than expected,
	// never silently less. A segment quietly not running is the failure that
	// cannot be seen — nothing in the response says it was skipped.
	//
	// This only ever changes anything for segments that resolve no value.
	// A rule, static or percentage segment answers the layer's question with
	// one value, so the first one to answer ends the layer whatever this says
	// — two transfer fees is not a thing the response can express. A checklist
	// resolves no value and contributes findings to a list, so several of them
	// merge without ambiguity. See Evaluator.evaluateLayer.
	FirstMatchOnly bool `json:"firstMatchOnly,omitempty"`
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
	// Tests are saved evaluation inputs, filed per layer. Authoring data
	// rather than evaluation config: nothing in the engine reads them.
	Tests []SavedTest `json:"tests,omitempty"`
}
