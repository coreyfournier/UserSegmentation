# Centralized Lookup Tables

**Date:** 2026-07-12
**Status:** Design — approved

## Summary

A snapshot-global collection of named **lookup tables** to simplify rules. Each
table stores typed key/label entries. Rules match a field against a table's keys
via two new operators (`in_lookup` / `not_in_lookup`), referencing the table by a
stable internal **id**. Execution treats the table's keys the same as an inline
array (funnels into the existing `in` logic), but centralizes maintenance.

## Decisions (from brainstorming)

1. **Dedicated operators** `in_lookup` / `not_in_lookup` (not a reuse of `in`),
   so the config makes the lookup explicit. The expression's `value` holds the
   table **id**.
2. **Snapshot-global** `lookups` collection, shared across all layers.
3. **Immutable internal id** (auto-slugged from the display name at creation)
   used by references; **mutable display name**. Renaming the display name never
   breaks references.
4. **keyType is immutable** after creation (protects references and entry types);
   `name` and `entries` are editable.
5. **Both operators** added (positive + negated).
6. **Delete is blocked while referenced** (409, reporting referencing rules).
7. Key data types: `string | number | boolean` (no `array`).

## Data model (`internal/domain/model`)

```go
type LookupEntry struct {
    Key   interface{} `json:"key"`             // value used for matching, typed per KeyType
    Label string      `json:"label,omitempty"` // optional human description
}
type LookupTable struct {
    ID      string        `json:"id"`       // immutable slug, used by references
    Name    string        `json:"name"`     // mutable display name
    KeyType FieldType     `json:"keyType"`  // string | number | boolean
    Entries []LookupEntry `json:"entries"`
}
// Snapshot gains:
Lookups []LookupTable `json:"lookups,omitempty"`
```

## Operators (`model/operators.go`)

- Add `OpInLookup = "in_lookup"` and `OpNotInLookup = "not_in_lookup"`.
- `OperatorTypes` entries: both support `string`, `number`, `boolean`.

## Execution (`strategy`)

- `EvalContext` gains `Lookups map[string]model.LookupTable` (keyed by id).
- `evaluateRule` and `EvalExpression` gain a `lookups` parameter (threaded from
  `EvalContext`). RuleStrategy and EvalOverrides pass `ctx.Lookups`.
- The evaluator builds the id→table map once from `snap.Lookups` and sets it on
  each `EvalContext`.
- `evalOp` handles:
  - `in_lookup`: resolve table by id (`expected` is the id string), collect entry
    keys into `[]interface{}`, reuse `evalIn(actual, keys)`.
  - `not_in_lookup`: negation.
  - Missing/dangling table → `false` (consistent with missing-field behavior).

## Validation (`domain/validation`)

- **Tables:** id non-empty / unique / slug format; name non-empty; keyType in
  {string, number, boolean}; each entry key coerces to keyType.
- **References:** `in_lookup`/`not_in_lookup` `value` must name an existing table;
  when the field type is known from `inputSchema`, it must equal the table
  `keyType`.

## Admin API (`application` + `infrastructure/http`)

- CRUD endpoints:
  - `GET /v1/admin/lookups` — list
  - `POST /v1/admin/lookups` — create (auto-slug id from display name, unique)
  - `PUT /v1/admin/lookups/{id}` — update name/entries (id + keyType locked)
  - `DELETE /v1/admin/lookups/{id}` — **409 while referenced**, reporting the
    referencing segment/rule
- `lookups` included in snapshot get/import/export.
- Slug generation + reference-scan helpers live in the admin use case; deletes
  and edits go through `commitSnapshot` (validate + save + swap).

## UI (`ui/`)

- **New "Lookups" section** (Sidebar nav + route):
  - List: display name, id, key type, entry count; create/edit/delete.
  - Create form: display name + key type; previews the locked auto-slug id.
  - Edit form: id + keyType read-only; edit name and entries (key + optional
    label rows).
  - Delete surfaces the 409 reference list.
- **ExpressionEditor:** when the operator is `in_lookup`/`not_in_lookup`, replace
  the value input with a table dropdown filtered to tables whose `keyType`
  matches the field's type (show display name, store id).
- **OperatorSelect:** add the two operators for string/number/boolean fields.
- **types.ts:** `LookupTable`, `LookupEntry`, `Snapshot.lookups`, operator union.
- **api/lookups.ts:** query + mutation hooks.

## Testing

- Go: operator tests (`in_lookup`/`not_in_lookup`, missing table, numeric
  coercion), evaluator threading test, validation tests (bad key type, dangling
  ref, keyType/field mismatch), admin delete-while-referenced test.
- UI: Docker build/typecheck.

## Out of scope

- Value-side lookups (mapping a key to its label as a computed field) — the label
  is descriptive only for now.
- Per-layer scoping; nested/hierarchical tables.
