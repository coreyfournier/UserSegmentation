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
 * Go's `strconv.ParseFloat` grammar, in its decimal forms.
 *
 * Deliberately NOT `Number()`. `Number()` is looser than ParseFloat in ways
 * that matter here: it accepts `" 42"`, `"42 "`, `"0x10"` and whitespace-only
 * strings (as 0), every one of which ParseFloat rejects. The engine calls
 * ParseFloat, so using `Number()` would let the editor bless a literal the
 * save then rejects — precisely the failure this module exists to prevent.
 *
 * It is marginally stricter than ParseFloat in one respect: Go accepts
 * underscore separators (`1_0`) and the Inf/NaN spellings. Both are
 * vanishingly rare in an authored constant, and stricter-in-the-editor is the
 * safe direction — the author simply types an ordinary number.
 */
const DECIMAL_FLOAT = /^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$/;

/**
 * Exactly the set `strconv.ParseBool` accepts. Not just "true"/"false":
 * accepting fewer spellings than the engine would reject a literal that loads
 * perfectly well.
 */
const GO_BOOLS = new Set([
  '1', 't', 'T', 'TRUE', 'true', 'True',
  '0', 'f', 'F', 'FALSE', 'false', 'False',
]);

/**
 * Checks an authored literal against its declared type.
 *
 * Only literal mode is checked. An empty value means "not authored" — the
 * engine treats it that way too — so it is not an invalid literal.
 */
export function validateLiteralValue(field: OutputField, raw: string): string | null {
  if (evalModeOf(field) !== 'literal' || raw === '') return null;
  // The regex settles syntax; isFinite settles magnitude. ParseFloat(_, 64)
  // returns ErrRange for a syntactically valid literal that overflows a
  // float64 — "1e999" parses to +Inf and errors — and the engine treats any
  // non-nil error as a rejection. Underflow is not an error there ("1e-999"
  // yields 0 with err nil), and Number() agrees on both, so this one extra
  // condition matches Go exactly at the range boundary.
  if (field.type === 'number' && (!DECIMAL_FLOAT.test(raw) || !Number.isFinite(Number(raw)))) {
    return `"${raw}" does not parse as a number`;
  }
  if (field.type === 'boolean' && !GO_BOOLS.has(raw)) {
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
  /** A rule segment with a default reads no rule values on that path, so only
   *  a segment-level value can satisfy the field. */
  defaultNeedsSegmentValue: boolean;
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
 */
export function fieldCoverage(seg: Segment, name: string): FieldCoverage {
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
    defaultNeedsSegmentValue: seg.strategy === 'rule' && !!seg.default,
  };
}
