/**
 * Renders one resolved output value for a person reading a test result.
 *
 * JSON.stringify alone — what the panel did before — is unreadable for the two
 * shapes the engine actually emits: a lookup-bound field resolves to the whole
 * entry, `{key, value, order}`, and a signal list to `[{Name, Value}, …]`.
 * Both are objects whose interesting part is a field or two, so those are
 * spelled out and anything else falls back to JSON rather than guessing.
 */
export function formatOutput(v: unknown): string {
  if (v === null || v === undefined) return '';
  if (Array.isArray(v)) return v.map(formatOutput).join(', ');
  if (typeof v === 'object') {
    const o = v as Record<string, unknown>;
    // A resolved lookup entry: the key is the value, the label explains it.
    // `order` is deliberately not shown — it is the table's ordering, not
    // anything about this subject.
    if ('key' in o) {
      const label = o.value === undefined || o.value === '' ? '' : ` — ${String(o.value)}`;
      return `${String(o.key)}${label}`;
    }
    // A signal: one named measurement.
    if ('Name' in o && 'Value' in o) return `${String(o.Name)}=${formatOutput(o.Value)}`;
    return JSON.stringify(v);
  }
  return String(v);
}
