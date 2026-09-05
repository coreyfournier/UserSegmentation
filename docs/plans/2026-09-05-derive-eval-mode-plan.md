# Derive Eval Mode From Type Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the `eval` choice from an output field. A `string` field is always a template; every other type is always an expression.

**Why.** The mode was a separate axis from the type, so `{type: "object", eval: "template"}` was expressible and had to be policed — by two validation rules, a type-snapping routine in each editor, a dropdown hack that keeps an illegal type visible so the select does not lie, and four `<select>`s of UI. Deriving the mode from the type makes those states unrepresentable instead of invalid, and deletes all of it.

It loses nothing. A template token is a full expression (`evalMessageExpr` compiles it), so `${ company.name + " Inc" }` computes a string; a no-token template renders to itself, so constants still work. For non-strings, a bare `3` or `true` is already a valid expression. And it removes a real trap: under expression mode an unquoted `Critical` parses as an identifier lookup and yields **nil with no error**, which is why string constants needed `literal` — a problem that cannot arise once strings are never expression-mode.

**The one accepted cost:** a string constant containing `${…}` becomes inexpressible — it would attempt interpolation, fail, and drop the field. `renderTemplate` has no escape syntax and none is being added. Message templates have had the identical limitation since they shipped. Recorded, not fixed.

**Architecture:** `OutputField` loses `Eval`; `EvalMode` and its constants go entirely. `evaluateOutputs` branches on `decl.Type == FieldTypeString`. Because Go's decoder drops unknown fields silently and a stale `"eval": "expression"` on a string field would change that value's meaning from an expression to a template, `Segment`-style legacy detection is added: `OutputField.LegacyEval` bound to `json:"eval"`, rejected at load with a message saying the mode is now derived.

**Tech Stack:** Go 1.26, `github.com/expr-lang/expr` v1.17.8, standard library testing. React 19 / TypeScript ~5.9 / Vite 7.

## Global Constraints

- **The mode is derived, never stored.** After this change nothing persists an eval mode. Do not add a computed field, a helper that returns one from config, or a "mode" column anywhere — the type *is* the mode.
- **A `string` field is a template. Everything else is an expression.** No exceptions, including `array` and `object`, which were already expression-only.
- **Legacy `eval` is rejected, not ignored.** A stale `{"type":"string","eval":"expression"}` whose value is `"\"Critical\""` would silently start rendering with the quotes included. Reject at load and say why.
- **`literalTypeErrors` goes away** — there are no literals left to type-check. In its place, `validateOutputExpressionSyntax` broadens from expression-mode fields to **every non-string field**, so load-time syntax checking is not lost.
- **The client loses its literal parse check**, and that is expected. Typing `high` into a number field no longer errors in the editor; it is now an expression that resolves to nil at runtime and drops the field with a render error. Do not attempt to compile expr in the browser to compensate.
- Everything else about output schemas is unchanged: name binding, lookup `keyType` agreement, the `Required` load gate and evaluation warning, all-or-nothing degradation, the layer-level declaration, per-value rows with a field dropdown.
- **No guarding beyond what exists.** Do not add lookup membership, order uniqueness, or contiguity checks.
- Go: `go build ./...`, `go vet ./...` and `go test ./...` must pass before every commit. Go is on `PATH` (go1.26.5); no `export PATH` needed.
- UI: `npm run build`, `npm run verify:output-schema` and `npm run verify:rules` must pass from `ui/`. **`npm run lint` has a red baseline of exactly 2 pre-existing errors** (`SegmentEditor.tsx` refs-during-render, `StrategyPicker.tsx:9` react-refresh) — leave both alone; gate on the count not rising above 2.
- **There is no JS test runner** and adding one is out of scope.
- **`config/segments.json` may hold uncommitted work that is not yours** — the UI proxies to a container bind-mounted to `./config`, so the file changes as someone uses the app. Do not `git checkout` it, do not commit it, and do not point admin calls at port 8080. Run your own server on a high port against a **copy**.

