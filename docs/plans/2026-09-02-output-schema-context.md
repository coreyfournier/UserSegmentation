# Output Schema — session context and handoff

Read this first if you are picking up the output-schema work cold. It records why
the feature exists, what was decided and why, and what was deliberately left out.
The design detail is in the sibling documents:

| Document | Holds |
| --- | --- |
| `2026-09-02-output-schema-todo.md` | the design, the ordering model, open questions |
| `2026-09-02-output-schema-engine-plan.md` | the TDD implementation plan, 8 tasks |
| `2026-09-04-balance-diagnostics-port.md` | how the source service becomes layers, segments and rules |
| `2026-09-04-output-schema-ui-plan.md` | the UI plan, 6 tasks |
| `2026-09-04-layer-schemas-plan.md` | moving both schemas onto the layer, 6 tasks |
| `2026-09-04-output-field-picker-plan.md` | choosing a rule's output field from a dropdown, 2 tasks |
| `2026-09-05-derive-eval-mode-plan.md` | dropping the eval mode, deriving it from the type, 4 tasks |
| `.superpowers/sdd/balance-diagnostics-survey.md` | what the source service actually does, surveyed |
| `README.md` § Production readiness | why this is a POC and what production needs |

## Where this came from

The question was not "segmentation needs an output schema." It was: **can this
engine do the diagnostic work that a different service does in hand-written C#?**

That service is `balance-diagnostics` (`C:\repos\balance-diagnostics`). It answers
"why can this employee not take a wage advance?" by running *diagnosers* over data
it fetches, each producing zero or more `Diagnosis` records. A `Diagnosis` carries:
a type key, severity, category, title, description, a user-facing message, a
technical explanation, a resolution (type + detail), and a list of name/value
evidence pairs called *signals*.

**Corrected counts, and the count is the part people keep getting wrong.** Earlier
drafts of this document said "seven diagnosers" and "52 possible diagnoses". A
survey of the source (`.superpowers/sdd/balance-diagnostics-survey.md`) found
**three** registered `IDiagnoser` implementations, one of which fans out into five
internal static sub-diagnosers — so **eight distinct rule groups**.

The diagnosis count has been wrong three times: 52 originally, 46 in the survey,
49 from a naive grep. It is **50**. `DiagnosisType` has fifty members, and its last
one carries no trailing comma, which is exactly what makes a line-counting regex
report 49. Verified two ways — counting members directly, and summing each group's
owned types against its diagnoser's source (13+4+11+12+9+1 = 50, with none left
over). If you need the number, count it again rather than trusting any prose,
including this sentence.

The shape of the reasoning below does not depend on the number, but do not trust
"seven", "52", "46" or "49" if you see them anywhere.

The engine turned out to be a close structural match. `ChecklistStrategy` exists to
"run a list of checks and report every one that fires," which is what a diagnoser
does, and `FailureDTO` is the same idea as an itemised finding. What it could not
do was **emit a record rich enough to be one** — `FailureDTO` has a rule name and a
message, and that is all.

Hence the output schema. The feature is general, but the shape it has to be able to
express is that `Diagnosis`.

## The decisions, and why

**Data acquisition is out of scope, by design.** The engine has no data sources —
`internal/infrastructure/` is config, hash, http and store. It evaluates a
caller-supplied context map. `balance-diagnostics` spends most of its code fetching
in parallel from three upstreams and deriving values before any rule runs. That
stays the caller's job. The engine should not grow data coupling.

**Exclusive findings use a computed winner field inside one checklist.** The
conclusion is the valuable part and it holds — no `supersedes` mechanism, no
`match: first` mode, no engine change. But an earlier draft of this document had
the *mechanism* wrong, and the wrong version is easy to re-derive from first
principles, so it is recorded here rather than deleted.

That draft claimed exclusive groups become a `rule` segment sitting beside the
`checklist` segment holding the independent ones, "so both coexist in one
dimension." **They cannot coexist.** `evaluateLayer` returns on the first segment
that produces a result (`evaluator.go:203`), and a checklist always produces one —
`collectViolations` "always succeeds; a checklist has no notion of 'no rule
matched'" (`rule.go:93`). Two checklist segments in one layer therefore report only
the first, and the second is dead config that nothing in `internal/domain/validation/`
flags. A `rule` segment beside a `checklist` is worse than useless: they become
alternatives, the checklist running only when no rule matched.

Exclusivity belongs *inside* the checklist, as a computed field naming the winner:

```json
"computed": [{ "name": "payrollBlocker", "type": "string",
               "formula": "anchorDate == nil ? \"noAnchor\" : (payFrequency == nil ? \"noFrequency\" : \"none\")" }]
```

Each alternative is then an ordinary item conditioning on `payrollBlocker eq
"noAnchor"`, so at most one can fire, and each keeps its own `ruleName`,
`errorMessage` and `outputs`. Independent items sit alongside, unaffected.

Nested And/Or cannot do this job, which is why the mechanism has to be a computed
field: a group reports **once**, under its own name and message
(`TestChecklist_AndGroupIsOneItem`), so it cannot emit a distinct record per
alternative.

