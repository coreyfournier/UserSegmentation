# Layer Dependencies and Failure Collection — Design

**Date:** 2026-08-22
**Status:** Implemented

## Summary

Two additions to the layer model, both general-purpose:

1. **`dependsOn`** — an explicit dependency edge between layers. It replaces
   `order` as the ordering mechanism and adds *gating*: a layer whose dependency
   did not resolve is **skipped** rather than silently evaluating against absent
   context.
2. **`assert`** — a new strategy type composing the existing
   `ExpressionStrategy` → `RuleStrategy` chain. It inherits expr-lang computed
   fields, walks the whole rule tree instead of short-circuiting so every
   failing rule is reported, and emits an explicit
   `satisfied` / `violated` / `unevaluable` status.

Together these express **progressive validation gates** without a gate engine,
a gate model type, or an entity-schema subsystem. A "gate" is a layer of
`assert` segments with a `dependsOn` edge to the gate before it.

## Motivating problem

A company entity has a product type (Express, Time and Attendance, Precision).
Each type requires a different set of configuration values, and misconfiguration
causes downstream process failures or a degraded employee experience. The same
applies per-employee.

Validation must run as **progressive gates**: each gate itemizes *all* of its
problems at once so the person resolving them does not make repeated round
trips, and later gates may assume earlier gates passed — so they neither
re-check the same values nor need defensive guards for values an earlier gate
already established.

Entity types are expressed as **segments within a layer** (first-match-wins is
already type dispatch), each carrying its own `inputSchema` and rules. No new
entity or schema model is required.

## Decisions

| Decision | Choice |
|---|---|
| Dependency declaration | **Explicit** `dependsOn: []string` on every layer, not inferred from `layer:` references |
| Scope of `dependsOn` | **All layers**, not only validation layers |
| Ordering | `dependsOn` defines execution order via topological sort |
| `order` field | **Removed.** One-time config migration declares the real edges; no permanent compatibility shim |
| Gating semantics | Dependency unresolved ⇒ dependent layer is **skipped**, not evaluated against absent context |
| `layer:<name>.status` in context | **Not included.** Skip propagation covers the real cases. Purely additive if ever needed |
| Assert | A strategy **type** — `strategy: "assert"`, composing `ExpressionStrategy` → `RuleStrategy` |
| Failure collection | Implied by the `assert` strategy. No user-facing flag; signalled internally on `EvalContext` |
| Assert layer status | Explicit `satisfied` / `violated` / `unevaluable`. Consumers never inspect array length |
| Non-assert layer status | `resolved` / `unresolved` / `skipped` |
| Expression runtime failure | Marks the layer **unevaluable** — never a silent false that reads as a violation |
| Or-node failures | A failing `Or` emits **one** failure for the Or node, not one per branch |
| Failure record | `{ rule, message }` only |
| Issue identifier | `ruleName` — descriptive and stable. No separate `code` field |
| Field path in output | **Not included.** Conditional presence would be unpredictable and unexplainable to consumers |
| Entity model | An entity **is a schema** over the evaluation context. No entity type, no levels |
| Nested entities | Context may nest (an employee with its parent company). Requires dotted-path resolution — see below |
| Type dispatch | `when` predicate on Segment. Unselected segments produce **no output** — not a reported state |
| "Not applicable" | **Not a state.** If a check doesn't apply to a type, it isn't in that type's rule set |
| `ruleName` uniqueness | Enforced by the **ConfigSource** (storage concern), not domain validation |
| Cross-layer ref validation | Rule referencing `layer:x` requires `x` in `dependsOn` — config error otherwise |
| Declared-but-unreferenced dep | **Legal, no warning.** Ordering/gating is a valid reason to declare an edge |

## Status

`assert` is a strategy type, so it owns assertion vocabulary directly. The
engine emits the status explicitly — a consumer must **never** infer meaning
from `failures.length`.

