# Shared Schemas Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an input schema and an output schema each be declared once at snapshot level and referenced by any number of segments, so the contract a caller must satisfy is constant no matter which layers they request.

**Why, in one paragraph.** Today a segment declares its own `inputSchema`, so the set of required fields depends on which layers the caller asked for. Evaluating `["ewa-eligibility"]` with a given context passes clean; adding `"ewa-risk"` to the same request produces `field "NetPay" required field missing from context` — a warning that appeared only because the caller widened the request, not because anything about their data changed. Centralising the schema makes the contract a property of the config rather than of the request: supply the declared context once and adding a layer never surprises you. The same reasoning applies to output schemas from the other side — a consumer receiving records from several layers should see one shape, declared once, not seven copies that can drift apart.

**Architecture:** Additive, and modelled directly on `Snapshot.Lookups`, which already solves this for lookup tables. `Snapshot.InputSchemas` and `Snapshot.OutputSchemas` hold named schemas; a segment sets `inputSchemaRef` / `outputSchemaRef` instead of the inline form. **References resolve on read, never flattened into the segment** — the admin API replaces whole segments on save, so a flattened copy would be written back on the next edit and the sharing would decay into duplicates. Evaluation receives the tables the way it already receives lookups: indexed once by the evaluator, passed on `EvalContext`.

**Tech Stack:** Go 1.26, `github.com/expr-lang/expr` v1.17.8, standard library testing. React 19 / TypeScript ~5.9 / Vite 7 for the UI half.

## Global Constraints

- **Inline and reference are mutually exclusive, per schema kind.** A segment declares `inputSchema` **or** `inputSchemaRef`, and `outputSchema` **or** `outputSchemaRef`. Declaring both of a kind is a load error. Do not implement merge-or-override semantics: it would make "what does this segment read / emit" a computation rather than a lookup.
- **The nil-schema escape hatch must survive.** `ValidateSnapshot` currently skips rule-field validation entirely when a segment has no `inputSchema` and no `computed` (`validator.go`, the `continue`). Five shipped segments rely on it. "No schema" must continue to mean *neither inline nor ref*, and must behave exactly as it does today.
- **A shared schema's `required` binds every referencing segment.** That is the point. But an error or warning must name the shared schema, so an author or caller can tell where the obligation came from.
- **Every existing rule applies unchanged to a resolved schema.** Rule-field typing, operator/type compatibility, lookup `keyType` agreement, output name binding, literal type parsing, template-must-be-string, `array`/`object` require expression, the output `Required` load gate and evaluation warning, and all-or-nothing degradation. Resolution happens *before* those checks; none of them change.
- `static` and `percentage` segments remain exempt from output-schema enforcement. A ref on one is as inert as an inline schema is today.
- **No guarding beyond what exists.** Do not add lookup membership, order uniqueness, or contiguity checks.
- **Recorded trade.** Centralising an input schema widens the type environment each segment's rules are checked against, so a rule referencing a field that belongs to a different concern is no longer caught — measured: with a per-segment schema, a `company-health` rule reading `employee.hireDate` errors; against a union schema it validates cleanly. Outright typos are still caught. This is accepted deliberately: the target consumer supplies one fetched context to every layer, so the union is the accurate description, and caller-side predictability is worth more than that check. If it ever bites, the fix is an optional per-segment narrowing list, not un-sharing.
- Go: `go build ./...`, `go vet ./...` and `go test ./...` must pass before every commit. Go is on `PATH` (go1.26.5); no `export PATH` needed.
- UI: `npm run build`, `npm run verify:output-schema` and `npm run verify:rules` must pass from `ui/`. **`npm run lint` has a red baseline of exactly 2 pre-existing errors** (`SegmentEditor.tsx` refs-during-render, `StrategyPicker.tsx:9` react-refresh) — leave both alone; gate on the count not rising above 2.
- **There is no JS test runner** and adding one is out of scope. Pure UI logic is verified by a `verify-*.mjs` script using `node:assert`; components by build plus a manual check.
- The admin API replaces whole objects on save. Never construct a segment, layer or snapshot from scratch — build payloads by spreading.

---

### Task 1: Model and the resolvers

