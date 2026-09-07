# Output Field Picker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a rule's output values a list of *authored values*, each choosing its field from a dropdown of the layer's schema, instead of a fixed row per declared field with a free-text box for new ones.

**Why.** Two problems with the current shape. A mis-typed field name declares junk on the layer and every rule then grows a row for it, with no way to correct the value's field short of editing the layer schema. And a ten-field schema renders ten rows on every check, so the balance-diagnostics port — fifty-two checks, most setting two or three fields — would show several hundred mostly-empty inputs. Per-value rows with a field dropdown fix both: a mis-pick is corrected by reopening the dropdown, and a check shows only what it actually sets.

**Architecture:** The row model is extracted into pure functions in `outputSchemaRules.ts` and verified with the existing `node:assert` harness, because that is where the regression risk lives. `OutputValuesEditor` becomes a thin renderer over them. The component already serves **both** surfaces — per-rule values inside `RuleNode`, and segment-level constants in `SegmentEditor` — so changing it once updates both, and both must be checked.

**Tech Stack:** React 19, TypeScript ~5.9, Vite 7. No CSS framework — plain CSS modules.

## Global Constraints

- **A required field always shows a row, authored or not.** Hiding an obligation would be hostile: the engine rejects the save when a required field is unauthored, so the author must be able to see what is owed. Its remove control clears the value rather than removing the row.
- **An optional field shows a row only when it is authored.** That is what makes the list compact.
- **An orphaned key — one present in `outputs` but absent from the schema — must show a row, flagged.** These are stale values left behind when a field was deleted from the layer schema, and today they are invisible while still causing `output "x" is not declared in outputSchema` on save. Surfacing them is a direct fix for that, and the dropdown is how the author re-points or clears one.
- **Re-pointing a row preserves its value.** Changing the dropdown from `severty` to `severity` moves the authored text across, deletes the old key, and revalidates against the new field's type and eval mode. A value that was valid may now show an error; that is correct and useful.
- **A field may be chosen once per editor.** The dropdown excludes fields already used by other rows, except the row's own current selection.
- **Row order is deterministic:** required fields first, then orphans, then authored optional fields, each group alphabetical. Deterministic ordering means two authors see the same list, and a re-point moves the row predictably rather than leaving it wherever it was inserted.
- Emptying the last value still emits `undefined`, not `{}`, so the key is omitted from JSON.
- **`Required` still defaults to `false`** when declaring a new field inline. Declaring mid-edit must never block a save.
- **There is no JS test runner** and adding one is out of scope. Pure logic goes in `outputSchemaRules.ts` and is verified by `ui/verify-output-schema.mjs` with `node:assert`; the component is verified by build plus a manual check.
- **`npm run lint` has a red baseline of exactly 2 pre-existing errors** (`SegmentEditor.tsx` refs-during-render, `StrategyPicker.tsx:9` react-refresh) — leave both alone; gate on the count not rising above 2.
- Do not change the engine. Nothing in `internal/` is touched by this plan; the JSON shape of `outputs` is unchanged.
- Ports 8080 and 8081 may be occupied by containers that are not yours. Use a high port. Restore `config/segments.json` with `git checkout --` if you write to it, stop every server, delete scratch files.

---

### Task 1: The row model

**Files:**
- Modify: `ui/src/components/schema/outputSchemaRules.ts`
- Modify: `ui/verify-output-schema.mjs`

**Interfaces:**
- Consumes: `OutputSchema`, `OutputField` from `ui/src/api/types.ts`.
- Produces, all pure:
  - `outputValueRows(schema: OutputSchema, outputs?: Record<string,string>): OutputRow[]` where `OutputRow = { name: string; field?: OutputField; value: string; required: boolean; orphaned: boolean }`
  - `availableOutputFields(schema: OutputSchema, outputs: Record<string,string> | undefined, current: string): string[]`
  - `renameOutputKey(outputs: Record<string,string> | undefined, from: string, to: string): Record<string,string> | undefined`

  Task 2 consumes all three.

- [ ] **Step 1: Write the failing verifier cases**

Append to `ui/verify-output-schema.mjs`, before implementing:

