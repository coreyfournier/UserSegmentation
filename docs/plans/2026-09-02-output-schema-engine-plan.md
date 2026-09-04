# Output Schema (Engine) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a segment declare an output schema so each reported item emits a structured record — enough to reproduce a consumer's own domain shape without hand-mapping.

**Architecture:** Additive to the existing delegation chain. `Segment.OutputSchema` declares fields; each top-level rule authors their values; a new `evaluateOutputs` helper in the strategy package resolves them per reported item using machinery already there — `renderTemplate` for `${…}` templates, `compileFormula` for whole-value expressions. Lookup-bound fields are enriched to `{key, value, order}` from the lookup table. Ordering is persisted on the lookup entry.

A field may be marked `Required`, which is the caller's contract and is enforced at two points: an error at snapshot load if no authoring path supplies it (Task 5), and a warning at evaluation if it is nonetheless absent from what was emitted (Task 6). The two are not redundant — config validity cannot guarantee runtime presence, because an expression can fail and an override emits no outputs at all.

**Tech Stack:** Go 1.26, `github.com/expr-lang/expr` v1.17.8, standard library testing.

## Global Constraints

- Domain packages take no external dependencies beyond `expr-lang`. `internal/domain` must not import `internal/application` or `internal/infrastructure`.
- Values are authored on a **reporting rule** — a top-level entry in `Segment.Rules`, or, from Task 7, in `Segment.Overrides`. Never on inner And/Or branches: they do not report, so they carry no output values. Fields that do not vary per reporting rule are set once in `Segment.Outputs`.
- **No guarding, with one exception.** Do not validate lookup membership at evaluation time, and do not check order uniqueness or contiguity. Unknown keys and gaps are intentional. The table `Description` is where authors record invariants. The one exception is `OutputField.Required`, which *is* enforced — at snapshot load as an error (Task 5) and at evaluation as a warning (Task 6). It was added deliberately after this constraint was first written, because an unpopulated required output field breaks a caller who cannot see the omission, unlike the other invariants which an author holds for themselves.
- Order is **always persisted**, including when inferred from list position.
- A failed output value degrades: record a `RenderError`, **omit that field entirely**, keep the item. This is all-or-nothing in every eval mode, including `template` — a template whose token fails must NOT emit a half-rendered string, even though `renderTemplate` returns one. Omission is what keeps Task 6's required-field warning meaningful, since it tests presence.
- Consumers must not persist an emitted order or compare it across snapshots. Only relative order carries meaning.
- Run `go build ./...` and `go test ./...` before every commit. **Go is already on `PATH`** — verified `go version go1.26.5 windows/amd64` at `/c/Program Files/Go/bin/go`. Ignore the `export PATH="/c/Users/Corey/go/bin:$PATH"` line in `CLAUDE.md`: that directory does not exist on this machine, and it is why the session that wrote this plan could not run any of its verification steps. No export is needed.

---

### Task 1: Lookup ordering, description, and order inference

**Files:**
- Modify: `internal/domain/model/lookup.go`
- Modify: `internal/application/admin_lookup.go` (functions `CreateLookup`, `UpdateLookup`)
- Test: `internal/application/admin_lookup_order_test.go` (create)

**Interfaces:**
- Consumes: nothing.
- Produces: `model.LookupEntry.Order int`, `model.LookupTable.Description string`, `model.LookupTable.EmitOrder bool`, `model.LookupTable.CustomOrder bool`. Task 3 reads `Order` and `EmitOrder` when enriching lookup-bound output values.

- [ ] **Step 1: Write the failing test**

Create `internal/application/admin_lookup_order_test.go`:

```go
package application

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func TestCreateLookup_InfersOrderFromPosition(t *testing.T) {
	uc, _ := newAdminHarness(t)

	snap, err := uc.CreateLookup(model.LookupTable{
		Name:    "severity",
		KeyType: model.FieldTypeString,
		Entries: []model.LookupEntry{
			{Key: "Critical", Value: "Blocks access"},
			{Key: "Warning", Value: "Needs attention"},
		},
	})
	if err != nil {
		t.Fatalf("CreateLookup: %v", err)
	}

	tbl := snap.Lookups[len(snap.Lookups)-1]
	if tbl.Entries[0].Order != 0 || tbl.Entries[1].Order != 1 {
		t.Fatalf("expected inferred orders 0,1 got %d,%d",
			tbl.Entries[0].Order, tbl.Entries[1].Order)
	}
}

func TestUpdateLookup_CustomOrderIsPreserved(t *testing.T) {
	uc, _ := newAdminHarness(t)

	created, err := uc.CreateLookup(model.LookupTable{
		Name:    "diagnosis-type",
		KeyType: model.FieldTypeString,
		Entries: []model.LookupEntry{{Key: "A"}, {Key: "B"}},
	})
	if err != nil {
		t.Fatalf("CreateLookup: %v", err)
	}
	id := created.Lookups[len(created.Lookups)-1].ID

	snap, err := uc.UpdateLookup(id, model.LookupTable{
		Name:        "diagnosis-type",
		Description: "interleaves with severity: odds here, evens there",
		CustomOrder: true,
		Entries: []model.LookupEntry{
			{Key: "A", Order: 1},
			{Key: "B", Order: 5},
		},
	})
	if err != nil {
		t.Fatalf("UpdateLookup: %v", err)
	}

	var tbl model.LookupTable
	for _, l := range snap.Lookups {
		if l.ID == id {
			tbl = l
		}
	}
	if tbl.Entries[0].Order != 1 || tbl.Entries[1].Order != 5 {
		t.Fatalf("custom orders were overwritten: %d,%d",
			tbl.Entries[0].Order, tbl.Entries[1].Order)
	}
	if tbl.Description == "" {
		t.Fatal("description was not persisted")
	}
}
```

`internal/application/admin_test.go` already builds an `AdminUseCase` against a `mockSink`. Reuse that file's existing helper — if it is not named `newAdminHarness`, use the real name. Do not write a second harness.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/application/ -run 'TestCreateLookup_InfersOrder|TestUpdateLookup_CustomOrder' -v`
Expected: FAIL — `Order`, `Description`, and `CustomOrder` are not fields of the model types.

- [ ] **Step 3: Add the model fields**

In `internal/domain/model/lookup.go`, replace the `LookupEntry` declaration:

```go
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
```

Add three fields to `LookupTable`, keeping the existing ones:

```go
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
```

- [ ] **Step 4: Add order inference to the admin use case**

In `internal/application/admin_lookup.go`, add:

```go
// normalizeOrder stamps list position onto each entry unless the table authors
// its own numbers. The result is always persisted, so no store has to infer it.
func normalizeOrder(t *model.LookupTable) {
	if t.CustomOrder {
		return
	}
	for i := range t.Entries {
		t.Entries[i].Order = i
	}
}
```

In `CreateLookup`, immediately before `snap.Lookups = append(snap.Lookups, table)`:

```go
	normalizeOrder(&table)
```

In `UpdateLookup`, replace the two lines that assign `Name` and `Entries` with:

