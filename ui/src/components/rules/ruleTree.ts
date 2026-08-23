import type { Rule } from '../../api/types';

/**
 * Address of a node in the rule tree: the index at each level, from the root
 * array down. `[0, 2]` is `rules[0].rules[2]`.
 *
 * Rule editing used to run through parent-owned closures, which can only ever
 * reorder siblings. Addressing nodes by path is what lets an expression move
 * into a group, out of one, or across to another.
 */
export type RulePath = number[];

/** Marks the original node during a move so it can be removed by identity. */
const MOVING = '__moving';

export function samePath(a: RulePath, b: RulePath): boolean {
  return a.length === b.length && a.every((v, i) => b[i] === v);
}

/** True when `to` addresses something inside the subtree rooted at `from`. */
export function isInsideSubtree(from: RulePath, to: RulePath): boolean {
  return to.length > from.length && from.every((v, i) => to[i] === v);
}

/** Reads the node at `path`, or null when the path does not resolve. */
export function nodeAt(rules: Rule[], path: RulePath): Rule | null {
  let list: Rule[] | undefined = rules;
  let node: Rule | undefined;
  for (const index of path) {
    if (!list) return null;
    node = list[index];
    if (!node) return null;
    list = node.rules;
  }
  return node ?? null;
}

/**
 * Resolves the child array at `path`, creating empty arrays along the way.
 * Only ever called on a private clone, never on live state.
 */
function ensureListAt(rules: Rule[], path: RulePath): Rule[] | null {
  let list: Rule[] = rules;
  for (const index of path) {
    const node = list[index];
    if (!node) return null;
    if (!node.rules) node.rules = [];
    list = node.rules;
  }
  return list;
}

/**
 * Whether the node at `from` may be inserted at `to`.
 *
 * `to` is an insertion point: the last element is the index the node would take
 * within the parent addressed by the rest of the path.
 */
export function canDrop(rules: Rule[], from: RulePath, to: RulePath): boolean {
  if (from.length === 0 || to.length === 0) return false;

  // A node cannot be placed inside itself — that would detach the subtree.
  if (samePath(from, to) || isInsideSubtree(from, to)) return false;

  // Only the root list and groups hold children; a leaf has no interior.
  const toParent = to.slice(0, -1);
  if (toParent.length > 0) {
    const parent = nodeAt(rules, toParent);
    if (!parent || parent.expression) return false;
  }

  // Landing immediately before or after itself among the same siblings would
  // not move anything.
  if (samePath(from.slice(0, -1), toParent)) {
    const fromIndex = from[from.length - 1];
    const toIndex = to[to.length - 1];
    if (toIndex === fromIndex || toIndex === fromIndex + 1) return false;
  }

  return true;
}

/**
 * Returns a new tree with the node at `from` moved to the insertion point `to`.
 * Returns the original array unchanged when the move is not allowed.
 *
 * The node is inserted first and the original removed afterwards by marker
 * rather than by index. Inserting shifts sibling indices, so removing by index
 * would need correction arithmetic that differs depending on whether source and
 * destination share a parent, and on which comes first. Identity sidesteps all
 * of it.
 */
export function moveRule(rules: Rule[], from: RulePath, to: RulePath): Rule[] {
  if (!canDrop(rules, from, to)) return rules;

  const tree: Rule[] = structuredClone(rules);

  const sourceParent = ensureListAt(tree, from.slice(0, -1));
  const source = sourceParent?.[from[from.length - 1]];
  if (!sourceParent || !source) return rules;

  // Clone before marking so the copy that lands at the destination is clean.
  const moved: Rule = structuredClone(source);
  (source as unknown as Record<string, unknown>)[MOVING] = true;

  const destParent = ensureListAt(tree, to.slice(0, -1));
  if (!destParent) return rules;

  const index = Math.min(to[to.length - 1], destParent.length);
  destParent.splice(index, 0, moved);

  return stripMoved(tree);
}

function stripMoved(rules: Rule[]): Rule[] {
  return rules
    .filter((r) => !(r as unknown as Record<string, unknown>)[MOVING])
    .map((r) => (r.rules ? { ...r, rules: stripMoved(r.rules) } : r));
}

/** Human-readable description of a node, for drag feedback. */
export function describeRule(rule: Rule): string {
  if (rule.expression) {
    const { field, operator } = rule.expression;
    return rule.ruleName || `${field || 'field'} ${operator}`;
  }
  return rule.ruleName || `${rule.operator ?? 'And'} group`;
}
