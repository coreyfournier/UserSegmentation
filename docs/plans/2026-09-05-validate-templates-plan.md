# Validate Template Tokens Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Catch a `${…}` token that references a field the schema does not declare, at save, instead of silently emitting nothing at evaluation — and make a dotted field name work in a template at all.

**Two problems, both measured.**

A token naming an unbound identifier renders as nothing, with no error: `"Hello ${nam}"` → `"Hello "`, zero bad tokens. Nothing at load or runtime says a word.

A token naming a **flat dotted key** fails outright. The engine's context is a flat map — `ResolveField` tries `ctx[field]` first, which is why a *condition* on `company.payFrequency` works. A *template* hands the same string to expr, which reads it as member access on an unbound `company`. Shipped config already hits this at `config/segments.json:572`:

```
condition matched?  failures=1
rendered message:   "Pay frequency ${company.payFrequency} is not supported."
warning:            cannot fetch payFrequency from <nil>
```

The rule fires correctly and the user is shown the raw token.

**Why validation never caught either:** templates are not validated at all. Message templates (`errorMessage`, `messages`, `defaultMessages`) are never examined by the validation package. String output fields are explicitly skipped — `if !f.IsTemplate()` guards the only syntax check there is. Expressions get a bare `expr.Compile`, which accepts unknown identifiers because it has no env to check them against.

**Architecture.** Load and runtime resolve a token the same way, in the same order, so they cannot disagree:

| | Runtime | Load |
| --- | --- | --- |
| 1 | `ResolveField(ctx, token)` — flat key, then nested walk | is the token a field the effective schema declares? |
| 2 | otherwise compile and run as expr | otherwise compile with `expr.Env(schemaEnv)` **and the runtime's own options** |

Step 2's options must be the runtime's, exported rather than duplicated — an Env-constrained compile rejects `pow(2, 3)` unless `mathOptions` is passed, and a validator that rejects working config is worse than the gap it closes.

**Tech Stack:** Go 1.26, `github.com/expr-lang/expr` v1.17.8, standard library testing.

## Global Constraints

- **Load and runtime must agree.** Any rule added at load must accept everything the runtime accepts. A false rejection blocks working config and is worse than the silent gap being closed. Where the two could drift, share the code rather than restating it.
- **`mathOptions` is exported, not copied.** Validation compiles with the exact option set `compileFormula` uses. Two lists that must match are a defect waiting to happen.
- **A declared field path wins before expr is consulted**, at both ends. That is what makes `${company.payFrequency}` legal, and dotted names are the convention throughout this config — `company.ein`, `employee.hireDate`. Forbidding them in templates is not an acceptable outcome.
- **The new load check is an error, not a warning.** A token naming an undeclared field always renders wrong; there is no case where it is intended.
- **Do not touch the escape hatch.** A layer with no `inputSchema` skips rule-field validation, and several shipped layers rely on it. With no declared fields there is nothing to check a token against, so **skip token validation for that layer too** rather than rejecting every token in it.
- **Message templates are in scope.** `errorMessage`, per-language `messages`, and `defaultMessages` have never been validated. They are the same `${…}` syntax against the same context, so they get the same check.
- **No new guarding beyond this.** Do not add lookup membership, order uniqueness, or contiguity checks.
- Go: `go build ./...`, `go vet ./...` and `go test ./...` must pass before every commit. Go is on `PATH` (go1.26.5); no `export PATH` needed.
- **`config/segments.json` holds uncommitted work that is not yours** — the UI writes through to it. Do not `git checkout` it, do not `git add` it, do not point anything at port 8080. Use a high port and a copy.

---

### Task 1: Resolve a declared field before consulting expr

**Files:**
- Modify: `internal/domain/strategy/message.go`
- Modify: `internal/domain/strategy/formula.go` (export the options)
- Test: `internal/domain/strategy/template_field_test.go` (create)

**Interfaces:**
- Produces: `strategy.ExprOptions() []expr.Option` returning the same slice `compileFormula` uses. Task 2 consumes it.
- Changes: a `${…}` token resolves through `model.ResolveField` before falling back to expr.

- [ ] **Step 1: Write the failing test**

Create `internal/domain/strategy/template_field_test.go` covering `renderTemplate` against an env holding a flat dotted key, a nested map, and a plain key:

1. `"EIN ${company.ein}"` with `{"company.ein": "12-3456789"}` renders `EIN 12-3456789` and reports **no** bad token. **This is the shipped-config bug and the reason for the task.**
2. `"EIN ${nested.ein}"` with `{"nested": {"ein": "99-9"}}` still renders — the nested walk must keep working.
3. `"Hello ${name}"` with `{"name": "Ada"}` still renders.
4. `"Total ${totalHours * 2}"` with `{"totalHours": 3}` still renders `6` — a compound expression must fall through to expr, because no field is named `totalHours * 2`.
5. `"Bad ${amount *}"` still reports a bad token — syntax errors must still fail.
6. A field whose declared value is literally `nil` renders empty without a bad token, as today.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/domain/strategy/ -run TestRenderTemplate_DeclaredField -v`
Expected: FAIL on case 1 — the token is returned unrendered with one bad token.

- [ ] **Step 3: Resolve the field first**

In `message.go`, where the token is currently evaluated:

```go
		val, err := evalMessageExpr(exprStr, env)
```

becomes:

```go
		// A declared field wins before expr is consulted. The context is a flat
		// map whose keys may contain dots — "company.ein" is one key, not member
		// access — and ResolveField is what conditions already use to read it.
		// Handing the same string straight to expr instead reads it as member
		// access on an unbound "company" and fails, which is why a condition on
		// a dotted field works today while a template on the same field does not.
		val, err := resolveTokenValue(exprStr, env)
```

with:

```go
func resolveTokenValue(exprStr string, env map[string]interface{}) (interface{}, error) {
	if v, ok := model.ResolveField(env, exprStr); ok {
		return v, nil
	}
	return evalMessageExpr(exprStr, env)
}
```

- [ ] **Step 4: Export the compile options**

In `formula.go`, add beside `mathOptions`:

```go
// ExprOptions returns the option set every expression in this engine compiles
// with. Validation compiles with these too: an env-constrained compile rejects
// pow(2, 3) without them, so a validator using a different set would reject
// config that runs perfectly well.
func ExprOptions() []expr.Option { return mathOptions }
```

- [ ] **Step 5: Run the suite and commit**

Run: `go build ./... && go vet ./... && go test ./...` — all green. The shipped config's message at `config/segments.json:572` now renders; if any existing test asserted the broken output, that assertion was pinning the bug — fix it and say so in your report.

```bash
git add internal/domain/strategy/
git commit -m "fix: resolve a declared field before treating a token as an expression"
```

---

### Task 2: Validate every token at load

**Files:**
- Modify: `internal/domain/validation/validator.go`
- Test: `internal/domain/validation/template_test.go` (create)

**Interfaces:**
- Consumes: `strategy.ExprOptions()` (Task 1), `model.ResolveField` semantics, the effective schema `ValidateSnapshot` already builds.
- Produces: nothing exported.

**Where the tokens are.** Every one of these is `${…}` against the same context and none is validated today:
- each rule's `errorMessage`, at every depth of the rule tree
- each rule's `messages` values, per language
- the segment's `defaultMessages` values
- each **string** output field's authored value — segment-level, per rule, and per override

- [ ] **Step 1: Write the failing test**

Create `internal/domain/validation/template_test.go` covering:

1. `"Hello ${nam}"` where the schema declares `name` — **error**, naming the rule and the unknown token. The silent typo, caught.
2. `"Pay frequency ${company.payFrequency}"` where the schema declares `company.payFrequency` — **valid**. The dotted case must pass.
3. `"${totalHours * 2}"` where `totalHours` is declared — **valid**. Compound expressions still work.
4. `"${pow(totalHours, 2)}"` where `totalHours` is declared — **valid**. This is the false-rejection trap: without the runtime's options it errors.
5. `"${amount *}"` — **error**, syntax.
6. A token referencing a **computed** field — **valid**. The effective schema includes them.
7. The same checks applied to an output field's value, and to a `messages` entry.
8. A layer with **no inputSchema** — every token accepted, because there is nothing to check against and the escape hatch must not turn into a wall.
9. A token inside a **nested** And/Or rule's `errorMessage` is still checked — the walk must recurse.

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/domain/validation/ -run TestValidate_Template -v`
Expected: FAIL — no token validation exists, so 1, 5 and 9 pass silently.