```go
	// Preserve immutable id and keyType.
	snap.Lookups[idx].Name = updated.Name
	snap.Lookups[idx].Description = updated.Description
	snap.Lookups[idx].EmitOrder = updated.EmitOrder
	snap.Lookups[idx].CustomOrder = updated.CustomOrder
	snap.Lookups[idx].Entries = entries
	normalizeOrder(&snap.Lookups[idx])
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/application/ -v`
Expected: PASS, including the pre-existing admin tests.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/model/lookup.go internal/application/admin_lookup.go internal/application/admin_lookup_order_test.go
git commit -m "feat: persist lookup entry order, add table description and order flags"
```

---

### Task 2: Output schema model

**Files:**
- Create: `internal/domain/model/output.go`
- Modify: `internal/domain/model/operators.go`
- Modify: `internal/domain/model/segment.go`
- Modify: `internal/domain/model/rule.go`
- Modify: `internal/domain/model/status.go`
- Test: `internal/domain/model/output_test.go` (create)

**Interfaces:**
- Consumes: nothing.
- Produces: `model.EvalMode` with constants `EvalLiteral`, `EvalTemplate`, `EvalExpression`; `model.OutputField{Type, Eval, Lookup, Required}` with method `EvalMode() EvalMode`; `model.OutputSchema map[string]OutputField`; `model.FieldTypeObject`; `Segment.OutputSchema`, `Segment.Outputs`, `Rule.Outputs`, `Failure.Outputs`. Tasks 3, 4, 5 and 6 all build on these names.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/model/output_test.go`:

```go
package model

import "testing"

// An object field exists for expression-mode outputs that return a map. It must
// never be usable in a condition, so no operator may accept it.
func TestFieldTypeObject_NotUsableInConditions(t *testing.T) {
	for op := range OperatorTypes {
		if OperatorSupportsType(op, FieldTypeObject) {
			t.Fatalf("operator %q must not accept the object type", op)
		}
	}
}

func TestOutputField_EvalDefaultsToLiteral(t *testing.T) {
	f := OutputField{Type: FieldTypeString}
	if f.EvalMode() != EvalLiteral {
		t.Fatalf("expected literal by default, got %q", f.EvalMode())
	}
	f.Eval = EvalExpression
	if f.EvalMode() != EvalExpression {
		t.Fatalf("expected expression, got %q", f.EvalMode())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/model/ -run 'TestFieldTypeObject|TestOutputField' -v`
Expected: FAIL — `FieldTypeObject`, `OutputField`, `EvalLiteral` undefined.

- [ ] **Step 3: Add the object field type**

In `internal/domain/model/operators.go`, add to the `FieldType` const block:

```go
	// FieldTypeObject is for expression-mode output fields that return a map.
	// It is deliberately absent from OperatorTypes so it can never appear in a
	// condition.
	FieldTypeObject FieldType = "object"
```

Do **not** add an entry for it to `OperatorTypes`.

- [ ] **Step 4: Create the output model**

Create `internal/domain/model/output.go`:

```go
package model

// EvalMode says how an output field's authored value becomes a value.
//
//	literal     the value is a constant
//	template    the value is text with ${ ... } tokens, rendered to a string
//	expression  the value is one whole expr-lang expression, keeping its type
type EvalMode string

const (
	EvalLiteral    EvalMode = "literal"
	EvalTemplate   EvalMode = "template"
	EvalExpression EvalMode = "expression"
)

// OutputField declares one field a segment emits with each reported item.
//
// Lookup names a table whose keys are the field's permitted values. A
// lookup-bound field is emitted as {key, value} — plus order when the table sets
// EmitOrder — so a consumer can sort and display without reading the table.
//
// Required is the caller's contract, and it is checked at two different points.
// At snapshot load it is an error for a required field to be unauthored, so an
// author cannot silently drop a field a consumer depends on (Task 5). At
// evaluation it is a warning for a required field to be absent from what was
// actually emitted, because config validity cannot guarantee runtime presence —
// an expression can fail, and an override emits no outputs at all (Task 6). It
// deliberately mirrors SchemaField.Required in declaration, but note the runtime
// halves are the only halves that behave alike: an input field's required-ness
// cannot be checked at load, because config does not know the caller's context.
type OutputField struct {
	Type     FieldType `json:"type"`
	Eval     EvalMode  `json:"eval,omitempty"`
	Lookup   string    `json:"lookup,omitempty"`
	Required bool      `json:"required,omitempty"`
}

// EvalMode returns the declared mode, defaulting to literal.
func (f OutputField) EvalMode() EvalMode {
	if f.Eval == "" {
		return EvalLiteral
	}
	return f.Eval
}

// OutputSchema maps output field names to their declarations.
type OutputSchema map[string]OutputField
```

- [ ] **Step 5: Add the carrier fields**

In `internal/domain/model/segment.go`, add to `Segment`:

```go
	// OutputSchema declares the fields this segment emits with each reported
	// item. Outputs holds the values for fields that do not vary per item; a
	// rule's own Outputs take precedence.
	OutputSchema OutputSchema      `json:"outputSchema,omitempty"`
	Outputs      map[string]string `json:"outputs,omitempty"`
```

In `internal/domain/model/rule.go`, add to `Rule`:

```go
	// Outputs are this item's authored values for the segment's output schema,
	// keyed by field name. Only a top-level rule reports, so only a top-level
	// rule's Outputs are read.
	Outputs map[string]string `json:"outputs,omitempty"`
```

In `internal/domain/model/status.go`, add to `Failure`:

```go
	Outputs map[string]interface{} `json:"outputs,omitempty"`
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/domain/model/ -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/model/
git commit -m "feat: add output schema model and object field type"
```

---

### Task 3: Emit outputs from checklist items

**Files:**
- Create: `internal/domain/strategy/output.go`
- Modify: `internal/domain/strategy/rule.go` (functions `appendFailure`, `collectViolations`)
- Modify: `internal/application/dto.go` (`FailureDTO`)
- Modify: `internal/application/evaluate.go` (the `FailureDTO` construction)
- Test: `internal/domain/strategy/output_test.go` (create)

**Interfaces:**
- Consumes: everything Task 2 produced, plus `LookupEntry.Order` and `LookupTable.EmitOrder` from Task 1. Also the existing unexported `renderTemplate(tmpl string, env map[string]interface{}) (string, []tokenErr)` and `compileFormula(source string) (runFn, error)` in this package, and `RenderError{Language, Token, Err}`.
- Produces: `evaluateOutputs(seg *model.Segment, itemOutputs map[string]string, ctx *EvalContext) (map[string]interface{}, []RenderError)` and `enrichLookupValue(tableID string, key interface{}, ctx *EvalContext) interface{}`. Task 4 calls `evaluateOutputs`.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/strategy/output_test.go`:

```go
package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func outputSeg() *model.Segment {
	return &model.Segment{
		ID:       "employee-account",
		Strategy: model.StrategyChecklist,
		OutputSchema: model.OutputSchema{
			"category":      {Type: model.FieldTypeString},
			"diagnosisType": {Type: model.FieldTypeString, Lookup: "diagnosis-type"},
			"description":   {Type: model.FieldTypeString, Eval: model.EvalTemplate},
			"signals":       {Type: model.FieldTypeObject, Eval: model.EvalExpression},
		},
		Outputs: map[string]string{"category": "EmployeeAccountStatus"},
		Rules: []model.Rule{{
			RuleName:     "timesheetHoursAbnormallyLow",
			ErrorMessage: "Hours look low.",
			Outputs: map[string]string{
				"diagnosisType": "TimesheetHoursAbnormallyLow",
				"description":   "${ totalHours } hours over ${ daysElapsed } days",
				"signals":       "{ TotalHours: totalHours, DaysElapsed: daysElapsed }",
			},
			Condition: &model.Condition{
				Field:    "totalHours",
				Operator: model.OpLt,
				Value:    2,
			},
		}},
	}
}