**Files:**
- Modify: `internal/domain/model/layer.go` (the `Snapshot` struct)
- Modify: `internal/domain/model/segment.go`
- Create: `internal/domain/model/schema_ref.go`
- Test: `internal/domain/model/schema_ref_test.go` (create)

**Interfaces:**
- Consumes: `model.InputSchema`, `model.OutputSchema` (existing).
- Produces: `Snapshot.InputSchemas map[string]InputSchema`, `Snapshot.OutputSchemas map[string]OutputSchema`, `Segment.InputSchemaRef string`, `Segment.OutputSchemaRef string`, and two resolvers:
  - `ResolveInputSchema(seg *Segment, shared map[string]InputSchema) (InputSchema, bool)`
  - `ResolveOutputSchema(seg *Segment, shared map[string]OutputSchema) (OutputSchema, bool)`

  Every later task calls these. After this lands, nothing may read `seg.InputSchema` or `seg.OutputSchema` directly outside the resolvers.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/model/schema_ref_test.go`:

```go
package model

import "testing"

func TestResolveInputSchema(t *testing.T) {
	shared := map[string]InputSchema{
		"context": {"company.ein": {Type: FieldTypeString, Required: true}},
	}

	inline := &Segment{InputSchema: InputSchema{"country": {Type: FieldTypeString}}}
	got, ok := ResolveInputSchema(inline, shared)
	if !ok {
		t.Fatal("inline did not resolve")
	}
	if _, has := got["country"]; !has {
		t.Errorf("inline schema lost: %v", got)
	}

	ref := &Segment{InputSchemaRef: "context"}
	got, ok = ResolveInputSchema(ref, shared)
	if !ok {
		t.Fatal("ref did not resolve")
	}
	if _, has := got["company.ein"]; !has {
		t.Errorf("ref resolved to the wrong schema: %v", got)
	}

	// A dangling ref must not resolve. Returning an empty schema would silently
	// disable rule-field validation for that segment, which is the opposite of
	// what an author typing a ref intends.
	if _, ok := ResolveInputSchema(&Segment{InputSchemaRef: "nope"}, shared); ok {
		t.Error("a dangling ref must not resolve")
	}

	// Neither declared is a clean "no schema" — the escape hatch five shipped
	// segments rely on. It must stay distinguishable from a dangling ref.
	if got, ok := ResolveInputSchema(&Segment{}, shared); ok || got != nil {
		t.Errorf("bare segment: ok=%v got=%v", ok, got)
	}

	if _, ok := ResolveInputSchema(&Segment{InputSchemaRef: "context"}, nil); ok {
		t.Error("nil shared table must not resolve")
	}
}