---

### Task 1: Drop `Eval` from the model

**Files:**
- Modify: `internal/domain/model/output.go`
- Test: `internal/domain/model/output_test.go`

**Interfaces:**
- Produces: `OutputField{Type, Lookup, Required, LegacyEval}`. Removes `EvalMode`, `EvalLiteral`, `EvalTemplate`, `EvalExpression` and the `EvalMode()` accessor. Every later task depends on this.

**Expect the build to break.** Removing the type and accessor will fail compilation wherever they are referenced. That is how the readers get found; Tasks 2 and 3 fix them.

- [ ] **Step 1: Write the failing test**

In `internal/domain/model/output_test.go`, replace any test of `EvalMode()` with one asserting the new shape and the legacy capture:

```go
func TestOutputFieldDropsEval(t *testing.T) {
	// A stale "eval" key must be captured, not dropped. Silently ignoring it
	// would turn an expression-mode string value into a template, changing
	// what it emits with no error anywhere.
	var f OutputField
	if err := json.Unmarshal([]byte(`{"type":"string","eval":"expression"}`), &f); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if f.Type != FieldTypeString {
		t.Errorf("type = %v", f.Type)
	}
	if f.LegacyEval == "" {
		t.Error("legacy eval was dropped instead of captured")
	}

	// A current field round-trips with no eval key at all.
	b, err := json.Marshal(OutputField{Type: FieldTypeNumber, Required: true})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "eval") {
		t.Errorf("eval must not be emitted: %s", b)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/domain/model/ -run TestOutputFieldDropsEval -v`
Expected: FAIL — `LegacyEval` does not exist.

- [ ] **Step 3: Rewrite the type**

In `internal/domain/model/output.go`, delete `EvalMode`, its three constants and the `EvalMode()` method, and replace `OutputField` with:

```go
// OutputField declares one field a segment emits with each reported item.
//
// How the authored value becomes a value is derived from Type, not declared:
// a string field is a template, every other type is an expression. Those were
// once a separate axis, which made pairings like {object, template} writable
// and therefore something validation had to reject. Deriving it makes them
// unwritable.
//
// The derivation loses nothing. A template token is a full expression, so a
// string can still be computed — ${ company.name + " Inc" } — and a template
// with no tokens renders to itself, so constants still work. For a non-string,
// a bare 3 or true is already a valid expression. It also removes a trap: in
// expression mode an unquoted Critical is an identifier lookup that yields nil
// with no error, which is exactly why string constants used to need a literal
// mode.
//
// One thing is given up: a string constant cannot contain ${…}, because
// renderTemplate has no escape. Message templates have always had this
// limitation.
type OutputField struct {
	Type   FieldType `json:"type"`
	Lookup string    `json:"lookup,omitempty"`
	// Required is the caller's contract: an error at snapshot load if no
	// authoring path supplies it, a warning at evaluation if it is absent
	// anyway.
	Required bool `json:"required,omitempty"`
	// LegacyEval exists only to catch config written when the mode was
	// declared. It carries no behaviour; validation rejects any field where it
	// is set. Without it the decoder would drop the key, and a string field
	// that said eval:"expression" would silently start being rendered as a
	// template — its value quietly changing meaning. Delete once no config in
	// flight carries it.
	LegacyEval string `json:"eval,omitempty"`
}

// IsTemplate reports whether this field's authored value is a ${…} template
// rather than an expression. The single place the derivation lives.
func (f OutputField) IsTemplate() bool { return f.Type == FieldTypeString }
```

- [ ] **Step 4: Run the model tests, then commit red**

Run: `go test ./internal/domain/model/ -v` — must pass. `go build ./...` will fail elsewhere; that is expected.

```bash
git add internal/domain/model/
git commit -m "feat!: derive an output field's eval mode from its type

The build is intentionally red after this commit; the compiler is locating
every reader. Tasks 2 and 3 fix them."
```

---

### Task 2: Validation

