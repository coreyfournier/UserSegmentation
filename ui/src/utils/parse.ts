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
