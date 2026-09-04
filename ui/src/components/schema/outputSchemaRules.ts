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
