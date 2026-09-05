# Layer-Level Schemas Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a layer declare an input schema and an output schema once, so every segment in that layer shares them instead of repeating them.

**Architecture:** Additive and non-breaking. `Layer` gains `inputSchema` and `outputSchema` alongside the ones `Segment` already has. A segment's **effective** schema is its own when it declares one, and the layer's otherwise — inheritance with a per-segment override, so nothing already authored changes meaning. Resolution happens through two small helpers that every reader calls; no reader touches `seg.InputSchema` or `seg.OutputSchema` directly once this lands. Computed fields stay per segment and still merge on top of the resolved input schema, because they are derived by one segment alone.

**Tech Stack:** Go 1.26, `github.com/expr-lang/expr` v1.17.8, standard library testing. React 19 / TypeScript ~5.9 / Vite 7 for the UI half.

## Global Constraints

- **Inheritance, not merge.** A segment's own schema **replaces** the layer's, it does not extend it. A merge would make "what does this segment read" a computation across two levels; a replacement is a lookup. This also keeps the escape hatch below coherent.
- **The nil-schema escape hatch must survive, and it now has two levels.** `ValidateSnapshot` skips rule-field validation entirely when a segment has no input schema and no `computed`. Five shipped segments rely on it. After this change "no input schema" means *neither the segment's nor its layer's*. A segment in a layer that declares a schema is no longer in the escape hatch — that is the intended consequence of inheritance, but it means **adding a layer schema can newly invalidate rules in a segment that declared nothing**. Task 2 must test exactly that.
- **Nothing already authored may change meaning.** The shipped `config/segments.json` declares no layer schemas, so every segment keeps its own and the whole file must validate and evaluate byte-identically. This is the regression bar for every task.
- **Every existing rule applies unchanged to a resolved schema.** Rule-field typing, operator/type compatibility, lookup `keyType` agreement, output name binding, literal type parsing, template-must-be-string, `array`/`object` require expression, the output `Required` load gate and evaluation warning, and all-or-nothing degradation. Resolution happens *before* those checks; none of them change.
- `static` and `percentage` segments remain exempt from output-schema enforcement. An inherited output schema on one is as inert as an inline one is today.
- **No guarding beyond what exists.** Do not add lookup membership, order uniqueness, or contiguity checks.
- **Scope note, recorded so it is not mistaken for an oversight.** Layer-level schemas let a layer's segments share; they do **not** make the caller's required-field set constant across layers. `layers: ["a"]` and `layers: ["a","b"]` still impose different required fields, because each layer carries its own schema. Measured before this plan: the same context passes clean for one layer and warns `field "NetPay" required field missing from context` when a second is added. Making that constant needs a level above the layer and was considered and set aside; see the shared-schemas plan in git history.
- Go: `go build ./...`, `go vet ./...` and `go test ./...` must pass before every commit. Go is on `PATH` (go1.26.5); no `export PATH` needed.
- UI: `npm run build`, `npm run verify:output-schema` and `npm run verify:rules` must pass from `ui/`. **`npm run lint` has a red baseline of exactly 2 pre-existing errors** (`SegmentEditor.tsx` refs-during-render, `StrategyPicker.tsx:9` react-refresh) — leave both alone; gate on the count not rising above 2.
- **There is no JS test runner** and adding one is out of scope. Pure UI logic is verified by a `verify-*.mjs` script using `node:assert`; components by build plus a manual check.
- The admin API replaces whole objects on save. Never construct a layer or segment from scratch — build payloads by spreading, or fields you did not name are destroyed.

---

### Task 1: Model and the resolvers

**Files:**
- Modify: `internal/domain/model/layer.go`
- Create: `internal/domain/model/schema_resolve.go`
- Test: `internal/domain/model/schema_resolve_test.go` (create)

**Interfaces:**
- Consumes: `model.InputSchema`, `model.OutputSchema`, `model.Segment`, `model.Layer` (existing).
- Produces: `Layer.InputSchema InputSchema`, `Layer.OutputSchema OutputSchema`, and two resolvers:
  - `EffectiveInputSchema(layer *Layer, seg *Segment) InputSchema`
  - `EffectiveOutputSchema(layer *Layer, seg *Segment) OutputSchema`

  Every later task calls these. After this lands nothing reads `seg.InputSchema` or `seg.OutputSchema` directly outside the resolvers.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/model/schema_resolve_test.go`:

```go
package model

import "testing"

