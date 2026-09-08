import type { FieldType, InputSchema, Operator, Rule, Segment } from '../../api/types';
import { LOOKUP_OPERATORS, OPERATOR_TYPES, UNARY_OPERATORS } from '../../api/types';

/**
 * Which operators a field type admits, restated for the editor.
 *
 * The engine is the authority (model.OperatorTypes): a condition whose operator
 * does not admit its field's type is refused at load. These exist so the editor
 * can keep an author from building that config in the first place, and can say
 * so about config that already has.
 */

/** Whether `op` can be applied to a field of `type`. Unknown type means unknown
 *  field, and nothing is claimed about it. */
export function operatorSupports(op: Operator, type: FieldType | undefined): boolean {
  if (!type) return true;
  const allowed = OPERATOR_TYPES[op];
  return !allowed || allowed.includes(type);
}

/** One condition the editor cannot represent or the engine will refuse. */
export interface OperatorBreak {
  segment: string;
  rule: string;
  operator: Operator;
}

function walkRule(
  r: Rule,
  segmentID: string,
  field: string,
  next: FieldType,
  out: OperatorBreak[],
) {
  const c = r.condition;
  // A cross-layer reference takes its type from the other layer, not from the
  // field being retyped here.
  if (c && c.field === field && !c.field.startsWith('layer:') && !operatorSupports(c.operator, next)) {
    out.push({ segment: segmentID, rule: r.ruleName || '(unnamed)', operator: c.operator });
  }
  for (const child of r.rules ?? []) walkRule(child, segmentID, field, next, out);
}

/**
 * Conditions on `field` that retyping it to `next` would invalidate, across a
 * segment's rules, overrides and dispatch predicate — at every depth, since
 * only a leaf names a field.
 *
 * This is what turns a save-time rejection into something an author is told
 * before they cause it. The rejection names a rule they may not have touched
 * in the session that broke it: "10 >= 1" is a comparison, so a field holding
 * it is easily left as a number while a rule compares it with gte, and only
 * later declared the boolean it always was.
 */
export function segmentRetypeBreaks(seg: Segment, field: string, next: FieldType): OperatorBreak[] {
  const out: OperatorBreak[] = [];
  for (const r of seg.rules ?? []) walkRule(r, seg.id, field, next, out);
  for (const r of seg.overrides ?? []) walkRule(r, seg.id, field, next, out);
  if (seg.when) walkRule(seg.when, seg.id, field, next, out);
  return out;
}

/** The same, across every segment in a layer. */
export function layerRetypeBreaks(
  segments: Segment[],
  field: string,
  next: FieldType,
): OperatorBreak[] {
  return segments.flatMap((seg) => segmentRetypeBreaks(seg, field, next));
}

/** Renders a break the way both warnings phrase it. */
export function describeBreak(b: OperatorBreak): string {
  return `segment "${b.segment}" rule "${b.rule}" uses ${b.operator}`;
}

/** Every operator, in the order the picker lists them. */
export const ALL_OPERATORS: Operator[] = [
  'eq', 'neq', 'gt', 'gte', 'lt', 'lte', 'in', 'not_in', 'contains', 'in_lookup', 'not_in_lookup',
  'is_null', 'is_null_or_empty',
];

/**
 * The options an operator picker must offer, given what is currently stored.
 *
 * The invariant is that the list always contains `value`. Offering only the
 * compatible operators is not enough, and the failure is worse than untidy: a
 * select whose value matches no option displays the *first* one, so a stored
 * gte on a boolean field reads as "eq". Worse, choosing eq then changes
 * nothing the browser can see, so no change event fires and the stored gte
 * cannot be corrected through the picker at all — the config keeps a value the
 * UI insists is not there, until the engine refuses the save.
 */
export function operatorOptions(
  value: Operator,
  fieldType: FieldType | undefined,
): { op: Operator; compatible: boolean }[] {
  const compatible = ALL_OPERATORS.filter((op) => operatorSupports(op, fieldType));
  if (compatible.includes(value)) {
    return compatible.map((op) => ({ op, compatible: true }));
  }
  // The stranded value leads, so it is what the closed select displays.
  return [{ op: value, compatible: false }, ...compatible.map((op) => ({ op, compatible: true }))];
}

/**
 * Operators that cannot compare against another field. Mirrors
 * validation.validateValueRef: a unary operator tests its field and takes no
 * right-hand side at all, and a lookup operator's value is a table id, which
 * is a literal by definition.
 */
export function supportsValueField(op: Operator): boolean {
  return !UNARY_OPERATORS.includes(op) && !LOOKUP_OPERATORS.includes(op);
}

/**
 * The fields a condition may be compared against, given its left-hand field
 * and operator.
 *
 * Mirrors validation.validateValueRef exactly, and the mirroring is the point:
 * every option this omits is a save the engine would refuse, and every option
 * it offers must be one it accepts. The rules are not simply "same type" —
 *
 *   in / not_in   the right-hand side is the list, so it must be an array
 *   contains      over an array it is one element, and an array's element type
 *                 is not declared, so anything goes; over a string it is a
 *                 substring, which the same-type rule already covers
 *   otherwise     both sides are compared as they are, so the types must agree
 *
 * The left field itself is never offered: comparing a field to itself is
 * constant, and validation rejects it.
 */
export function valueFieldOptions(
  schema: InputSchema | undefined,
  leftField: string,
  operator: Operator,
): string[] {
  if (!supportsValueField(operator)) return [];
  const leftType = schema?.[leftField]?.type;
  return Object.entries(schema ?? {})
    .filter(([name, sf]) => {
      if (name === leftField) return false;
      if (operator === 'in' || operator === 'not_in') return sf.type === 'array';
      if (operator === 'contains' && leftType === 'array') return true;
      // An unknown left type cannot constrain anything — the layer declares no
      // schema for it, and validation skips the check for the same reason.
      return leftType === undefined || sf.type === leftType;
    })
    .map(([name]) => name)
    .sort((a, b) => a.localeCompare(b));
}
