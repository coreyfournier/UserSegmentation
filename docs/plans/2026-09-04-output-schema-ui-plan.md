# Output Schema (UI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let an author declare an output schema, author per-item output values, and hand-order lookup entries through the admin UI, instead of hand-writing JSON.

**Architecture:** Additive to the existing segment editor. A new `OutputSchemaEditor` mirrors `InputSchemaEditor` and sits beside it in `ui/src/components/schema/`. The engine rules that constrain a valid declaration — added by Task 8 of the engine plan — are extracted into one pure module, `outputSchemaRules.ts`, so the editor can *prevent* invalid config rather than surfacing a load error after the save round-trips. Per-item values ride the same prop path that `schema` and `perRuleMessages` already travel into `RuleNode`.

**Tech Stack:** React 19, TypeScript ~5.9, Vite 7, `@tanstack/react-query` 5. No CSS framework — plain CSS modules alongside each component, matching every existing component.

## Global Constraints

- **There is no JS test runner in this project.** `ui/verify-rule-tree.mjs` says so in its own header and asserts with `node:assert/strict` against a `tsc`-compiled module. Follow that pattern exactly for pure logic; do NOT add vitest, jest, or `@testing-library`. Adding a test framework is out of scope and will be rejected.
- Verification for React components is `npm run build` (which runs `tsc -b` first, so type errors fail the build) plus `npm run lint`, plus the manual check each task names. **`npm run lint` has a red baseline of exactly 2 pre-existing errors** (`SegmentEditor.tsx:33` refs-during-render, `StrategyPicker.tsx:9` react-refresh) — leave both alone and gate on the count not rising above 2.
- `npm install` in `ui/` first — `ui/node_modules` is not present. Node v22.20.0 and npm 10.9.3 are on PATH. `ui/Dockerfile` and the `ui` service in `docker-compose.yml` remain available as an alternative.
- **`static` and `percentage` segments are exempt from output-schema enforcement** in the engine, because those strategies never populate a record. The UI must not offer an output schema for them; show the one-line note specified in Task 3 instead.
- **`Required` must default to `false`** on every path that creates a field. This is what makes the fast authoring flow work: declaring a field mid-edit must never block a save.
- Do not change any existing behaviour of `InputSchemaEditor`, `ConditionEditor`, `MessagesEditor`, or the rule drag-and-drop. This plan only adds.
- The admin API replaces a whole segment on save (`PUT /v1/admin/layers/{name}/segments/{id}`). `SegmentEditor` already `structuredClone`s the fetched segment and only ever spreads, so unknown fields survive — preserve that property; never construct a segment object from scratch.
- Run `npm run build && npm run lint` from `ui/` before every commit.

---

### Task 1: Extend the API types

**Files:**
- Modify: `ui/src/api/types.ts`

**Interfaces:**
- Consumes: nothing.
- Produces: `FieldType` gains `'object'`; `EvalMode`; `OutputField`; `OutputSchema`; `Segment.outputSchema`; `Segment.outputs`; `Rule.outputs`; `LookupEntry.order`; `LookupTable.description`/`emitOrder`/`customOrder`; `Failure.outputs`; `LayerResult.outputs`. Every later task depends on these exact names — they mirror the Go JSON tags, so a plausible-but-different name silently drops data on save.

- [ ] **Step 1: Widen `FieldType` and add the output types**

In `ui/src/api/types.ts`, replace line 1:

```ts
export type FieldType = 'string' | 'number' | 'boolean' | 'array' | 'object';
```

Then add, directly after the `InputSchema` declaration (currently line 37):

```ts
/**
 * How an output field's authored value is turned into the emitted value.
 * `literal` is the default when absent, matching the Go `EvalMode()` accessor.
 */
export type EvalMode = 'literal' | 'template' | 'expression';

export interface OutputField {
  type: FieldType;
  /** Absent means `literal`. */
  eval?: EvalMode;
  /** Id of a lookup table whose keys are this field's permitted values. */
  lookup?: string;
  /**
   * The caller's contract. Enforced twice by the engine: an error at snapshot
   * load if no authoring path supplies it, and a warning at evaluation if it
   * is absent anyway. Defaults to false so declaring a field never blocks a
   * save.
   */
  required?: boolean;
}

export type OutputSchema = Record<string, OutputField>;
```

- [ ] **Step 2: Add the carrier fields**

In the `Segment` interface, after `inputSchema?: InputSchema;`:

```ts
  /** Declares the fields this segment emits with each reported item. */
  outputSchema?: OutputSchema;
  /** Values for output fields that do not vary per reported item. */
  outputs?: Record<string, string>;
```

In the `Rule` interface, after `messages?: Record<string, string>;`:

```ts
  /**
   * This item's authored values for the segment's output schema, keyed by
   * field name. Only a reporting rule's outputs are read — never an inner
   * And/Or branch's.
   */
  outputs?: Record<string, string>;
```

In `LookupEntry`, add `order`:

```ts
export interface LookupEntry {
  key: unknown;
  value?: string;
  /**
   * Position in the table's ordering. Always persisted, even when inferred
   * from list position, because a relational store cannot reorder rows
   * cheaply.
   */
  order?: number;
}
```

In `LookupTable`, add the three new fields:

```ts
export interface LookupTable {
  id: string;
  name: string;
  keyType: FieldType;
  /** The author's note on how the table is meant to be used, including any cross-table ordering scheme. */
  description?: string;
  /** Include each entry's `order` in the evaluation response. */
  emitOrder?: boolean;
  /** Numbers are hand-authored rather than inferred from list position. */
  customOrder?: boolean;
  entries: LookupEntry[];
}
```

- [ ] **Step 3: Add the response-side fields**

In the `Failure` interface, add:

```ts
  /** The resolved output record for this finding. */
  outputs?: Record<string, unknown>;
```

Find the layer-result interface (the one with `status`, `segment`, `strategy`, `reason`, `computed`, `messages`, `failures`) and add the same field:

```ts
  /** The resolved output record, when a single-value strategy reported one. */
  outputs?: Record<string, unknown>;
```

- [ ] **Step 4: Check `OPERATOR_TYPES` still compiles**

`OPERATOR_TYPES` is `Record<Operator, FieldType[]>`, which is keyed by operator and so is unaffected by widening `FieldType`. Confirm no operator entry needs `'object'` added — the engine deliberately excludes the object type from every operator, because an object is only ever an output value, never a condition operand. **Do not add `'object'` to any operator's list.**

- [ ] **Step 5: Verify the build**