func outputCtx() *EvalContext {
	return &EvalContext{
		Context: map[string]interface{}{"totalHours": 1.5, "daysElapsed": 3},
		Lookups: map[string]model.LookupTable{
			"diagnosis-type": {
				ID:        "diagnosis-type",
				KeyType:   model.FieldTypeString,
				EmitOrder: true,
				Entries: []model.LookupEntry{
					{Key: "TimesheetHoursAbnormallyLow", Value: "Hours abnormally low", Order: 30},
				},
			},
		},
		CollectFailures: true,
	}
}

func TestChecklist_FailureCarriesOutputs(t *testing.T) {
	var s ChecklistStrategy
	res, ok := s.Evaluate(outputSeg(), outputCtx())
	if !ok || len(res.Failures) != 1 {
		t.Fatalf("expected one failure, got ok=%v n=%d", ok, len(res.Failures))
	}
	out := res.Failures[0].Outputs

	// literal from segment level, because the item does not set it
	if out["category"] != "EmployeeAccountStatus" {
		t.Errorf("category = %v", out["category"])
	}

	// template renders to a string
	if out["description"] != "1.5 hours over 3 days" {
		t.Errorf("description = %v", out["description"])
	}

	// lookup-bound field is enriched with value and order
	lk, isMap := out["diagnosisType"].(map[string]interface{})
	if !isMap {
		t.Fatalf("diagnosisType should be a map, got %T", out["diagnosisType"])
	}
	if lk["key"] != "TimesheetHoursAbnormallyLow" || lk["value"] != "Hours abnormally low" || lk["order"] != 30 {
		t.Errorf("lookup enrichment = %v", lk)
	}

	// expression keeps its type
	sig, isMap := out["signals"].(map[string]interface{})
	if !isMap {
		t.Fatalf("signals should be a map, got %T", out["signals"])
	}
	if sig["TotalHours"] != 1.5 {
		t.Errorf("signals = %v", sig)
	}
}

// A broken expression drops its own field and records an error. The finding
// still reports — evidence failing must not take the diagnosis with it.
func TestChecklist_BadOutputExpressionDegrades(t *testing.T) {
	seg := outputSeg()
	seg.Rules[0].Outputs["signals"] = "totalHours +"

	var s ChecklistStrategy
	res, _ := s.Evaluate(seg, outputCtx())
	if len(res.Failures) != 1 {
		t.Fatalf("expected the failure to survive, got %d", len(res.Failures))
	}
	if _, present := res.Failures[0].Outputs["signals"]; present {
		t.Error("broken expression should not emit a value")
	}
	if len(res.RenderErrors) == 0 {
		t.Error("expected a render error to be recorded")
	}
	if res.Failures[0].Outputs["category"] != "EmployeeAccountStatus" {
		t.Error("other fields should still be emitted")
	}
}

// An unknown key passes through bare. Lookup membership is documented, not
// enforced at evaluation time.
func TestEnrichLookupValue_UnknownKeyPassesThrough(t *testing.T) {
	got := enrichLookupValue("diagnosis-type", "NotInTable", outputCtx())
	if got != "NotInTable" {
		t.Fatalf("expected the bare key, got %v", got)
	}
}
```

If `model.Condition`'s field names differ from `Field`/`Operator`/`Value`, read the file that declares it and use the real ones. Do not change what the tests assert.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/strategy/ -run 'TestChecklist_FailureCarriesOutputs|TestChecklist_BadOutputExpression|TestEnrichLookupValue' -v`
Expected: FAIL — `enrichLookupValue` undefined, and `Failures[0].Outputs` is nil.

- [ ] **Step 3: Write the output evaluator**

Create `internal/domain/strategy/output.go`:

```go
package strategy

import (
	"fmt"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// evaluateOutputs resolves a segment's declared output fields for one reported
// item. A value authored on the item wins; otherwise the segment-level value
// stands in, which is how fields that do not vary per item are declared once.
//
// A field whose value fails to render or evaluate is omitted and an error is
// recorded — the item still reports, because evidence failing must not take the
// finding with it.
func evaluateOutputs(seg *model.Segment, itemOutputs map[string]string, ctx *EvalContext) (map[string]interface{}, []RenderError) {
	if len(seg.OutputSchema) == 0 {
		return nil, nil
	}

	out := make(map[string]interface{}, len(seg.OutputSchema))
	var errs []RenderError

	for name, decl := range seg.OutputSchema {
		raw, ok := itemOutputs[name]
		if !ok || raw == "" {
			raw, ok = seg.Outputs[name]
		}
		if !ok || raw == "" {
			continue
		}

		var value interface{}
		switch decl.EvalMode() {
		case model.EvalExpression:
			fn, err := compileFormula(raw)
			if err != nil {
				errs = append(errs, RenderError{Token: raw, Err: err.Error()})
				continue
			}
			v, err := fn(ctx.Context)
			if err != nil {
				errs = append(errs, RenderError{Token: raw, Err: err.Error()})
				continue
			}
			value = v
		case model.EvalTemplate:
			// renderTemplate degrades per token: it writes the literal ${...}
			// back and reports the error. That is right for a human-readable
			// message but wrong for a structured field, so a template output is
			// all-or-nothing like an expression — any failed token omits the
			// whole field. Emitting a half-rendered string would also blind
			// Task 6's required-field warning, which tests presence: the field
			// would be there, carrying template syntax, and nothing would say so.
			rendered, bad := renderTemplate(raw, ctx.Context)
			if len(bad) > 0 {
				for _, te := range bad {
					errs = append(errs, RenderError{
						Language: ctx.DefaultLanguage,
						Token:    te.token,
						Err:      te.err,
					})
				}
				continue
			}
			value = rendered
		default:
			value = raw
		}

		if decl.Lookup != "" {
			value = enrichLookupValue(decl.Lookup, value, ctx)
		}
		out[name] = value
	}

	if len(out) == 0 {
		return nil, errs
	}
	return out, errs
}

// enrichLookupValue turns an authored key into the {key, value} shape a consumer
// displays with, adding order when the table emits it. An unknown key — or an
// unknown table — passes through unchanged: membership is the author's
// invariant, recorded in the table description, not enforced here.
func enrichLookupValue(tableID string, key interface{}, ctx *EvalContext) interface{} {
	table, ok := ctx.Lookups[tableID]
	if !ok {
		return key
	}
	for _, e := range table.Entries {
		if fmt.Sprint(e.Key) != fmt.Sprint(key) {
			continue
		}
		enriched := map[string]interface{}{"key": e.Key, "value": e.Value}
		if table.EmitOrder {
			enriched["order"] = e.Order
		}
		return enriched
	}
	return key
}
```

- [ ] **Step 4: Attach outputs to each failure**

In `internal/domain/strategy/rule.go`, change `appendFailure` to take the segment:

```go
func appendFailure(res *Result, seg *model.Segment, r *model.Rule, ctx *EvalContext) {
	f := model.Failure{Rule: r.RuleName}
```

Immediately before its closing `res.Failures = append(res.Failures, f)`:

```go
	outputs, outErrs := evaluateOutputs(seg, r.Outputs, ctx)
	f.Outputs = outputs
	res.RenderErrors = append(res.RenderErrors, outErrs...)
```

Update its one caller in `collectViolations`:

```go
			appendFailure(&res, seg, r, ctx)
```

- [ ] **Step 5: Run strategy tests to verify they pass**

