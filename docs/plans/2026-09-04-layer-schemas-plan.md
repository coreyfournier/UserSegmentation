# Layer-Level Schemas Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move both the input schema and the output schema from the segment to the layer, so a layer declares once what all of its segments read and emit.

**Architecture:** A schema belongs to a layer and nowhere else. There is **no inheritance, no override and no fallback** — `Layer.InputSchema` and `Layer.OutputSchema` are the only places either is declared, and every reader takes the layer's. The segment fields are removed. This is a breaking config change: the twelve segment-declared schemas in `config/segments.json` move up to their layers as part of the work.

**Tech Stack:** Go 1.26, `github.com/expr-lang/expr` v1.17.8, standard library testing. React 19 / TypeScript ~5.9 / Vite 7 for the UI half.

## Global Constraints

- **One place, no resolution.** A reader reads `layer.InputSchema` or `layer.OutputSchema`. There is no "segment's if set, layer's otherwise" anywhere — that is the simplicity this change buys, and reintroducing a fallback would spend it.
- **Removing the struct fields is the safety mechanism.** Deleting `Segment.InputSchema` and `Segment.OutputSchema` makes the compiler point at all 14 non-test readers and all 37 test constructions. Do not keep them as deprecated aliases — an alias would let a reader be missed. The one exception is the migration detector below, which is a *different* field with a different name.
- **A silently ignored schema is the danger, so detect it.** Go's JSON decoder drops unknown fields without error. A segment left carrying `inputSchema` after this change would decode to nothing, and since a missing input schema is the escape hatch that disables rule-field validation entirely, that config would load with checking quietly switched off. Task 1 adds `Segment.LegacyInputSchema`/`LegacyOutputSchema` bound to the old JSON names purely so validation can reject them with a message telling the author to move the schema to the layer. They carry no behaviour and can be deleted once no config in flight has them.
- **The nil-schema escape hatch survives, at layer scope.** A layer that declares no input schema, whose segments declare no `computed`, still skips rule-field validation. Five shipped segments rely on this; after migration the five layers containing them still declare nothing, so it must behave identically.
- **Every existing rule applies unchanged.** Rule-field typing, operator/type compatibility, lookup `keyType` agreement, output name binding, literal type parsing, template-must-be-string, `array`/`object` require expression, the output `Required` load gate and evaluation warning, all-or-nothing degradation. Only where the schema is read changes.
- Computed fields stay on the **segment** and still merge on top of the layer's input schema for that segment's rule validation. They are derived per segment and are not moving.
- `static` and `percentage` segments remain exempt from output-schema enforcement.
- **No guarding beyond what exists.** Do not add lookup membership, order uniqueness, or contiguity checks.
- **Scope note, recorded so it is not mistaken for an oversight.** Layer schemas let a layer's segments share; they do not make the caller's required-field set constant across layers. `layers: ["a"]` and `layers: ["a","b"]` still impose different required fields. Measured before this plan: the same context passes clean for one layer and warns `field "NetPay" required field missing from context` when a second is added. Only a level above the layer would fix that; it was considered and set aside.
- Go: `go build ./...`, `go vet ./...` and `go test ./...` must pass before every commit. Go is on `PATH` (go1.26.5); no `export PATH` needed.
- UI: `npm run build`, `npm run verify:output-schema` and `npm run verify:rules` must pass from `ui/`. **`npm run lint` has a red baseline of exactly 2 pre-existing errors** (`SegmentEditor.tsx` refs-during-render, `StrategyPicker.tsx:9` react-refresh) — leave both alone; gate on the count not rising above 2.
- **There is no JS test runner** and adding one is out of scope.
- The admin API replaces whole objects on save. Never construct a layer or segment from scratch — build payloads by spreading.

---

### Task 1: Move the fields

**Files:**
- Modify: `internal/domain/model/layer.go`
- Modify: `internal/domain/model/segment.go`
- Test: `internal/domain/model/layer_schema_test.go` (create)

**Interfaces:**
- Produces: `Layer.InputSchema InputSchema`, `Layer.OutputSchema OutputSchema`, and the two migration detectors `Segment.LegacyInputSchema` / `Segment.LegacyOutputSchema`. Removes `Segment.InputSchema` and `Segment.OutputSchema`. Every later task depends on this.

