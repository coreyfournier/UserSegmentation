/**
 * Parses a raw string to a number, but returns the raw string unchanged for
 * intermediate typing states (trailing dot, lone minus sign) so inputs remain
 * usable while the user is mid-entry.
 */
export function parseNumericInput(raw: string): number | string {
  const trimmed = raw.trim();
  if (trimmed === '' || trimmed === '-' || trimmed.endsWith('.')) return raw;
  const n = Number(trimmed);
  return isNaN(n) ? raw : n;
}

/**
 * Parses what was typed into a context field to the value the engine will see.
 *
 * Boolean is deliberately absent: a two-valued field is a picker, not a text
 * box, and the parse that made it one (`raw === 'true'`) is what made it
 * impossible to type — the "t" of "true" became false, the box repainted as
 * "false", and true was then unreachable.
 */
export function parseContextValue(type: string | undefined, raw: string): unknown {
  if (type === 'number') return parseNumericInput(raw);
  if (type === 'array') {
    return raw
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
  }
  return raw;
}

/**
 * Renders a context value back into its text box.
 *
 * Paired with parseContextValue, and the pairing is the thing to get right: an
 * editor whose box always shows `display(parse(text))` can only be typed into
 * while that composition returns what was typed. It does for strings and
 * numbers. It does not for arrays — `"a,"` parses to `["a"]`, which renders as
 * `"a"`, so the separator disappears under the cursor — which is why
 * ContextEditor holds the raw text for those separately.
 */
export function displayContextValue(type: string | undefined, value: unknown): string {
  if (value === undefined) return '';
  if (type === 'array') return Array.isArray(value) ? value.join(', ') : String(value);
  return String(value);
}