Run: `go test ./internal/domain/strategy/ -v`
Expected: PASS, including the pre-existing checklist tests.

- [ ] **Step 6: Surface outputs on the DTO**

In `internal/application/dto.go`, add to `FailureDTO`:

```go
	Outputs  map[string]interface{} `json:"outputs,omitempty"`
```

In `internal/application/evaluate.go`, the `FailureDTO` construction becomes:

```go
		for _, f := range lr.Failures {
			dto.Failures = append(dto.Failures, FailureDTO{
				Rule:     f.Rule,
				Message:  f.Message,
				Messages: f.Messages,
				Outputs:  f.Outputs,
			})
		}
```

- [ ] **Step 7: Run the full suite and build**

Run: `go build ./... && go test ./...`
Expected: build clean, all packages PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/domain/strategy/output.go internal/domain/strategy/output_test.go internal/domain/strategy/rule.go internal/application/dto.go internal/application/evaluate.go
git commit -m "feat: emit declared outputs with each checklist item"
```

---

### Task 4: Emit outputs from a rule-strategy winner

**Files:**
- Modify: `internal/domain/strategy/strategy.go` (`Result`)
- Modify: `internal/domain/strategy/rule.go` (the first-match and default branches of `Evaluate`)
- Modify: `internal/domain/engine/evaluator.go` (assignment construction)
- Modify: `internal/application/dto.go` (`LayerResultDTO`)
- Modify: `internal/application/evaluate.go` (assignment mapping)
- Test: `internal/domain/strategy/output_rule_test.go` (create)

**Interfaces:**
- Consumes: `evaluateOutputs` (Task 3).
- Produces: `Result.Outputs map[string]interface{}`, and `LayerResultDTO.Outputs map[string]interface{}`.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/strategy/output_rule_test.go`:

```go
package strategy

import (
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

// Exclusive findings are a rule segment: first match wins and emits its outputs.
func TestRule_WinnerCarriesOutputs(t *testing.T) {
	seg := &model.Segment{
		ID:       "balance-availability",
		Strategy: model.StrategyRule,
		OutputSchema: model.OutputSchema{
			"severity": {Type: model.FieldTypeString},
			"title":    {Type: model.FieldTypeString, Eval: model.EvalTemplate},
		},
		Outputs: map[string]string{"severity": "Warning"},
		Rules: []model.Rule{
			{
				RuleName: "noDaysWorkedYet",
				Outputs:  map[string]string{"title": "No days worked in ${ cycleName }"},
				Condition: &model.Condition{
					Field: "calcMode", Operator: model.OpEq, Value: "SalaryNonClocking",
				},
			},
			{
				RuleName: "noPayDataAvailable",
				Outputs:  map[string]string{"title": "No pay data"},
				Condition: &model.Condition{
					Field: "calcMode", Operator: model.OpEq, Value: "NetPayCalculator",
				},
			},
		},
	}
	ctx := &EvalContext{Context: map[string]interface{}{
		"calcMode":  "SalaryNonClocking",
		"cycleName": "March",
	}}

	var s RuleStrategy
	res, ok := s.Evaluate(seg, ctx)
	if !ok {
		t.Fatal("expected the segment to resolve")
	}
	if res.Outputs["severity"] != "Warning" {
		t.Errorf("severity = %v", res.Outputs["severity"])
	}
	if res.Outputs["title"] != "No days worked in March" {
		t.Errorf("title = %v", res.Outputs["title"])
	}
}

// Falling through to the default emits the segment-level values only.
func TestRule_DefaultCarriesSegmentOutputs(t *testing.T) {
	seg := &model.Segment{
		ID:           "balance-availability",
		Strategy:     model.StrategyRule,
		OutputSchema: model.OutputSchema{"severity": {Type: model.FieldTypeString}},
		Outputs:      map[string]string{"severity": "Info"},
		Default:      "none",
		Rules: []model.Rule{{
			RuleName:  "never",
			Condition: &model.Condition{Field: "calcMode", Operator: model.OpEq, Value: "nope"},
		}},
	}
	ctx := &EvalContext{Context: map[string]interface{}{"calcMode": "other"}}

	var s RuleStrategy
	res, ok := s.Evaluate(seg, ctx)
	if !ok || res.Segment != "none" {
		t.Fatalf("expected the default, got ok=%v segment=%q", ok, res.Segment)
	}
	if res.Outputs["severity"] != "Info" {
		t.Errorf("severity = %v", res.Outputs["severity"])
	}
}
```

If the rule-strategy constant is not `model.StrategyRule`, use the real name from the file that also declares `model.StrategyChecklist`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/strategy/ -run TestRule_WinnerCarriesOutputs -v`
Expected: FAIL — `res.Outputs` undefined on `Result`.

- [ ] **Step 3: Add Outputs to Result and populate it**

In `internal/domain/strategy/strategy.go`, add to `Result`:

```go
	// Outputs are the declared output fields resolved for the reported item —
	// the winning rule under first-match, or the segment default.
	Outputs map[string]interface{}
```

In `internal/domain/strategy/rule.go`, in the first-match branch, after `applyMessages(&res, r.Messages, ctx)`:

```go
			outputs, outErrs := evaluateOutputs(seg, r.Outputs, ctx)
			res.Outputs = outputs
			res.RenderErrors = append(res.RenderErrors, outErrs...)
```

and in the default branch, after `applyMessages(&res, seg.DefaultMessages, ctx)`:

```go
		outputs, outErrs := evaluateOutputs(seg, nil, ctx)
		res.Outputs = outputs
		res.RenderErrors = append(res.RenderErrors, outErrs...)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/domain/strategy/ -v`
Expected: PASS.

- [ ] **Step 5: Carry Outputs through the evaluator and DTO**

In `internal/domain/model/assignment.go`, add to `Assignment`:

```go
	// Outputs holds the segment's declared output fields, resolved for the
	// rule/override/default that produced this assignment.
	Outputs map[string]interface{} `json:"outputs,omitempty"`
```

In `internal/domain/engine/evaluator.go` there are two `&model.Assignment{...}` literals. Add `Outputs` to the primary-strategy one (the block that already sets `Computed: res.Computed`):

```go
			lr.Assignment = &model.Assignment{
				Segment:     res.Segment,
				Strategy:    seg.Strategy,
				Reason:      res.Reason,
				Computed:    res.Computed,
				Messages:    res.Messages,
				Outputs:     res.Outputs,
			}
```

Leave the **override** literal (the one with `Strategy: "override"`) alone *for now*. `EvalOverrides` does not yet receive the segment, so it resolves no output schema. **Task 7 changes this** and adds `Outputs` to that literal too — do not anticipate it here, because Task 6's warning needs the un-emitting override path to exist in order to be tested.

In `internal/application/dto.go`, add to `LayerResultDTO`:

```go
	Outputs     map[string]interface{} `json:"outputs,omitempty"`
```

In `internal/application/evaluate.go`, inside the `if a := lr.Assignment; a != nil` block:

```go
			dto.Outputs = a.Outputs
```

- [ ] **Step 6: Run the full suite and build**

Run: `go build ./... && go test ./...`
Expected: build clean, all packages PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/strategy/ internal/domain/engine/evaluator.go internal/application/dto.go internal/application/evaluate.go
git commit -m "feat: emit declared outputs from a rule-strategy winner"
```

---

### Task 5: Validate output declarations