func TestResolveOutputSchema(t *testing.T) {
	shared := map[string]OutputSchema{
		"diagnosis": {"severity": {Type: FieldTypeString, Required: true}},
	}

	inline := &Segment{OutputSchema: OutputSchema{"title": {Type: FieldTypeString}}}
	if got, ok := ResolveOutputSchema(inline, shared); !ok || got["title"].Type != FieldTypeString {
		t.Fatalf("inline: ok=%v got=%v", ok, got)
	}
	if got, ok := ResolveOutputSchema(&Segment{OutputSchemaRef: "diagnosis"}, shared); !ok || got["severity"].Type != FieldTypeString {
		t.Fatalf("ref: ok=%v got=%v", ok, got)
	}
	if _, ok := ResolveOutputSchema(&Segment{OutputSchemaRef: "nope"}, shared); ok {
		t.Error("a dangling ref must not resolve")
	}
	if got, ok := ResolveOutputSchema(&Segment{}, shared); ok || len(got) != 0 {
		t.Errorf("bare segment: ok=%v got=%v", ok, got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/model/ -run TestResolve -v`
Expected: FAIL — the resolvers and the ref fields do not exist.

- [ ] **Step 3: Add the carrier fields**

In `internal/domain/model/layer.go`, add to `Snapshot`:

```go
	// InputSchemas and OutputSchemas are named schemas any segment may
	// reference, so a contract shared across layers is declared once. Modelled
	// on Lookups, which solves the same problem for lookup tables.
	//
	// Centralising the input schema is what makes the caller's contract a
	// property of the config rather than of the request: the required set no
	// longer changes when a caller adds a layer to `layers`.
	InputSchemas  map[string]InputSchema  `json:"inputSchemas,omitempty"`
	OutputSchemas map[string]OutputSchema `json:"outputSchemas,omitempty"`
```

In `internal/domain/model/segment.go`, add beside the existing schema fields:

```go
	// InputSchemaRef names a schema in Snapshot.InputSchemas. It is an
	// alternative to InputSchema, never a supplement — declaring both is
	// rejected at load, because a merge would make "what does this segment
	// read" a computation rather than a lookup.
	InputSchemaRef string `json:"inputSchemaRef,omitempty"`
	// OutputSchemaRef names a schema in Snapshot.OutputSchemas, on the same terms.
	OutputSchemaRef string `json:"outputSchemaRef,omitempty"`
```

- [ ] **Step 4: Add the resolvers**

Create `internal/domain/model/schema_ref.go` with both functions. Each returns `(schema, true)` for an inline declaration, follows the ref into the shared table when the inline form is empty, and returns `(nil, false)` for a dangling ref, an empty shared table, or neither declared.

Document on `ResolveInputSchema` why a dangling ref must not fall back to an empty schema: an empty input schema is the escape hatch that disables rule-field validation, so silently returning one would turn a typo into "no checking at all".

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/domain/model/ -v`
Expected: PASS, including the pre-existing model tests.

- [ ] **Step 6: Build and commit**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/domain/model/
git commit -m "feat: add snapshot-level input and output schemas with segment references"
```

---

### Task 2: Validation resolves both references

**Files:**
- Modify: `internal/domain/validation/validator.go`
- Test: `internal/domain/validation/schema_ref_test.go` (create)

**Interfaces:**
- Consumes: both resolvers, `Snapshot.InputSchemas`/`OutputSchemas`, both ref fields (Task 1).
- Produces: nothing new exported. Existing checks now run against resolved schemas.

**What changes and what must not.** `buildEffectiveSchema(seg)` currently merges `seg.InputSchema` with `seg.Computed`. It must resolve the reference first, then merge computed exactly as now — the merge stays per segment, because computed fields are per segment. Every output check must likewise take the resolved output schema. **The checks themselves do not change**; that is the point of resolving first.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/validation/schema_ref_test.go` covering, in the file's existing style:

1. A segment with a valid `inputSchemaRef` whose rule reads a field from the shared schema validates cleanly.
2. A **dangling** `inputSchemaRef` errors, naming the segment and the missing schema — and specifically does **not** silently skip rule validation.
3. Declaring both `inputSchema` and `inputSchemaRef` errors saying they are alternatives. Same for the output pair.
4. A segment with a `inputSchemaRef` **and** `computed` fields validates a rule that reads a computed field — proving the merge survives resolution.
5. **The escape hatch still works**: a segment with no input schema, no ref and no computed still skips rule-field validation entirely, exactly as today.
6. A `required` field in a **shared output schema** is enforced on a referencing segment, and the message names the shared schema.
7. An undeclared output key on a segment using an `outputSchemaRef` is rejected, proving name binding resolves through the reference.
8. A `static` segment carrying an `outputSchemaRef` is still exempt.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/validation/ -run TestValidate_SchemaRef -v`
Expected: FAIL — refs are ignored, so cases 1, 2, 4, 6 and 7 all behave wrongly.

- [ ] **Step 3: Resolve once per segment**

In `ValidateSnapshot`'s per-segment loop, before any schema-dependent check, add the structural checks and resolve both:

```go
			// Inline and reference are alternatives, never a merge.
			if len(seg.InputSchema) > 0 && seg.InputSchemaRef != "" {
				errs = append(errs, fmt.Sprintf(
					"segment %q: declares both inputSchema and inputSchemaRef %q — they are "+
						"alternatives; use one or the other", seg.ID, seg.InputSchemaRef))
			}
			if len(seg.OutputSchema) > 0 && seg.OutputSchemaRef != "" {
				errs = append(errs, fmt.Sprintf(
					"segment %q: declares both outputSchema and outputSchemaRef %q — they are "+
						"alternatives; use one or the other", seg.ID, seg.OutputSchemaRef))
			}
			// A dangling input ref must not fall through to "no schema", which
			// would disable rule-field validation for the whole segment.
			if seg.InputSchemaRef != "" && len(seg.InputSchema) == 0 {
				if _, ok := snap.InputSchemas[seg.InputSchemaRef]; !ok {
					errs = append(errs, fmt.Sprintf(
						"segment %q: inputSchemaRef %q is not declared in inputSchemas",
						seg.ID, seg.InputSchemaRef))
				}
			}
			if seg.OutputSchemaRef != "" && len(seg.OutputSchema) == 0 {
				if _, ok := snap.OutputSchemas[seg.OutputSchemaRef]; !ok {
					errs = append(errs, fmt.Sprintf(
						"segment %q: outputSchemaRef %q is not declared in outputSchemas",
						seg.ID, seg.OutputSchemaRef))
				}
			}
			inSchema, _ := model.ResolveInputSchema(&seg, snap.InputSchemas)
			outSchema, _ := model.ResolveOutputSchema(&seg, snap.OutputSchemas)
```

Change `buildEffectiveSchema` to take the resolved input schema rather than the segment:

```go
// buildEffectiveSchema merges a segment's resolved input schema with its
// expression-defined fields. Computed fields stay per segment even when the
// input schema is shared, because they are derived by that segment alone.
func buildEffectiveSchema(inSchema model.InputSchema, computed []model.ComputedField) model.InputSchema {
```

Update the escape-hatch guard so "no schema" means neither form:

```go
			if inSchema == nil && len(seg.Computed) == 0 {
				continue
			}
```

Thread `outSchema` into every output check in place of `seg.OutputSchema`, changing each helper's signature to accept the schema rather than re-resolving. One resolution per segment, passed down.

For the shared output case, extend the required-field error so the author can tell where the obligation originated, appending to the existing messages without changing their existing wording — tests match on substrings:

```go
	origin := ""
	if seg.OutputSchemaRef != "" {
		origin = fmt.Sprintf(" (required by shared schema %q)", seg.OutputSchemaRef)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/domain/validation/ -v`
Expected: PASS, including every pre-existing test. A pre-existing failure means the *inline* path changed behaviour — a regression, not an expected change.

- [ ] **Step 5: Confirm the shipped config still loads**

Run: `go test ./internal/infrastructure/config/ -v`
Expected: PASS. `config/segments.json` declares no shared schemas, so it must be entirely unaffected — including the five segments that rely on the nil-schema escape hatch.

- [ ] **Step 6: Build and commit**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/domain/validation/
git commit -m "feat: validate through shared schema references"
```

---

### Task 3: Evaluation resolves both references

**Files:**
- Modify: `internal/domain/strategy/strategy.go` (`EvalContext`)
- Modify: `internal/domain/strategy/output.go`
- Modify: `internal/domain/strategy/override.go`
- Modify: `internal/domain/engine/evaluator.go`
- Modify: `internal/domain/validation/validator.go` (`CheckRequiredFields`, `CheckRequiredOutputs`)
- Test: `internal/domain/engine/schema_ref_test.go` (create)

**Interfaces:**
- Consumes: both resolvers (Task 1).
- Produces: `EvalContext.OutputSchemas map[string]model.OutputSchema`, and two changed signatures taking a resolved schema:
  - `CheckRequiredFields(seg *model.Segment, schema model.InputSchema, ctx map[string]interface{}) []model.Warning`
  - `CheckRequiredOutputs(seg *model.Segment, schema model.OutputSchema, a *model.Assignment, failures []model.Failure) []model.Warning`

**Follow the lookups precedent exactly.** `EvalContext.Lookups` already carries a snapshot-level table into evaluation: the evaluator indexes `snap.Lookups` once and passes it on every `EvalContext`. Shared output schemas thread identically. Input schemas do **not** need to reach the strategies — only `evaluateLayer` uses them, and it has the snapshot — so resolve those in the evaluator and pass the result to `CheckRequiredFields`.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/engine/schema_ref_test.go` covering:

1. A checklist segment using an `outputSchemaRef` emits the resolved fields on its `Failure.Outputs`, proving the ref reached evaluation.
2. A `required` field in a shared output schema, absent because its expression failed, produces the evaluation warning naming the field.
3. **The headline case.** Two layers both using the same `inputSchemaRef`. Evaluate with `layers: ["a"]` and then `layers: ["a","b"]` against the same context that is missing a required field. Assert the *set of distinct missing fields warned about* is identical in both — adding a layer must not introduce a new required-field surprise. This is the behaviour the whole plan exists for; pin it.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/engine/ -run TestSchemaRef -v`
Expected: FAIL — refs are ignored, so no outputs are emitted and the required set still varies by layer selection.

- [ ] **Step 3: Carry the output table into evaluation**

In `EvalContext`, beside `Lookups`:

```go
	// OutputSchemas maps name to schema so a segment's OutputSchemaRef resolves
	// during evaluation. Populated by the evaluator, exactly as Lookups is.
	OutputSchemas map[string]model.OutputSchema
```

In `evaluator.go`, index `snap.OutputSchemas` once alongside the existing lookup indexing and set the field on the `EvalContext` literal beside `Lookups`.

In `output.go`, replace the direct reads at the top of `evaluateOutputs` with one resolution against `ctx.OutputSchemas`, returning early when it does not resolve. In `override.go`, replace the `len(seg.OutputSchema) > 0` guard the same way.

- [ ] **Step 4: Resolve input schemas in the evaluator**

In `evaluateLayer`, resolve the input schema per segment and pass it to `CheckRequiredFields`. Resolve the output schema once per segment too, and pass it to both `CheckRequiredOutputs` call sites.

`evaluateLayer` does not currently receive the snapshot — thread the two maps in as parameters from `Evaluate`, which already holds them, rather than reaching for a global.

- [ ] **Step 5: De-duplicate required-input warnings**

With N segments referencing one shared input schema, a single missing required field currently produces N identical warnings — one per segment — which buries the predictability win in noise.

In `Evaluate`, when appending a layer's warnings to `result.Warnings`, drop a warning whose `Field` and `Message` duplicate one already recorded **and** whose segment differs only because the schema is shared. The simplest correct rule: de-duplicate `result.Warnings` on the `(Field, Message)` pair for warnings whose message is the required-field one, keeping the first. Do not de-duplicate render errors or output warnings — those genuinely differ per segment and per finding.

Add a test asserting that three segments sharing one input schema, with one required field missing, produce exactly one warning about it.

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/domain/engine/ ./internal/domain/strategy/ ./internal/domain/validation/ -v`
Expected: PASS, including every pre-existing test.

- [ ] **Step 7: Build and commit**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/domain/
git commit -m "feat: resolve shared schemas during evaluation, dedupe required warnings"
```

---

### Task 4: Admin CRUD for shared schemas

**Files:**
- Create: `internal/application/admin_schemas.go`
- Create: `internal/infrastructure/http/schemas_handler.go`
- Modify: `internal/infrastructure/http/server.go`
- Test: `internal/application/admin_schemas_test.go` (create)

**Interfaces:**
- Consumes: `Snapshot.InputSchemas`/`OutputSchemas` (Task 1), `AdminUseCase.cloneSnapshot`/`commitSnapshot` (existing, unexported).
- Produces: `ListInputSchemas`, `PutInputSchema`, `DeleteInputSchema` and the same trio for output schemas, plus `SchemaReferencedError`.

**Mirror `admin_lookup.go`** — read it first. Same clone-mutate-commit shape and the same reference guard on delete. There is no create/update split: a schema is keyed by name in a map, so `Put` upserts.

- [ ] **Step 1: Write the failing test**

Create `internal/application/admin_schemas_test.go`, reusing the existing admin test harness (find its real name in `admin_test.go`; do not write a second one). Cover: put creates; put again updates; delete removes; **delete is refused with `SchemaReferencedError` naming every referencing segment**; and a put that would invalidate a referencing segment — removing a field a rule reads, or a required output field no rule authors — is rejected by `commitSnapshot`'s validation.

That last case matters most: editing a shared schema can break segments the editor is not looking at. `commitSnapshot` validates the whole snapshot, so it is caught. Pin that it is.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/application/ -run TestSchema -v`
Expected: FAIL — none of the methods exist.

- [ ] **Step 3: Implement the use case**

Create `internal/application/admin_schemas.go` mirroring `admin_lookup.go`, including a `SchemaReferencedError{Kind, Name, Segments}` and a `schemaReferences(snap, kind, name) []string` walking every layer's segments for a matching ref.

- [ ] **Step 4: Wire the HTTP handlers**

Create `internal/infrastructure/http/schemas_handler.go` mirroring `lookup_handler.go`, mapping `SchemaReferencedError` to `409 Conflict` listing the referencing segments. Register beside the lookup routes:

```go
	mux.HandleFunc("GET /v1/admin/input-schemas", admin.ListInputSchemas)
	mux.HandleFunc("PUT /v1/admin/input-schemas/{name}", admin.PutInputSchema)
	mux.HandleFunc("DELETE /v1/admin/input-schemas/{name}", admin.DeleteInputSchema)
	mux.HandleFunc("GET /v1/admin/output-schemas", admin.ListOutputSchemas)
	mux.HandleFunc("PUT /v1/admin/output-schemas/{name}", admin.PutOutputSchema)
	mux.HandleFunc("DELETE /v1/admin/output-schemas/{name}", admin.DeleteOutputSchema)
```

- [ ] **Step 5: Run tests, build and commit**

Run: `go test ./internal/application/ ./internal/infrastructure/http/ -v`, then `go build ./... && go vet ./... && go test ./...`

```bash
git add internal/application/ internal/infrastructure/http/
git commit -m "feat: admin endpoints for shared input and output schemas"
```

---

### Task 5: UI types, hooks, and the shared resolvers

**Files:**
- Modify: `ui/src/api/types.ts`
- Create: `ui/src/api/schemas.ts`
- Modify: `ui/src/components/schema/outputSchemaRules.ts`
- Modify: `ui/verify-output-schema.mjs`

**Interfaces:**
- Consumes: the endpoints from Task 4.
- Produces: `Snapshot.inputSchemas`/`outputSchemas`, `Segment.inputSchemaRef`/`outputSchemaRef`, the six react-query hooks, and `resolveInputSchema` / `resolveOutputSchema` in `outputSchemaRules.ts`.

- [ ] **Step 1: Types and hooks**

Add the four new fields to `Snapshot` and `Segment` in `types.ts`. Create `ui/src/api/schemas.ts` mirroring `ui/src/api/lookups.ts` exactly — same react-query shape, query keys `['inputSchemas']` and `['outputSchemas']`, invalidating on mutation.

- [ ] **Step 2: Add both resolvers, verifier first**

Add to `ui/verify-output-schema.mjs` before implementing — mirroring the Go semantics exactly, including that a dangling ref resolves to nothing rather than an empty schema:

```js
const sharedIn = { context: { 'company.ein': { type: 'string', required: true } } };
assert.deepEqual(resolveInputSchema({ inputSchema: { a: { type: 'string' } } }, sharedIn), { a: { type: 'string' } });
assert.deepEqual(resolveInputSchema({ inputSchemaRef: 'context' }, sharedIn), sharedIn.context);
assert.equal(resolveInputSchema({ inputSchemaRef: 'nope' }, sharedIn), undefined);
assert.equal(resolveInputSchema({}, sharedIn), undefined);

const sharedOut = { diagnosis: { severity: { type: 'string', required: true } } };
assert.deepEqual(resolveOutputSchema({ outputSchemaRef: 'diagnosis' }, sharedOut), sharedOut.diagnosis);
assert.equal(resolveOutputSchema({ outputSchemaRef: 'nope' }, sharedOut), undefined);
```

Run `npm run verify:output-schema`, watch it fail, implement both resolvers, confirm it passes.

Also change `fieldCoverage` to take a resolved schema rather than reading `seg.outputSchema`, and add a verifier case proving coverage works for a referencing segment.

- [ ] **Step 3: Verify and commit**

Run from `ui/`: `npm run verify:output-schema`, `npm run build`, `npm run lint`.

```bash
git add ui/src/api/ ui/src/components/schema/outputSchemaRules.ts ui/verify-output-schema.mjs
git commit -m "feat(ui): types, hooks and resolvers for shared schemas"
```

---

### Task 6: Reference mode in both editors

**Files:**
- Create: `ui/src/components/schema/SchemaSourcePicker.tsx`
- Modify: `ui/src/components/schema/InputSchemaEditor.tsx`
- Modify: `ui/src/components/schema/OutputSchemaEditor.tsx`
- Modify: `ui/src/components/segments/SegmentEditor.tsx`

**Interfaces:**
- Consumes: the hooks and resolvers from Task 5.
- Produces: `SchemaSourcePicker`, a shared *Inline / Shared* control used by both editors so the two behave identically.

- [ ] **Step 1: Build the source picker**

One small component: a mode toggle plus, in shared mode, a `<select>` of available names and a line saying how many other segments reference the chosen one. Both editors use it, so the interaction is learned once.

- [ ] **Step 2: Wire it into both editors**

In shared mode each editor renders the resolved fields **read-only**, with a link to manage the schema. Editing a shared schema happens in its manager (Task 7), never inline on a segment — otherwise an author edits every referencing segment's contract while believing they are editing one.

Mode switching must be explicit and lossless:
- Inline → Shared with fields already declared: warn that the inline fields will be dropped, naming how many.
- Shared → Inline: **copy** the resolved schema into the inline field as a starting point. That is the natural way to fork a shared shape.

Wire from `SegmentEditor` through the existing `update(partial)` helper only. When setting a ref, clear the inline schema in the same update, and vice versa — they are mutually exclusive and the engine rejects both.

- [ ] **Step 3: Verify**

Run from `ui/`: `npm run build`, `npm run lint`, `npm run verify:output-schema`. Then by hand: point a segment at a shared input schema, confirm the rule field picker still offers those fields plus the segment's computed fields, and confirm switching to Inline copies them.

- [ ] **Step 4: Commit**

```bash
git add ui/src/components/
git commit -m "feat(ui): reference a shared input or output schema from a segment"
```

---

### Task 7: The shared schema manager

**Files:**
- Create: `ui/src/components/schema/SharedSchemaList.tsx`
- Create: `ui/src/components/schema/SharedSchemaList.module.css`
- Modify: whichever component renders the left-hand nav — `Lookups` sits there today; find it and follow its pattern

**Interfaces:**
- Consumes: the hooks from Task 5, and both editors in inline mode for the field tables.
- Produces: nothing consumed elsewhere.

- [ ] **Step 1: Build the manager**

One page with two sections, input schemas and output schemas, each listing name, field count, and referencing-segment count. Create, rename, edit fields, delete. Reuse `InputSchemaEditor` and `OutputSchemaEditor` in inline mode for the field tables, so those tables exist in exactly one place.

On delete, surface the `409` from Task 4 by listing the referencing segments rather than showing a raw error.

**Show the blast radius before a save.** Editing a shared schema changes every referencing segment and can invalidate ones the author is not looking at — removing an input field a rule reads, or an output field a rule authors, makes the whole snapshot invalid. State the referencing-segment count beside the editor, and surface the validation error verbatim on a rejected save, since it names the offending segment and rule.

- [ ] **Step 2: Add to the nav, beside Lookups**

- [ ] **Step 3: Verify by hand**

With both halves running (`go run ./cmd/segmentation -config config/segments.json -addr :8080` and `npm run dev` from `ui/`), confirm: create a shared input schema; point two segments in different layers at it; evaluate with one layer then both and see the **same** required-field warnings either way — the behaviour this plan exists for; try to delete the schema and get the conflict listing both segments; remove a field a rule reads and see the save rejected naming that rule.

Playwright with Chromium is installed; a driver script must live inside `ui/` to resolve the import. **Restore `config/segments.json` with `git checkout --` afterwards**, stop every server, delete scratch files.

- [ ] **Step 4: Commit**

```bash
git add ui/src/
git commit -m "feat(ui): manage shared input and output schemas"
```

---

## Out of scope for this plan

- **Migrating the shipped config.** `config/segments.json` declares no shared schemas and must keep validating untouched. Converting its twelve inline input schemas into shared ones is a separate, deliberate decision.
- **Per-segment narrowing of a shared input schema.** The recorded trade above accepts that a shared input schema widens the type environment. If that ever bites, an optional `uses: [...]` list scoping which of the shared fields a segment's rules may reference is the fix — additive, and not needed until there is evidence.
- **Versioning a shared schema.** Editing one changes every referencing segment immediately. That is the intent.