**A segment's computed fields are a shared failure domain.** This is what limits
how many diagnoses one segment should hold, and it is the reason the port is
several layers rather than one. `enrichWithComputed` evaluates every formula before
any rule runs, and under collection a single failure voids the entire segment
(`rule.go:33-42`):

```
status="unevaluable"  reason="formula error: advanceRatio"  failures=[]
```

Two unrelated diagnoses in that segment reported nothing, because one formula
referenced a value an upstream fetch did not return. The behaviour is deliberate —
a rule consuming a field that could not be computed must not fire and be read as a
real problem — but it means every formula added to the scratchpad widens the blast
radius of the first absent value. All 50 diagnoses in one segment would be one
empty fetch away from returning nothing at all.

So the eight rule groups want to be eight layers. `LayerResultDTO` reports
`unevaluable` per layer, so one group going dark leaves the others
reporting — which is the partial-failure behaviour three upstreams demand. The
shipped config is already shaped this way: `company-identity`,
`company-payroll-setup`, `employee-readiness`, `CompanyValidation` and `T&A Gates`
are five separate checklist layers.

**Overrides are a segmentation feature, not a diagnostic one.** A checklist may not
declare them — rejected at load. `EvalOverrides` resolves a segment value and the
evaluator reports `resolved`, both outside the checklist vocabulary
`LayerResultDTO` promises, and a checklist has no bucket to force anything into.
The segment editor already refuses to offer overrides for a checklist and says so
in prose, so this only closes the raw-JSON and admin-API paths that bypass the UI.
On a `rule` segment overrides remain legitimate and now author output values like
any other reporting path — their *conditions* still match raw input only, which is
long-standing deliberate behaviour. See *Overrides* in the todo document.

**Values are authored on the top-level rule.** Not on tree leaves.
`TestChecklist_AndGroupIsOneItem` settles it: an And/Or group reports **once**,
under its own `RuleName` and `ErrorMessage`, and inner branches are unnamed by
default. So the output editor belongs beside `ruleName`, and everything below it in
the tree is condition-only. Getting this wrong would put fields on branches that
can never report.

**Lookup `Key` is the stable contract; `Value` is free to change.** This is what
makes moving an enum into config safe. A consumer references only the specific keys
it cares about and treats the rest as data, so there is no exhaustive mapping to
drift. Renaming display text needs no deploy. Key immutability is convention, not
enforcement — see below.

**Ordering lives on the lookup entry, persisted even when inferred.** Array
position is not enough: a relational store cannot reorder rows cheaply. Two
independent flags on the table — emit order, custom order. Hand-authored numbers
let one ordering span several tables (severity takes 1, 3, 5; type takes 2, 4, 6),
which a declared `orderBy: [a, b]` cannot express at all, because lexicographic
composition can only put all of one dimension ahead of the other.

**No guarding, with one deliberate exception.** Nothing validates lookup
membership at evaluation time, order uniqueness, or range disjointness. This is a
deliberate trade for flexibility: the tool accepts the risk and the table
`Description` is where an author records the invariant they are holding. A
collision produces a tie that a stable sort resolves by encounter order — silent,
not an error. Do not add checks "helpfully"; the plan's global constraints say so
because it is easy to undo this decision by accident.

The exception is **`OutputField.Required`**, which was added later and on purpose.
The original constraint read "do not require any output field to be populated";
that is now narrowed to the three items above. The reasoning is that the other
guards protect an author from *their own* invariants, which they can hold
themselves, whereas an unpopulated required output field breaks a **caller** who
has no way to see the omission. Without it an author can silently drop a field the
consumer depends on. Nothing else about the no-guarding trade changed — see
*Required outputs* in the todo document for the exact semantics.

**Schemas stay at segment level, because that is where their values live.** The
candidate change was moving `outputSchema` — and `inputSchema` with it — up to the
layer. The reason to reject it is structural rather than arithmetical: everything
that populates an output schema is segment-scoped. `Segment.Outputs` holds the
constants tier, `Rule.Outputs` holds the per-item values on rules the segment owns,
and `Segment.Computed` is the scratchpad every template token and expression
resolves against. A declaration one level above the values that fill it, and above
the scratchpad those values read, buys nothing and splits one unit across two
levels.

The counting argument that first settled this is true but weak — 14 of 16 layers
hold exactly one segment, and the only `when`-variant pair carrying schemas
(`company-payroll-setup`) has two *distinct* ones — so it would stop being an
argument the moment a layer grew a second segment. The coupling argument does not.

Two candidate justifications for moving the declaration up were examined and
**both are false.** They are recorded because both are plausible enough to be
re-derived:

- *"A consumer cannot tell which `when` variant fired, so the emitted shape must
  not be allowed to vary by variant."* It can tell. `collectViolations` sets
  `Reason: "checklist:" + seg.ID` (`rule.go:96`) and nothing blanks it, so the
  response carries `"reason": "checklist:precision"` or `"checklist:express"`.
  Only `Assignment.Segment` is cleared for checklists (`checklist.go:46`), and
  `Reason` survives into `LayerResultDTO`.