Run from `ui/`: `npm run build`
Expected: PASS. If `tsc` reports an error in a file this task did not touch, an existing exhaustive `switch` over `FieldType` now misses `'object'` — report which file rather than adding a case that guesses at behaviour.

- [ ] **Step 6: Commit**

```bash
git add ui/src/api/types.ts
git commit -m "feat(ui): add output schema and lookup ordering types"
```

---

### Task 2: The output schema rules module

**Files:**
- Create: `ui/src/components/schema/outputSchemaRules.ts`
- Create: `ui/verify-output-schema.mjs`
- Modify: `ui/package.json` (add one script)

**Interfaces:**
- Consumes: `EvalMode`, `FieldType`, `OutputField`, `OutputSchema`, `LookupTable`, `Segment`, `Rule` from Task 1.
- Produces: `allowedTypesForMode`, `validateOutputField`, `validateLiteralValue`, `supportsOutputSchema`, `fieldCoverage` and the `FieldCoverage` type. Tasks 3, 4 and 5 all consume these.

**Why a separate module.** These are the engine's Task 8 rules restated for the client, and they are the only logic in this plan that carries real regression risk — everything else is presentation. Keeping them pure means they can be verified with node's `assert` the way `verify-rule-tree.mjs` verifies the drag logic, which is this project's only testing mechanism.

- [ ] **Step 1: Write the failing verifier**

Create `ui/verify-output-schema.mjs`. Model its compile-and-import scaffolding on `ui/verify-rule-tree.mjs` — read that file first and reuse its `execFileSync`/`tsc`/`pathToFileURL` approach verbatim, changing only the module path and the assertions:

```js
/**
 * Checks the output-schema rules. There is no JS test runner in this project,
 * so this compiles the one module that carries real regression risk and
 * asserts against it with node's built-in assert.
 *
 *   npm run verify:output-schema
 */
import assert from 'node:assert/strict';
// ...same compile scaffolding as verify-rule-tree.mjs, targeting
// src/components/schema/outputSchemaRules.ts

const {
  allowedTypesForMode,
  validateOutputField,
  validateLiteralValue,
  supportsOutputSchema,
  fieldCoverage,
} = mod;

// A template always produces a string, so only string may be declared.
assert.deepEqual(allowedTypesForMode('template'), ['string']);

// A literal cannot express a collection.
assert.deepEqual(allowedTypesForMode('literal'), ['string', 'number', 'boolean']);

// An expression returns a typed value, so anything goes.
assert.deepEqual(
  allowedTypesForMode('expression'),
  ['string', 'number', 'boolean', 'array', 'object'],
);

// Mode/type mismatches are rejected, with the mode named.
assert.match(
  validateOutputField('rank', { type: 'number', eval: 'template' }, []),
  /template/,
);
assert.equal(validateOutputField('title', { type: 'string', eval: 'template' }, []), null);
assert.match(validateOutputField('bag', { type: 'object' }, []), /expression/);
assert.equal(validateOutputField('bag', { type: 'object', eval: 'expression' }, []), null);

// A lookup-bound field must exist and must agree with the table's key type.
const tables = [{ id: 'sev', name: 'severity', keyType: 'string', entries: [] }];
assert.match(validateOutputField('s', { type: 'string', lookup: 'nope' }, tables), /nope/);
assert.match(validateOutputField('s', { type: 'number', lookup: 'sev' }, tables), /keyType|key type/);
assert.equal(validateOutputField('s', { type: 'string', lookup: 'sev' }, tables), null);

// A literal must parse as its declared type.
assert.equal(validateLiteralValue({ type: 'number' }, '3'), null);
assert.match(validateLiteralValue({ type: 'number' }, 'high'), /number/);
assert.equal(validateLiteralValue({ type: 'boolean' }, 'true'), null);
assert.match(validateLiteralValue({ type: 'boolean' }, 'yes'), /boolean/);
assert.equal(validateLiteralValue({ type: 'string' }, 'anything'), null);
// Only literal mode is checked — a template or expression is not a literal.
assert.equal(validateLiteralValue({ type: 'number', eval: 'expression' }, 'a + b'), null);
assert.equal(validateLiteralValue({ type: 'number', eval: 'template' }, '${x}'), null);
// An empty value is "not authored", not an invalid literal.
assert.equal(validateLiteralValue({ type: 'number' }, ''), null);

// Strategies that emit no record cannot carry an output schema.
assert.equal(supportsOutputSchema('checklist'), true);
assert.equal(supportsOutputSchema('rule'), true);
assert.equal(supportsOutputSchema('static'), false);
assert.equal(supportsOutputSchema('percentage'), false);

// Coverage counts reporting rules only, ignores disabled ones, and reports
// a segment-level value as covering everything.
const seg = {
  id: 's',
  strategy: 'checklist',
  outputSchema: { cat: { type: 'string' } },
  rules: [
    { ruleName: 'a', outputs: { cat: 'x' }, condition: { field: 'f', operator: 'eq', value: 1 } },
    { ruleName: 'b', condition: { field: 'f', operator: 'eq', value: 2 } },
    { ruleName: 'c', enabled: false, condition: { field: 'f', operator: 'eq', value: 3 } },
  ],
};
assert.deepEqual(fieldCoverage(seg, 'cat'), { authored: 1, total: 2, segmentLevel: false });
assert.deepEqual(
  fieldCoverage({ ...seg, outputs: { cat: 'y' } }, 'cat'),
  { authored: 1, total: 2, segmentLevel: true },
);
// An empty segment-level value does not count — the engine treats it as unauthored.
assert.deepEqual(
  fieldCoverage({ ...seg, outputs: { cat: '' } }, 'cat'),
  { authored: 1, total: 2, segmentLevel: false },
);

console.log('output schema rules OK');
```

Add to `ui/package.json` scripts, after `verify:rules`:

```json
    "verify:output-schema": "node verify-output-schema.mjs"
```

- [ ] **Step 2: Run it to verify it fails**

Run from `ui/`: `npm run verify:output-schema`
Expected: FAIL — `src/components/schema/outputSchemaRules.ts` does not exist, so the `tsc` compile step errors.

- [ ] **Step 3: Write the module**

Create `ui/src/components/schema/outputSchemaRules.ts`:

```ts
import type {
  EvalMode,
  FieldType,
  LookupTable,
  OutputField,
  Rule,
  Segment,
  StrategyType,
} from '../../api/types';

/**
 * The engine's rules for a valid output declaration, restated for the editor.
 *
 * These mirror validation the engine performs at snapshot load. Enforcing them
 * here is not belt-and-braces: it is the difference between an author being
 * told "a template always produces a string" while choosing the type, and
 * being handed a rejected save with a message about config they have already
 * moved on from.
 */

/** `literal` is the default when `eval` is absent, matching the Go accessor. */
export function evalModeOf(field: OutputField): EvalMode {
  return field.eval ?? 'literal';
}

/**
 * Which declared types each mode can honour.
 *
 * A template concatenates text, so it can only ever produce a string. A
 * literal is authored as text and coerced, so it covers the scalars but
 * cannot express a collection. Only an expression returns an arbitrary typed
 * value.
 */
export function allowedTypesForMode(mode: EvalMode): FieldType[] {
  switch (mode) {
    case 'template':
      return ['string'];
    case 'expression':
      return ['string', 'number', 'boolean', 'array', 'object'];
    default:
      return ['string', 'number', 'boolean'];
  }
}

/** Returns an error message, or null when the declaration is valid. */
export function validateOutputField(
  name: string,
  field: OutputField,
  lookups: LookupTable[],
): string | null {
  const mode = evalModeOf(field);
  const allowed = allowedTypesForMode(mode);
  if (!allowed.includes(field.type)) {
    if (mode === 'template') {
      return `${name}: a template always produces a string, so the type must be "string", not "${field.type}"`;
    }
    return `${name}: the "${field.type}" type cannot be authored as a ${mode}, so it requires eval "expression"`;
  }

  if (field.lookup) {
    const table = lookups.find((t) => t.id === field.lookup);
    if (!table) {
      return `${name}: lookup "${field.lookup}" does not exist`;
    }
    if (table.keyType !== field.type) {
      return `${name}: type "${field.type}" does not match lookup "${table.name}" key type "${table.keyType}"`;
    }
  }

  return null;
}

/**
 * Checks an authored literal against its declared type.
 *
 * Only literal mode is checked. An empty value means "not authored" — the
 * engine treats it that way too — so it is not an invalid literal.
 */
export function validateLiteralValue(field: OutputField, raw: string): string | null {
  if (evalModeOf(field) !== 'literal' || raw === '') return null;
  if (field.type === 'number' && Number.isNaN(Number(raw))) {
    return `"${raw}" does not parse as a number`;
  }
  if (field.type === 'boolean' && raw !== 'true' && raw !== 'false') {
    return `"${raw}" does not parse as a boolean — use true or false`;
  }
  return null;
}

/**
 * Whether a strategy emits a record at all.
 *
 * `static` and `percentage` never populate one, so the engine exempts them
 * from output-schema enforcement entirely. Offering an editor there would
 * produce config that silently does nothing.
 */
export function supportsOutputSchema(strategy: StrategyType | string): boolean {
  return strategy === 'checklist' || strategy === 'rule';
}

export interface FieldCoverage {
  /** Enabled reporting rules that author this field. */
  authored: number;
  /** Enabled reporting rules in total. */
  total: number;
  /** A non-empty segment-level value, which covers every path at once. */
  segmentLevel: boolean;
}

/**
 * How completely a field is authored across the segment.
 *
 * Only top-level rules are counted, because only they report. Disabled rules
 * are excluded, matching the engine's gate, which exempts them so a
 * work-in-progress item cannot block an unrelated save.
 */
export function fieldCoverage(seg: Segment, name: string): FieldCoverage {
  const reporting = (seg.rules ?? []).filter((r: Rule) => r.enabled !== false);
  const authored = reporting.filter((r) => !!r.outputs?.[name]).length;
  return {
    authored,
    total: reporting.length,
    segmentLevel: !!seg.outputs?.[name],
  };
}
```

`StrategyType` is exported from `types.ts:17` as `'static' | 'rule' | 'percentage' | 'checklist'`.

- [ ] **Step 4: Run the verifier to confirm it passes**

Run from `ui/`: `npm run verify:output-schema`
Expected: `output schema rules OK`

- [ ] **Step 5: Build and lint**

Run from `ui/`: `npm run build`, then `npm run lint`
Expected: build PASSES. **Lint has a red baseline of exactly 2 pre-existing errors** — `Cannot access refs during render` in `SegmentEditor.tsx:33` and `react-refresh/only-export-components` in `StrategyPicker.tsx:9`. Neither is yours; do not fix them as a drive-by. The gate is that lint still reports **2 problems and no more**, and that neither new error names a file you touched.

- [ ] **Step 6: Commit**

```bash
git add ui/src/components/schema/outputSchemaRules.ts ui/verify-output-schema.mjs ui/package.json
git commit -m "feat(ui): add output schema rules module with verifier"
```

---

### Task 3: The output schema editor and its reference panels

**Files:**
- Create: `ui/src/components/schema/OutputSchemaEditor.tsx`
- Create: `ui/src/components/schema/OutputSchemaEditor.module.css`
- Create: `ui/src/components/schema/EmittedFieldsReference.tsx`
- Modify: `ui/src/components/segments/SegmentEditor.tsx`

**Interfaces:**
- Consumes: `OutputSchema`, `OutputField`, `LookupTable`, `EvalMode`, `FieldType` (Task 1); `allowedTypesForMode`, `validateOutputField`, `evalModeOf`, `supportsOutputSchema` (Task 2).
- Produces: default-exported `OutputSchemaEditor` with props `{ value?: OutputSchema; onChange: (s?: OutputSchema) => void; lookups: LookupTable[] }`, and default-exported `EmittedFieldsReference` taking no props. Task 4 consumes neither; Task 4 consumes `Segment.outputSchema` directly.

- [ ] **Step 1: Write `EmittedFieldsReference`**

Create `ui/src/components/schema/EmittedFieldsReference.tsx`. This is the read-only "you get these for free" panel, so an author declares only what is missing:

```tsx
/**
 * What the evaluation response already carries without any declaration.
 *
 * Shown read-only beside the output schema editor so an author declares only
 * what is missing instead of restating the baseline. These are not values an
 * output expression can read — outputs resolve inside the strategy, before the
 * layer result exists — so this is documentation, not a field picker.
 */
const PER_LAYER = [
  ['status', 'satisfied / violated / unevaluable for a checklist; resolved / unresolved / skipped otherwise'],
  ['segment', 'the segment value a single-value strategy resolved (blank for a checklist)'],
  ['strategy', 'which strategy produced the result, or "override"'],
  ['reason', 'why, e.g. rule:<name> or checklist:<segment>'],
  ['computed', 'the segment’s computed fields, as evaluated'],
  ['messages', 'rendered localized messages for the winning rule'],
  ['outputs', 'the record your declared fields below produce'],
];

const PER_RESPONSE = [
  ['subject_key', 'the subject that was evaluated'],
  ['warnings', 'required inputs missing from context, render errors, absent required outputs'],
  ['evaluated_at', 'RFC3339 timestamp'],
  ['duration_us', 'evaluation time in microseconds'],
];

export default function EmittedFieldsReference() {
  return (
    <details>
      <summary style={{ cursor: 'pointer', fontSize: 12, color: 'var(--text-muted)' }}>
        Always emitted — no declaration needed
      </summary>
      <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 8 }}>
        <p style={{ margin: '0 0 6px' }}>Per layer:</p>
        <ul style={{ margin: '0 0 10px', paddingLeft: 18 }}>
          {PER_LAYER.map(([f, d]) => (
            <li key={f}><code>{f}</code> — {d}</li>
          ))}
        </ul>
        <p style={{ margin: '0 0 6px' }}>On the response:</p>
        <ul style={{ margin: 0, paddingLeft: 18 }}>
          {PER_RESPONSE.map(([f, d]) => (
            <li key={f}><code>{f}</code> — {d}</li>
          ))}
        </ul>
      </div>
    </details>
  );
}
```

- [ ] **Step 2: Write the editor**

Create `ui/src/components/schema/OutputSchemaEditor.module.css` by copying `ui/src/components/schema/InputSchemaEditor.module.css` verbatim — the table shape is the same, and matching it keeps the two sections visually consistent.

Create `ui/src/components/schema/OutputSchemaEditor.tsx`:

```tsx
import { useState, useRef } from 'react';
import type { EvalMode, FieldType, LookupTable, OutputField, OutputSchema } from '../../api/types';
import { allowedTypesForMode, evalModeOf, validateOutputField } from './outputSchemaRules';
import styles from './OutputSchemaEditor.module.css';

interface Props {
  value?: OutputSchema;
  onChange: (s?: OutputSchema) => void;
  lookups: LookupTable[];
}

const MODES: EvalMode[] = ['literal', 'template', 'expression'];

const MODE_HINT: Record<EvalMode, string> = {
  literal: 'a constant, emitted as the declared type',
  template: 'text with ${ ... } tokens, always a string',
  expression: 'one whole expression, returning a typed value',
};

export default function OutputSchemaEditor({ value, onChange, lookups }: Props) {
  const schema = value ?? {};
  const entries = Object.entries(schema);
  const [newField, setNewField] = useState('');
  const [newMode, setNewMode] = useState<EvalMode>('literal');
  const [newType, setNewType] = useState<FieldType>('string');
  const addRowRef = useRef<HTMLTableRowElement>(null);

  const write = (next: OutputSchema) => onChange(Object.keys(next).length ? next : undefined);

  const remove = (field: string) => {
    const next = { ...schema };
    delete next[field];
    write(next);
  };

  const patch = (field: string, partial: Partial<OutputField>) => {
    const merged: OutputField = { ...schema[field], ...partial };
    // Changing the mode can invalidate the type, so snap it to something legal
    // rather than leaving a declaration the engine will reject at load.
    const allowed = allowedTypesForMode(evalModeOf(merged));
    if (!allowed.includes(merged.type)) merged.type = allowed[0];
    // A lookup binding is only meaningful while the types agree.
    if (merged.lookup) {
      const table = lookups.find((t) => t.id === merged.lookup);
      if (!table || table.keyType !== merged.type) delete merged.lookup;
    }
    write({ ...schema, [field]: merged });
  };

  const add = () => {
    if (!newField || schema[newField]) return;
    const field: OutputField = { type: newType };
    if (newMode !== 'literal') field.eval = newMode;
    // Required deliberately defaults to false: declaring a field must never
    // block a save, or the fast authoring flow stops being usable.
    write({ ...schema, [newField]: field });
    setNewField('');
    setNewMode('literal');
    setNewType('string');
  };

  const handleRowBlur = (e: React.FocusEvent) => {
    if (!newField) return;
    if (addRowRef.current?.contains(e.relatedTarget as Node)) return;
    add();
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      add();
    }
  };

  const newAllowed = allowedTypesForMode(newMode);

  return (
    <div>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Field</th><th>Eval</th><th>Type</th><th>Lookup</th><th>Required</th><th></th>
          </tr>
        </thead>
        <tbody>
          {entries.map(([name, f]) => {
            const mode = evalModeOf(f);
            const err = validateOutputField(name, f, lookups);
            const candidates = lookups.filter((t) => t.keyType === f.type);
            return (
              <tr key={name}>
                <td>
                  {name}
                  {err && (
                    <div style={{ fontSize: 10, color: 'var(--danger, #ef4444)' }}>{err}</div>
                  )}
                </td>
                <td>
                  <select value={mode} onChange={(e) => patch(name, { eval: e.target.value as EvalMode })} title={MODE_HINT[mode]}>
                    {MODES.map((m) => <option key={m} value={m}>{m}</option>)}
                  </select>
                </td>
                <td>
                  <select value={f.type} onChange={(e) => patch(name, { type: e.target.value as FieldType })}>
                    {allowedTypesForMode(mode).map((t) => <option key={t} value={t}>{t}</option>)}
                  </select>
                </td>
                <td>
                  <select
                    value={f.lookup ?? ''}
                    onChange={(e) => patch(name, { lookup: e.target.value || undefined })}
                    disabled={candidates.length === 0}
                    title={candidates.length === 0 ? `No lookup table has key type "${f.type}"` : undefined}
                  >
                    <option value="">—</option>
                    {candidates.map((t) => <option key={t.id} value={t.id}>{t.name}</option>)}
                  </select>
                </td>
                <td>
                  <input
                    type="checkbox"
                    checked={!!f.required}
                    onChange={() => patch(name, { required: !f.required })}
                    style={{ width: 'auto' }}
                    title="Every reporting rule must author this field, or the segment must set it once. Enforced on save."
                  />
                </td>
                <td><button className="btn-danger btn-sm" onClick={() => remove(name)}>x</button></td>
              </tr>
            );
          })}
          <tr ref={addRowRef} onBlur={handleRowBlur}>
            <td>
              <input value={newField} onChange={(e) => setNewField(e.target.value)} onKeyDown={handleKeyDown} placeholder="field name" />
            </td>
            <td>
              <select
                value={newMode}
                onChange={(e) => {
                  const m = e.target.value as EvalMode;
                  setNewMode(m);
                  const allowed = allowedTypesForMode(m);
                  if (!allowed.includes(newType)) setNewType(allowed[0]);
                }}
              >
                {MODES.map((m) => <option key={m} value={m}>{m}</option>)}
              </select>
            </td>
            <td>
              <select value={newType} onChange={(e) => setNewType(e.target.value as FieldType)}>
                {newAllowed.map((t) => <option key={t} value={t}>{t}</option>)}
              </select>
            </td>
            <td colSpan={2} style={{ fontSize: 10, color: 'var(--text-muted)' }}>{MODE_HINT[newMode]}</td>
            <td><button className="btn-primary btn-sm" onClick={add}>+</button></td>
          </tr>
        </tbody>
      </table>
      <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
        Declared fields are optional until you tick Required. Values are authored per check,
        below — or once for the whole segment when they do not vary.
      </p>
    </div>
  );
}
```

