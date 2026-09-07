# Porting `balance-diagnostics` to the rules engine

This is the port itself — the previous documents justified *why* the engine grew
an output schema; this one shows *how* the eight rule groups in
`balance-diagnostics` actually become `config/segments.json`. Read
`.superpowers/sdd/balance-diagnostics-survey.md` first if you have not; this
document assumes its findings and corrects two of them where reading the C#
directly (`C:\repos\balance-diagnostics`) turned up more precision than the
survey had room for.

**Shape of the result.** Seven layers, seven segments — one per rule group,
with `BalanceServiceDiagnoser`'s five-step waterfall expressed as `dependsOn`
edges between four of those layers rather than becoming a segment of its own.
`EffectiveLimitDiagnoser`'s internal switch-plus-additive-check stays one
segment (a checklist with a computed winner field), so the count is eight rule
groups → seven segments, not eight — the waterfall collapses into ordering,
it does not add a segment.

**A correction before the table.** Both the survey (§3) and the context
document state the enum has 46 values. It does not — `DiagnosisType.cs`, read
directly, has **50** members. Summing every group's owned types against its
diagnoser's actual source confirms 50, not 46: ValidationCodeDiagnoser 13,
ResultCodeDiagnoser 4, EffectiveLimitDiagnoser 11 (10 switch branches plus
`AccessibleWageZeroDespiteDays`, which the switch's `AccessibleWage` branch
can also reach), CompanyHealthDiagnoser 12, EmployeeAccountDiagnoser 9, and
`BalanceCalculationError` shared by `ProcessingErrorDiagnoser` and
`MessageFallbackDiagnoser` (one type, two producers) = 1. `13+4+11+12+9+1 =
50`, with no leftover — every one of the 50 is reachable through exactly one
of the seven segments below. Treat "46" as wrong wherever it appears upstream
of this document.

| # | Rule group | Layer | Segment strategy | Distinct `DiagnosisType`s reachable |
|---|---|---|---|---|
| 1 | ProcessingErrorDiagnoser | `balance-processing-error` | `rule` | 1 |
| 2 | ValidationCodeDiagnoser | `balance-validation-codes` | `checklist` | 13 |
| 3 | ResultCodeDiagnoser | `balance-result-code` | `rule` | 4 |
| 4 | MessageFallbackDiagnoser | `balance-message-fallback` | `checklist` (does not fit cleanly — see §1.3) | 0 new (shares `BalanceCalculationError` with row 1) |
| 5 | EffectiveLimitDiagnoser | `balance-effective-limit` | `checklist` + computed winner | 11 (at most 2 fire in any one evaluation — see §1.4) |
| 6 | CompanyHealthDiagnoser | `company-health` | `checklist` | 12 |
| 7 | EmployeeAccountDiagnoser | `employee-account` | `checklist` | 9 |

50 of 50. Every `DiagnosisType` value is reachable through exactly one
segment; nothing is left over and nothing is double-counted except the one
type two producers deliberately share.

---

## 1. The mapping table, and why

### 1.1 Genuine enum switches → `rule` strategy

Two groups really are "one field, one switch, exactly one branch fires,"
which is what `RuleStrategy`'s first-match-wins evaluation already is with no
extra mechanism needed:

- **`ResultCodeDiagnoser`** — `switch (balance.ResultCode)`, 4 branches
  (`NoPayDataAvailable`, `NoDaysWorkedYet`, `OutsideAvailableBalanceWindow`,
  `NetPayIsZero`) plus an implicit "no diagnosis" default. Ordinary `rules`
  list, one condition per branch on a single field, `default: "none"`.
- **The primary half of `EffectiveLimitDiagnoser`** — `switch
  (balance.EffectiveLimit)`, 9 branches (10 counting the internal
  `PostTransferBalanceCap` branch that maps to `null` — "internal fail-safe,
  not a user-facing diagnosis," confirmed in
  `EffectiveLimitDiagnoser.cs:65`). This one is *not* alone in its group,
  though — see §1.4.

### 1.2 Independent, all-fire groups → `checklist` strategy

Three groups are unconditional `if`s with no early return, several of which
can co-occur in one response — exactly the shape `ChecklistStrategy` exists
for ("run every rule, report every one that fires"):

- **`ValidationCodeDiagnoser`** — 13 independent `list.Contains(code)` checks
  against `balance.ValidationFailures[*].ValidationCodes` flattened to one
  list.
- **`CompanyHealthDiagnoser`** — 10 checks against `CompanyHealth` fields plus
  2 more (`NoTimesheetsCompanyWide`, `TimeClockIssue`) gated only on
  `CompanyCalcMethod`, independent of whether a `CompanyHealth` record exists
  at all.
- **`EmployeeAccountDiagnoser`** — 9 independent checks across `Employee`,
  `CalculatedBalance`, `TimesheetMetadata`, and mapping-status fields.

### 1.3 The waterfall: better than it first reads, with one real gap

`BalanceServiceDiagnoser.Diagnose()` (read directly, not just the survey's
description of it) is:

```csharp
// Priority 1: short-circuit
var processingErrors = ProcessingErrorDiagnoser.Diagnose(context.Balance);
if (processingErrors.Count > 0) return processingErrors;

// Priority 2 and 3: both unconditional once priority 1 has not fired
var diagnoses = ValidationCodeDiagnoser.Diagnose(context.Balance);
diagnoses.AddRange(ResultCodeDiagnoser.Diagnose(context.Balance, context));

// Priority 4: gated on 2 and 3 having produced nothing
if (diagnoses.Count == 0 && context.Balance.ResultCode is not (BalanceAvailable or NoBalanceAvailable))
    diagnoses.AddRange(MessageFallbackDiagnoser.Diagnose(context.Balance));

// Priority 5: unconditional once priority 1 has not fired
diagnoses.AddRange(EffectiveLimitDiagnoser.Diagnose(context.Balance));
```

That is **not** a five-deep chain. Priorities 2, 3 and 5 each depend on
priority 1 alone — they do not depend on each other, and nothing stops them
running in parallel once priority 1 is clear. Only priority 4 has a real
cross-group gate. So the honest picture is:

- **Priority 1 → everything downstream.** A clean fit. `balance-processing-error`
  is a `rule` segment resolving to `"processing-error"` or `"none"`; every
  other balance-* layer declares `dependsOn: ["balance-processing-error"]` and
  gates its segment on `when: { field: "layer:balance-processing-error",
  operator: "eq", value: "none" }`. This is the same mechanism already in
  production for `experiments`/`promotions`/`features` gating on
  `layer:base-tier` (`config/segments.json`).
