# Layer Categories — Design

**Date:** 2026-06-24
**Status:** Validated, ready for implementation planning

## Summary

Layers can be grouped by a single **category** (theme) so they can be organized
and evaluated by theme rather than always all-together. Example themes: Balance,
Compliance, Risk, Fraud. Categories are not hard-coded — they are data, defined
in config.

Each category has a human-readable `name` (changeable) and a stable `apiName`
(the contract automation references, so renaming the display name isolates
change). A layer belongs to exactly one category and may only reference other
layers in the **same** category, making categories real isolation boundaries.

## Decisions

| Decision | Choice |
|---|---|
| Category cardinality per layer | Exactly one (`category`, not tags) |
| Category identity | `{ apiName, name }` — `apiName` stable, `name` editable |
| Selection at evaluation | New `categories` request field (list of apiNames) |
| `layers` + `categories` together | Successive narrowing (categories scopes, then layers narrows). No AND. |
| Cross-layer references | A layer may reference only same-category layers (enforced at config time) |
| Required / default | Required; empty `categoryApiName` auto-defaults to the seeded `default` category at load |
| Request omits `categories` | Evaluate all layers (backward compatible) |
| Persistence vs memory | Persist **normalized** (registry once + per-layer apiName); in-memory **denormalized** (each layer holds resolved `{apiName, name}`) |
| Scope | Full stack (domain, validation, evaluation, API, UI) |

## Section 1 — Data Model

### Persisted form (normalized) — `config/segments.json`

```json
{
  "version": 19,
  "categories": [
    { "apiName": "risk", "name": "Risk Assessment" },
    { "apiName": "default", "name": "Default" }
  ],
  "layers": [
    { "name": "Risk Rating", "order": 1, "categoryApiName": "risk", "segments": [...] }
  ]
}
```

The registry holds each `{apiName, name}` once; layers carry only the stable
`categoryApiName` token. No display-name duplication on disk; a rename touches
exactly one record.

### In-memory form (denormalized) — domain model

```go
type Category struct {
    APIName string
    Name    string
}

type Layer struct {
    Name     string
    Order    int
    Category Category   // resolved — full {apiName, name}
    Segments []Segment
}
```

Each `Layer` holds its resolved category, so the evaluator and consumers get the
display name and apiName directly. `Snapshot` also keeps the full
`Categories []Category` catalog so categories with zero layers remain
enumerable (UI selector, automation listing).

### Mapping (infrastructure — `ConfigSource` adapter)

- **Load:** decode normalized JSON → resolve each layer's `categoryApiName`
  against the registry → populate `Layer.Category`. Missing/empty resolves to the
  seeded `{apiName: "default", name: "Default"}`. Runs before validation.
- **Save:** marshal the domain model back to normalized JSON (registry once +
  per-layer apiName).

The domain model stays ignorant of the wire format; only the adapter knows both
shapes. The normalizer seeds the `default` category if absent.

## Section 2 — Validation Rules

Added to `validation.ValidateSnapshot`. All run at config load and on every
hot-reload before the atomic swap, so a bad config never becomes live.

1. **Registry integrity.** Every `apiName` is unique and non-empty. The seeded
   `default` category always exists.

2. **Layer → category reference resolvable.** Every layer's `categoryApiName`
   matches a registry entry.
   Error: `layer "Risk Rating": unknown category "rsk"`.

3. **Same-category cross-layer constraint (core new rule).** Replaces the
   current no-op where `layer:` references are accepted unconditionally
   (`validator.go:68-70`):
   - Build `map[layerName]categoryApiName`.
   - Walk every segment's `rules` **and** `overrides` recursively. For each leaf
     whose `field` starts with `layer:`, extract the referenced layer name.
   - Reject if the referenced layer does not exist, or exists but has a different
     category.
   - Errors:
     - `segment "summer-sale" rule "tier-check": references unknown layer "base-tier"`
     - `segment "summer-sale" rule "tier-check": cross-category reference "layer:base-tier" (category "balance") not allowed from category "risk"`

This makes it structurally impossible to save a config where a Risk layer reads a
Balance layer's result.

