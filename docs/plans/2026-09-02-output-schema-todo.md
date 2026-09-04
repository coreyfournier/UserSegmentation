# TODO — output schema: where is it declared?

**Status:** placement **resolved — segment level**. Nothing built yet; the engine
plan stands as written and needs no change from this decision.

## What the output schema is for

Letting a checklist or rule segment emit a structured record rather than just a
failure name and a message — so a consumer receives a populated object instead of
mapping one by hand. The driving case is reproducing the `Diagnosis` shape from
balance-diagnostics: a stable type key, severity, category, title, description, a
user-facing message, and a set of evidence values.

Working shape, per declared field: a name, a data type, and an evaluation mode.

| Mode | Value is | Result |
| --- | --- | --- |
| `literal` | a constant | as declared |
| `template` | text with `${…}` tokens | string |
| `expression` | one whole expr | typed |

Fields whose domain is enumerated bind to a lookup table, so the authoring UI shows
a dropdown. The validator checks at snapshot load that the referenced table exists
and that the field's declared type matches its `keyType` — it does **not** check
that an emitted value is one of the table's keys; membership stays the author's
invariant, same as everywhere else lookups are used. The lookup entry's `Key` is
the stable identifier a consumer may reference in code; `Value` is the
human-readable label, free to change without a deploy.

Values are authored on the **top-level rule** — the checklist item, or the winning
rule in a `rule` segment — which is where `ruleName` and `errorMessage` already
live. Inner branches of an And/Or never report and get no output editor. Fields
that do not vary per item are set once at segment level, the way `Default` and
`DefaultMessages` already work.

## Required outputs

A declared field may be marked `Required`, mirroring `SchemaField.Required` so the
declaration reads the same on both schemas. Its purpose is the caller's contract:
without it an author can omit a value the consumer depends on, and nothing says so.

**It is checked at two points, because they answer different questions.**

| | Snapshot load / save | Evaluation |
| --- | --- | --- |
| Asks | did the author supply a value? | did the caller receive one? |
| On failure | **error** — config rejected | **warning** — evaluation continues |
| Input `Required` equivalent | none — config cannot know the caller's context | the same warning |

So an output `Required` is a superset of an input `Required`, not a homonym: both
warn at evaluation, and the output one additionally errors at load.

**The load-time gate.** A required field is satisfied for a segment if
`Segment.Outputs` supplies it — one value covers every path, which is what that
tier is for. Failing that, every **enabled** top-level rule must supply it in its
own `Outputs`; disabled rules are exempt so a work-in-progress item cannot wedge an
unrelated save. A segment declaring a `Default` can *only* be satisfied by
`Segment.Outputs`, because the default branch resolves outputs with no rule values
to read.

Because the check lives in `ValidateSnapshot`, it needs no new plumbing to mean
"you cannot save the layer" — every admin write and every hot-reload already passes
through there. It also means an unauthored field in one segment blocks saves to
unrelated segments, which existing validation already does for a bad formula but
which this check will trigger far more often. Hence `Required` defaults to
**false**: declaring a field is free and non-blocking, and promoting it is the
deliberate act. That is what keeps the fast authoring flow workable — a field can
be added to the schema mid-edit without invalidating its 51 siblings.

**The evaluation warning.** Config validity cannot guarantee runtime presence.
Three paths leave a required field absent, and only the third is preventable at
load:

| Path | Cause | Resolution |
| --- | --- | --- |
| expression or template fails | deliberate degradation — record the error, drop the field, keep the item | **the warning is the answer** |
| a `rule` segment falls to `Default` | no rule values to read | prevented by the load gate |
| an override wins on a `rule` segment | `EvalOverrides` resolved no schema | fixed — overrides now author outputs |
| an override on a `checklist` | the combination is incoherent | rejected at load |

So only the first remains, and for it the warning *is* the design: expression
failure must degrade, so a required field can always go missing at runtime and the
caller has to be told. The warning names the field and, for a finding, the rule that
did not emit it.

## Overrides

Two decisions, taken together, retire what was previously an open gap.

**A checklist cannot declare overrides.** This is not an output-schema rule — it is a
pre-existing invariant that was documented and enforced in the UI but never at the
engine boundary. `EvalOverrides` resolves a segment value and the evaluator reports
`resolved`; both are outside the checklist vocabulary of
`satisfied`/`violated`/`unevaluable` that `LayerResultDTO` promises a consumer, and
`ChecklistStrategy` blanks `res.Segment` precisely because "a checklist resolves no
segment value." Measured: an override on a checklist layer yields
`status="resolved" segment="bypassed" failures=0`, a case a consumer switching on
status was told could not occur.

The segment editor already declines to offer overrides for a checklist — the
Overrides section is gated on `strategy !== 'rule' && strategy !== 'checklist'`, and
the checklist branch tells the author in prose that "there is no default and no
overrides." So rejecting it at load takes away nothing an author could reach; it
closes the raw-JSON, admin-API and hand-edited-config paths that bypass the UI. No
shipped config is affected — the single override in `config/segments.json` is on a
`percentage` segment.