- [ ] **Step 3: Wire both into the segment editor**

In `ui/src/components/segments/SegmentEditor.tsx`:

Add the imports beside the existing `InputSchemaEditor` import:

```tsx
import OutputSchemaEditor from '../schema/OutputSchemaEditor';
import EmittedFieldsReference from '../schema/EmittedFieldsReference';
import { supportsOutputSchema } from '../schema/outputSchemaRules';
```

The editor needs the lookup tables. The hook already exists — `useLookups()` in `ui/src/api/lookups.ts`, which queries `/v1/admin/lookups` with query key `['lookups']`. Add the import and call it alongside the existing layers hook:

```tsx
import { useLookups } from '../../api/lookups';
// ...
  const { data: lookups } = useLookups();
```

Give the existing Input Schema section an anchor id so the reference panel can link to it. Change its opening tag to:

```tsx
      <section id="input-schema" className={`card ${styles.section}`}>
```

Then add a new section immediately **after** the Input Schema section and **before** the "Applies When" section:

```tsx
      {/* Output Schema — after the input schema, because an output value
          interpolates the fields declared there. */}
      <section className={`card ${styles.section}`}>
        <h3>Output Schema</h3>
        {supportsOutputSchema(seg.strategy) ? (
          <>
            <p style={{ fontSize: 12, color: 'var(--text-muted)', margin: '0 0 12px' }}>
              Declares the record emitted with each reported item, so a consumer receives a
              populated object instead of mapping one by hand.
            </p>
            <OutputSchemaEditor
              value={seg.outputSchema}
              onChange={(s) => update({ outputSchema: s })}
              lookups={lookups ?? []}
            />
            <div style={{ marginTop: 12 }}>
              <EmittedFieldsReference />
            </div>
            <details style={{ marginTop: 8 }}>
              <summary style={{ cursor: 'pointer', fontSize: 12, color: 'var(--text-muted)' }}>
                Fields available to output values
              </summary>
              <div style={{ fontSize: 11, color: 'var(--text-muted)', marginTop: 8 }}>
                {Object.keys(effectiveSchema(seg) ?? {}).length === 0 ? (
                  <p style={{ margin: 0 }}>
                    None declared yet — <a href="#input-schema">add input fields</a>.
                  </p>
                ) : (
                  <>
                    <p style={{ margin: '0 0 6px' }}>
                      From the <a href="#input-schema">input schema</a> and computed fields:
                    </p>
                    <ul style={{ margin: 0, paddingLeft: 18 }}>
                      {Object.entries(effectiveSchema(seg) ?? {}).map(([f, sf]) => (
                        <li key={f}><code>{f}</code> — {sf.type}</li>
                      ))}
                    </ul>
                  </>
                )}
              </div>
            </details>
          </>
        ) : (
          <p style={{ fontSize: 12, color: 'var(--text-muted)', margin: 0 }}>
            A <code>{seg.strategy}</code> segment resolves a segment value rather than reporting
            an item, so it emits no record and an output schema would do nothing. Output schemas
            apply to <code>checklist</code> and <code>rule</code> segments.
          </p>
        )}
      </section>
```

`effectiveSchema` already exists in this file and merges `inputSchema` with computed fields — reuse it rather than re-deriving the merge.

- [ ] **Step 4: Build and lint**

Run from `ui/`: `npm run build`, then `npm run lint`
Expected: build PASSES. **Lint has a red baseline of exactly 2 pre-existing errors** — `Cannot access refs during render` in `SegmentEditor.tsx:33` and `react-refresh/only-export-components` in `StrategyPicker.tsx:9`. Neither is yours; do not fix them as a drive-by. The gate is that lint still reports **2 problems and no more**, and that neither new error names a file you touched.

- [ ] **Step 5: Verify by hand**

Start the stack (`go run ./cmd/segmentation -config config/segments.json -addr :8080` and `npm run dev` in `ui/`), open a checklist segment such as `employee-readiness`, and confirm:
1. An Output Schema section appears after Input Schema.
2. Adding a field defaults to `literal`/`string` and **Required unticked**.
3. Switching Eval to `template` reduces the Type options to `string` alone; switching to `expression` offers all five including `object`.
4. Setting Type to `object` while in `literal` mode snaps the mode's allowed set — the row shows an error naming `expression` until the mode is changed.
5. The Lookup select offers only tables whose key type matches, and is disabled with a tooltip when none do.
6. Opening `base-tier` (a `static` segment) shows the not-applicable note and no editor.
7. Saving, reloading, and reopening preserves everything.

- [ ] **Step 6: Commit**

```bash
git add ui/src/components/schema/ ui/src/components/segments/SegmentEditor.tsx
git commit -m "feat(ui): add output schema editor and emitted-fields reference"
```

---

### Task 4: Per-item output values

**Files:**
- Create: `ui/src/components/rules/OutputValuesEditor.tsx`
- Create: `ui/src/components/rules/OutputValuesEditor.module.css`
- Modify: `ui/src/components/rules/RuleNode.tsx`
- Modify: `ui/src/components/rules/RuleList.tsx`
- Modify: `ui/src/components/rules/RuleTreeBuilder.tsx`
- Modify: `ui/src/components/segments/RuleConfig.tsx`
- Modify: `ui/src/components/segments/SegmentEditor.tsx`

**Interfaces:**
- Consumes: `OutputSchema`, `OutputField`, `Rule` (Task 1); `evalModeOf`, `validateLiteralValue`, `fieldCoverage` (Task 2).
- Produces: default-exported `OutputValuesEditor` with props `{ outputs?: Record<string, string>; schema: OutputSchema; onChange: (o?: Record<string, string>) => void; onDeclare: (name: string, field: OutputField) => void }`. Two new optional props thread down the rule tree: `outputSchema?: OutputSchema` and `onDeclareOutput?: (name: string, field: OutputField) => void`.

