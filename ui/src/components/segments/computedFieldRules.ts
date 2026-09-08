import type { ComputedField } from '../../api/types';

/**
 * Computed fields evaluate top to bottom, each one seeing the fields above it
 * and nothing below (internal/domain/strategy/formula.go, enrichWithComputed).
 * So position is not presentation — it decides what a formula can read.
 */

/** Escapes a field name for use inside a RegExp — a name is author-typed and
 *  may hold characters the engine tolerates but a pattern does not. */
function escapeForPattern(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * For each field, the names it appears to reference that are declared *below*
 * it — the references that cannot resolve, because the value does not exist
 * yet when the formula runs.
 *
 * A hint rather than a verdict: the match is a word-boundary search over the
 * formula text, so a name mentioned inside a string literal counts as a
 * reference when it is not one. That errs toward pointing at a real ordering
 * problem and occasionally at a harmless one, which is the right way round for
 * something whose only consequence is a line of explanatory text — the engine
 * remains the authority.
 */
export function forwardReferences(defs: ComputedField[]): Map<number, string[]> {
  const out = new Map<number, string[]>();
  defs.forEach((def, i) => {
    if (!def.formula) return;
    const later: string[] = [];
    for (let j = i + 1; j < defs.length; j++) {
      const name = defs[j].name;
      if (!name) continue;
      if (new RegExp(`\\b${escapeForPattern(name)}\\b`).test(def.formula)) {
        later.push(name);
      }
    }
    if (later.length) out.set(i, later);
  });
  return out;
}