| Strategy | Status values |
|---|---|
| `assert` | `satisfied` · `violated` · `unevaluable` |
| all others | `resolved` · `unresolved` · `skipped` |

How an assert layer's status is determined:

| Condition | Status |
|---|---|
| Layer skipped by DAG gating | `unevaluable` |
| Runtime expression failure | `unevaluable` |
| Evaluated, no failures | `satisfied` |
| Evaluated, one or more failures | `violated` |

`failures` is still present and authoritative for *what* is wrong; `status` is
authoritative for *whether* anything is. They are always consistent, and the
status is computed once by the engine rather than re-derived by every consumer.

Rollup: **ready = no `violated` and no `unevaluable`.** An unevaluable layer
blocks readiness because its outcome is unknown, not because it failed.

## Data model (`internal/domain/model`)

```go
type Layer struct {
    Name            string    `json:"name"`
    DependsOn       []string  `json:"dependsOn,omitempty"`
    Segments        []Segment `json:"segments"`
    DefaultLanguage string    `json:"defaultLanguage,omitempty"`
}

type Segment struct {
    // ...existing fields...
    When *Rule `json:"when,omitempty"` // dispatch predicate; unselected ⇒ no output
}

type Failure struct {
    Rule    string            `json:"rule"`
    Message string            `json:"message"`
    Messages map[string]string `json:"messages,omitempty"` // localized, existing mechanism
}
```

`LayerResult` in [internal/domain/engine/evaluator.go](../../internal/domain/engine/evaluator.go)
gains `Status` and `Failures` alongside the existing `Assignment` and `Warnings`.