> Note: this is stricter than today. The existing `config/segments.json` passes
> only because the normalizer puts every existing layer in `default`, so existing
> `layer:base-tier` references stay same-category. Self-references and `order`
> cycles are pre-existing concerns, out of scope.

## Section 3 — Evaluation Scoping

### Request contract

Add `categories []string` (apiNames) to `EvaluateRequest`, alongside `layers`:

```json
{ "subject_key": "user-001", "context": { }, "categories": ["risk", "fraud"] }
```

### Semantics — true hard scope

Today the evaluator runs every layer and the filter only controls output. For
categories, change *what executes*:

- If `categories` is set, only layers whose `Category.APIName` is in the set are
  evaluated at all. Out-of-category layers do not run.
- Safe because of Section 2: a layer references only same-category layers, so a
  scoped run never needs an out-of-category `layer:` result.
- Cross-layer injection (`evalCtx["layer:"+name]`) still happens for every
  in-scope layer, so same-category dependencies resolve normally.

### Interaction with `layers` filter — successive narrowing

1. Start with all layers.
2. If `categories` set → keep only those in the category set (bounds execution).
3. If `layers` set → further keep only those names.

### Backward compatibility

`categories` omitted → no category narrowing → all layers evaluated, as today.

### Unknown apiName

A requested category not in the registry matches no layers and emits a warning
(not an error), consistent with the existing `layers` filter behavior.

### Batch

`evaluate/batch` inherits this automatically (each subject is an
`EvaluateRequest`).

## Section 4 — API & Response Surface

- **`GET /v1/segments`** returns the snapshot including the `categories` registry
  and per-layer `categoryApiName`. Serialized admin shape stays **normalized**;
  the UI rebuilds the denormalized view client-side. The save path writes the
  same normalized form; existing `last_modified` bump unchanged.

- **Evaluate response.** `LayerResultDTO` gains a resolved `category`:

  ```json
  "Risk Rating": {
    "segment": "false",
    "strategy": "expression",
    "reason": "rule:RiskEval",
    "category": { "apiName": "risk", "name": "Risk Assessment" }
  }
  ```

  Additive; cheap because in-memory `Layer.Category` already carries both values.

- **Warnings.** A requested apiName matching no layers emits a `WarningDTO`
  (e.g. `category "frad" matched no layers`).

- **No new endpoints.** Categories ride on existing `/v1/evaluate`,
  `/v1/evaluate/batch`, `/v1/segments`. The registry is edited through the
  existing config-save flow and hot-reloads normally. `/v1/reload` picks up
  changes via the standard load→validate→swap path.

## Section 5 — UI (full stack)

- **Types** (`ui/src/api/types.ts`). Add `Category { apiName; name }`;
  `categories: Category[]` on `Snapshot`; `categoryApiName: string` on `Layer`;
  optional `categories?: string[]` on `EvaluateRequest`; `category?: Category` on
  `LayerResult`. Client mirrors the server: load normalized, resolve per-layer.

- **Category management.** Lightweight editor (modal or Layers-page section) to
  add/rename/remove registry entries. `apiName` set on create, then read-only
  (stable contract); `name` freely editable. The seeded `default` category cannot
  be deleted. Deleting a category is blocked while any layer references it (or
  offers reassign-to-`default`).

- **Layers page** (`LayersPage.tsx`, `LayerCard.tsx`). Category shown as a
  colored badge (reuse strategy color-coding). Add group-by-category view and a
  category filter dropdown.

- **Layer form** (`LayerForm.tsx`). Required category `<select>` from the
  registry, defaulting to `Default`, with an inline "add category" option.

- **Testing Zone** (`TestingZone.tsx`). Category multi-select populating the
  request's `categories`, to evaluate one theme in isolation. `ResultDisplay.tsx`
  shows the returned category badge per layer and surfaces "matched no layers"
  warnings.

- **Cross-category guard.** Where rules reference `layer:<name>`
  (`RuleNode.tsx` / `ExpressionEditor.tsx`), scope the layer picker to
  same-category layers only, so the UI cannot build a config that Section 2
  validation would reject.

## Out of Scope

- Multiple categories/tags per layer.
- Dedicated category CRUD endpoints (categories live in the config snapshot).
- Self-reference and layer `order` cycle detection (pre-existing).
- Per-category access control / auth.