- *"Every rule group emits one shared `Diagnosis` shape, so declare it once
  at snapshot level and reference it by id, the way lookups already work."* Note
  first that this is an argument against layer level too — the duplication would be
  *across* layers, which neither placement reaches. But it assumes a duplication no
  written config exhibits. If the port lands and the schemas really are identical,
  a snapshot-level `outputSchemaRef` is purely additive to segment-level
  declaration and nothing here forecloses it. Until then it is indirection bought
  on speculation.

`inputSchema` stays put for two reasons of its own. The symmetry argument — "if
`outputSchema` moves, `inputSchema` should follow" — is a false lead in both
directions:

1. `CheckRequiredFields` runs *after* the `when` dispatch (`evaluator.go:145-150`),
   so required-ness is already variant-scoped. Hoisting it would make every Express
   company warn on four Precision-only fields that are correctly absent — not a
   loss of precision but guaranteed false warnings against config shipped today.
2. `buildEffectiveSchema` merges per-segment `Computed` into the type environment
   rule validation runs against (`validator.go:80-89`). A layer-level `inputSchema`
   would only ever be half of it, so validation would still assemble the rest per
   segment.

## Still open

- **Key immutability.** Editing an entry key silently breaks every consumer
  reference and every stored record pointing at it, and config validation cannot
  catch it because the new key is still a valid member of its own table.
  Convention: rename the `Value`, add a new key rather than editing an old one.
  A `deprecated` flag would let the editor hide retired keys while keeping them
  resolvable for historical display.
- ~~**Whether output expressions can be syntax-checked.**~~ **Resolved during
  implementation: yes.** The worry was that validation calls `expr.Compile` with no
  options while runtime compilation uses `mathOptions`, so a validator might reject
  an expression that works. Task 5's probe disproved it — bare `expr.Compile`
  accepts `pow(2, 3)` even though `pow` is registered only via `mathOptions`,
  because expr-lang defers unresolved calls to runtime when given no typed
  environment. So the syntax check was added. (Since superseded: the eval mode is
  no longer declared but derived from the field's type, so the syntax check now
  covers every non-string field.) One thing fell out of review and is worth
  keeping in mind if this check is ever extended: it must skip disabled rules, or
  a parked half-written expression wedges every other segment's save.
- **An unreachable second checklist segment is silent.** Because a checklist always
  succeeds, any checklist segment after the first one a layer reaches is dead
  config, and nothing rejects it. There is precedent for catching this class at
  load — `validator.go:31` rejects an unknown strategy precisely because "the
  segment would just never produce anything" — and it is a different class from the
  deliberate no-guarding above, which concerns lookup membership and ordering
  rather than structural reachability. Any check must stay checklist-specific:
  `transfer-fee`'s two `rule` segments are a legitimate fallthrough chain, since
  `RuleStrategy` returns false when nothing matches and no default is set.
  Deliberately out of scope for the engine plan; decide it separately.
- **The UI.** A separate plan against `ui/`: the output schema editor, per-item
  value editors, lookup order authoring, the two table flags, and disabling
  drag-and-drop once custom numbers are allowed. The engine plan makes the fields
  exist and be exercisable through `POST /v1/evaluate` and the admin lookup
  endpoints; nothing authors them yet except raw JSON.

## What the consumer side would look like

Not in scope for this repo, recorded so the shape of the target is not lost.
`balance-diagnostics` would map an emitted record into its own `Diagnosis`:

| Emitted | Becomes |
| --- | --- |
| `outputs.diagnosisType.key` | the type key; `.order` feeds ranking |
| `outputs.severity` / `.category` | severity and category |
| `outputs.title` | `Title` — currently a computed property there, would need a setter with the derived value as fallback |
| `outputs.description` / `userMessage` / `technicalExplanation` | the prose fields |
| `outputs.resolution.type` / `.detail` | the resolution; needs no complex type, dotted keys suffice |
| `outputs.signals` | the evidence map, projected to name/value pairs |

Two things that service does today would simply retire: its ranking options
dictionary, since order arrives on the record, and its catalog endpoint plus the
synthetic-input provider behind it, since the config *is* the catalog and
`GET /v1/segments` already serves it.

## Before you run the plan

**Go needs no PATH setup, and `CLAUDE.md` is wrong about this.** It claims Go is at
`C:\Users\Corey\go`; that directory does not exist. Go 1.26.5 is at
`/c/Program Files/Go/bin/go` and already on `PATH` — `go build ./...` and
`go test ./...` run as-is, and the full suite passes on `main`. The stale note below
is kept because it explains why the plan's steps were never executed.

Go is at `C:\Users\Corey\go` per `CLAUDE.md` — `export PATH="/c/Users/Corey/go/bin:$PATH"`
first. The plan's verification steps were never executed in the session that wrote
it, because that session's shell could not reach Go. Treat every "Expected: PASS"
as unverified until you run it.

Two execution paths, both from the superpowers skills: subagent-driven (a fresh
agent per task, review between) or inline (batch with checkpoints).