`EvalContext` ([strategy.go:6-15](../../internal/domain/strategy/strategy.go#L6-L15))
gains an internal `CollectFailures bool`, set by `AssertStrategy` when it
delegates downward. It is **not** a config field — selecting `strategy: "assert"`
is the only way to turn collection on.

## Execution (`internal/domain/engine`)

`Evaluate` currently sorts by `Order`
([evaluator.go:44-48](../../internal/domain/engine/evaluator.go#L44-L48)).
Replace with:

1. **Topological sort** over `dependsOn`. Independent layers have no ordering
   requirement — output is a map and context injection is name-keyed, so there
   are no cross-layer side effects that execution order could affect.
2. **Skip propagation** — if any dependency is `skipped`, or produced no
   assignment, the dependent layer is `skipped` and is not evaluated.
3. **Context injection** unchanged for values (`layer:<name>`).

Filtered evaluation improves as a side effect: today every layer is evaluated
regardless of the output filter
([evaluator.go:77-87](../../internal/domain/engine/evaluator.go#L77-L87))
because the engine cannot know which layers the filtered layer needs. With
declared edges it evaluates the requested layers plus their transitive
dependencies only.

## Execution (`internal/domain/strategy`)

`evaluateRule` short-circuits by design
([rule.go:56-76](../../internal/domain/strategy/rule.go#L56-L76)). Add a collect
mode — do **not** change default behavior, since walking all children of a
50-rule segmentation layer that could stop at rule 3 is real cost on the hot
path.

Collect-mode semantics:

- **And** — evaluate every enabled child; collect a failure for each that fails.
- **Or** — if any child succeeds, no failure. If all fail, emit **one** failure
  for the Or node itself, using its `errorMessage` / `Messages`.

  Rationale: emitting one failure per branch tells the resolver to set three
  fields when any one of them would do.
- **Leaf** — failure carries the rule's `ruleName` and rendered message.

### The `assert` strategy

`assert` is a third link in an existing delegation chain. `ExpressionStrategy`
already enriches the context with expr-lang computed fields and delegates to
`RuleStrategy`
([expression.go:106](../../internal/domain/strategy/expression.go#L106)).
`AssertStrategy` composes `ExpressionStrategy` the same way:

```
AssertStrategy  →  ExpressionStrategy  →  RuleStrategy
   sets                enriches with          collects failures
   CollectFailures     computed fields        instead of short-circuiting
   computes Status
```

So assert inherits computed fields for free, and the whole-tree walk lives in
one place — `RuleStrategy` — rather than being duplicated per strategy.

Registration: add `"assert"` to the strategy map in the composition root
(`cmd/segmentation/main.go`).

Three details:

**1. Plumbing.** `CollectFailures` travels on `EvalContext`, which is constructed
per-segment at
[evaluator.go:118-125](../../internal/domain/engine/evaluator.go#L118-L125) and
rebuilt by `ExpressionStrategy` — see the defect below.

**2. Fix the context rebuild first — pre-existing defect.**
[expression.go:99-105](../../internal/domain/strategy/expression.go#L99-L105)
rebuilds `EvalContext` field by field and **omits `Lookups`**, which the
evaluator had populated. Any rule using `in_lookup` / `not_in_lookup` inside an
`expression`-strategy segment therefore receives a nil table map and silently
evaluates false, while passing config validation — `validateLookupRef` checks
the snapshot's tables, not the runtime context.

This is independent of the present design, but the same manual-copy pattern
would silently swallow a new `CollectFailures` field. Replace the rebuild with a
struct copy so it is immune to future field additions:

```go
derived := *ctx
derived.Context = enriched
```

**3. Runtime expression failure ⇒ unevaluable.**
[expression.go:87-96](../../internal/domain/strategy/expression.go#L87-L96)
silently `continue`s when a computation fails. Under segmentation that is benign
— the computed field is simply absent. Under collection it is not: the dependent
rule evaluates false and is reported as a **violation**, telling the resolver a
value is wrong when it could not in fact be computed.

In collect mode, a runtime expression error marks the layer unevaluable. Compile
errors are already caught at load
([validator.go:23-27](../../internal/domain/validation/validator.go#L23-L27)),
so only runtime failure applies.

Granularity note: this marks the whole layer unevaluable even when only one of
several expressions failed, because there is no rule-to-expression dependency
tracking. This is the same coarseness tradeoff as layer-level skipping, resolved
the same way — split into another layer when finer resolution is needed.

Computed values need no special handling in output: `applyMessages` renders
against the enriched context, so a failure message can interpolate `${MaxAllowed}`
today, and `Result.Expressions` already carries the computed values.

Messages reuse the existing localization path
([rule.go:34-43](../../internal/domain/strategy/rule.go#L34-L43)) — `errorMessage`
and `Messages` already exist on `Rule`
([model/rule.go:17-23](../../internal/domain/model/rule.go#L17-L23)).

## Entities and context shape

An entity is **a schema over the evaluation context** — nothing more. There is
no entity type in the model and no notion of levels. The caller sends whatever
document the rules need:

```jsonc
{ "company": { … } }                          // company on its own
{ "employee": { … } }                          // employee on its own
{ "employee": { … }, "company": { … } }        // employee with its parent
```

Rules reference `company.payFrequency` and `employee.hireDate` in the same
context. "Company checks" and "employee checks" are just layers whose rules
happen to read different subtrees — no special machinery, no new request shape.

### Prerequisite: dotted-path resolution

The two halves of the engine currently disagree about what a dotted field means:

- `EvalExpression`
  ([operators.go:13](../../internal/domain/strategy/operators.go#L13)) does a
  **flat** map lookup — `company.ein` is a literal key containing a dot.
- `ExpressionStrategy` passes the context to expr-lang as an env, which resolves
  nested paths **natively**.

Nested entities require reconciling this. Three small changes:

1. `EvalExpression` — attempt the flat key first (preserving any existing
   key-with-dot), then traverse the path segment by segment. Flat-first keeps
   current behavior intact; the present config uses undotted names
   (`EarnedWages`, `DaysWorked`), so collision risk is nil.
2. `CheckRequiredFields`
   ([validator.go:127](../../internal/domain/validation/validator.go#L127))
   uses the same flat lookup and needs the same resolution.
3. `inputSchema` keys become paths. No model change — `validateRuleTree`
   already compares field strings against schema keys.

A missing intermediate segment resolves to absent, which is the existing
"field not in context" path — a required-field violation, not an error.

## Validation (`internal/domain/validation`)

Additions to `ValidateSnapshot`:

- **Compile expressions for `assert` segments.**
  [validator.go:23](../../internal/domain/validation/validator.go#L23) currently
  gates expression compilation on `seg.Strategy == "expression"`. Since `assert`
  also carries `Expressions`, it must be included or assert segments lose
  compile-time syntax checking — pushing what should be a load-time error into a
  runtime failure that reports as `unevaluable`.
- **Cycle detection** over `dependsOn`.
- **Dependency existence** — every name in `dependsOn` names a real layer.
- **Cross-layer reference check** — a rule referencing `layer:x` requires `x` in
  the layer's `dependsOn`. This closes an existing hole:
  [validator.go:73-76](../../internal/domain/validation/validator.go#L73-L76)
  accepts *any* `layer:` prefixed field unconditionally, so a typo or a
  backwards reference passes config validation and then evaluates false forever.

Not a domain concern:

- **`ruleName` uniqueness** belongs to the `ConfigSource` implementation
  ([internal/infrastructure/config/file_source.go](../../internal/infrastructure/config/file_source.go)).
  It is a property of the persisted collection, not of a rule's meaning. A
  future database-backed source gets it from a unique index rather than
  reimplementing a domain check.

## Config format

```json
{
  "layers": [
    {
      "name": "company-identity",
      "segments": [
        {
          "id": "all-types",
          "strategy": "assert",
          "inputSchema": { "company.ein": { "type": "string", "required": true } },
          "rules": [
            {
              "ruleName": "companyHasFederalEIN",
              "expression": { "field": "company.ein", "operator": "neq", "value": "" },
              "errorMessage": "Federal EIN is required before payroll can be configured."
            }
          ]
        }
      ]
    },
    {
      "name": "company-payroll-setup",
      "dependsOn": ["company-identity"],
      "segments": [
        {
          "id": "precision",
          "when": {
            "ruleName": "isPrecision",
            "expression": { "field": "company.productType", "operator": "eq", "value": "Precision" }
          },
          "strategy": "assert",
          "inputSchema": {
            "company.payFrequency": { "type": "string", "required": true },
            "company.anchorDate":   { "type": "string", "required": true }
          },
          "rules": [
            {
              "ruleName": "payFrequencySupportedForPrecision",
              "expression": { "field": "company.payFrequency", "operator": "in_lookup", "value": "precision-frequencies" },
              "errorMessage": "Pay frequency ${company.payFrequency} is not supported for Precision."
            },
            {
              "ruleName": "payPeriodDatesFormValidSequence",
              "operator": "And",
              "errorMessage": "Pay period end must be after start, and check date must follow both.",
              "rules": [ "…" ]
            }
          ]
        },
        {
          "id": "express",
          "when": {
            "ruleName": "isExpress",
            "expression": { "field": "company.productType", "operator": "eq", "value": "Express" }
          },
          "strategy": "assert",
          "rules": [ "…Express-only checks; pay-group rules simply absent…" ]
        }
      ]
    }
  ]
}
```

`company-payroll-setup` needs no null guards on `company.ein` — if identity
failed, this layer never runs.

## Supersedes Layer Categories (design removed)

`2026-06-24-layer-categories-design.md` was marked *Validated, ready for
implementation planning* but never implemented — no `category` exists anywhere
in `internal/`. It has been **deleted** as part of this design; the rationale is
recorded here, and the document remains in git history.

It did four things; `dependsOn` subsumes the two that touched execution.

### Subsumed — reference isolation (its Section 2, rule 3)

That rule targets the same hole this design closes, and says so: *"Replaces the
current no-op where `layer:` references are accepted unconditionally
(validator.go:68-70)."* The checks are not equivalent:

| | Existence | Declared | Ordering | Cycles | Thematic fence |
|---|---|---|---|---|---|
| Categories | ✅ | — | — | — | ✅ |
| `dependsOn` | ✅ | ✅ | ✅ | ✅ | — |

Categories' Section 2 explicitly defers the last two: *"Self-references and layer
`order` cycles are pre-existing concerns, out of scope."*

The only unique contribution is the thematic fence, which is **policy, not
correctness** — and possibly wrong policy here: a compliance gate that
legitimately needs a balance layer's result becomes unbuildable, and the
workaround (merge the categories) dissolves the fence. It is also currently
vacuous, as that design notes — every existing layer normalizes into `default`,
so the constraint has never bound anything.

### Subsumed — evaluation scoping (its Section 3)

Section 3 rests its safety entirely on Section 2: *"Safe because of Section 2: a
layer references only same-category layers, so a scoped run never needs an
out-of-category `layer:` result."*

The isolation rule exists to make category-scoped execution safe. With a declared
DAG that job is done by **transitive dependency closure** — scope to any
selection, then evaluate what it needs. No constraint required, and nothing
forbidden.

### Not subsumed

Display grouping (badges, group-by view, filter dropdown) and the stable
`apiName` registry that lets a display name be renamed without breaking
automation. `dependsOn` does nothing for either.

### Recommendation

Categories are dropped. Cross-layer reference validation belongs to `dependsOn`.
If layer count later makes browsing painful, reintroduce `category` as an
**optional label with zero validation or scoping behavior** — a small fraction
of the removed design's UI section, landable at any time without touching the
engine.

## Migration — removing `order`

`order` is deleted from the model. `dependsOn` is the only ordering mechanism.

**A permanent `order`-to-chain compatibility shim was considered and rejected.**
Converting a sequential `order` into a dependency chain is not semantically
faithful — it manufactures edges that do not exist. The current
`config/segments.json` has eleven layers and exactly **three** cross-layer
references, all to `base-tier`:

| Layer | Real dependency |
|---|---|
| `experiments`, `promotions`, `features` | `base-tier` |
| `base-tier`, `CT Rule`, `Risk Rating`, `dSDsd`, `ct-fee-override`, `ewa-eligibility`, `ewa-risk`, `transfer-fee` | none |

A synthesized chain would produce seven-plus fabricated edges. Under skip
propagation that is actively harmful: an unresolved `experiments` would skip
`transfer-fee`, which has no relationship to it. And the shim buys nothing
either way — if synthesized edges gate, they cause spurious skips; if they do
not gate, they are `order` under a different name.

`order` is also a weaker guarantee than it appears. Four layers share order 1
and two share order 5, and
[evaluator.go:46](../../internal/domain/engine/evaluator.go#L46) uses
`sort.Slice`, which is **not stable** — the relative sequence of equal-order
layers can already permute between runs. This is harmless today only because no
equal-order layers reference each other.

### Migration steps

1. Add `"dependsOn": ["base-tier"]` to `experiments`, `promotions`, `features`.
2. Delete `order` from all eleven layers.
3. Bump `Snapshot.Version`.

A one-time load-migrate-save through the existing
[ConfigSink](../../internal/domain/ports/config_sink.go) can perform this by
deriving edges from actual `layer:` references — not from `order` — and gating
on `Version`. Unlike the rejected shim, this runs once and produces the true
graph.

Any config still carrying `order` after migration should **fail** load rather
than be silently reinterpreted, so a stale config is loud instead of subtly
misordered.

## API (`internal/application` + `internal/infrastructure/http`)

`/v1/evaluate` and `/v1/evaluate/batch` response gains per-layer `status` and
`failures`:

```json
{
  "layers": {
    "company-payroll-setup": {
      "status": "violated",
      "failures": [
        { "rule": "payPeriodDatesFormValidSequence",
          "message": "Pay period end must be after start, and check date must follow both." }
      ]
    },
    "company-tax-setup": { "status": "unevaluable" }
  }
}
```

`status` is authoritative — a consumer never needs to inspect `failures.length`.

The readiness rollup across layers (`ready = no violated and no unevaluable`) is
**not** computed by the service.

## UI (`ui/`)

The UI renders `status` directly, groups `failures` by layer, and maps `rule`
names to remediation destinations. That last mapping is UI knowledge, not config
knowledge — an admin console and a mobile app deep-link differently, which is
why no field path is emitted.

The layer editor needs `assert` added to the strategy selector, and the
`dependsOn` picker should offer other layers by name.

## Testing

- Topological sort: diamond dependency, deterministic output, cycle rejection.
- Skip propagation: transitive skipping; a skipped layer injects no context.
- `assert` collection: And collects all failing children; Or emits one failure
  for the node; nested And-under-Or.
- `rule` and `expression` strategies still short-circuit — regression guard on
  the segmentation hot path.
- `assert` inherits computed fields: values interpolate into failure messages
  and `Result.Expressions` is populated.
- Status is explicit and consistent with `failures` in all four cases
  (skipped, expression failure, zero failures, some failures).
- `in_lookup` inside an `expression` or `assert` segment resolves — regression
  guard for the dropped-`Lookups` defect.
- Runtime expression failure yields `unevaluable`, not violations from the rules
  that consumed the missing computed field.
- An `assert` segment with a bad expression fails config load rather than
  reaching runtime.
- `when` dispatch: unselected segments produce no output of any kind.
- Path resolution: nested lookup; flat-key precedence over path traversal;
  missing intermediate segment behaves as absent; existing undotted fields
  unaffected.
- Migration: the current `config/segments.json` migrates to exactly three
  `dependsOn` edges, and post-migration evaluation matches today's results for
  every layer.
- A config still carrying `order` fails load.
- Validation: undeclared `layer:` reference errors; declared-but-unreferenced
  dependency passes clean.
- `file_source`: duplicate `ruleName` rejected at load.

## Out of scope

- Entity / schema subsystem (`EntityKind`, `EntityType`, discriminator model) —
  segments with `when` plus existing `inputSchema` cover type dispatch.
- A `Gate` model type or separate gate engine.
- Stable issue `code` field separate from `ruleName`.
- Field paths in failure output.
- Parallel execution of independent DAG branches. Evaluation is in-memory and
  the batch path already parallelizes across subjects.

## Open questions

None outstanding.

### Resolved

- **Status vocabulary.** Earlier drafts had the engine emit neutral
  `evaluated`/`skipped` with consumers deriving the assertion reading, on the
  grounds that assertion framing was UI-only. Making `assert` a strategy type
  removes that objection — the type legitimately owns its vocabulary, and an
  explicit status is better than requiring consumers to infer meaning from
  `failures.length`. See *Status*.

- **Two-level entities.** An entity is a schema over a possibly-nested context,
  so company-and-employee is one document, not two levels. No batch memoization
  or new request shape. See *Entities and context shape*.
- **Layer categories.** Not needed for execution. Reference isolation and
  category scoping are both subsumed by `dependsOn` plus dependency closure; the
  residual value is display grouping, deferrable as a semantics-free label. See
  *Supersedes the execution half of Layer Categories*.
- **`layer:<name>.status` in context.** Skipped. A layer either declares a
  dependency and is gated by it, or does not reference it at all. Injecting
  status would only serve a layer that wants to run *despite* an upstream
  failure and behave differently — not a case the readiness model needs. The
  addition is purely additive to context, so it can be introduced later without
  breaking anything.