```js
// --- output value rows -------------------------------------------------
const rowSchema = {
  severity: { type: 'string', required: true },
  category: { type: 'string', required: true },
  title:    { type: 'string' },
  notes:    { type: 'string' },
};

// Required fields always appear, authored or not. Optional ones only when
// authored. Orphans — keys with no declaration — appear flagged, because a
// stale value is otherwise invisible while still breaking the save.
let rows = outputValueRows(rowSchema, { title: 'T', ghost: 'G' });
assert.deepEqual(rows.map(r => r.name), ['category', 'severity', 'ghost', 'title']);
assert.deepEqual(rows.map(r => r.required), [true, true, false, false]);
assert.deepEqual(rows.map(r => r.orphaned), [false, false, true, false]);
assert.equal(rows.find(r => r.name === 'severity').value, '');
assert.equal(rows.find(r => r.name === 'title').value, 'T');
assert.equal(rows.find(r => r.name === 'ghost').field, undefined);

// No outputs at all still shows the two required fields.
assert.deepEqual(outputValueRows(rowSchema, undefined).map(r => r.name), ['category', 'severity']);

// An empty schema with an authored key is all orphans.
assert.deepEqual(outputValueRows({}, { x: '1' }).map(r => r.orphaned), [true]);

// --- the dropdown's options -------------------------------------------
// A field already used by ANOTHER row is not offered; the row's own current
// selection always is, or the select would have no matching option.
assert.deepEqual(
  availableOutputFields(rowSchema, { severity: 'C', title: 'T' }, 'title'),
  ['category', 'notes', 'title'],
);
assert.deepEqual(
  availableOutputFields(rowSchema, {}, ''),
  ['category', 'notes', 'severity', 'title'],
);
// An orphan's own name is offered so the select can display it.
assert.ok(availableOutputFields(rowSchema, { ghost: 'G' }, 'ghost').includes('ghost'));

// --- re-pointing a row -------------------------------------------------
// The value follows the rename, the old key goes, order is irrelevant.
assert.deepEqual(renameOutputKey({ severty: 'Critical' }, 'severty', 'severity'),
                 { severity: 'Critical' });
// Renaming onto a name already present overwrites it — the caller prevents
// this via availableOutputFields, but the function must not corrupt the map.
assert.deepEqual(renameOutputKey({ a: '1', b: '2' }, 'a', 'b'), { b: '1' });
// Renaming the only key to itself is a no-op, not a delete.
assert.deepEqual(renameOutputKey({ a: '1' }, 'a', 'a'), { a: '1' });
// Emptying yields undefined so the key is omitted from JSON.
assert.equal(renameOutputKey({ a: '1' }, 'a', ''), undefined);
assert.equal(renameOutputKey(undefined, 'a', 'b'), undefined);
```

- [ ] **Step 2: Run it to verify it fails**

Run from `ui/`: `npm run verify:output-schema`
Expected: FAIL — the three functions do not exist, so the `tsc` compile step errors.

- [ ] **Step 3: Implement**

Add to `ui/src/components/schema/outputSchemaRules.ts`:

```ts
export interface OutputRow {
  name: string;
  /** Absent when the key is orphaned — authored but no longer declared. */
  field?: OutputField;
  value: string;
  required: boolean;
  orphaned: boolean;
}

/**
 * The rows an output-value editor shows.
 *
 * Required fields always appear even when unauthored, because the engine
 * rejects the save without them and an invisible obligation is worse than a
 * long list. Optional fields appear only once authored, which is what keeps a
 * ten-field schema from rendering ten inputs on every check.
 *
 * Orphans — keys with no declaration, left behind when a field was removed
 * from the layer's schema — always appear, flagged. They already break the
 * save with "output %q is not declared in outputSchema"; showing them is what
 * lets an author re-point or clear one.
 *
 * Order is required, then orphaned, then authored optional, alphabetical
 * within each group, so the list is stable across authors and a re-point moves
 * a row predictably.
 */
export function outputValueRows(
  schema: OutputSchema,
  outputs?: Record<string, string>,
): OutputRow[] {
  const vals = outputs ?? {};
  const required: OutputRow[] = [];
  const optional: OutputRow[] = [];
  const orphaned: OutputRow[] = [];

  for (const [name, field] of Object.entries(schema)) {
    const row: OutputRow = {
      name,
      field,
      value: vals[name] ?? '',
      required: !!field.required,
      orphaned: false,
    };
    if (row.required) required.push(row);
    else if (name in vals) optional.push(row);
  }
  for (const name of Object.keys(vals)) {
    if (!(name in schema)) {
      orphaned.push({ name, value: vals[name], required: false, orphaned: true });
    }
  }

  const byName = (a: OutputRow, b: OutputRow) => a.name.localeCompare(b.name);
  return [...required.sort(byName), ...orphaned.sort(byName), ...optional.sort(byName)];
}

/**
 * The field names a row's dropdown may offer: everything declared, minus what
 * other rows already use, plus this row's own current selection — without
 * which the select would render with no matching option and silently display
 * the wrong one.
 */
export function availableOutputFields(
  schema: OutputSchema,
  outputs: Record<string, string> | undefined,
  current: string,
): string[] {
  const used = new Set(Object.keys(outputs ?? {}));
  used.delete(current);
  const names = new Set(Object.keys(schema).filter((n) => !used.has(n)));
  if (current) names.add(current);
  return [...names].sort((a, b) => a.localeCompare(b));
}

/**
 * Re-points an authored value at a different field, carrying the value across.
 * This is how a mis-picked or mis-typed field is corrected without retyping.
 */
export function renameOutputKey(
  outputs: Record<string, string> | undefined,
  from: string,
  to: string,
): Record<string, string> | undefined {
  if (!outputs) return undefined;
  const next = { ...outputs };
  const value = next[from];
  delete next[from];
  if (to) next[to] = value;
  return Object.keys(next).length ? next : undefined;
}
```