**Files:**
- Modify: `internal/domain/validation/validator.go`
- Test: `internal/domain/validation/output_test.go` (create)

**Interfaces:**
- Consumes: `model.OutputSchema`, `model.OutputField`, `model.FieldTypeObject`, `model.EvalExpression`, `OutputField.EvalMode()` (Task 2); the snapshot-level `lookups` map that `validateLookups` populates in `ValidateSnapshot`.
- Produces: `validateOutputSchema(seg *model.Segment, lookups map[string]model.LookupTable) []string` and its helper `requiredOutputErrors(seg *model.Segment, name string) []string`. Nothing downstream consumes either.

**Scope note:** this validates three things — that a referenced lookup table exists, that the object type is confined to expression mode, and that every `Required` field is authored on some path that can actually supply it. It deliberately does **not** check lookup key membership or order-number uniqueness; those are the author's invariants per the global constraints.

**Why the required check belongs at load, not only in the UI.** `ValidateSnapshot` is the single chokepoint every write passes through — `UpdateLayer`, `UpdateSegment`, `CreateSegment` and `ImportSnapshot` all route through `AdminUseCase.commitSnapshot`, which validates before saving (`admin.go:197-207`), and the file watcher validates before swapping (`watcher.go:73-77`). So "you cannot save the layer with a required field unauthored" needs no new plumbing; it falls out of putting the check here. A hand-edited config file is covered too, and safely: a failed validation logs and returns without swapping, so the last good snapshot keeps serving.

**Consequence to expect.** `ValidateSnapshot` validates the whole snapshot, so an unauthored required field in one segment blocks saves to unrelated segments until it is fixed. Existing validation already behaves this way for a bad formula, but those are rare typos — this check will fire during ordinary authoring, so it will be met far more often. This is the main reason `Required` defaults to `false`: declaring a field is free, and promoting it is the deliberate act.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/validation/output_test.go`:

```go
package validation

import (
	"strings"
	"testing"

	"github.com/segmentation-service/segmentation/internal/domain/model"
)

func snapWithOutputField(f model.OutputField) *model.Snapshot {
	return &model.Snapshot{
		Layers: []model.Layer{{
			Name: "diagnostics",
			Segments: []model.Segment{{
				ID:           "employee",
				Strategy:     model.StrategyChecklist,
				OutputSchema: model.OutputSchema{"field": f},
				InputSchema:  model.InputSchema{"x": {Type: model.FieldTypeString}},
				Rules: []model.Rule{{
					RuleName:  "someCheck",
					Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
				}},
			}},
		}},
	}
}

func TestValidate_OutputLookupMustExist(t *testing.T) {
	err := ValidateSnapshot(snapWithOutputField(model.OutputField{
		Type:   model.FieldTypeString,
		Lookup: "no-such-table",
	}))
	if err == nil || !strings.Contains(err.Error(), "no-such-table") {
		t.Fatalf("expected an unknown-lookup error, got %v", err)
	}
}

func TestValidate_ObjectTypeRequiresExpressionMode(t *testing.T) {
	err := ValidateSnapshot(snapWithOutputField(model.OutputField{
		Type: model.FieldTypeObject,
	}))
	if err == nil || !strings.Contains(err.Error(), "object") {
		t.Fatalf("expected an object-type error, got %v", err)
	}

	if err := ValidateSnapshot(snapWithOutputField(model.OutputField{
		Type: model.FieldTypeObject,
		Eval: model.EvalExpression,
	})); err != nil {
		t.Fatalf("object with expression mode should be valid, got %v", err)
	}
}

func TestValidate_RequiredOutputMustBeAuthored(t *testing.T) {
	required := model.OutputField{Type: model.FieldTypeString, Required: true}

	// The rule in the fixture authors nothing, so a required field is an error.
	err := ValidateSnapshot(snapWithOutputField(required))
	if err == nil || !strings.Contains(err.Error(), "someCheck") {
		t.Fatalf("expected the unauthored rule to be named, got %v", err)
	}

	// A value on the rule satisfies it.
	snap := snapWithOutputField(required)
	snap.Layers[0].Segments[0].Rules[0].Outputs = map[string]string{"field": "x"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("rule-level value should satisfy, got %v", err)
	}

	// So does one segment-level value, for every rule at once.
	snap = snapWithOutputField(required)
	snap.Layers[0].Segments[0].Outputs = map[string]string{"field": "x"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("segment-level value should satisfy, got %v", err)
	}

	// A disabled rule is exempt: a work-in-progress item must not wedge a save.
	snap = snapWithOutputField(required)
	disabled := false
	snap.Layers[0].Segments[0].Rules[0].Enabled = &disabled
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("disabled rule should be exempt, got %v", err)
	}

	// An optional field is never required to be authored.
	if err := ValidateSnapshot(snapWithOutputField(model.OutputField{
		Type: model.FieldTypeString,
	})); err != nil {
		t.Fatalf("optional field should not be gated, got %v", err)
	}
}

func TestValidate_RequiredOutputWithDefaultNeedsSegmentValue(t *testing.T) {
	// The default branch reads no rule values, so a rule-level value cannot
	// cover it — only a segment-level one can.
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeString, Required: true})
	seg := &snap.Layers[0].Segments[0]
	seg.Strategy = model.StrategyRule
	seg.Default = "fallback"
	seg.Rules[0].Outputs = map[string]string{"field": "x"}

	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "default") {
		t.Fatalf("expected a default-path error, got %v", err)
	}

	seg.Outputs = map[string]string{"field": "x"}
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("segment-level value should satisfy the default path, got %v", err)
	}
}

func TestValidate_ChecklistCannotDeclareOverrides(t *testing.T) {
	snap := snapWithOutputField(model.OutputField{Type: model.FieldTypeString})
	seg := &snap.Layers[0].Segments[0] // the fixture's strategy is checklist
	seg.Overrides = []model.Rule{{
		RuleName:  "vipBypass",
		Condition: &model.Condition{Field: "x", Operator: model.OpIsNull},
	}}

	err := ValidateSnapshot(snap)
	if err == nil || !strings.Contains(err.Error(), "override") {
		t.Fatalf("expected an override-on-checklist error, got %v", err)
	}

	// The same overrides on a rule segment are legitimate.
	seg.Strategy = model.StrategyRule
	if err := ValidateSnapshot(snap); err != nil {
		t.Fatalf("overrides on a rule segment should be valid, got %v", err)
	}
}
```

If `ValidateSnapshot` returns `[]string` rather than `error`, join the slice in the assertions. Do not change what is asserted. `Rule.Enabled` is `*bool` (nil means enabled, `rule.go:16-31`), which is why the disabled case takes an addressable `false`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/validation/ -run 'TestValidate_Output|TestValidate_ObjectType|TestValidate_Required|TestValidate_Checklist' -v`
Expected: FAIL — no output validation exists and overrides are unchecked, so every snapshot validates cleanly.

- [ ] **Step 3: Add the validation**

In `internal/domain/validation/validator.go`, add:

```go
// validateOutputSchema checks a referenced lookup table exists, the object type
// is confined to expression mode, and every Required field is actually authored.
// Lookup membership and order uniqueness remain the author's invariants and are
// deliberately not checked.
func validateOutputSchema(seg *model.Segment, lookups map[string]model.LookupTable) []string {
	var errs []string
	for name, f := range seg.OutputSchema {
		if f.Lookup != "" {
			if _, ok := lookups[f.Lookup]; !ok {
				errs = append(errs, fmt.Sprintf(
					"segment %q output %q: lookup %q does not exist",
					seg.ID, name, f.Lookup))
			}
		}
		if f.Type == model.FieldTypeObject && f.EvalMode() != model.EvalExpression {
			errs = append(errs, fmt.Sprintf(
				"segment %q output %q: the object type requires eval \"expression\"",
				seg.ID, name))
		}
		if f.Required {
			errs = append(errs, requiredOutputErrors(seg, name)...)
		}
	}
	return errs
}

// requiredOutputErrors reports every authoring path that would leave a required
// output field unset.
//
// A segment-level value covers every path at once, which is the intended way to
// satisfy a field that does not vary per item. Failing that, each enabled
// top-level rule must set it — disabled rules are exempt so a work-in-progress
// item cannot wedge an unrelated save. A declared Default has no rule to read
// from at all (the default branch calls evaluateOutputs with nil item values),
// so only a segment-level value can satisfy it.
func requiredOutputErrors(seg *model.Segment, name string) []string {
	if _, ok := seg.Outputs[name]; ok {
		return nil
	}

	var errs []string
	// Only the rule strategy reads Segment.Default (strategy/rule.go, default
	// branch). A checklist delegates to RuleStrategy but returns from
	// collectViolations before that branch, and static uses Static.Default
	// instead — so a stray Default on any other strategy is inert, and gating
	// on it would reject config that evaluates perfectly well.
	if seg.Strategy == model.StrategyRule && seg.Default != "" {
		errs = append(errs, fmt.Sprintf(
			"segment %q output %q: required, and a default is declared, so it must be set in "+
				"the segment's outputs — the default path reads no rule values",
			seg.ID, name))
	}
	for i := range seg.Rules {
		r := &seg.Rules[i]
		if !r.IsEnabled() {
			continue
		}
		if _, ok := r.Outputs[name]; !ok {
			errs = append(errs, fmt.Sprintf(
				"segment %q rule %q: required output %q has no value (set it on the rule, "+
					"or once in the segment's outputs)",
				seg.ID, r.RuleName, name))
		}
	}
	return errs
}
```

Only top-level rules are examined. Inner And/Or branches never report, so they carry no output values — the same reason they get no output editor.

Call it from the per-segment loop in `ValidateSnapshot`, alongside the existing rule-tree validation, passing the same `lookups` map:

```go
		errs = append(errs, validateOutputSchema(&seg, lookups)...)
```

Then, in the same loop beside the existing unknown-strategy check, reject overrides on a checklist:

```go
			// A checklist has no "nothing matched" outcome, so an override has
			// nothing to override. Worse, EvalOverrides resolves a segment value
			// and the evaluator reports StatusResolved — both outside the
			// checklist vocabulary, so a consumer switching on status meets a
			// case it was told could not happen. The segment editor already tells
			// authors a checklist has no overrides and renders no editor for them
			// (SegmentEditor.tsx); this makes the engine agree, closing the raw
			// JSON, admin API and hand-edited config paths that bypass the UI.
			if seg.Strategy == model.StrategyChecklist && len(seg.Overrides) > 0 {
				errs = append(errs, fmt.Sprintf(
					"segment %q: a checklist cannot declare overrides — an override resolves a "+
						"segment value and reports %q, which is not part of the checklist "+
						"vocabulary (%q, %q, %q)",
					seg.ID, model.StatusResolved,
					model.StatusSatisfied, model.StatusViolated, model.StatusUnevaluable))
			}
```

This is not an output-schema rule, which is why it sits in the loop rather than in `validateOutputSchema` — it is a pre-existing invariant that the UI documents and enforces but the engine never did. No shipped config is affected: the only override in `config/segments.json` is on a `percentage` segment.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/domain/validation/ -v`
Expected: PASS, including the pre-existing validator tests.

- [ ] **Step 5: Check whether expression syntax can also be validated**

Existing formula validation calls `expr.Compile(def.Formula)` with no options, while runtime compilation uses `mathOptions`. Before adding a syntax check for expression-mode outputs, confirm bare `Compile` does not reject valid input. Add a temporary test to `output_test.go`:

```go
func TestExprCompile_AcceptsRegisteredMathFunctions(t *testing.T) {
	if _, err := expr.Compile("pow(2, 3)"); err != nil {
		t.Fatalf("bare Compile rejects a registered math function: %v", err)
	}
}
```

Run: `go test ./internal/domain/validation/ -run TestExprCompile -v`

- If it **PASSES**: add an expression syntax check to `validateOutputSchema` for fields whose `EvalMode()` is `EvalExpression`, mirroring the existing formula check, and keep this test.
- If it **FAILS**: delete the test and add no syntax check — a validator that rejects valid expressions is worse than none. Record the finding under *Also unresolved* in `docs/plans/2026-09-02-output-schema-todo.md`, noting that the existing formula validation has the same defect.

- [ ] **Step 6: Run the full suite and build**

Run: `go build ./... && go test ./...`
Expected: build clean, all packages PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/validation/
git commit -m "feat: validate output field lookup references and object type usage"
```

---

### Task 6: Warn when a required output is absent at evaluation

**Files:**
- Modify: `internal/domain/validation/validator.go`
- Modify: `internal/domain/engine/evaluator.go` (function `evaluateLayer`)
- Test: `internal/domain/engine/output_warning_test.go` (create)

**Interfaces:**
- Consumes: `model.OutputField.Required` (Task 2); `Failure.Outputs` (Task 3); `Assignment.Outputs` (Task 4).
- Produces: `validation.CheckRequiredOutputs(seg *model.Segment, a *model.Assignment, failures []model.Failure) []model.Warning`. Nothing downstream consumes it beyond the warnings already flowing to `EvaluateResponse.Warnings`.

**Why this exists even though Task 5 gates the config.** Task 5 guarantees an author did not *forget* a value. It cannot guarantee the caller *receives* one, and there are three ways a required field is still absent at runtime:

| Path | Cause | Fixed elsewhere? |
| --- | --- | --- |
| expression or template fails | deliberate degradation — record the error, drop the field, keep the item | **no** — structural, and intended |
| an override wins | `EvalOverrides` receives no segment, so no schema is resolved (Task 4 Step 5 leaves that branch alone) | **yes, by Task 7** — build this task against the un-emitting path, then narrow its test |
| a `rule` segment falls to `Default` | the default branch passes nil item values | **yes, by Task 5** — requires a segment-level value |
| overrides on a `checklist` | the whole combination is incoherent | **yes, by Task 5** — rejected at load |

Only the first survives all of it, which is the point: expression failure is deliberate degradation, so a required field can always go missing at runtime and the caller has to be told. Build this task before Task 7, because the override path is the easiest way to exercise a wholly absent output record.

This is also what makes `OutputField.Required` and `SchemaField.Required` genuinely alike: both warn at evaluation and let it continue. `Required` on an output additionally errors at load, which has no input equivalent because config cannot know the caller's context.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/engine/output_warning_test.go`. Cover three cases: a required field absent from a reported finding warns; a required field present does not; and an override winning warns with a message naming the override, since that path emits no outputs at all. Assert on `EvalResult.Warnings` — `evaluateLayer` appends into `lr.Warnings`, which `Evaluate` hoists into `result.Warnings`.