**The authoring flow this enables.** An author writing check number three of fifty-two can type a new field name, give it a value, and tick *add to schema* — the field is declared with `required: false`, so nothing else in the segment becomes invalid and the save is not blocked. Promoting it to Required later is a separate, deliberate act in the schema editor, and that is where the coverage count tells them how many items still need it.

- [ ] **Step 1: Write the editor**

Create `ui/src/components/rules/OutputValuesEditor.module.css`:

```css
.grid {
  display: grid;
  grid-template-columns: minmax(90px, auto) 1fr;
  gap: 4px 8px;
  align-items: start;
  margin-top: 6px;
}
.name {
  font-size: 11px;
  color: var(--text-muted);
  padding-top: 5px;
}
.err {
  font-size: 10px;
  color: var(--danger, #ef4444);
}
.addRow {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-top: 6px;
}
```

Create `ui/src/components/rules/OutputValuesEditor.tsx`:

```tsx
import { useState } from 'react';
import type { OutputField, OutputSchema } from '../../api/types';
import { evalModeOf, validateLiteralValue } from '../schema/outputSchemaRules';
import styles from './OutputValuesEditor.module.css';

interface Props {
  outputs?: Record<string, string>;
  schema: OutputSchema;
  onChange: (o?: Record<string, string>) => void;
  /** Declares a new field on the segment's schema, so it can be authored inline. */
  onDeclare: (name: string, field: OutputField) => void;
}

const PLACEHOLDER: Record<string, string> = {
  literal: 'constant value',
  template: 'text with ${ tokens }',
  expression: 'one whole expression',
};

export default function OutputValuesEditor({ outputs, schema, onChange, onDeclare }: Props) {
  const [newName, setNewName] = useState('');

  const set = (name: string, raw: string) => {
    const next = { ...(outputs ?? {}) };
    if (raw === '') delete next[name];
    else next[name] = raw;
    onChange(Object.keys(next).length ? next : undefined);
  };

  const declareAndFocus = () => {
    if (!newName || schema[newName]) return;
    // Required stays false so declaring a field mid-edit invalidates nothing.
    onDeclare(newName, { type: 'string' });
    setNewName('');
  };

  const names = Object.keys(schema);

  return (
    <div>
      {names.length > 0 && (
        <div className={styles.grid}>
          {names.map((name) => {
            const field = schema[name];
            const raw = outputs?.[name] ?? '';
            const err = validateLiteralValue(field, raw);
            return (
              <div key={name} style={{ display: 'contents' }}>
                <div className={styles.name} title={`${evalModeOf(field)} · ${field.type}${field.required ? ' · required' : ''}`}>
                  {name}{field.required ? ' *' : ''}
                </div>
                <div>
                  <input
                    value={raw}
                    onChange={(e) => set(name, e.target.value)}
                    placeholder={PLACEHOLDER[evalModeOf(field)]}
                  />
                  {err && <div className={styles.err}>{err}</div>}
                </div>
              </div>
            );
          })}
        </div>
      )}
      <div className={styles.addRow}>
        <input
          value={newName}
          onChange={(e) => setNewName(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault();
              declareAndFocus();
            }
          }}
          placeholder="new output field"
          style={{ fontSize: 11 }}
        />
        <button className="btn-secondary btn-sm" onClick={declareAndFocus} disabled={!newName || !!schema[newName]}>
          + add to schema
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Thread the props down the rule tree**

Each of these files already threads `schema` and `perRuleMessages` through in exactly this shape — add the two new props alongside them, changing nothing else.

In `ui/src/components/rules/RuleNode.tsx`, add to `Props`:

```tsx
  /** The segment's output schema. Present only when the segment declares one. */
  outputSchema?: OutputSchema;
  onDeclareOutput?: (name: string, field: OutputField) => void;
```

Add both to the destructured parameter list, import the types and the editor, and render it inside the leaf/reporting branch — immediately after the `errorMessage` input, guarded so inner And/Or branches never get one:

```tsx
        {/* Only a reporting rule emits a record, so only a reporting rule gets
            output values. An inner And/Or branch never reports. */}
        {outputSchema && onDeclareOutput && (perRuleMessages || !isLeaf) && (
          <OutputValuesEditor
            outputs={rule.outputs}
            schema={outputSchema}
            onChange={(o) => onChange({ ...rule, outputs: o })}
            onDeclare={onDeclareOutput}
          />
        )}
```

Pass both props through `RuleList.tsx` to each `RuleNode`, and through `RuleTreeBuilder.tsx` to `RuleList`, mirroring how `schema` already flows.

In `ui/src/components/segments/RuleConfig.tsx`, add the same two props and forward them to the **rules** `RuleTreeBuilder` only — not the overrides one. Overrides can carry outputs in the engine, but leave them out of this task: the overrides editor is a separate surface and adding it here would widen the diff without a requirement asking for it.

In `ui/src/components/segments/SegmentEditor.tsx`, define the declare handler once and pass it to both `RuleConfig` (rule strategy) and the checklist `RuleTreeBuilder`:

```tsx
  const declareOutput = (name: string, field: OutputField) =>
    update({ outputSchema: { ...(seg.outputSchema ?? {}), [name]: field } });
```

Pass `outputSchema={seg.outputSchema}` and `onDeclareOutput={declareOutput}` at both call sites. Import `OutputField` and `OutputSchema` types as needed.

- [ ] **Step 3: Add the segment-level values and coverage to the schema editor**

A required field is satisfied for every path at once by a segment-level value, which is the cheapest way to fill one across fifty-two items. Surface it where the author is looking at Required.

In `ui/src/components/schema/OutputSchemaEditor.tsx`, extend `Props`:

```tsx
  /** Values for fields that do not vary per item, and the segment's coverage. */
  segmentOutputs?: Record<string, string>;
  onSegmentOutputsChange?: (o?: Record<string, string>) => void;
  coverage?: (name: string) => { authored: number; total: number; segmentLevel: boolean };
