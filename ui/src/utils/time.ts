/**
 * How long ago a timestamp was, in words.
 *
 * Shared by the last-changed indicator and the conflict dialog because they
 * are answering the same question — "how stale is what I am looking at" — and
 * two vocabularies for it would make the dialog appear to contradict the line
 * the author was just reading ("3 min ago" beside "180s ago").
 *
 * Deliberately coarse and never exact: the precise instant is available as a
 * tooltip everywhere this is shown, and rounding is what makes the answer
 * readable at a glance. `now` is a parameter so this is testable without
 * mocking the clock.
 *
 * A time in the future — a clock skew between this browser and the store, not
 * a fiction — falls back to the absolute date rather than claiming a negative
 * age.
 */
export function ago(iso: string, now: number = Date.now()): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return iso;
  const secs = Math.round((now - then) / 1000);
  // A few seconds of skew is not worth an absolute date; a real one is.
  if (secs < -5) return new Date(iso).toLocaleString();
  if (secs < 45) return 'just now';
  if (secs < 5400) return `${Math.max(1, Math.round(secs / 60))} min ago`;
  if (secs < 86400) return `${Math.round(secs / 3600)} hr ago`;
  if (secs < 7 * 86400) return `${Math.round(secs / 86400)} days ago`;
  return new Date(iso).toLocaleDateString();
}

/**
 * A duration in microseconds, in the shortest form that still says something.
 *
 * Microseconds are the engine's own unit (`duration_us`), and evaluations here
 * span three orders of magnitude — a static lookup is tens of microseconds, a
 * checklist over a dozen rules is single-digit milliseconds. One fixed unit
 * would print either "0 ms" or "4183 µs", so the unit follows the value.
 *
 * Significant digits, not decimal places: "4.2 ms" and "142 ms" carry the same
 * information, and "142.37 ms" claims precision that a single run of an
 * evaluation does not have.
 */
export function duration(micros: number): string {
  if (!Number.isFinite(micros) || micros < 0) return '—';
  if (micros < 1000) return `${Math.round(micros)} µs`;
  const ms = micros / 1000;
  if (ms < 10) return `${ms.toFixed(1)} ms`;
  if (ms < 1000) return `${Math.round(ms)} ms`;
  return `${(ms / 1000).toFixed(2)} s`;
}