Model the fixtures on the existing engine tests. For the override case, give the segment an `Overrides` entry that matches and a required output field, then assert a warning mentioning the field name.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/engine/ -run TestRequiredOutput -v`
Expected: FAIL — no such check exists, so no warnings are produced.

- [ ] **Step 3: Add the check**

In `internal/domain/validation/validator.go`, beside `CheckRequiredFields`:

```go
// CheckRequiredOutputs returns warnings for required output fields absent from
// what a segment actually emitted.
//
// Config validation already rejects a required field no authoring path supplies,
// so reaching here means something ran and the value still did not arrive: an
// expression failed and the field was dropped, or an override resolved the
// segment and overrides compute no outputs. Neither is recoverable at load, so
// the caller is told and decides.
func CheckRequiredOutputs(seg *model.Segment, a *model.Assignment, failures []model.Failure) []model.Warning {
	var required []string
	for name, f := range seg.OutputSchema {
		if f.Required {
			required = append(required, name)
		}
	}
	if len(required) == 0 {
		return nil
	}
	sort.Strings(required) // stable output; map iteration is not ordered

	var warnings []model.Warning
	missing := func(field, detail string) {
		warnings = append(warnings, model.Warning{
			Segment: seg.ID,
			Field:   field,
			Message: "required output field absent from emitted record: " + detail,
		})
	}

	// Branch on the strategy, NOT on len(failures). A checklist attaches a
	// record to each Failure and never to the Assignment, so zero findings
	// means nothing was reported — not that a record came back short. Testing
	// len(failures) instead falls through to the Assignment path, where
	// a.Outputs is structurally always empty for a checklist, and every clean
	// pass then warns. ChecklistStrategy always succeeds, so lr.Assignment is
	// never nil here and the a == nil guard below cannot catch it.
	//
	// Each finding is checked separately, because one item's expression can
	// fail while its siblings resolve.
	if seg.Strategy == model.StrategyChecklist {
		for _, f := range failures {
			for _, name := range required {
				if _, ok := f.Outputs[name]; !ok {
					missing(name, fmt.Sprintf("finding %q did not emit it", f.Rule))
				}
			}
		}
		return warnings
	}

	if a == nil {
		return nil // nothing reported, so no record is missing anything
	}
	for _, name := range required {
		if _, ok := a.Outputs[name]; !ok {
			detail := "the segment did not emit it"
			if a.Strategy == "override" {
				detail = "an override resolved this segment, and overrides compute no outputs"
			}
			missing(name, detail)
		}
	}
	return warnings
}
```

Add `"sort"` to the imports if absent.

In `internal/domain/engine/evaluator.go`, call it at the two exits where something was emitted, immediately beside the existing `renderWarnings` calls. In the override branch:

```go
				lr.Warnings = append(lr.Warnings, renderWarnings(seg.ID, res.RenderErrors)...)
				lr.Warnings = append(lr.Warnings, validation.CheckRequiredOutputs(seg, lr.Assignment, nil)...)
				return lr
```

and in the primary-strategy branch:

```go
			lr.Warnings = append(lr.Warnings, renderWarnings(seg.ID, res.RenderErrors)...)
			lr.Warnings = append(lr.Warnings, validation.CheckRequiredOutputs(seg, lr.Assignment, lr.Failures)...)
			return lr
```

Leave the final `return lr` (the fallthrough where no segment produced a result) alone: nothing was reported, so no record is missing a field. This matches `CheckRequiredFields`, which runs only for segments that pass their `when` dispatch.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/domain/engine/ -v`
Expected: PASS, including the pre-existing evaluator tests.

- [ ] **Step 5: Run the full suite and build**

Run: `go build ./... && go test ./...`
Expected: build clean, all packages PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/validation/validator.go internal/domain/engine/
git commit -m "feat: warn when a required output field is absent at evaluation"
```

---

### Task 7: Resolve outputs on the override path

**Files:**
- Modify: `internal/domain/strategy/override.go`
- Modify: `internal/domain/strategy/override_test.go` (three call sites, signature change)
- Modify: `internal/domain/engine/evaluator.go` (function `evaluateLayer`)
- Modify: `internal/domain/validation/validator.go` (function `requiredOutputErrors`)
- Test: `internal/domain/strategy/override_output_test.go` (create)

**Interfaces:**
- Consumes: `evaluateOutputs` (Task 3); `Assignment.Outputs` (Task 4); `OutputField.Required` (Task 2).
- Produces: a changed signature, `EvalOverrides(seg *model.Segment, ctx *EvalContext) (Result, bool)`, matching `Strategy.Evaluate`. Task 6's `CheckRequiredOutputs` needs no change — its override-specific message stays correct for the case where an override authored a value whose expression then failed.

**Why.** After Task 5, only a `rule` segment can declare overrides, and the segment editor does offer an overrides editor there ([`RuleConfig.tsx`](../../ui/src/components/segments/RuleConfig.tsx)). So an author can create an override on a rule segment carrying a required output field and receive a record with none of its declared fields. This is the one reachable path left, and it is reachable through the UI rather than only through raw JSON.

**The two contexts, which must not be conflated.** An override's *condition* matches against raw input only — the override editor is handed `seg.inputSchema`, not the effective schema, and says so: *"Only raw input fields are available."* That is existing, deliberate behaviour and this task must not change it. Its *output values* are new, so they resolve against the computed-enriched context, because a `${derivedValue}` token is otherwise unusable there.

Enriching on this path is safe, and the earlier note claiming otherwise was wrong. An override never runs in collect mode — `ChecklistStrategy` sets `CollectFailures` on a copy of the context *inside* the strategy, after `evaluateLayer` has already checked overrides — and outside collect mode a failed formula leaves its field absent and evaluation proceeds (`rule.go:33-42`). No override can be driven to `unevaluable` by a formula belonging to a rule it pre-empted.

Message rendering keeps its current behaviour and stays on the raw context. Enriching it too would be more consistent, but it would change the rendered output of existing config, which this task is not for. Recorded as a known asymmetry.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/strategy/override_output_test.go`. A `rule` segment with an `OutputSchema`, a `Computed` field, and one override whose `Outputs` include both a literal and a template reading the computed field. Assert the returned `Result.Outputs` carries both, resolved.

Add a second case asserting the override's *condition* still cannot see computed fields — an override whose condition tests a computed field must **not** match. This pins the asymmetry above so a later change cannot erase it silently.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/strategy/ -run TestEvalOverrides_Outputs -v`
Expected: FAIL — `EvalOverrides` takes no segment, so the file does not compile against the current signature.

- [ ] **Step 3: Change the signature and resolve outputs**

In `internal/domain/strategy/override.go`:

```go
// EvalOverrides returns the first enabled override that matches.
//
// Conditions match against raw input only: the override editor offers the
// segment's inputSchema and nothing else, so computed fields are deliberately
// out of scope for matching. Output values are different — they may read
// computed fields, so they resolve against the enriched context. Enriching here
// cannot void the segment, because an override never runs in collect mode.
func EvalOverrides(seg *model.Segment, ctx *EvalContext) (Result, bool) {
	for i := range seg.Overrides {
		r := &seg.Overrides[i]
		if !r.IsEnabled() {
			continue
		}
		if !evaluateRule(r, ctx.Context, ctx.Lookups) {
			continue
		}

		event := r.SuccessEvent
		if event == "" {
			event = r.RuleName
		}
		res := Result{Segment: event, Reason: "override:" + r.RuleName}
		applyMessages(&res, r.Messages, ctx)

		if len(seg.OutputSchema) > 0 {
			outCtx := ctx
			if len(seg.Computed) > 0 {
				enriched, _, _ := enrichWithComputed(seg.Computed, ctx.Context)
				derived := *ctx
				derived.Context = enriched
				outCtx = &derived
			}
			outputs, outErrs := evaluateOutputs(seg, r.Outputs, outCtx)
			res.Outputs = outputs
			res.RenderErrors = append(res.RenderErrors, outErrs...)
		}
		return res, true
	}
	return Result{}, false
}
```