**Expect the build to break.** After this task `go build ./...` fails in roughly 14 places and `go test ./...` in 13 more files. That is intended and is how the change is made safe — Tasks 2 and 3 fix them. Do not stub anything to keep the build green.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/model/layer_schema_test.go` asserting that `Layer` carries both schemas and round-trips through JSON under the keys `inputSchema` and `outputSchema`, and that a `Segment` decoding a legacy `inputSchema` key lands in `LegacyInputSchema` rather than being dropped:

```go
package model

import (
	"encoding/json"
	"testing"
)

func TestLayerCarriesSchemas(t *testing.T) {
	in := Layer{
		Name:         "company-identity",
		InputSchema:  InputSchema{"company.ein": {Type: FieldTypeString, Required: true}},
		OutputSchema: OutputSchema{"severity": {Type: FieldTypeString}},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Layer
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.InputSchema["company.ein"].Required {
		t.Errorf("input schema lost: %s", b)
	}
	if out.OutputSchema["severity"].Type != FieldTypeString {
		t.Errorf("output schema lost: %s", b)
	}
}

func TestSegmentCapturesLegacySchemas(t *testing.T) {
	// A schema left on a segment must be captured, not silently dropped — a
	// dropped input schema turns rule-field validation off without saying so.
	var seg Segment
	err := json.Unmarshal([]byte(`{"id":"s","strategy":"rule",
		"inputSchema":{"age":{"type":"number"}},
		"outputSchema":{"severity":{"type":"string"}}}`), &seg)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(seg.LegacyInputSchema) != 1 {
		t.Errorf("legacy input schema not captured: %+v", seg)
	}
	if len(seg.LegacyOutputSchema) != 1 {
		t.Errorf("legacy output schema not captured: %+v", seg)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/model/ -run 'TestLayerCarriesSchemas|TestSegmentCapturesLegacy' -v`
Expected: FAIL — `Layer` has no schema fields and `Segment` has no legacy fields.

- [ ] **Step 3: Add the layer fields**

In `internal/domain/model/layer.go`, add to `Layer` after `Segments`:

```go
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
```

- [ ] **Step 4: Replace the segment fields with detectors**

In `internal/domain/model/segment.go`, **delete** `InputSchema` and `OutputSchema` and add:

```go
	// LegacyInputSchema and LegacyOutputSchema exist only to catch config
	// written before schemas moved to the layer. They carry no behaviour:
	// validation rejects any segment where either is non-empty, telling the
	// author to move it up.
	//
	// Without them Go's decoder would drop the old keys silently, and a dropped
	// input schema switches rule-field validation off rather than failing — so
	// the config would load, look fine, and check nothing. Delete both once no
	// config in flight still carries them.
	LegacyInputSchema  InputSchema  `json:"inputSchema,omitempty"`
	LegacyOutputSchema OutputSchema `json:"outputSchema,omitempty"`
```

- [ ] **Step 5: Run the model tests**

Run: `go test ./internal/domain/model/ -v`
Expected: PASS. `go build ./...` will now fail elsewhere — that is expected and Task 2 fixes it.

- [ ] **Step 6: Commit**

Commit with the build red, so the move and its fallout stay separable in history:

```bash
git add internal/domain/model/
git commit -m "feat!: move input and output schemas from segment to layer

The build is intentionally red after this commit; the compiler is being
used to find every reader. Tasks 2 and 3 fix them."
```

---

### Task 2: Validation reads the layer

**Files:**
- Modify: `internal/domain/validation/validator.go`
- Modify: every validation test the compiler flags
- Test: `internal/domain/validation/layer_schema_test.go` (create)

**Interfaces:**
- Consumes: `Layer.InputSchema`, `Layer.OutputSchema`, the two legacy detectors (Task 1).
- Produces: `buildEffectiveSchema(inSchema model.InputSchema, computed []model.ComputedField) model.InputSchema`, and every output helper taking an explicit schema.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/validation/layer_schema_test.go` covering:

1. A rule reading a field declared on its **layer** validates; one reading an undeclared field errors.
2. A layer schema plus a segment's `computed` — a rule reading the computed field validates, proving the merge still happens per segment.
3. The escape hatch: a layer with no input schema whose segments declare no computed still skips rule-field validation entirely.
4. **A segment carrying a legacy `inputSchema` is rejected**, with a message naming the segment and telling the author to move it to the layer. Same for `outputSchema`.
5. A `required` output field on the layer is enforced against a segment that authors nothing.
6. An undeclared output key is rejected against the layer's output schema.
7. A `static` segment in a layer with an output schema is still exempt.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/validation/ 2>&1 | head -20`
Expected: compilation failure — the package still references the removed segment fields.

- [ ] **Step 3: Rework the validator**

In `ValidateSnapshot`'s per-segment loop, read from the layer and reject legacy fields:

```go
			// Schemas live on the layer. A segment still carrying one is config
			// written against the old shape; rejecting it beats decoding it to
			// nothing and silently disabling rule-field validation.
			if len(seg.LegacyInputSchema) > 0 {
				errs = append(errs, fmt.Sprintf(
					"segment %q: inputSchema is declared on the layer now, not the segment — "+
						"move these %d field(s) to layer %q",
					seg.ID, len(seg.LegacyInputSchema), layer.Name))
			}
			if len(seg.LegacyOutputSchema) > 0 {
				errs = append(errs, fmt.Sprintf(
					"segment %q: outputSchema is declared on the layer now, not the segment — "+
						"move these %d field(s) to layer %q",
					seg.ID, len(seg.LegacyOutputSchema), layer.Name))
			}
```

Then use `layer.InputSchema` and `layer.OutputSchema` directly wherever the segment fields were read. Change `buildEffectiveSchema` to take the layer's schema plus the segment's computed fields, and update the escape-hatch guard:

```go
			if layer.InputSchema == nil && len(seg.Computed) == 0 {
				continue
			}
```

Thread `layer.OutputSchema` into every output helper in place of `seg.OutputSchema`, changing each signature to accept it explicitly.

- [ ] **Step 4: Fix the validation tests the compiler flags**

Each construction that set `InputSchema:` on a `model.Segment` moves it to the enclosing `model.Layer`. Where a test builds a bare segment with no layer, wrap it or set the schema on the layer the helper creates. **Do not change what any test asserts** — only where the schema is declared.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/domain/validation/ -v`
Expected: PASS, including every pre-existing test with its assertions unchanged.

- [ ] **Step 6: Commit**

```bash
git add internal/domain/validation/
git commit -m "feat: validate against the layer's schemas, reject segment-level ones"
```

---

### Task 3: Evaluation reads the layer

**Files:**
- Modify: `internal/domain/strategy/strategy.go` (`EvalContext`)
- Modify: `internal/domain/strategy/output.go`
- Modify: `internal/domain/strategy/override.go`
- Modify: `internal/domain/engine/evaluator.go`
- Modify: `internal/domain/validation/validator.go` (`CheckRequiredFields`, `CheckRequiredOutputs`)
- Modify: every engine, strategy, application and infrastructure test the compiler flags
- Test: `internal/domain/engine/layer_schema_test.go` (create)

**Interfaces:**
- Produces: `EvalContext.OutputSchema model.OutputSchema` — the layer's schema, passed in — and two changed signatures:
  - `CheckRequiredFields(seg *model.Segment, schema model.InputSchema, ctx map[string]interface{}) []model.Warning`
  - `CheckRequiredOutputs(seg *model.Segment, schema model.OutputSchema, a *model.Assignment, failures []model.Failure) []model.Warning`

- [ ] **Step 1: Write the failing test**

Create `internal/domain/engine/layer_schema_test.go` covering:

1. A checklist segment emits the fields declared by its **layer's** output schema on each `Failure.Outputs`.
2. A `required` layer output field, absent because its expression failed, produces the evaluation warning naming it.
3. Required **input** fields on the layer produce the missing-field warning.
4. **The duplicate case.** `CheckRequiredFields` runs for every segment that passes its `when`, and the loop only exits when a strategy *succeeds* — so a `rule` segment matching nothing falls through and the next segment is checked too, against the same layer schema. Build exactly that (a layer with a required field and two segments, the first a `rule` segment matching nothing) and assert the missing field is reported **once**.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/engine/ 2>&1 | head -20`
Expected: compilation failure across the packages still referencing the removed fields.

- [ ] **Step 3: Pass the layer's schemas down**

In `evaluateLayer`, which already holds `layer *model.Layer`, pass `layer.InputSchema` to `CheckRequiredFields` and `layer.OutputSchema` to both `CheckRequiredOutputs` call sites, and set it on the `EvalContext` literal:

```go
	// OutputSchema is the layer's — the only place it is declared. Strategies
	// never look it up themselves.
	OutputSchema model.OutputSchema
```

In `output.go`, read `ctx.OutputSchema` at the top of `evaluateOutputs`, returning early when empty. In `override.go`, replace the guard the same way.

- [ ] **Step 4: De-duplicate required-input warnings**

Before returning from `evaluateLayer`, collapse warnings that share a `(Field, Message)` pair **and** carry the required-field message. Do not collapse render errors or output warnings — those genuinely differ per segment and per finding.

- [ ] **Step 5: Fix every remaining test the compiler flags**

Across `internal/domain/engine/`, `internal/domain/strategy/`, `internal/application/` and `internal/infrastructure/`, move each `InputSchema:`/`OutputSchema:` from the segment literal to its layer. **Assertions do not change.** Where a strategy test constructs a bare segment with no layer, pass the schema through `EvalContext.OutputSchema` instead.

- [ ] **Step 6: Run the full suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: all green. This is the first point since Task 1 that the build is whole.

- [ ] **Step 7: Commit**

```bash
git add internal/
git commit -m "feat: evaluate against the layer's schemas"
```

---

### Task 4: Migrate the shipped config

**Files:**
- Modify: `config/segments.json`
- Test: `internal/infrastructure/config/migration_test.go` (extend)

**Interfaces:** none. This is data.

**What moves.** Twelve segments declare an input schema; each moves to its layer. Ten of those layers hold exactly one segment, so the move is mechanical. Two need a decision:

- **`transfer-fee`** — two segments, neither declares a schema. Nothing to move.
- **`company-payroll-setup`** — two segments with different schemas:
  - `precision`: `company.anchorDate*`, `company.checkDate*`, `company.defaultPayRate`, `company.payFrequency*`, `company.periodEnd*`, `company.productType*`
  - `express`: `company.fundingAccount*`, `company.productType*`

  (`*` = required.) **Union them** — seven fields, and a field is required on the layer if it was required on either segment. So six of the seven end up required; only `company.defaultPayRate` stays optional.

  This is the whole point of one schema per layer: every subject in the layer is checked against the same contract. The observable consequence is that an **Express** company now warns about the four Precision-only fields it does not send, and a **Precision** company warns about `company.fundingAccount`. Those are warnings, not errors — evaluation continues and the findings are unaffected — and they are the honest report that the caller's context does not satisfy the layer's declared contract. If they turn out to be noise in practice, the fix is to relax `required` on the layer, not to reintroduce per-segment schemas.

Only input schemas migrate: no segment in the shipped config declares an output schema.

- [ ] **Step 1: Move each schema**

For each layer with exactly one segment declaring `inputSchema`, cut the block from the segment and paste it onto the layer, as a sibling of `segments`. Then handle `company-payroll-setup` as described.

- [ ] **Step 2: Confirm it loads and is unchanged in behaviour**

Run: `go test ./internal/infrastructure/config/ -v`
Expected: PASS, including the test that loads and validates the shipped file.

Then verify no segment retains a legacy schema, which validation would now reject:

```bash
node -e 'const d=require("./config/segments.json");
for(const L of d.layers) for(const s of L.segments||[])
  if(s.inputSchema||s.outputSchema) console.log("STILL ON SEGMENT:",L.name,s.id);
console.log("done")'
```

- [ ] **Step 3: Confirm evaluation is unchanged**

Capture a `POST /v1/evaluate` response **before** starting the migration, for a subject exercising `company-identity`, `company-payroll-setup` and `employee-readiness`. Re-run it after and diff.

Findings, statuses and assignments must be **identical**. The only acceptable difference is additional required-field *warnings* on `company-payroll-setup`, where the unioned contract now asks each variant for the other's fields. Confirm the difference is confined to warnings — a change in `failures` or `status` means the migration altered behaviour and something moved wrongly.

- [ ] **Step 4: Commit**

```bash
git add config/segments.json internal/infrastructure/config/
git commit -m "refactor: move segment schemas up to their layers"
```

---

### Task 5: Admin API

**Files:**
- Modify: `internal/application/admin.go` (`CreateLayer`, `UpdateLayer`)
- Test: `internal/application/admin_layer_schema_test.go` (create)

**Interfaces:** no new exported names. `UpdateLayer` must carry the two new fields.

**The trap this task exists for.** `UpdateLayer` copies named fields from the incoming layer onto the stored one, so any field it does not name is silently discarded on every layer save. Read it first and confirm exactly which fields it copies today.

- [ ] **Step 1: Write the failing test**

Create `internal/application/admin_layer_schema_test.go`, reusing the existing admin harness (find its real name in `admin_test.go`; do not write a second one). Assert: creating a layer with both schemas persists them; updating a layer carries them through; and setting a layer schema that invalidates one of its segments — a rule reading a field the new schema does not declare — is rejected by `commitSnapshot`.

State in the test name whether an omitted schema on update clears it or preserves it, decide deliberately, and make the doc comment say so. Replacement is consistent with how the admin API treats whole objects elsewhere.

- [ ] **Step 2: Run test to verify it fails, then implement, then re-run**

Run: `go test ./internal/application/ -run TestLayerSchema -v`

- [ ] **Step 3: Build and commit**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/application/
git commit -m "feat: persist layer schemas through the admin API"
```

---

### Task 6: Move both editors to the layer

**Files:**
- Modify: `ui/src/api/types.ts`
- Modify: `ui/src/components/segments/SegmentEditor.tsx`
- Modify: `ui/src/components/schema/outputSchemaRules.ts`
- Modify: `ui/verify-output-schema.mjs`
- Modify: whichever component edits a layer — find it by searching `ui/src/components/` for `dependsOn` and `defaultLanguage`

**Interfaces:**
- Consumes: the API from Task 5.
- Produces: `Layer.inputSchema`/`outputSchema` in types; `Segment.inputSchema`/`outputSchema` removed.

- [ ] **Step 1: Types**

Move both fields from the `Segment` interface to the `Layer` interface, matching the Go JSON tags. TypeScript will now flag every UI reader — 33 references — which is the same compiler-as-safety-net trick Task 1 used.

- [ ] **Step 2: Update the rules module and its verifier**

`fieldCoverage` currently reads `seg.outputSchema`; change it to take the schema explicitly. Update the verifier's fixtures accordingly and confirm `npm run verify:output-schema` still passes with its assertions unchanged.

- [ ] **Step 3: Move the editors**

Add Input Schema and Output Schema sections to the layer editor, reusing `InputSchemaEditor` and `OutputSchemaEditor` unchanged — both are already generic over a value and an onChange.

Remove both sections from `SegmentEditor`. In their place show the layer's schemas **read-only**, labelled as coming from the layer, with a link to edit them there. Without that an author sees no schema on the segment and concludes their rules are unchecked.

`SegmentEditor` already looks its layer up to read `dependsOn` for the rule field picker, so the layer is in scope. Point the field picker and `effectiveSchema` at the layer's input schema merged with the segment's computed fields, exactly as the engine does.

The per-check output value editors stay on the segment — values are authored per rule and are not moving. They read the layer's output schema for the field list.

- [ ] **Step 4: Verify**

From `ui/`: `npm run verify:output-schema`, `npm run build`, `npm run lint` (exactly 2 problems).

By hand, with both halves running: declare an input schema on a layer and confirm its segments' rule field pickers offer those fields plus each segment's own computed fields; declare an output schema on the layer and confirm per-check value editors list its fields; evaluate and confirm the emitted records and required-field warnings match.

Playwright with Chromium is installed; a driver script must live inside `ui/` to resolve the import. **Restore `config/segments.json` with `git checkout --` afterwards**, stop every server, delete scratch files.

- [ ] **Step 5: Commit**

```bash
git add ui/
git commit -m "feat(ui): edit schemas on the layer, show them read-only on segments"
```

---

## Out of scope for this plan

- **A level above the layer.** Layer schemas do not make the caller's required-field set constant across layers; only a snapshot-level declaration would. Considered and set aside — see git history for the analysis and measurement.
- **Deleting the legacy detectors.** `Segment.LegacyInputSchema`/`LegacyOutputSchema` stay until no config in flight carries the old keys. Removing them is a one-line follow-up.
- **Per-segment schema divergence.** There is none by design. A layer whose segments genuinely need different schemas should be two layers.