```

Add a column rendering, per row, the segment-level value input and the coverage summary:

```tsx
                <td>
                  {onSegmentOutputsChange && (
                    <input
                      value={segmentOutputs?.[name] ?? ''}
                      onChange={(e) => {
                        const next = { ...(segmentOutputs ?? {}) };
                        if (e.target.value === '') delete next[name];
                        else next[name] = e.target.value;
                        onSegmentOutputsChange(Object.keys(next).length ? next : undefined);
                      }}
                      placeholder="set once for the segment"
                      style={{ fontSize: 11 }}
                    />
                  )}
                  {coverage && (() => {
                    const c = coverage(name);
                    if (c.segmentLevel) {
                      return <div style={{ fontSize: 10, color: 'var(--text-muted)' }}>set for the whole segment</div>;
                    }
                    const short = c.total - c.authored;
                    return (
                      <div style={{ fontSize: 10, color: short && f.required ? 'var(--danger, #ef4444)' : 'var(--text-muted)' }}>
                        authored on {c.authored} of {c.total} checks
                        {short && f.required ? ` — ${short} will block saving` : ''}
                      </div>
                    );
                  })()}
                </td>
```

Add a matching `<th>Segment value</th>` to the header row, and a matching empty `<td>` to the add row so the column counts stay aligned. Adjust the add row's `colSpan` accordingly.

Wire it from `SegmentEditor.tsx`:

```tsx
            <OutputSchemaEditor
              value={seg.outputSchema}
              onChange={(s) => update({ outputSchema: s })}
              lookups={lookups ?? []}
              segmentOutputs={seg.outputs}
              onSegmentOutputsChange={(o) => update({ outputs: o })}
              coverage={(name) => fieldCoverage(seg, name)}
            />
```

Import `fieldCoverage` from `../schema/outputSchemaRules`.

- [ ] **Step 4: Build and lint**

Run from `ui/`: `npm run build`, then `npm run lint`
Expected: build PASSES. **Lint has a red baseline of exactly 2 pre-existing errors** — `Cannot access refs during render` in `SegmentEditor.tsx:33` and `react-refresh/only-export-components` in `StrategyPicker.tsx:9`. Neither is yours; do not fix them as a drive-by. The gate is that lint still reports **2 problems and no more**, and that neither new error names a file you touched.

- [ ] **Step 5: Verify by hand**

On a checklist segment with an output schema:
1. Each check shows an input per declared field, placeholdered by eval mode.
2. Typing a name and pressing *+ add to schema* declares it with Required unticked, and it appears immediately in both the schema table and every check's value list.
3. Typing `high` into a field declared `literal`/`number` shows the parse error inline.
4. Nested And/Or branches show **no** output inputs.
5. The schema table's coverage reads "authored on N of M checks", and ticking Required turns a shortfall red with the blocking note.
6. Typing a segment-level value flips the row to "set for the whole segment".
7. Save round-trips everything.

- [ ] **Step 6: Commit**

```bash
git add ui/src/components/rules/ ui/src/components/segments/ ui/src/components/schema/OutputSchemaEditor.tsx
git commit -m "feat(ui): author output values per check, declare fields inline"
```

---

### Task 5: Lookup ordering, description, and the two flags

**Files:**
- Modify: `ui/src/components/lookups/LookupForm.tsx`
- Modify: `ui/src/components/lookups/LookupForm.module.css`

**Interfaces:**
- Consumes: `LookupEntry.order`, `LookupTable.description`/`emitOrder`/`customOrder` (Task 1).
- Produces: nothing consumed by later tasks.

**The two flags are independent, and all four combinations are meaningful.** A table may be ordered for admin display without emitting the number, and inferred positions may be emitted without being hand-authored. Keeping them separate avoids forcing an author to hand-number a table merely to get the number into the response.

**Drag-and-drop belongs to inferred mode only.** Once custom numbers are allowed, the list is authored by number and reordering by drag is disabled — two editing models, rather than a hybrid that fights itself. Entering custom mode seeds the numbers from current list position, so the author starts where the list already sits and the first save changes nothing. Leaving custom mode discards the authored numbers in favour of list position.

- [ ] **Step 1: Add the description field and the two flags**

Read `ui/src/components/lookups/LookupForm.tsx` first to find the existing state hooks and the form layout. Add state mirroring the existing `entries` hook:

```tsx
  const [description, setDescription] = useState(initial?.description ?? '');
  const [emitOrder, setEmitOrder] = useState(!!initial?.emitOrder);
  const [customOrder, setCustomOrder] = useState(!!initial?.customOrder);
```

Add the controls above the entries list:

```tsx
      <div className="form-group">
        <label>Description</label>
        <textarea
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          rows={2}
          placeholder="How this table is meant to be used — including any cross-table ordering scheme"
        />
        <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
          Ordering invariants are documented here rather than validated. Nothing checks that
          numbers are unique or contiguous — gaps are the mechanism for interleaving several
          tables into one ordering.
        </p>
      </div>

      <div className="form-group">
        <label style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
          <input type="checkbox" checked={emitOrder} onChange={(e) => setEmitOrder(e.target.checked)} style={{ width: 'auto' }} />
          Emit order in the evaluation response
        </label>
        <label style={{ display: 'flex', gap: 6, alignItems: 'center', marginTop: 6 }}>
          <input
            type="checkbox"
            checked={customOrder}
            onChange={(e) => {
              const on = e.target.checked;
              setCustomOrder(on);
              setEntries((es) =>
                on
                  // Seed from current position so the first save changes nothing.
                  ? es.map((entry, i) => ({ ...entry, order: entry.order ?? i }))
                  // Leaving custom mode discards authored numbers for position.
                  : es.map(({ order: _order, ...rest }) => rest),
              );
            }}
            style={{ width: 'auto' }}
          />
          Hand-author order numbers
        </label>
        <p style={{ fontSize: 11, color: 'var(--text-muted)', margin: '4px 0 0' }}>
          {customOrder
            ? 'The list is authored by number, so drag-to-reorder is off. Numbers may skip — that is how one ordering spans several tables.'
            : 'Order is inferred from list position. Turn this on to hand-author the numbers instead.'}
        </p>
      </div>
```

- [ ] **Step 2: Add the order column and gate reordering**

Give each entry row an order input, shown only in custom mode:

```tsx
            {customOrder && (
              <input
                type="number"
                value={entry.order ?? i}
                onChange={(e) => setEntries((es) =>
                  es.map((x, j) => (j === i ? { ...x, order: Number(e.target.value) } : x)),
                )}
                style={{ width: 70 }}
                aria-label={`order for entry ${i + 1}`}
              />
            )}