**Files:**
- Modify: `internal/domain/validation/validator.go`
- Modify: the validation tests the compiler flags
- Test: `internal/domain/validation/eval_mode_test.go` (create)

**Interfaces:** no new exported names.

**What goes, and what replaces it:**
- The **template-must-be-string** check and the **object/array-requires-expression** check both disappear. Those pairings are now unwritable.
- **`literalTypeErrors` disappears entirely.** There are no literals; a non-string field's value is an expression, so there is nothing to parse-check at load.
- **`validateOutputExpressionSyntax` broadens** from expression-mode fields to every **non-string** field, so load-time syntax checking is not lost. Keep its existing behaviour of skipping disabled rules.
- **A new check rejects `LegacyEval`**, naming the field and saying the mode is derived from the type now.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/validation/eval_mode_test.go` covering:
1. A `string` output field with a `${…}` value validates.
2. A `number` output field with a syntactically broken expression (`"amount *"`) errors.
3. A `number` field with `"3"` validates — a bare numeral is a valid expression.
4. A field carrying a legacy `eval` key is rejected, with a message naming the field.
5. An `object` field validates with no eval anywhere — previously it required `eval: "expression"`.
6. A disabled rule's broken expression is still exempt.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/domain/validation/ 2>&1 | head -20`
Expected: compilation failure — the package still references `EvalMode`.

- [ ] **Step 3: Rework**

Delete `literalTypeErrors` and its call site. Delete the two pairing checks from `validateOutputSchema`. Change `validateOutputExpressionSyntax`'s predicate from "eval mode is expression" to `!f.IsTemplate()`. Add, in `validateOutputSchema`'s loop over fields:

```go
		if f.LegacyEval != "" {
			errs = append(errs, fmt.Sprintf(
				"segment %q output %q: \"eval\" is no longer declared — the mode is derived "+
					"from the type, so a string is a template and everything else is an "+
					"expression; remove it",
				seg.ID, name))
		}
```

- [ ] **Step 4: Fix the flagged tests, run, commit**

Move each construction off `Eval:`; **do not change what any test asserts**. If an assertion cannot hold under the new model, stop and report which rather than editing it.

Run: `go test ./internal/domain/validation/ -v` — all green.

```bash
git add internal/domain/validation/
git commit -m "feat: validate output fields without an eval mode"
```

---

### Task 3: Evaluation

**Files:**
- Modify: `internal/domain/strategy/output.go`
- Modify: every remaining Go file the compiler flags
- Test: `internal/domain/strategy/eval_mode_test.go` (create)

**Interfaces:** none new. This is the task that makes the build whole again.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/strategy/eval_mode_test.go` asserting, through `evaluateOutputs`:
1. A `string` field renders its value as a template — both a plain constant and one with a `${}` token.
2. A `number` field evaluates its value as an expression — both a bare `3` and a computed `a + b`.
3. A `boolean` field with `true` yields the boolean, not the string.
4. An `object` field with a map expression yields a map.
5. A failing value still degrades: error recorded, field omitted, item kept.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/domain/strategy/ 2>&1 | head -20`
Expected: compilation failure.

- [ ] **Step 3: Branch on the type**

In `evaluateOutputs`, replace the three-way switch on eval mode with:

```go
		var value interface{}
		if decl.IsTemplate() {
			rendered, bad := renderTemplate(raw, ctx.Context)
			if len(bad) > 0 {
				for _, te := range bad {
					errs = append(errs, RenderError{Language: ctx.DefaultLanguage, Token: te.token, Err: te.err})
				}
				continue
			}
			value = rendered
		} else {
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
		}
```

Keep the all-or-nothing degradation exactly as it is — a failed template omits the field rather than emitting a half-rendered string.

- [ ] **Step 4: Fix every remaining flagged file, run the full suite, commit**

Run: `go build ./... && go vet ./... && go test ./...` — all green.

```bash
git add internal/
git commit -m "feat: evaluate an output value by its declared type"
```