func TestEffectiveInputSchema(t *testing.T) {
	layerSchema := InputSchema{"company.ein": {Type: FieldTypeString, Required: true}}
	segSchema := InputSchema{"employee.hireDate": {Type: FieldTypeString}}

	// A segment with none of its own inherits the layer's.
	l := &Layer{InputSchema: layerSchema}
	got := EffectiveInputSchema(l, &Segment{})
	if _, has := got["company.ein"]; !has {
		t.Errorf("did not inherit: %v", got)
	}

	// Its own replaces the layer's outright — this is inheritance, not merge.
	got = EffectiveInputSchema(l, &Segment{InputSchema: segSchema})
	if _, has := got["company.ein"]; has {
		t.Errorf("segment schema must replace, not merge: %v", got)
	}
	if _, has := got["employee.hireDate"]; !has {
		t.Errorf("segment schema lost: %v", got)
	}

	// Neither declared stays nil, which is the escape hatch that disables
	// rule-field validation. It must not become an empty non-nil map.
	if got := EffectiveInputSchema(&Layer{}, &Segment{}); got != nil {
		t.Errorf("expected nil for neither declared, got %v", got)
	}

	// A nil layer must not panic — some call sites hold only a segment.
	if got := EffectiveInputSchema(nil, &Segment{InputSchema: segSchema}); len(got) != 1 {
		t.Errorf("nil layer: %v", got)
	}
}