**A `rule` segment's overrides author outputs.** This is the one path that was
genuinely reachable, since the rule strategy's config section *does* offer an
overrides editor. `Overrides` is `[]Rule`, the same type as `Rules`
(`segment.go:34`), so once `Rule.Outputs` exists every override rule already carries
the field — it was simply never read. The fix is to give `EvalOverrides` the segment
and resolve outputs in its match branch, exactly as the first-match branch does.

Two contexts must stay distinct there. An override's **condition** matches raw input
only: the editor is handed `seg.inputSchema` and states "only raw input fields are
available." That is existing deliberate behaviour and does not change. Its **output
values** resolve against the computed-enriched context, because a `${derivedValue}`
token is otherwise unusable. A test pins the asymmetry so it cannot be erased by
accident.

**Enriching there is safe, contrary to an earlier note in this document.** It does
not import the shared-failure-domain problem, because an override never runs in
collect mode — `ChecklistStrategy` sets `CollectFailures` on a *copy* of the context
inside the strategy, after `evaluateLayer` has already checked overrides. Outside
collect mode a failed formula is quiet, the field simply absent, per `rule.go:33-42`.
Measured: `non-collect: ok=true segment="matched" computed=map[good:2]` with the
failing field absent, against `collect: status="unevaluable"`.

Message rendering keeps the raw context. Enriching it too would be more consistent
but would change the rendered output of existing config, so it is left alone and
recorded as a known asymmetry.

**This narrows the no-guarding trade, on purpose.** The original constraint read
"do not require any output field to be populated." The other guards it covers —
lookup membership, order uniqueness, range disjointness — protect an author from
invariants they can hold themselves, and stay unguarded. An unpopulated required
output field is different in kind: it breaks a *caller*, who has no way to see the
omission.

## What the author declares, and what they do not

The response already carries a fixed set of fields without any declaration:
`status`, `segment`, `strategy`, `reason`, `computed` and `messages` per layer, and
`subject_key`, `warnings`, `evaluated_at` and `duration_us` on the envelope. The
output schema editor shows these as a read-only *always emitted* reference, so an
author declares only what is missing rather than restating the baseline.

These are a reference display, **not** values addressable inside output
expressions. Most of them cannot be: `evaluateOutputs` runs inside the strategy,
while `status` is derived by the evaluator afterwards and `layers` is the response
envelope. Only `segment` and `strategy` are knowable at that point, and exposing an
arbitrary subset would be worse than exposing none.

Alongside it, the segment's declared **input schema** is shown as a second
read-only reference with a link jumping to the Input Schema section, because that
section sits well above the rules in the segment editor and an author authoring
output expressions needs to see what fields exist.

## Resolved: segment level

Both schemas stay on `Segment`. `Segment.OutputSchema` as the engine plan already
declares it, and `Segment.InputSchema` unchanged.

**The reason is that a schema belongs with the values that fill it.** Everything
populating an output schema is segment-scoped: `Segment.Outputs` holds the
constants tier, `Rule.Outputs` holds per-item values on rules the segment owns, and
`Segment.Computed` is the scratchpad every `template` token and `expression`
resolves against. Declaring the shape one level above the values that fill it, and
above the scratchpad those values read, buys nothing and splits one unit across two
levels. This argument does not weaken as the config grows.

**The argument for layer level does not survive contact with the engine.** Its
premise was that a layer holds several `when`-dispatched variants emitting one
shape, with `LayerResultDTO` as the natural home. But a layer resolves to *exactly
one* segment — `evaluateLayer` returns on the first segment producing a result
(`evaluator.go:203`) — so there is never more than one schema in play per layer at
evaluation time. Layer-level and segment-level are isomorphic *in the response*;
the choice is purely about authoring and validation scope, where the coupling above
decides it.

Two further justifications were examined and **both are false.** Recorded so they
are not re-derived:

- *"A consumer cannot tell which variant fired, so the shape must not vary by
  variant."* It can. `collectViolations` sets `Reason: "checklist:" + seg.ID`
  (`rule.go:96`), which survives into `LayerResultDTO.Reason` — the response reads
  `"reason": "checklist:precision"`. Only `Assignment.Segment` is blanked for
  checklists (`checklist.go:46`).
- *"Seven diagnosers share one `Diagnosis` shape, so name it at snapshot level and
  reference it by id, as lookups are."* This is an argument against layer level
  too: the duplication would sit *across* layers, which neither placement reaches.
  And it presumes a duplication no written config exhibits. A snapshot-level
  `outputSchemaRef` stays purely additive if the port ever proves the need.

**The symmetry argument is dead in both directions** — `inputSchema` would not
follow `outputSchema` even if it moved, for two reasons of its own:

1. `CheckRequiredFields` runs *after* the `when` dispatch (`evaluator.go:145-150`),
   so required-ness is already variant-scoped. Hoisting it would make every Express
   company warn on four Precision-only fields that are correctly absent — not lost
   precision but guaranteed false warnings against config shipped today.
2. `buildEffectiveSchema` merges per-segment `Computed` into the type environment
   rule validation uses (`validator.go:80-89`), so a layer-level `inputSchema`
   would only ever be half of it.

## Decisions already made

- **Signals** are a per-item `expression`-mode field returning a map, projected by
  the consumer into name/value pairs. `Segment.Computed` stays the shared
  scratchpad; the item selects what to expose.
- **The scratchpad is a shared failure domain**, which caps how many items one
  segment should hold. Under collection a single failed formula returns
  `unevaluable` with *no* failures for the whole segment (`rule.go:33-42`), so every
  formula added widens the blast radius of the first absent upstream value. Prefer
  one layer per group of related findings over one segment holding all of them; see
  the context document for the measurement.
- **Title** is a declared output field, not derived from the lookup. Consumers that
  compute it today keep their derived value as a fallback when the field is absent.
- **Resolution** needs no complex type — it flattens to two scalar fields using
  dotted names, as `inputSchema` keys already do.
- **Ordering** is persisted on the lookup entry and controlled by two independent
  flags on the table. See *Ordering* below.
- **A `Description` on the lookup table** carries the author's note about how the
  table is meant to be used — including any cross-table ordering scheme, which is
  otherwise invisible. This is the documentation-instead-of-validation trade the
  tool is deliberately making.

## Ordering

`LookupEntry` carries a persisted `Order`. Array position is not sufficient: a
relational store cannot reorder rows cheaply, so implicit ordering would force
either an order column anyway or a delete-and-rewrite on every move. The number is
written even when it was inferred.

Two independent booleans on `LookupTable`:

| Flag | Controls |
| --- | --- |
| emit order | whether `order` appears in the evaluation response |
| custom order | numbers are hand-authored, rather than inferred from list position |

Both are meaningful in all four combinations — a table may be ordered for admin
display without emitting, and inferred positions can be emitted without being
hand-authored. Keeping them separate avoids forcing an author to hand-number a
table merely to get the number into the output.

**Drag-and-drop applies to inferred mode only.** Once custom numbers are allowed,
the list is authored by number and reordering by drag is disabled — two editing
models rather than a hybrid that fights itself. Entering custom mode seeds the
numbers from the current inferred order, so the author starts from where the list
already sits and the first save changes nothing. Leaving custom mode discards the
authored numbers in favour of list position.

### Interleaving across tables

Hand-authored numbers let one ordering span several lookups. Severity takes 1, 3, 5
and diagnosis type takes 2, 4, 6; each table reads in order on its own, and a single
sort over the union interleaves them correctly. The number space itself is the
composition, so no precedence needs declaring anywhere.

This is strictly more expressive than a declared `orderBy: [severity, type]`, which
can only place all of one dimension ahead of the other and cannot interleave at all.

**The risk is accepted, not guarded.** Nothing keeps the ranges disjoint, and nothing
checks uniqueness — within a table or across a group. A collision produces a tie, and
a stable sort then falls back to encounter order, so ordering becomes nondeterministic
without erroring. The author holds this invariant, and the table `Description` is
where they record it for whoever edits next. Contiguity must not be checked either:
gaps are the mechanism, not a defect.

Consumers must not persist an emitted order or compare it across snapshots. Only the
relative order carries meaning.
## Also unresolved

- Lookup entry keys are the stable contract a consumer may reference in code, so
  renaming one silently breaks those references. Consistent with the ordering trade
  above, this is convention rather than enforcement: rename the `Value`, add a new
  key rather than editing an old one, and record retired keys in the table
  `Description`. A `deprecated` flag would let the editor hide them from dropdowns
  while keeping them resolvable for historical display.
- `FieldType` has no object/map member, which `expression`-mode fields need.
- A failed output value degrades — record the error, drop the field, keep the
  finding — rather than failing the evaluation. When the dropped field was
  `Required`, the evaluation warning above reports it, so degradation stays silent
  only for optional fields.

  **Not "the way message tokens do", which an earlier draft said and which is
  wrong.** `renderTemplate` degrades *per token*: it writes the literal `${…}` back
  into the string and reports the error, so the caller still gets a value. That is
  right for a human-readable sentence, where half a rendered message beats none. It
  is wrong for a structured field, for two reasons: a consumer receiving
  `"1.5 hours over ${ daysElapsed } days"` cannot detect the failure without
  string-scanning, and — decisively — the required-field warning tests *presence*,
  so a half-rendered field would be present and the caller would never be told
  anything failed. Output values are therefore all-or-nothing in every eval mode,
  `template` included. This cost a review cycle during implementation, because the
  plan's own example code followed the misleading phrase rather than the constraint.
- Whether an override's *messages* should also render against the enriched context,
  the way its output values now do. Left as is because it would change the rendered
  output of existing config. See *Overrides* above.