- [ ] **Step 4: Run the verifier**

Run from `ui/`: `npm run verify:output-schema`
Expected: `output schema rules OK`.

- [ ] **Step 5: Build, lint, commit**

Run: `npm run build`, then `npm run lint` (exactly 2 problems).

```bash
git add ui/src/components/schema/outputSchemaRules.ts ui/verify-output-schema.mjs
git commit -m "feat(ui): row model for output values, keyed by authored value"
```

---

### Task 2: The picker

**Files:**
- Modify: `ui/src/components/rules/OutputValuesEditor.tsx`
- Modify: `ui/src/components/rules/OutputValuesEditor.module.css`

**Interfaces:**
- Consumes: `outputValueRows`, `availableOutputFields`, `renameOutputKey`, `evalModeOf`, `validateLiteralValue`, `FieldCoverage` (Task 1 and existing).
- Produces: no prop changes. The component's signature is unchanged, so **both** callers — `RuleNode` for per-rule values and `SegmentEditor` for segment-level constants — pick this up with no edit.

- [ ] **Step 1: Rewrite the body**

Replace the fixed field list with rows from `outputValueRows`. Each row renders:

- a `<select>` of `availableOutputFields(schema, outputs, row.name)`, plus a final `declare new…` option;
- the value `<input>`, placeholdered by the row's eval mode and validated by `validateLiteralValue`;
- a remove control — which **clears the value on a required row** (the row stays, because the obligation stays) and **removes the row** otherwise.

Changing the select calls `onChange(renameOutputKey(outputs, row.name, picked))`. Choosing `declare new…` reveals the existing name input and calls `onDeclare(name, { type: 'string' })` — keep the trim and the duplicate guard exactly as they are today.

An orphaned row renders its name in the select (it is in the options for that reason) with an inline note — *not declared on the layer; pick a field or remove* — in the danger colour. This is the case that was previously invisible.

Keep the existing coverage readout untouched: it is per field, still correct, and only supplied by the segment-level caller.

Add `aria-label` to both the select and the input, naming the field — the row has no visible label now that the name is a control.

- [ ] **Step 2: Adjust the stylesheet**

The grid is currently two columns, name and value. It becomes three: field select, value, remove. Keep the existing font sizes and muted colours so it still reads as part of the rule card.

- [ ] **Step 3: Build, verify, lint**

Run from `ui/`: `npm run build`, `npm run verify:output-schema`, `npm run verify:rules`, `npm run lint` (exactly 2 problems).

- [ ] **Step 4: Verify by hand, on both surfaces**

Start the engine on a high port and `npm run dev`. Playwright with Chromium is installed; a driver script must live inside `ui/` to resolve the import, and be deleted afterwards.

On a **checklist segment** in a layer with a multi-field output schema:
1. A check with no values shows only the required fields, empty — not every declared field.
2. Adding a value offers a dropdown of the remaining fields; picking one adds a row.
3. A field already used on that check is not offered again.
4. Entering a value then re-pointing the dropdown at another field carries the value across and revalidates it — re-point a `string` value to a `number` field and confirm the parse error appears.
5. Removing an optional row deletes it; the remove control on a required row clears the value but leaves the row.
6. `declare new…` still declares on the layer with `required` unticked.
7. Round-trip a save and confirm `outputs` is unchanged in shape.

Then the **orphan case**, which is the point of the feature: with a value authored, delete that field from the layer's output schema, return to the segment, and confirm the stale value now shows as a flagged row that can be re-pointed or removed — where before it was invisible and blocked the save.

Finally check the **segment-level** editor in `SegmentEditor` behaves the same, since it shares the component.

- [ ] **Step 5: Commit**

```bash
git add ui/src/components/rules/
git commit -m "feat(ui): choose a rule's output field from a dropdown"
```

---

## Out of scope for this plan

- **Renaming a field across the whole layer.** Re-pointing changes one value's field. Renaming a declared field everywhere at once is a layer-schema operation and a separate feature.
- **Any engine change.** The JSON shape of `outputs` is unchanged; this is presentation only.
- **The input-schema side.** Rule *conditions* already pick their field from the layer's input schema; nothing there changes.