- [ ] **Step 3: Implement**

Add to `validator.go`:

```go
// envFromSchema builds a compile-time environment from the effective schema so
// expr can report an unknown identifier. Values are zero values of the declared
// type; only the names and shapes matter here.
//
// TODO: an array or object field carries no declared element or member shape —
// SchemaField is {Type, Required} and nothing more — so a token that reaches
// inside one cannot be checked past its top-level name. ${employees} validates,
// ${employees[0].name} and ${payload.nested} do not, and both are accepted
// unchecked rather than falsely rejected. Closing this needs a nested schema
// type, which is a larger change than this validation.
func envFromSchema(schema model.InputSchema) map[string]interface{} { … }

// validateTemplateTokens reports any ${…} token that names something the schema
// does not declare, or that does not compile.
//
// A declared field wins first, exactly as it does at evaluation: the context is
// a flat map whose keys may contain dots, so "company.ein" is one key rather
// than member access, and rejecting it here would forbid the naming convention
// this config uses throughout.
//
// Compilation uses strategy.ExprOptions() — the runtime's own set. An
// env-constrained compile rejects pow(2, 3) without them, and a validator that
// rejects working config is worse than the gap it closes.
func validateTemplateTokens(where, tmpl string, schema model.InputSchema, env map[string]interface{}) []string { … }
```

Reuse the token-scanning shape from `renderTemplate` — same `${` to `}` rule, including its unterminated-token behaviour, so the two cannot disagree about what a token even is. Extract that scan into one place if it can be done without a cycle; otherwise mirror it and put a comment on both pointing at each other.

Call it from `ValidateSnapshot`'s per-segment work, for every site listed above, and **only when `layer.InputSchema != nil`**. Walk the rule tree recursively so a nested `errorMessage` is covered.

- [ ] **Step 4: Upgrade the expression check to use the env too**

`validateOutputExpressionSyntax` currently compiles with a bare `expr.Compile`, which accepts unknown identifiers. Give it `expr.Env(envFromSchema(...))` plus `strategy.ExprOptions()`, so a typo in a non-string output value is caught at load rather than resolving to nil at runtime. Keep its skip of disabled rules.

Add a test: a `number` output field authored `MaxAllowd` where the schema declares `MaxAllowed` is **rejected at load**, naming the field.

- [ ] **Step 5: Check the shipped config still validates**

Run: `go test ./internal/infrastructure/config/ -v`

The shipped config now has every message token checked for the first time. If it fails, **do not weaken the check and do not edit `config/segments.json`** — report exactly which token in which rule, and stop. A genuine pre-existing bug surfacing here is a result, not an obstacle.

- [ ] **Step 6: Run the suite and commit**

```bash
go build ./... && go vet ./... && go test ./...
git add internal/domain/validation/
git commit -m "feat: validate template tokens against the declared schema"
```

---

### Task 3: Documentation

**Files:**
- Modify: `README.md`
- Modify: `docs/plans/2026-09-02-output-schema-context.md`

- [ ] **Step 1: Document the rule**

In `README.md`'s output-schema and messages sections, state that a `${…}` token is resolved as a declared field first and as an expression otherwise, that both are checked at load against the schema, and that an unknown name is rejected with the rule and token named.

- [ ] **Step 2: Record the array-of-objects limit as a known gap**

In the context document's *Still open*, record it in the same voice as its siblings: `SchemaField` is `{Type, Required}`, so an `array` or `object` field declares no element or member shape; a token reaching inside one is accepted unchecked; closing it needs a nested schema type. Note that the failure mode is the safe direction — unchecked, not falsely rejected.

- [ ] **Step 3: Commit**

```bash
git add README.md docs/plans/
git commit -m "docs: template tokens are validated; record the nested-shape gap"
```

---

## Out of scope for this plan

- **A client-side version of the check.** The browser cannot compile expr, and a partial reimplementation would drift from the engine. The save reports it, as it does for expression syntax.
- **A nested schema type.** Recorded as the known gap above.
- **Changing what a nil-valued token renders.** A token that resolves to a declared field holding `nil` still renders empty. With load-time validation catching undeclared names, the remaining nil cases are deliberate.
- **An escape for a literal `${` in a template.** Still unavailable, still recorded.
