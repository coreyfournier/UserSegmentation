import type { FieldType, Operator, Rule, Segment } from '../../api/types';
import { OPERATOR_TYPES } from '../../api/types';

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