- **Priority 5's own entry guard** (`ResultCode is BalanceAvailable or
  NoBalanceAvailable`, `EffectiveLimitDiagnoser.cs:15`) is a plain condition on
  a raw field, not a cross-layer dependency — no gap here either.
- **Priority 4 is the one that does not fit.** Its gate needs to know whether
  the *checklist* layer (`balance-validation-codes`) produced any failures.
  But `ChecklistStrategy.Evaluate` deliberately blanks `res.Segment` —
  "a checklist resolves no segment value" (`checklist.go:46`) — precisely
  because a checklist's satisfied/violated/unevaluable vocabulary is not the
  same thing as a resolved segment value. Nothing is injected into
  `"layer:balance-validation-codes"` for a downstream layer to read, ever,
  by design. A `rule`-strategy layer's outcome is readable across
  `dependsOn`; a `checklist`-strategy layer's outcome is not. That is not a
  bug to route around — it is the same invariant the context document already
  established for the checklist/override boundary — but it does mean
  priority 4's gate cannot be expressed as a layer dependency at all.

  The only way to express it inside `segments.json` is to duplicate the
  predicate: recompute "no validation code matched" directly against the raw
  `ValidationCodes` list inside `balance-message-fallback`'s own `computed`
  block (an OR across the same 13 codes, copy-pasted from the checklist
  segment's own conditions) and combine it with `layer:balance-result-code eq
  "none"` (which *is* readable, since that layer is `rule`-strategy). This
  works, but it is real duplication with a real drift risk — if a fourteenth
  validation code is added to the checklist, this guard silently stops
  matching it.

  Given that `MessageFallbackDiagnoser` is noted in the survey as "currently
  dead code in practice — the comment says Balance Service never actually
  puts errors in `Message`," the honest recommendation is: **do not port
  it in the first several slices.** If it is ported later, accept the
  duplicated guard, and put a comment on both copies pointing at each other.
  There is no clean mechanism here — say so rather than inventing one.

### 1.4 `EffectiveLimitDiagnoser`: the exclusive-plus-additive case

This is the group the context document's computed-winner-field technique was
written for, and it needs it for a reason distinct from `ResultCodeDiagnoser`:
`EffectiveLimitDiagnoser` is a switch **plus** an independent, separately-
gated check that can co-fire alongside it:

```csharp
var limitDiagnosis = CheckEffectiveLimit(balance);       // exclusive switch
if (limitDiagnosis is not null) { ...; diagnoses.Add(limitDiagnosis); }
diagnoses.AddIfNotNull(CheckAccessibleWageZeroDespiteDays(balance));  // additive
```

A plain `rule` segment cannot hold the additive check — first-match-wins
resolves to *one* segment value, full stop. Two checklist segments in one
layer cannot both report — only the first one a layer reaches ever runs (this
document's context file already measured that: "a checklist always
succeeds... the second is dead config"). So the switch and the additive check
have to live in the **same** checklist segment, and the switch's mutual
exclusivity has to be recreated with a computed field naming the winner,
exactly as the context document describes. §2 below is the worked config for
this.

One more layer of exclusivity sits inside the switch itself:
`EffectiveLimit.AccessibleWage` does not map to one diagnosis — it branches
again on whether `EmployerAdvanceAllowedPercentage > 0`, landing on either
`EmployerPercentageLimit` or a second, differently-worded
`AccessibleWageZeroDespiteDays`. Same story for `EffectiveLimit.PreviousTransfers`,
which branches on `AmountTransferredInPayCycle >= AccessibleWage` to choose
between `HitPayCycleTransferAmountLimit` and `LimitedByPreviousTransfers`.
Both are just a second computed field (or a nested ternary in the same
`limitWinner` formula) — the technique nests without changing shape, so it is
worth doing right the first time rather than re-deriving it under either of
these two branches later.

### 1.5 The two survey findings worth restating here

**There is no `DiagnosisType → Category/Severity` table to move.** All ~50
`Diagnosis` construction sites in the C# hand-set `Category`, `Severity`, and
`Resolution` individually — there is no dictionary anywhere to port into a
lookup. The port authors these the same way: `category`, `severity`, and
`resolution.type` are set per rule in that rule's `outputs`, not resolved
from a table keyed by `diagnosisType`. **A lookup table is still worth it for
one thing**: the `diagnosisType` key set itself — 50 stable strings a
consumer will pattern-match on in code, plus an `order` a consumer can use in
place of the old `Dictionary<DiagnosisType,int>` ranking-priority map (see
§4). `severity`, `category`, and `resolution.type` get small lookup tables
too, mostly for load-time membership checking, not because anything needs to
map through them.

**`DiagnosisCatalogProvider` does not get ported at all.** It fabricates
synthetic `CalculatedBalance`/`CompanyHealth`/`Employee` inputs and runs every
diagnoser branch once, purely to enumerate one example of each of the 50
`DiagnosisType`s for a catalog/reference endpoint. There is no runtime logic
in it. Once the port lands, `GET /v1/segments` already serves this exact
purpose — it lists the real config, which *is* the catalog — so this class
is retired, not translated.

---

## 2. Worked config

Three pieces, matched against `internal/domain/model/{segment,rule,output,lookup}.go`
directly rather than approximated.

### 2.1 The `Diagnosis` output schema

One schema, declared per segment (the todo document settled this: schemas
stay at segment level, "because that is where their values live" —
`Segment.Outputs`, `Rule.Outputs`, and `Segment.Computed` are all
segment-scoped, so a layer- or snapshot-level declaration would sit above the
values that fill it). Repeat this block on every balance-* segment until a
real duplication shows up in written config — see the todo document's note
that a shared-schema-by-reference idea is "indirection bought on speculation"
until then.

```json
"outputSchema": {
  "diagnosisType": { "type": "string", "eval": "literal", "lookup": "diagnosisTypes", "required": true },
  "severity": { "type": "string", "eval": "literal", "lookup": "severities", "required": true },
  "category": { "type": "string", "eval": "literal", "lookup": "diagnosticCategories", "required": true },
  "title": { "type": "string", "eval": "literal", "required": true },
  "description": { "type": "string", "eval": "template" },
  "userMessage": { "type": "string", "eval": "template", "required": true },
  "technicalExplanation": { "type": "string", "eval": "template" },
  "resolution.type": { "type": "string", "eval": "literal", "lookup": "resolutionTypes", "required": true },
  "resolution.detail": { "type": "string", "eval": "template" },
  "signals": { "type": "object", "eval": "expression" }
}
```

Notes on the choices, checked against the model:

- `resolution.type` / `resolution.detail` are two scalar fields with a dotted
  name, not a nested object — the same flattening `inputSchema` already uses
  for `"company.ein"`, and exactly what the context document's consumer table
  says ("needs no complex type — dotted keys suffice").
- `signals` is `type: "object"`, which `operators.go` confirms is
  "deliberately absent from `OperatorTypes` so it can never appear in a
  condition" and is reachable only in `expression` mode —
  `validateOutputSchema` enforces that pairing at load. An authored value
  looks like `"{ValidationCode: \"PayCycleNotFound\"}"` — one expr-lang
  expression evaluating to a map, matching the decision doc's "Signals are a
  per-item expression-mode field returning a map."
- `userMessage` is the one field marked `required: true` beyond the
  identity/classification fields, because it is the one every C# call site
  populates with employee-facing text and a consumer cannot degrade
  gracefully without it. `description`/`technicalExplanation` are marked
  optional deliberately — see the todo document's reasoning: `Required`
  defaults to false and should be promoted only where an absent value would
  actually break a caller.
- `title` is `literal`, not `template` — most C# titles are static strings
  (`"Employer daily limit exhausted..."`-style titles are actually in
  `TechnicalExplanation`; `Title` itself comes from `ToTitle()`, a mostly
  mechanical PascalCase-split with 4 hardcoded overrides). Since the decision
  doc already settled that title is authored, not derived, author it as the
  plain literal text a human would read off the `DiagnosisType`.

Four lookup tables back the enum-like fields. `diagnosisTypes` shown partial
— all 50 keys get added incrementally as slices land (§6):

```json
"lookups": [
  {
    "id": "diagnosisTypes",
    "name": "Diagnosis Type",
    "keyType": "string",
    "customOrder": true,
    "emitOrder": true,
    "description": "Stable DiagnosisType key set. Order replaces the old runtime Dictionary<DiagnosisType,int> ranking-priority map — a consumer sorts by severity, then by this order, to reproduce DefaultDiagnosisRanker.",
    "entries": [
      { "key": "BalanceCalculationError", "value": "Balance Calculation Error", "order": 1 },
      { "key": "PayCycleNotFound", "value": "Pay Cycle Not Found", "order": 2 },
      { "key": "EmployeeNotFound", "value": "Employee Not Found", "order": 3 },
      { "key": "UserNotFound", "value": "User Not Found", "order": 4 },
      { "key": "CompanyNotFound", "value": "Company Not Found", "order": 5 },
      { "key": "CompanyOptedOut", "value": "Company Opted Out", "order": 6 },
      { "key": "HitDailyTransferAmountLimit", "value": "Hit Daily Transfer Amount Limit", "order": 7 },
      { "key": "HitUserDailyLimit", "value": "Hit User Daily Limit", "order": 8 },
      { "key": "HitEmployerDailyLimit", "value": "Hit Employer Daily Limit", "order": 9 },
      { "key": "HitGlobalTransferLimit", "value": "Hit Global Transfer Limit", "order": 10 },
      { "key": "AccessibleWageZeroDespiteDays", "value": "Accessible Wage Zero Despite Days", "order": 11 }
    ]
  },
  {
    "id": "severities",
    "name": "Severity",
    "keyType": "string",
    "entries": [
      { "key": "Info", "value": "Info", "order": 1 },
      { "key": "Warning", "value": "Warning", "order": 2 },
      { "key": "Critical", "value": "Critical", "order": 3 }
    ]
  },
  {
    "id": "diagnosticCategories",
    "name": "Diagnostic Category",
    "keyType": "string",
    "entries": [
      { "key": "BalanceCalculationError", "value": "Balance Calculation Error", "order": 1 },
      { "key": "BalanceAvailability", "value": "Balance Availability", "order": 2 },
      { "key": "BalanceEffectiveLimit", "value": "Balance Effective Limit", "order": 3 },
      { "key": "DataSync", "value": "Data Sync", "order": 4 },
      { "key": "MissingEntity", "value": "Missing Entity", "order": 5 },
      { "key": "ConfigurationIssue", "value": "Configuration Issue", "order": 6 },
      { "key": "EmployeeAccountStatus", "value": "Employee Account Status", "order": 7 }
    ]
  },
  {
    "id": "resolutionTypes",
    "name": "Resolution Type",
    "keyType": "string",
    "entries": [
      { "key": "SelfService", "value": "Self Service", "order": 1 },
      { "key": "ContactEmployer", "value": "Contact Employer", "order": 2 },
      { "key": "NoActionRequired", "value": "No Action Required", "order": 3 },
      { "key": "EscalateToOperations", "value": "Escalate to Operations", "order": 4 },
      { "key": "EscalateToEngineering", "value": "Escalate to Engineering", "order": 5 }
    ]
  }
]
```

`diagnosisTypes` uses `customOrder` because the order **is** the old
ranking-priority map, hand-authored on purpose, not inferred from list
position — see §4 on why the sort itself still happens outside the engine.
The other three tables are plain inferred order, included mainly for
load-time membership checking on a small closed set.

### 2.2 Independent checks: `balance-validation-codes`

`ValidationCodeDiagnoser` — 13 unconditional membership tests against one
flattened list, 5 shown; the other 8 are the same shape (condition tests
`contains` against `ValidationCodes`, `category` is `MissingEntity` for the
"could not find X" checks and `ConfigurationIssue` for the "not configured"
checks, `resolution.type` is `EscalateToOperations` for every one of the 13
except `CompanyOptedOut`, which is `NoActionRequired`):

```json
{
  "name": "balance-validation-codes",
  "dependsOn": ["balance-processing-error"],
  "segments": [
    {
      "id": "validation-code-checks",
      "when": {
        "ruleName": "noProcessingError",
        "condition": { "field": "layer:balance-processing-error", "operator": "eq", "value": "none" }
      },
      "strategy": "checklist",
      "inputSchema": {
        "ValidationCodes": { "type": "array", "required": true }
      },
      "outputSchema": {
        "diagnosisType": { "type": "string", "eval": "literal", "lookup": "diagnosisTypes", "required": true },
        "severity": { "type": "string", "eval": "literal", "lookup": "severities", "required": true },
        "category": { "type": "string", "eval": "literal", "lookup": "diagnosticCategories", "required": true },
        "title": { "type": "string", "eval": "literal", "required": true },
        "description": { "type": "string", "eval": "template" },
        "userMessage": { "type": "string", "eval": "template", "required": true },
        "technicalExplanation": { "type": "string", "eval": "template" },
        "resolution.type": { "type": "string", "eval": "literal", "lookup": "resolutionTypes", "required": true },
        "resolution.detail": { "type": "string", "eval": "template" },
        "signals": { "type": "object", "eval": "expression" }
      },
      "rules": [
        {
          "ruleName": "payCycleNotFound",
          "errorMessage": "Balance API could not find the pay cycle — pay cycle configuration issue.",
          "condition": { "field": "ValidationCodes", "operator": "contains", "value": "PayCycleNotFound" },
          "outputs": {
            "diagnosisType": "PayCycleNotFound",
            "severity": "Critical",
            "category": "MissingEntity",
            "title": "Pay Cycle Not Found",
            "description": "Balance API could not find the pay cycle — pay cycle configuration issue.",
            "userMessage": "There's a configuration issue with the pay schedule on your employer's account. I'm escalating this to our team to get it fixed. You should see your balance once it's resolved.",
            "resolution.type": "EscalateToOperations",
            "resolution.detail": "Pay cycle configuration needs correction. Escalated to operations.",
            "signals": "{ValidationCode: \"PayCycleNotFound\"}"
          }
        },
        {
          "ruleName": "employeeNotFound",
          "errorMessage": "Balance API could not find the employee record.",
          "condition": { "field": "ValidationCodes", "operator": "contains", "value": "EmployeeNotFound" },
          "outputs": {
            "diagnosisType": "EmployeeNotFound",
            "severity": "Critical",
            "category": "MissingEntity",
            "title": "Employee Not Found",
            "userMessage": "We couldn't find your employee record. Our team has been notified and is looking into it.",
            "resolution.type": "EscalateToOperations",
            "resolution.detail": "Employee record not found in system. Escalate to operations.",
            "signals": "{ValidationCode: \"EmployeeNotFound\"}"
          }
        },
        {
          "ruleName": "userNotFound",
          "condition": { "field": "ValidationCodes", "operator": "contains", "value": "UserNotFound" },
          "outputs": {
            "diagnosisType": "UserNotFound",
            "severity": "Critical",
            "category": "MissingEntity",
            "title": "User Not Found",
            "userMessage": "We couldn't find your account record. Our team has been notified and is looking into it.",
            "resolution.type": "EscalateToOperations",
            "resolution.detail": "User record not found for the registered employee. Escalate to operations.",
            "signals": "{ValidationCode: \"UserNotFound\"}"
          }
        },
        {
          "ruleName": "companyNotFound",
          "condition": { "field": "ValidationCodes", "operator": "contains", "value": "CompanyNotFound" },
          "outputs": {
            "diagnosisType": "CompanyNotFound",
            "severity": "Critical",
            "category": "MissingEntity",
            "title": "Company Not Found",
            "userMessage": "There's an issue with your employer's account configuration. Our team has been notified and is looking into it.",
            "resolution.type": "EscalateToOperations",
            "resolution.detail": "Company record not found. Escalate to operations.",
            "signals": "{ValidationCode: \"CompanyNotFound\"}"
          }
        },
        {
          "ruleName": "companyOptedOut",
          "condition": { "field": "ValidationCodes", "operator": "contains", "value": "CompanyOptedOut" },
          "outputs": {
            "diagnosisType": "CompanyOptedOut",
            "severity": "Info",
            "category": "BalanceAvailability",
            "title": "Company Opted Out",
            "userMessage": "Your employer has opted out of this service, so a balance isn't available.",
            "resolution.type": "NoActionRequired",
            "resolution.detail": "Company-level business decision. No action required unless opt-out was unintended.",
            "signals": "{ValidationCode: \"CompanyOptedOut\"}"
          }
        }
      ]
    }
  ]
}
```

### 2.3 Exclusive-plus-additive: `balance-effective-limit`

The computed-winner-field technique, extended to hold an independent check
alongside the switch, matched against the real predicates in
`EffectiveLimitDiagnoser.cs` (4 of 9 switch branches shown; the remaining 5 —
`PersonalizationWorkflow`, `RegulatoryWorkflow`, `TransferCountLimit`, and the
two nested `AccessibleWage`/`PreviousTransfers` branches from §1.4 — extend
`limitWinner`'s ternary the same way):

```json
{
  "name": "balance-effective-limit",
  "dependsOn": ["balance-processing-error"],
  "segments": [
    {
      "id": "effective-limit-checks",
      "when": {
        "ruleName": "balanceOutcome",
        "operator": "And",
        "rules": [
          { "ruleName": "noProcessingError", "condition": { "field": "layer:balance-processing-error", "operator": "eq", "value": "none" } },
          { "ruleName": "resultCodeIsBalanceOutcome", "condition": { "field": "ResultCode", "operator": "in", "value": ["BalanceAvailable", "NoBalanceAvailable"] } }
        ]
      },
      "strategy": "checklist",
      "inputSchema": {
        "EffectiveLimit": { "type": "string", "required": true },
        "AmountTransferredToday": { "type": "number", "required": false },
        "UserDailyLimit": { "type": "number", "required": false },
        "EmployerDailyLimit": { "type": "number", "required": false },
        "WorkedDays": { "type": "number", "required": false },
        "AccessibleWage": { "type": "number", "required": false }
      },
      "computed": [
        {
          "name": "limitWinner",
          "type": "string",
          "formula": "EffectiveLimit == \"DailyTransferAmountLimit\" ? \"dailyTransferAmount\" : (EffectiveLimit == \"UserDailyLimit\" && (UserDailyLimit ?? 0) > 0 && (AmountTransferredToday ?? 0) >= UserDailyLimit ? \"userDailyLimit\" : (EffectiveLimit == \"EmployerDailyLimit\" && (EmployerDailyLimit ?? 0) > 0 && (AmountTransferredToday ?? 0) >= EmployerDailyLimit ? \"employerDailyLimit\" : (EffectiveLimit == \"GlobalTransferLimit\" ? \"globalTransferLimit\" : \"none\")))"
        }
      ],
      "outputSchema": {
        "diagnosisType": { "type": "string", "eval": "literal", "lookup": "diagnosisTypes", "required": true },
        "severity": { "type": "string", "eval": "literal", "lookup": "severities", "required": true },
        "category": { "type": "string", "eval": "literal", "lookup": "diagnosticCategories", "required": true },
        "title": { "type": "string", "eval": "literal", "required": true },
        "description": { "type": "string", "eval": "template" },
        "userMessage": { "type": "string", "eval": "template", "required": true },
        "technicalExplanation": { "type": "string", "eval": "template" },
        "resolution.type": { "type": "string", "eval": "literal", "lookup": "resolutionTypes", "required": true },
        "resolution.detail": { "type": "string", "eval": "template" },
        "signals": { "type": "object", "eval": "expression" }
      },
      "rules": [
        {
          "ruleName": "hitDailyTransferAmountLimit",
          "condition": { "field": "limitWinner", "operator": "eq", "value": "dailyTransferAmount" },
          "outputs": {
            "diagnosisType": "HitDailyTransferAmountLimit",
            "severity": "Info",
            "category": "BalanceEffectiveLimit",
            "title": "Hit Daily Transfer Amount Limit",
            "userMessage": "You've transferred your daily limit today. It resets tomorrow.",
            "resolution.type": "NoActionRequired",
            "resolution.detail": "Daily transfer amount limit is the binding constraint. Resets at midnight.",
            "signals": "{EffectiveLimit: EffectiveLimit, AmountTransferredToday: AmountTransferredToday}"
          }
        },
        {
          "ruleName": "hitUserDailyLimit",
          "condition": { "field": "limitWinner", "operator": "eq", "value": "userDailyLimit" },
          "outputs": {
            "diagnosisType": "HitUserDailyLimit",
            "severity": "Info",
            "category": "BalanceEffectiveLimit",
            "title": "Hit User Daily Limit",
            "userMessage": "You've reached your personal daily transfer limit. It resets tomorrow.",
            "resolution.type": "NoActionRequired",
            "resolution.detail": "User daily limit exhausted. Resets at midnight.",
            "signals": "{EffectiveLimit: EffectiveLimit, AmountTransferredToday: AmountTransferredToday, UserDailyLimit: UserDailyLimit}"
          }
        },
        {
          "ruleName": "hitEmployerDailyLimit",
          "condition": { "field": "limitWinner", "operator": "eq", "value": "employerDailyLimit" },
          "outputs": {
            "diagnosisType": "HitEmployerDailyLimit",
            "severity": "Info",
            "category": "BalanceEffectiveLimit",
            "title": "Hit Employer Daily Limit",
            "userMessage": "You've reached your employer's daily transfer limit. It resets tomorrow.",
            "resolution.type": "NoActionRequired",
            "resolution.detail": "Employer daily limit exhausted. Resets at midnight.",
            "signals": "{EffectiveLimit: EffectiveLimit, AmountTransferredToday: AmountTransferredToday, EmployerDailyLimit: EmployerDailyLimit}"
          }
        },
        {
          "ruleName": "hitGlobalTransferLimit",
          "condition": { "field": "limitWinner", "operator": "eq", "value": "globalTransferLimit" },
          "outputs": {
            "diagnosisType": "HitGlobalTransferLimit",
            "severity": "Info",
            "category": "BalanceEffectiveLimit",
            "title": "Hit Global Transfer Limit",
            "userMessage": "Your transfer amount has been capped at the system-wide maximum for a single transfer.",
            "resolution.type": "NoActionRequired",
            "resolution.detail": "System-wide single transfer cap applied. Employee can make additional transfers.",
            "signals": "{EffectiveLimit: EffectiveLimit}"
          }
        },
        {
          "ruleName": "accessibleWageZeroDespiteDays",
          "operator": "And",
          "errorMessage": "Employee has worked days but accessible wage is zero.",
          "rules": [
            { "ruleName": "workedDaysPositive", "condition": { "field": "WorkedDays", "operator": "gt", "value": 0 } },
            { "ruleName": "accessibleWageNotPositive", "condition": { "field": "AccessibleWage", "operator": "lte", "value": 0 } },
            { "ruleName": "notAlreadyAccessibleWage", "condition": { "field": "EffectiveLimit", "operator": "neq", "value": "AccessibleWage" } }
          ],
          "outputs": {
            "diagnosisType": "AccessibleWageZeroDespiteDays",
            "severity": "Warning",
            "category": "BalanceAvailability",
            "title": "Accessible Wage Zero Despite Days",
            "userMessage": "We can see you've worked shifts, but not enough to cover your obligations. This should update soon.",
            "resolution.type": "NoActionRequired",
            "resolution.detail": "Accessible wage should update after next balance calculation cycle.",
            "signals": "{WorkedDays: WorkedDays, AccessibleWage: AccessibleWage}"
          }
        }
      ]
    }
  ]
}
```

Note this segment holds both an exclusive group (`limitWinner`'s four
branches, mutually exclusive by construction — only one value of
`limitWinner` can match) and one fully independent item
(`accessibleWageZeroDespiteDays`, which conditions on raw fields, not on
`limitWinner`, and can fire whether or not the switch also fired) — the same
mixture the context document describes as "independent items sit alongside,
unaffected."

Contrast this with `balance-result-code` (§1.1): a plain `switch` with no
additive check needs none of this — an ordinary `rule` segment with one
condition per branch and `default: "none"` reproduces it exactly, no computed
field required. The computed-winner trick earns its keep only when an
exclusive group has to share a checklist segment with something independent.

---

## 3. The context contract

The engine has no data sources; everything below is what the caller's
context-builder must assemble before calling `/v1/evaluate`, mirroring
`DiagnosticsDatasetProvider.GetDataset` but flattening what it fetches into a
plain map.

### 3.1 Many-consumer values — read straight from upstream, no derivation

Nearly the whole `CalculatedBalance` response reads directly into context
with no transformation: `ResultCode`, `EffectiveLimit`, `GrossPay`, `NetPay`,
`AmountTransferredToday`, `AmountTransferredInPayCycle`, `UserDailyLimit`,
`EmployerDailyLimit`, `EwaTransferLimitPerPayCycle`,
`EmployerAdvanceAllowedPercentage`, `WorkedDays`, `AccessibleWage`,
`ValidationCodes` (flattened from `ValidationFailures[*].ValidationCodes`
into one list, which the C# already does before its own diagnosers run —
`ValidationCodeDiagnoser.cs:14`), and a `HasProcessingErrors` boolean the
caller derives from `balance.ProcessingErrors.Count > 0`. `Employee`,
`CompanyHealth`, `TimesheetMetadata`, and `TimeclockMappingStatus` fields feed
`company-health`/`employee-account` the same way. None of this belongs in
`computed` — it is a rename, not a derivation, and putting a pass-through in
`computed` only adds one more formula that can fail and take its segment down
for no benefit.

### 3.2 One-consumer derivations — safe to compute in-segment

Pure arithmetic on already-flat fields, consumed by exactly one rule, is
where `computed` earns its keep: `dropPct` (`PaycheckDrop` only, `(1 -
currentGross/avgGross) * 100`), the `ratio` behind `LargeDeductions`, and
`limitWinner` itself (§2.3) — each is one formula, isolated to the segment
that uses it, so a failure there only ever costs that segment's own
diagnoses.

### 3.3 What must be pre-computed by the caller, never as `computed`

Three shapes are fragile enough, or require capabilities the engine flatly
does not have, that putting them in a segment's `computed` block would put
every other diagnosis in that segment at risk over one bad derivation (the
shared-failure-domain constraint — "a segment consuming a field that could
not be computed must not fire," and every unrelated formula in the same
segment shares that fate):

- **Timezone-resolved "today," and every duration built from it.**
  `OutsideAvailableBalanceWindow`'s scenario classification, `daysElapsed` for
  `TimesheetHoursAbnormallyLow`, and the staleness age behind `TimeClockIssue`
  all depend on `EmployerTimeZoneResolver`'s IANA→Windows→`StandardName`
  lookup chain with a hardcoded `America/Chicago` fallback. expr-lang has no
  timezone database. The caller resolves the employer's timezone **once**
  per evaluation (not once per diagnosis — the C# does this independently
  per call site today, which the port should not repeat) and hands the
  engine already-resolved flat values: `TodayInEmployerTz`,
  `DaysElapsedInPayPeriod`, `HoursSinceLastCompanyWorkDate`. These are
  read-only inputs, never `computed` formulas, precisely because a bad
  timezone lookup must not be allowed to take the whole segment dark.
- **The waterfall collection search.** `EnrichWithWaterfallSignals` finds the
  first `WaterfallStep` where `Balance <= 0 && PreviousBalance > 0` and reads
  four fields off it. The engine's context is a flat map with no "find the
  matching array element" primitive — `contains`/`in` test membership, not
  shape. The caller runs this exact `FirstOrDefault` before evaluation and
  hands the engine `WaterfallSource`, `WaterfallPreviousBalance`,
  `WaterfallBalance`, `WaterfallMessage` as four flat, already-resolved
  fields (absent, not derived-and-possibly-failing, when no step matches).

Putting any of these three in a segment's `computed` block is the mistake to
avoid: a bad IANA lookup or a `nil` waterfall match would silently blank
every other finding sharing that segment.

---

## 4. What does not port, and where it goes instead

Three things named in the survey stay outside `segments.json` entirely — not
because the engine is missing a feature it should grow, but because they are
a different kind of operation than what a segment does.

1. **The waterfall collection search** (`EffectiveLimitDiagnoser.EnrichWithWaterfallSignals`).
   Belongs in the caller's context-builder, pre-flattened, per §3.3. There is
   no engine-side fix that would not also require adding array-search
   semantics to conditions, which is a much bigger change than this port
   should ask for.
2. **Timezone-aware date/duration arithmetic** with the hardcoded
   `America/Chicago` fallback, repeated across `OutsideAvailableBalanceWindow`,
   `TimesheetHoursAbnormallyLow`, and `TimeClockIssue`. Belongs in the
   caller's context-builder, resolved once per evaluation, per §3.3. expr-lang
   has no timezone database and should not be asked to grow one for this.
3. **Global post-diagnosis ranking and allowlist filtering**
   (`DefaultDiagnosisRanker`, `AllowlistDiagnosisFilter`). These run across
   the whole flattened list of diagnoses from every diagnoser, after all of
   them have produced results — sort by `Severity` descending, then by a
   runtime `Dictionary<DiagnosisType,int>` priority map, then drop anything
   outside a hot-reloadable allowlist. Nothing in the engine's model
   aggregates across layers this way — each layer resolves to at most one
   segment, in isolation, and there is no "look at everything every layer
   emitted before deciding what to keep" hook. This is consumer-side
   post-processing on the `/v1/evaluate` response, full stop. The
   `diagnosisTypes` lookup's `order` field (§2.1) gives the consumer the
   per-type priority number without needing its own hot-reloadable config
   source — `config/segments.json`'s existing hot-reload already covers that
   half — but the two-level sort (severity, then type order) and the
   allowlist membership check both still run in consumer code, reading
   `outputs.severity` and `outputs.diagnosisType.{key,order}` off each
   emitted finding.

`DiagnosisCatalogProvider` (§1.5) is a fourth thing that does not port, but
for a different reason: it is test-data plumbing with no runtime behavior,
not a capability gap. `GET /v1/segments` replaces it outright.

---

## 5. The consumer mapping

The context document's table (reproduced below) holds up against the
survey's actual `Diagnosis` shape (`Diagnosis.cs`, survey §2), with one
correction and one addition worth flagging to whoever writes the adapter.

| Emitted | Becomes | Note |
|---|---|---|
| `outputs.diagnosisType.key` | `DiagnosisType` | `.order` replaces the old ranking-priority dictionary (§4) |
| `outputs.severity` | `Severity` | now `{key, value, order}` via the `severities` lookup, not a bare string — read `.key` |
| `outputs.category` | `DiagnosticCategory` | same — read `.key` from the `diagnosticCategories` lookup |
| `outputs.title` | `Title` | `Title` is a get-only computed property in C#; the port needs a setter, falling back to the existing `ToTitle()` derivation only when the field is absent |
| `outputs.description` / `.userMessage` / `.technicalExplanation` | the three prose fields | direct |
| `outputs.resolution.type` / `.resolution.detail` | `Resolution.Type` / `.Detail` | direct, dotted keys as authored |
| `outputs.signals` | `Signals` (`List<Signal(Name, Value)>`) | see correction below |

**Correction to make explicit, because it is easy to miss:** the context
document's table lists `outputs.severity`/`.category` as if they become plain
scalars. Once they are declared as `lookup`-bound fields (§2.1, and there is
good reason to — §1.5's "stable key set" case applies to these too, just more
weakly than to `diagnosisType`), they are emitted as `{key, value, order}`
objects, the same shape any other lookup-bound output field takes
(`output.go:119`, `enrichLookupValue`). The adapter reads `.key`, not the
field itself. This is not a defect, just a detail the earlier table's
phrasing glossed over.

**Addition the earlier table did not mention:** `signals` degrades from an
ordered `List<Signal>` to a Go `map[string]interface{}`, and
`encoding/json` marshals Go maps with **alphabetically sorted keys** — insertion
order is not preserved across the wire. If a consumer's UI ever relied on
`Signals` appearing in the order the diagnoser added them (nothing in the
survey suggests it does, but it is worth checking before assuming otherwise),
the port changes that ordering. If order turns out to matter, the fix is an
`array`-typed `expression` field returning `[{name, value}, ...]` instead of
an object — more verbose to author, but order-preserving — not a change to
the engine.

---

## 6. Sequencing

A port this size should not be attempted as one config change. Proposed
order, each slice runnable and demonstrable on its own before the next:

1. **`balance-processing-error` + `balance-validation-codes`** (14 of 50
   diagnoses). This is the smallest slice that proves the whole mechanism
   end to end: a `rule` segment short-circuit, `dependsOn` + `when` gating a
   downstream `checklist` layer off it, the full `outputSchema` +
   four lookup tables authored for real, and `Rule.Outputs` populated on
   every item. Nothing here needs timezone math, collection search, or the
   computed-winner technique — those are deliberately deferred to the next
   slice, so slice 1 either fully works or fully fails on the parts that are
   genuinely novel to this engine (output schema + lookups), not on the
   parts that are merely tedious (13 near-identical checklist items).
2. **`balance-result-code` + `balance-effective-limit`** (4 + 11 more, 29
   total). This is where the computed-winner technique and the caller-side
   timezone/waterfall pre-computation (§3.3) get proven for real, and it is
   the highest-risk slice — build and unit-test the employer-timezone
   resolver and the waterfall-search flattening as standalone caller-side
   functions before wiring them into the context builder, since a bug in
   either one degrades an entire segment's worth of findings, not just one
   diagnosis.
3. **`company-health`** (12 more, 41 total). Mostly independent checks
   already proven by slice 1's shape; `TimeClockIssue` reuses the timezone
   resolver built in slice 2.
4. **`employee-account`** (9 more, 50 total). `PaycheckDrop` and
   `TimesheetHoursAbnormallyLow` reuse the one-consumer `computed` pattern
   from §3.2 and the timezone resolver from slice 2 respectively.
5. **`balance-message-fallback`**, last, optional. Given §1.3's finding that
   this diagnoser is dead code in the source today, treat this as the one
   slice that may simply not be worth doing — if it is done, budget time for
   the duplicated guard and its drift risk, not just for the config itself.

In parallel with slices 1–4: the consumer-side adapter (§5) and the
post-processing ranker/allowlist (§4) can be built against slice 1's output
alone, since the `Diagnosis` shape does not change across slices — only the
number of `diagnosisType` keys the `diagnosisTypes` lookup and the adapter
need to know about grows.
