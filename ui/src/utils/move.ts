/**
 * Moves the item at `from` to `to`, returning a new array.
 *
 * Out-of-range indices return the input unchanged, so a caller need not guard
 * the ends of the list — the first row's "up" and the last row's "down" are
 * no-ops rather than corruptions.
 *
 * Shared because more than one list's order is load-bearing: a computed field
 * can only read the fields above it, and a lookup table's emitted order is its
 * list position unless the numbers are hand-authored.
 */
export function moveItem<T>(items: T[], from: number, to: number): T[] {
  if (from === to) return items;
  if (from < 0 || from >= items.length) return items;
  if (to < 0 || to >= items.length) return items;
  const next = [...items];
  const [moved] = next.splice(from, 1);
  next.splice(to, 0, moved);
  return next;
}