---

### Task 4: The UI loses the mode

**Files:**
- Modify: `ui/src/api/types.ts`
- Modify: `ui/src/components/schema/outputSchemaRules.ts`
- Modify: `ui/verify-output-schema.mjs`
- Modify: `ui/src/components/schema/OutputSchemaEditor.tsx`
- Modify: `ui/src/components/rules/OutputValuesEditor.tsx`

**Interfaces:**
- Produces: `OutputField` without `eval`; `EvalMode` removed from `types.ts`. `outputSchemaRules.ts` loses `EVAL_MODES`, `EVAL_MODE_HINT`, `allowedTypesForMode`, `evalModeOf` and `validateLiteralValue`, and gains `isTemplateField(field): boolean` plus a `placeholderFor(field): string`.

**Use TypeScript as the compiler-as-safety-net again:** remove `eval` from the `OutputField` interface first and let `tsc` flag all 36 references.

- [ ] **Step 1: Types and the rules module**

Drop `eval` from `OutputField` and delete the `EvalMode` union. In `outputSchemaRules.ts` delete the five exports above and add:

```ts
/** A string field's value is a template; everything else is an expression. */
export function isTemplateField(field: OutputField): boolean {
  return field.type === 'string';
}

/** What to show in an empty value input, so the author knows what to type. */
export function placeholderFor(field: OutputField): string {
  return isTemplateField(field)
    ? 'text, with ${ … } to interpolate'
    : 'an expression — a bare 3 or true is fine';
}
```

`validateOutputField` keeps only the lookup checks — existence and `keyType` agreement. The type/mode checks are gone.

- [ ] **Step 2: Update the verifier**

Delete the assertions covering `allowedTypesForMode` and `validateLiteralValue`; those functions no longer exist. **Keep every other assertion unchanged** — the row model, the field-picker options, the rename, the coverage cases all still apply. Add cases for `isTemplateField` across all five types.

Run `npm run verify:output-schema` and confirm it passes.

- [ ] **Step 3: Both editors**

`OutputSchemaEditor`: delete the **Eval** column, its header, and the type-snapping in `patch`. The Type select now offers all five types unconditionally — and the "keep an illegal type in the list so the select does not lie" hack goes with it, since no type is illegal any more. Remove the mode state from the add row.

`OutputValuesEditor`: delete the eval select from the inline declare panel and the `newMode` state; `declare()` builds `{ type: newType }`. The value input's placeholder comes from `placeholderFor`. Remove the `validateLiteralValue` call and the inline parse error it rendered — there are no literals left to check, and the engine reports a bad expression at save.

- [ ] **Step 4: Verify**

From `ui/`: `npm run verify:output-schema`, `npm run verify:rules`, `npm run build`, `npm run lint` (exactly 2 problems).

By hand, with an engine on a **high port** against a **copy** of the config and `npm run dev`: declare a string field inline and confirm its placeholder mentions `${ }`; declare a number field and confirm it mentions an expression; confirm no Eval column remains in the layer's schema editor; confirm all five types are offered; author a `${}` value on a string field and a bare `3` on a number field, save, and confirm the emitted record carries the interpolated string and the numeric 3.

Playwright with Chromium is installed; the driver script must live inside `ui/` and be deleted after. **The UI proxies `/v1` to port 8080, which is a container bind-mounted to `./config`** — redirect requests in the browser to your own server instead, and never let a test write to the working-tree config.

- [ ] **Step 5: Commit**

```bash
git add ui/
git commit -m "feat(ui): drop the eval mode, derive it from the field type"
```

---

## Out of scope for this plan

- **An escape for `${` in a string constant.** Accepted as a known limitation; `renderTemplate` gains no escape syntax, and message templates keep the same one.
- **Client-side expression syntax checking.** The engine checks it at save; compiling expr in the browser is not worth it.
- **Deleting `LegacyEval`.** It stays until no config in flight carries an `eval` key. A one-line follow-up.