The failed-formula return from `enrichWithComputed` is discarded on purpose: outside collect mode an unresolvable field is simply absent, and a required field that goes missing is reported by Task 6's warning rather than by failing here.

Update the three call sites in `override_test.go` from `EvalOverrides(overrides, ctx)` to pass a `&model.Segment{Overrides: overrides}`. Do not change what those tests assert.

- [ ] **Step 4: Carry the outputs through the evaluator**

In `internal/domain/engine/evaluator.go`, change the call and add `Outputs` to the override `Assignment` literal that Task 4 Step 5 deliberately left alone:

```go
		if len(seg.Overrides) > 0 {
			if res, ok := strategy.EvalOverrides(seg, evalCtx); ok {
				lr.Status = model.StatusResolved
				lr.Assignment = &model.Assignment{
					Segment:  res.Segment,
					Strategy: "override",
					Reason:   res.Reason,
					Messages: res.Messages,
					Outputs:  res.Outputs,
				}
```

Leave `Computed` off that literal. The enrichment above is scoped to resolving output values; an override still reports no computed map, because it never ran the strategy that owns them.

- [ ] **Step 5: Extend the load-time gate to override rules**

In `internal/domain/validation/validator.go`, in `requiredOutputErrors`, after the loop over `seg.Rules`, add the same loop over `seg.Overrides`:

```go
	for i := range seg.Overrides {
		r := &seg.Overrides[i]
		if !r.IsEnabled() {
			continue
		}
		if _, ok := r.Outputs[name]; !ok {
			errs = append(errs, fmt.Sprintf(
				"segment %q override %q: required output %q has no value (set it on the "+
					"override, or once in the segment's outputs)",
				seg.ID, r.RuleName, name))
		}
	}
```

An override that fires replaces the strategy result entirely, so it is a reporting path and carries the same obligation. A segment-level value still covers it, since the early return at the top of `requiredOutputErrors` is checked first.

Add a test for this to `internal/domain/validation/output_test.go`, mirroring `TestValidate_RequiredOutputMustBeAuthored` but with the rule replaced by an override on a `rule`-strategy segment.

- [ ] **Step 6: Run the full suite and build**

Run: `go build ./... && go test ./...`
Expected: build clean, all packages PASS. Task 6's override-path test must be revisited — with outputs now resolved, its assertion has to be that an override whose output *expression fails* warns, rather than that an override warns at all.

- [ ] **Step 7: Commit**

```bash
git add internal/domain/strategy/ internal/domain/engine/evaluator.go internal/domain/validation/
git commit -m "feat: resolve declared outputs on the override path"
```

---

### Task 8: Enforce declared output names and types

**Why.** The output schema exists so an implementer knows what to expect, which only works if the declaration is binding. Two ways it currently is not:

- **Names are advisory.** `evaluateOutputs` iterates the *schema*, so an authored key that is not declared is silently dropped. `outputs: {"catgeory": "x"}` against a `category` field vanishes with no diagnostic at load or runtime. This is the same class as the unknown-strategy rejection at `validator.go:31` — config that can never do anything — not the lookup-membership class the no-guarding decision deliberately leaves alone.
- **`Type` is decorative outside `object`.** Literal mode assigns the authored string verbatim and template mode always produces a string, so `{"rank": {"type": "number"}}` with `outputs: {"rank": "3"}` emits the JSON *string* `"3"`. A consumer generating types from `GET /v1/segments` mis-types every non-string field — which undercuts using the config as the catalog.

**The rule.** What is enforceable depends on the eval mode, so state it per mode:

| Mode | Declared type | Enforcement |
| --- | --- | --- |
| `literal` | any scalar | the authored value must parse as that type at load, **and is emitted as that type** |
| `template` | must be `string` | rejected at load otherwise — a template cannot produce anything else |
| `expression` | any, incl. `object` | not statically checkable; the expression's runtime value is trusted |

`array` and `object` are not expressible as a literal or a template, so they require `expression` mode — `object` already did.

**Files:**
- Modify: `internal/domain/validation/validator.go` (`validateOutputSchema`, and the shared authoring-site walk)
- Modify: `internal/domain/strategy/output.go` (`evaluateOutputs`, literal branch)
- Test: `internal/domain/validation/output_test.go`, `internal/domain/strategy/output_test.go`

**Interfaces:**
- Consumes: `model.OutputField{Type, Eval, Lookup, Required}`, `model.FieldType*` constants, the authoring-site helper added when the final-review fixes de-duplicated `requiredOutputErrors` and `validateOutputExpressionSyntax`.
- Produces: no new exported names.

- [ ] **Step 1: Write the failing tests**

Validation: an authored key absent from the schema is rejected, naming the key and the segment; a literal `"3"` declared `number` validates; a literal `"high"` declared `number` is rejected; a non-`string` type in `template` mode is rejected; `array`/`object` outside `expression` mode is rejected; a lookup-bound field whose declared type differs from the table's `KeyType` is rejected, mirroring the existing `validateLookupRef` check for conditions.

Emission: a literal declared `number` emits a JSON number, not a string; likewise `boolean`. Assert on the concrete Go type, not just the rendered text.

- [ ] **Step 2: Confirm they fail**

- [ ] **Step 3: Add the checks and the coercion**

Reuse the authoring-site walk so name checking covers the segment tier, enabled top-level rules and enabled overrides in one place. Coerce with `strconv.ParseFloat` / `ParseBool` in the literal branch of `evaluateOutputs`; a parse failure there degrades like any other failed value — record a `RenderError` naming the field, omit it, keep the result — because config validation should already have caught it and a runtime surprise must not fail the evaluation.

Skip all of this for `static` and `percentage`, consistent with their exemption from `Required`: those strategies emit no record, so nothing they declare is binding.

**One thing this fixes as a side effect.** A lookup-bound field with a numeric `KeyType` currently compares an authored string against numeric entry keys and never matches, so it emits a bare key with no `value` or `order`. Coercing the literal first makes the comparison work.

- [ ] **Step 4: Run the full suite and build**

`go build ./... && go vet ./... && go test ./...`, including `TestShippedConfig_LoadsAndValidates`.

- [ ] **Step 5: Commit**

---

## Out of scope for this plan

- **UI editors.** The output schema editor, per-item value editors, lookup order authoring, the two table flags and the drag-disable behaviour are a separate plan against `ui/`. This plan makes the fields exist and be exercisable through `POST /v1/evaluate` and the admin lookup endpoints.
- **Consumer mapping.** Projecting an emitted record into a consumer's own domain type belongs to that consumer.
- **The layer-level schema question.** Resolved as segment-level, so `Segment.OutputSchema` in Task 2 stands as written. A schema belongs with the values that fill it, and all of those are segment-scoped: `Segment.Outputs` (constants), `Rule.Outputs` (per item), `Segment.Computed` (the scratchpad templates and expressions read). A layer also resolves to exactly one segment (`evaluator.go:203`), so the two placements are isomorphic in the response and only authoring scope distinguishes them. See the todo document for the full argument and for the two plausible-but-false justifications that were rejected.
