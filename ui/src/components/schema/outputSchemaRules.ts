import type {
  ComputedField,
  FieldType,
  LookupTable,
  OutputField,
  OutputSchema,
  Rule,
  Segment,
  StrategyType,
} from '../../api/types';

/**
 * The engine's rules for a valid output declaration, restated for the editor.
 *
 * These mirror validation the engine performs at snapshot load.
 */

/** Every type an output field may declare, shared by the schema editor (which
 *  declares fields) and the values editor (which declares one inline). */
export const FIELD_TYPES: FieldType[] = ['string', 'number', 'boolean', 'array', 'object'];

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

/** Returns an error message, or null when the declaration is valid. */
export function validateOutputField(
  name: string,
  field: OutputField,
  lookups: LookupTable[],
): string | null {
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
  /** Enabled top-level rules that author this field. */
  authored: number;
  /** Enabled top-level rules in total. */
  total: number;
  /** A non-empty segment-level value, which covers every path at once. */
  segmentLevel: boolean;
  /** Enabled overrides, and how many author it. Values cannot be authored on
   *  overrides in this UI, so any shortfall needs the segment-level value. */
  overridesAuthored: number;
  overridesTotal: number;
  /** A rule segment with a default that has not authored this field itself.
   *  The default authors its own values now (Segment.defaultOutputs), so this
   *  is a shortfall to fill on the default — not, as it once was, a demand for
   *  a segment-level value that would then apply to every rule too. */
  defaultUnauthored: boolean;
}

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
  // `?? ''` because `from` may not be in the map at all: a required field is
  // always shown as a row while being absent from `outputs`, so repointing that
  // row carries an undefined value — and a key whose value is undefined is
  // dropped by JSON.stringify, so the row the author just created would
  // disappear on save.
  if (to) next[to] = value ?? '';
  return Object.keys(next).length ? next : undefined;
}

/**
 * How completely a field is authored across the segment.
 *
 * Only top-level rules are counted, because only they report. Disabled rules
 * are excluded, matching the engine's gate, which exempts them so a
 * work-in-progress item cannot block an unrelated save.
 *
 * Enabled overrides are also counted, matching the engine's own
 * requiredOutputErrors: an override that fires replaces the strategy result
 * entirely, so it carries the same reporting obligation as a rule. But this
 * editor never wires per-item values into the overrides tree, so an override
 * cannot author the field itself — any shortfall there can only be closed by
 * the segment-level value, which is why overridesAuthored/overridesTotal are
 * reported separately rather than folded into authored/total.
 *
 * A rule-strategy segment with a non-empty default is a separate case: the
 * default path calls evaluateOutputs with no rule values at all, so no number
 * of authored rules can satisfy a required field on it — only the
 * segment-level value can.
 *
 * `schema` is the declaration this field lives in — the layer's output
 * schema now, never the segment's — passed explicitly rather than read off
 * `seg`, since a `Segment` no longer carries one. It gates
 * `defaultNeedsSegmentValue`: a name no longer declared in the schema
 * currently in force has no engine obligation to be read regardless of the
 * segment's default, so a stale coverage call (e.g. mid-edit, just after a
 * field was removed from the layer) does not spuriously report that it still
 * needs one.
 */
export function fieldCoverage(seg: Segment, schema: OutputSchema | undefined, name: string): FieldCoverage {
  const reporting = (seg.rules ?? []).filter((r: Rule) => r.enabled !== false);
  const authored = reporting.filter((r) => !!r.outputs?.[name]).length;
  const overrides = (seg.overrides ?? []).filter((r: Rule) => r.enabled !== false);
  const overridesAuthored = overrides.filter((r) => !!r.outputs?.[name]).length;
  return {
    authored,
    total: reporting.length,
    segmentLevel: !!seg.outputs?.[name],
    overridesAuthored,
    overridesTotal: overrides.length,
    defaultUnauthored:
      seg.strategy === 'rule' && !!seg.default && !!schema?.[name] && !seg.defaultOutputs?.[name],
  };
}

/**
 * The computed field that could supply an output field's value: same name,
 * same type.
 *
 * Matching is on both, never on name alone. A computed `TransferFee` that is a
 * number and an output `TransferFee` declared a string are not the same thing,
 * and quietly wiring them together would emit the number's string form as if
 * that had been intended.
 *
 * This is a suggestion the UI acts on when told to, not something the engine
 * does at evaluation. The value it fills in is an ordinary expression written
 * into the config, so what runs is what an author can read — nothing is
 * resolved by a rule that only exists in the editor.
 */
export function matchingComputedField(
  name: string,
  field: OutputField | undefined,
  computed: ComputedField[] | undefined,
): ComputedField | undefined {
  if (!field) return undefined;
  return (computed ?? []).find((c) => c.name === name && c.type === field.type);
}

/**
 * Every output field on this segment that a computed field could supply and
 * that has no value at the given site yet. Drives the "fill these in" action,
 * which writes the mapping rather than implying it.
 */
export function unfilledComputedMatches(
  schema: OutputSchema | undefined,
  computed: ComputedField[] | undefined,
  values: Record<string, string> | undefined,
): ComputedField[] {
  if (!schema || !computed?.length) return [];
  return Object.entries(schema)
    .filter(([name]) => !values?.[name])
    .map(([name, field]) => matchingComputedField(name, field, computed))
    .filter((c): c is ComputedField => !!c);
}