func TestEffectiveOutputSchema(t *testing.T) {
	layerSchema := OutputSchema{"severity": {Type: FieldTypeString, Required: true}}
	segSchema := OutputSchema{"title": {Type: FieldTypeString}}

	l := &Layer{OutputSchema: layerSchema}
	if got := EffectiveOutputSchema(l, &Segment{}); got["severity"].Type != FieldTypeString {
		t.Errorf("did not inherit: %v", got)
	}
	got := EffectiveOutputSchema(l, &Segment{OutputSchema: segSchema})
	if _, has := got["severity"]; has {
		t.Errorf("segment schema must replace, not merge: %v", got)
	}
	if got := EffectiveOutputSchema(&Layer{}, &Segment{}); len(got) != 0 {
		t.Errorf("expected empty for neither declared, got %v", got)
	}
	if got := EffectiveOutputSchema(nil, &Segment{OutputSchema: segSchema}); len(got) != 1 {
		t.Errorf("nil layer: %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/model/ -run TestEffective -v`
Expected: FAIL — the resolvers and the layer fields do not exist.

- [ ] **Step 3: Add the layer fields**

In `internal/domain/model/layer.go`, add to `Layer`, after `Segments`:

```go
	// InputSchema and OutputSchema are declared once for the whole layer, so
	// its segments need not repeat them. A segment that declares its own
	// replaces the layer's outright — see EffectiveInputSchema.
	//
	// Declaring an input schema here takes the layer's segments out of the
	// nil-schema escape hatch: rules that were previously unchecked because
	// their segment declared nothing will start being validated against these
	// fields. That is intended, but it can invalidate config that loaded
	// before, so it is a deliberate act rather than a free addition.
	InputSchema  InputSchema  `json:"inputSchema,omitempty"`
	OutputSchema OutputSchema `json:"outputSchema,omitempty"`
```

- [ ] **Step 4: Add the resolvers**

Create `internal/domain/model/schema_resolve.go`:

```go
package model

// EffectiveInputSchema returns the schema a segment's rules are validated
// against: its own when it declares one, otherwise its layer's.
//
// Replacement rather than merge is deliberate. Merging would make "what does
// this segment read" a computation across two levels, and would make the
// nil-schema escape hatch — where declaring nothing disables rule-field
// validation — impossible to express at the segment level once a layer
// declared anything.
//
// nil is returned when neither declares one, and the nil is load-bearing:
// ValidateSnapshot treats it as "do not validate rule fields at all". An empty
// non-nil map would silently turn that off, so do not substitute one.
func EffectiveInputSchema(layer *Layer, seg *Segment) InputSchema {
	if len(seg.InputSchema) > 0 {
		return seg.InputSchema
	}
	if layer != nil && len(layer.InputSchema) > 0 {
		return layer.InputSchema
	}
	return nil
}

// EffectiveOutputSchema returns the schema a segment emits against, on the same
// terms: the segment's own when declared, otherwise its layer's.
func EffectiveOutputSchema(layer *Layer, seg *Segment) OutputSchema {
	if len(seg.OutputSchema) > 0 {
		return seg.OutputSchema
	}
	if layer != nil && len(layer.OutputSchema) > 0 {
		return layer.OutputSchema
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/domain/model/ -v`
Expected: PASS, including the pre-existing model tests.

- [ ] **Step 6: Build and commit**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/domain/model/
git commit -m "feat: declare input and output schemas on a layer"
```

---

### Task 2: Validation resolves through the layer

**Files:**
- Modify: `internal/domain/validation/validator.go`
- Test: `internal/domain/validation/layer_schema_test.go` (create)

**Interfaces:**
- Consumes: both resolvers and the two `Layer` fields (Task 1).
- Produces: nothing new exported. Existing checks run against resolved schemas.

**What changes and what must not.** `buildEffectiveSchema(seg)` merges `seg.InputSchema` with `seg.Computed`. It must take the *resolved* input schema and merge computed on top, exactly as now — the merge stays per segment because computed fields are per segment. Every output check must likewise take the resolved output schema. **The checks themselves do not change.**

- [ ] **Step 1: Write the failing test**

Create `internal/domain/validation/layer_schema_test.go` covering, in the file's existing style:

1. A segment declaring nothing, in a layer declaring an input schema, has its rules validated against the layer's fields — a rule reading a declared field passes, one reading an undeclared field errors.
2. A segment declaring its own input schema in a layer that also declares one is validated against **its own only** — a rule reading a layer-only field **errors**, proving replacement rather than merge.
3. A segment with an inherited input schema **and** its own `computed` validates a rule reading the computed field, proving the merge survives inheritance.
4. **The escape hatch, both halves.** A segment with no schema in a layer with no schema still skips rule-field validation entirely. And — the case that can break existing config — a segment with no schema in a layer that *does* declare one is now validated, so a rule of its that reads an undeclared field errors where it previously did not.
5. A `required` output field inherited from the layer is enforced on a segment that authors nothing, and the message names the field.
6. An undeclared output key is rejected against an inherited output schema, proving name binding resolves through inheritance.
7. A `static` segment in a layer declaring an output schema is still exempt.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/validation/ -run TestValidate_LayerSchema -v`
Expected: FAIL — layer schemas are ignored, so cases 1, 3, 4b, 5 and 6 behave wrongly.

- [ ] **Step 3: Resolve once per segment**

`ValidateSnapshot`'s per-segment loop currently ranges `for _, seg := range layer.Segments` inside `for _, layer := range snap.Layers`, so the layer is already in scope. Resolve both at the top of the segment body, before any schema-dependent check:

```go
			inSchema := model.EffectiveInputSchema(&layer, &seg)
			outSchema := model.EffectiveOutputSchema(&layer, &seg)
```

Take care with the loop variable: `layer` is a copy per iteration, which is fine to address here, but do not retain the pointer beyond the iteration.

Change `buildEffectiveSchema` to take the resolved schema and the segment's computed fields rather than the whole segment:

```go
// buildEffectiveSchema merges a segment's resolved input schema with its
// expression-defined fields. Computed fields stay per segment even when the
// input schema is inherited, because they are derived by that segment alone.
func buildEffectiveSchema(inSchema model.InputSchema, computed []model.ComputedField) model.InputSchema {
```

Update the escape-hatch guard so "no schema" means neither level:

```go
			if inSchema == nil && len(seg.Computed) == 0 {
				continue
			}
```

Thread `outSchema` into every output check in place of `seg.OutputSchema`, changing each helper's signature to accept the schema rather than re-resolving. One resolution per segment, passed down.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/domain/validation/ -v`
Expected: PASS, including every pre-existing test. A pre-existing failure means the *segment-declared* path changed behaviour — a regression, not an expected change.

- [ ] **Step 5: Confirm the shipped config is untouched**

Run: `go test ./internal/infrastructure/config/ -v`
Expected: PASS. `config/segments.json` declares no layer schemas, so all twelve segment-declared schemas and all five escape-hatch segments must behave exactly as before.

- [ ] **Step 6: Build and commit**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/domain/validation/
git commit -m "feat: validate segments against their layer's schemas"
```

---

### Task 3: Evaluation resolves through the layer

**Files:**
- Modify: `internal/domain/strategy/strategy.go` (`EvalContext`)
- Modify: `internal/domain/strategy/output.go`
- Modify: `internal/domain/strategy/override.go`
- Modify: `internal/domain/engine/evaluator.go`
- Modify: `internal/domain/validation/validator.go` (`CheckRequiredFields`, `CheckRequiredOutputs`)
- Test: `internal/domain/engine/layer_schema_test.go` (create)

**Interfaces:**
- Consumes: both resolvers (Task 1).
- Produces: `EvalContext.OutputSchema model.OutputSchema` — the *already resolved* schema for the segment being evaluated, not a table to look up in — and two changed signatures taking a resolved schema:
  - `CheckRequiredFields(seg *model.Segment, schema model.InputSchema, ctx map[string]interface{}) []model.Warning`
  - `CheckRequiredOutputs(seg *model.Segment, schema model.OutputSchema, a *model.Assignment, failures []model.Failure) []model.Warning`

**Why the resolved schema rather than a lookup table.** `EvalContext.Lookups` carries a *table* because any rule may reference any lookup by id. An output schema is resolved per segment before the strategy runs, and `evaluateLayer` already holds both the layer and the segment — so resolving once there and passing the result is simpler than threading a table and re-resolving inside the strategy.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/engine/layer_schema_test.go` covering:

1. A checklist segment declaring no output schema, in a layer that declares one, emits the inherited fields on its `Failure.Outputs`.
2. A segment declaring its own output schema in such a layer emits **only its own**, proving replacement at evaluation too.
3. A `required` field inherited from the layer, absent because its expression failed, produces the evaluation warning naming the field.
4. Required **input** fields inherited from the layer produce the missing-field warning for a segment that declares nothing.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/engine/ -run TestLayerSchema -v`
Expected: FAIL — layer schemas are ignored, so nothing is inherited.

- [ ] **Step 3: Resolve in `evaluateLayer` and carry it down**

`evaluateLayer` already receives `layer *model.Layer` and iterates its segments, so both resolvers are callable there directly. Resolve both immediately after the `when` dispatch, before `CheckRequiredFields`:

```go
		inSchema := model.EffectiveInputSchema(layer, seg)
		outSchema := model.EffectiveOutputSchema(layer, seg)

		lr.Warnings = append(lr.Warnings, validation.CheckRequiredFields(seg, inSchema, ctx)...)
```

Add the resolved output schema to the `EvalContext` literal:

```go
	// OutputSchema is resolved for this segment before the strategy runs —
	// its own, or its layer's. The strategies never resolve it themselves.
	OutputSchema model.OutputSchema
```

In `output.go`, replace the reads of `seg.OutputSchema` at the top of `evaluateOutputs` with `ctx.OutputSchema`, returning early when it is empty. In `override.go`, replace the `len(seg.OutputSchema) > 0` guard the same way.

Pass `outSchema` to both `CheckRequiredOutputs` call sites in `evaluateLayer`.

- [ ] **Step 4: Watch for the duplicate-warning case**

`CheckRequiredFields` runs for **every** segment that passes its `when` dispatch, and the loop only exits when a strategy succeeds — so a `rule` segment that matches nothing falls through and the next segment is checked too. With a shared layer schema both report the same missing field.

Add a test for it: a layer with a layer-level required input field and two segments, the first a `rule` segment matching nothing. Assert the missing field is reported **once**, not twice. De-duplicate the layer's warnings on the `(Field, Message)` pair before returning from `evaluateLayer`, and only for the required-field message — render errors and output warnings genuinely differ per segment and must not be collapsed.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/domain/engine/ ./internal/domain/strategy/ ./internal/domain/validation/ -v`
Expected: PASS, including every pre-existing test.

- [ ] **Step 6: Build and commit**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/domain/
git commit -m "feat: evaluate segments against their layer's schemas"
```

---

### Task 4: Admin and API surface

**Files:**
- Modify: `internal/application/admin.go` (functions `CreateLayer`, `UpdateLayer`)
- Test: `internal/application/admin_layer_schema_test.go` (create)

**Interfaces:**
- Consumes: the two `Layer` fields (Task 1).
- Produces: nothing new exported. `UpdateLayer` must carry the new fields.

**The trap this task exists for.** `UpdateLayer` copies named fields from the incoming layer onto the stored one. Any field it does not name is silently discarded on every layer save — the same defect found in `LookupList` on the UI side. Read it before changing it and confirm which fields it currently copies.

- [ ] **Step 1: Write the failing test**

Create `internal/application/admin_layer_schema_test.go`, reusing the existing admin harness (find its real name in `admin_test.go`; do not write a second one). Assert: creating a layer with both schemas persists them; updating a layer preserves them when they are not part of the update; updating them changes them; and — the important one — a `UpdateLayer` call that omits them does not silently erase them if that is the intended semantic, or does erase them if replacement is intended. **Decide which, state it in the test name, and make the doc comment say so.** Replacement is consistent with how the admin API treats whole objects elsewhere; whichever you choose, it must be deliberate and documented rather than accidental.

Also assert that setting a layer schema which invalidates one of its segments — a rule reading a field the new schema does not declare — is rejected by `commitSnapshot`, since that is the escape-hatch hazard from Task 2 reaching the API.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/application/ -run TestLayerSchema -v`
Expected: FAIL — the fields are dropped by `UpdateLayer`.

- [ ] **Step 3: Carry the fields**

Add `InputSchema` and `OutputSchema` to whatever `UpdateLayer` copies, and to `CreateLayer` if it filters fields too. Update the doc comment to say what an omitted schema means.

- [ ] **Step 4: Run tests, build and commit**

```bash
go test ./internal/application/ -v && go build ./... && go vet ./... && go test ./...
git add internal/application/
git commit -m "feat: persist layer schemas through the admin API"
```

---

### Task 5: UI types and the layer editor

**Files:**
- Modify: `ui/src/api/types.ts`
- Modify: `ui/src/components/schema/outputSchemaRules.ts`
- Modify: `ui/verify-output-schema.mjs`
- Modify: whichever component edits a layer — find it by searching for `dependsOn` and `defaultLanguage` in `ui/src/components/`

**Interfaces:**
- Consumes: the API from Task 4.
- Produces: `Layer.inputSchema`/`outputSchema` in types, and `effectiveInputSchema(layer, seg)` / `effectiveOutputSchema(layer, seg)` in `outputSchemaRules.ts`.

- [ ] **Step 1: Types**

Add `inputSchema?: InputSchema` and `outputSchema?: OutputSchema` to the `Layer` interface, mirroring the Go JSON tags exactly.

- [ ] **Step 2: Add both resolvers, verifier first**

Add to `ui/verify-output-schema.mjs` before implementing, mirroring the Go semantics including that neither-declared yields `undefined` rather than `{}`:

```js
const layer = { inputSchema: { 'company.ein': { type: 'string', required: true } } };
assert.deepEqual(effectiveInputSchema(layer, {}), layer.inputSchema);
// Replacement, not merge.
assert.deepEqual(effectiveInputSchema(layer, { inputSchema: { a: { type: 'string' } } }),
                 { a: { type: 'string' } });
assert.equal(effectiveInputSchema({}, {}), undefined);
assert.equal(effectiveInputSchema(undefined, {}), undefined);

const lo = { outputSchema: { severity: { type: 'string' } } };
assert.deepEqual(effectiveOutputSchema(lo, {}), lo.outputSchema);
assert.equal(effectiveOutputSchema({}, {}), undefined);
```

Run `npm run verify:output-schema`, watch it fail, implement, confirm it passes. Also change `fieldCoverage` to take a resolved schema rather than reading `seg.outputSchema`, and add a coverage case for an inherited schema.

- [ ] **Step 3: Put both editors on the layer**

Add an Input Schema and an Output Schema section to the layer editor, reusing `InputSchemaEditor` and `OutputSchemaEditor` unchanged — they are already generic over a schema value and an onChange.

State the consequence in the UI, because it is the one that surprises: a note under the layer's input schema saying that declaring it here validates the rules of every segment that does not declare its own, and that segments which currently declare nothing will start being checked.

- [ ] **Step 4: Show inheritance in the segment editor**

Where a segment declares no schema and its layer does, the segment editor must show the inherited fields **read-only**, labelled as inherited, with a link to the layer — otherwise an author sees an empty schema and believes their rules are unchecked. Offer a *declare its own* action that copies the inherited fields in as a starting point, which is the natural way to diverge.

Pass the layer into `SegmentEditor` — it already looks the layer up to read `dependsOn` for the rule field picker, so the value is in scope.

- [ ] **Step 5: Verify**

From `ui/`: `npm run verify:output-schema`, `npm run build`, `npm run lint` (exactly 2 problems).

Then by hand, with both halves running: declare an input schema on a layer, confirm a segment declaring nothing shows the fields as inherited and its rule field picker offers them; declare a schema on that segment and confirm the layer's fields disappear rather than merging; evaluate and confirm required-field warnings come from the inherited schema.

Playwright with Chromium is installed; a driver script must live inside `ui/` to resolve the import. **Restore `config/segments.json` with `git checkout --` afterwards**, stop every server, delete scratch files.

- [ ] **Step 6: Commit**

```bash
git add ui/src/ ui/verify-output-schema.mjs
git commit -m "feat(ui): declare schemas on a layer, show inheritance on segments"
```

---

## Out of scope for this plan

- **A level above the layer.** Layer schemas do not make the caller's contract constant across layers; only a snapshot-level declaration would. Considered and set aside — see the shared-schemas plan in git history for the analysis and the measurement.
- **Merging a layer schema with a segment's.** Replacement is the rule. A merge would make the effective schema a computation and would break the nil escape hatch.
- **Migrating the shipped config.** `config/segments.json` keeps its twelve segment-declared schemas and must validate untouched. Hoisting any of them into their layers is a separate, deliberate decision.