```

Add a matching column label beside the existing `Value (description, optional)` label at `LookupForm.tsx:82`, shown under the same `customOrder` condition.

**There is nothing to gate for drag-and-drop.** `LookupForm` has no drag handles and no move buttons — entry editing is `updateKey`, `removeEntry` and `addEntry` only. So the "drag-and-drop applies to inferred mode only" rule is satisfied vacuously today; the seeding and collision behaviour above is the whole of this task's ordering work. Do not add reordering controls in order to then disable them.

Fix the entry constructor at `LookupForm.tsx:42` so a new row in a custom-ordered table does not land on `0` alongside an existing entry:

```tsx
  const addEntry = () =>
    setEntries((es) => [
      ...es,
      customOrder
        // Continue past the current maximum rather than colliding on 0.
        ? { key: '', value: '', order: es.reduce((m, e) => Math.max(m, e.order ?? 0), -1) + 1 }
        : { key: '', value: '' },
    ]);
```

- [ ] **Step 3: Include the new fields in the submitted payload**

The form submits at `LookupForm.tsx:49`. Extend that call — omitting any field silently discards the author's input, because the whole table is replaced on save:

```tsx
          onSubmit({ id: initial?.id, name, keyType, description, emitOrder, customOrder, entries });
```

Note the existing state hooks read from a prop named `initial`, not `table`, so the Step 1 hooks must be `initial?.description`, `initial?.emitOrder` and `initial?.customOrder`.

- [ ] **Step 4: Build and lint**

Run from `ui/`: `npm run build`, then `npm run lint`
Expected: build PASSES. **Lint has a red baseline of exactly 2 pre-existing errors** — `Cannot access refs during render` in `SegmentEditor.tsx:33` and `react-refresh/only-export-components` in `StrategyPicker.tsx:9`. Neither is yours; do not fix them as a drive-by. The gate is that lint still reports **2 problems and no more**, and that neither new error names a file you touched.

- [ ] **Step 5: Verify by hand**

1. Create a table with two entries, leave both flags off, save, and confirm via `GET /v1/admin/lookups` that each entry has `order` 0 and 1 — the engine infers position on write.
2. Tick *Hand-author order numbers*: the order inputs appear, pre-seeded 0 and 1.
3. Change them to 1 and 5, save, reopen, and confirm both survived — the gap is preserved, not normalised.
4. Add an entry while in custom mode and confirm it gets 6, not 0.
5. Untick the flag, save, and confirm the numbers revert to list position.
6. Tick *Emit order*, then evaluate a segment with a lookup-bound output field and confirm the emitted record's field carries `order`. Untick it and confirm `order` disappears while `key` and `value` remain.

- [ ] **Step 6: Commit**

```bash
git add ui/src/components/lookups/
git commit -m "feat(ui): author lookup order, description, and the two order flags"
```

---

### Task 6: Show emitted records in the testing zone

**Files:**
- Modify: `ui/src/components/testing/ResultDisplay.tsx`
- Modify: `ui/src/components/testing/ResultDisplay.module.css`

**Interfaces:**
- Consumes: `Failure.outputs`, and the layer result's `outputs` (Task 1).
- Produces: nothing.

**Why this is its own task.** Everything above authors config; nothing yet lets the author *see* the record their schema produces. Without this, the only way to check the work is a raw `curl`.

- [ ] **Step 1: Render outputs on each failure**

In `ui/src/components/testing/ResultDisplay.tsx`, inside the `lr.failures.map(...)` list item, after the existing `f.messages` block, add:

```tsx
                  {f.outputs && Object.keys(f.outputs).length > 0 && (
                    <div className={styles.outputs}>
                      {Object.entries(f.outputs).map(([k, v]) => (
                        <div key={k} className={styles.output}>
                          <code>{k}</code>
                          <span>{typeof v === 'object' && v !== null ? JSON.stringify(v) : String(v)}</span>
                        </div>
                      ))}
                    </div>
                  )}
```

A lookup-bound field emits `{key, value, order}` and an `expression` field may emit any object, so stringify objects rather than assuming a scalar.

- [ ] **Step 2: Render outputs on the layer result**

After the existing `lr.messages` block, add the same treatment for a single-value strategy's record:

```tsx
          {lr.outputs && Object.keys(lr.outputs).length > 0 && (
            <div className={styles.outputs}>
              {Object.entries(lr.outputs).map(([k, v]) => (
                <div key={k} className={styles.output}>
                  <code>{k}</code>
                  <span>{typeof v === 'object' && v !== null ? JSON.stringify(v) : String(v)}</span>
                </div>
              ))}
            </div>
          )}
```

- [ ] **Step 3: Add the styles**

In `ui/src/components/testing/ResultDisplay.module.css`, add rules modelled on the existing `.messages` / `.message` pair so the two read consistently:

```css
.outputs {
  margin-top: 4px;
  padding-left: 10px;
  border-left: 2px solid var(--border, #334155);
}
.output {
  display: flex;
  gap: 6px;
  font-size: 11px;
  color: var(--text-muted);
}
```

- [ ] **Step 4: Build and lint**

Run from `ui/`: `npm run build`, then `npm run lint`
Expected: build PASSES. **Lint has a red baseline of exactly 2 pre-existing errors** — `Cannot access refs during render` in `SegmentEditor.tsx:33` and `react-refresh/only-export-components` in `StrategyPicker.tsx:9`. Neither is yours; do not fix them as a drive-by. The gate is that lint still reports **2 problems and no more**, and that neither new error names a file you touched.

- [ ] **Step 5: Verify by hand**

Evaluate a subject against a checklist segment carrying an output schema with all three eval modes plus a lookup-bound field. Confirm each reported finding lists its resolved fields, and that the lookup-bound one renders as an object with `key`, `value` and — when `emitOrder` is on — `order`. Then break one field's expression deliberately and confirm the field is **absent** from the record while the finding still appears, and that a warning about it shows in the warnings list.

- [ ] **Step 6: Commit**

```bash
git add ui/src/components/testing/
git commit -m "feat(ui): show emitted output records in the testing zone"
```

---

## Out of scope for this plan

- **Authoring outputs on overrides.** The engine resolves them, and `Rule.outputs` carries them, but the overrides editor is a separate surface. Task 4 deliberately wires per-item values into the rules tree only.
- **A JSON view or import/export UI.** `POST /v1/admin/import` and `GET /v1/admin/export` already exist for bulk work.
- **Validating expression syntax client-side.** The engine syntax-checks expression-mode values at snapshot load and returns the error on save. Duplicating expr-lang in the browser is not worth it.
- **Localizing output values.** Output fields have no per-language variants; only messages do.
