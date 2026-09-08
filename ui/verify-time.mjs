/**
 * Pins how time is shown to the author: the staleness vocabulary the
 * optimistic-locking UI speaks, and the duration a test run reports.
 *
 * Worth a test despite being presentational: the indicator and the conflict
 * dialog both render it, and every interesting case is a boundary — the
 * rounding thresholds, and a store whose clock runs ahead of the browser's.
 *
 *   npm run verify:time
 */
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, 'node_modules', '.time-check');

mkdirSync(out, { recursive: true });
execFileSync(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['tsc', 'src/utils/time.ts', '--outDir', out,
   '--module', 'esnext', '--target', 'es2022', '--moduleResolution', 'bundler'],
  { cwd: here, stdio: 'inherit', shell: process.platform === 'win32' }
);
writeFileSync(join(out, 'package.json'), '{"type":"module"}');

const { ago, duration } = await import(pathToFileURL(join(out, 'time.js')).href);

const now = Date.parse('2026-09-07T12:00:00Z');
const at = (secsAgo) => new Date(now - secsAgo * 1000).toISOString();

// Under the first threshold everything is "just now" — a fresh write and a
// 40-second-old one are equally not-stale.
assert.equal(ago(at(0), now), 'just now');
assert.equal(ago(at(44), now), 'just now');

// Minutes, rounded, and never "0 min": 45s is a minute's worth of staleness.
assert.equal(ago(at(45), now), '1 min ago');
assert.equal(ago(at(90), now), '2 min ago');
assert.equal(ago(at(3600), now), '60 min ago');
assert.equal(ago(at(5399), now), '90 min ago');

// Hours from an hour and a half, days from a day.
assert.equal(ago(at(5400), now), '2 hr ago');
assert.equal(ago(at(86399), now), '24 hr ago');
assert.equal(ago(at(86400), now), '1 days ago');
assert.equal(ago(at(6 * 86400), now), '6 days ago');

// Past a week the relative form stops helping, so it becomes a date. Only the
// shape is asserted — the text is locale-dependent by design.
const old = ago(at(30 * 86400), now);
assert.doesNotMatch(old, /ago|just now/, `expected a date, got ${old}`);

// A store clock a few seconds ahead of the browser's is skew, not news.
assert.equal(ago(at(-3), now), 'just now');
// A real future timestamp is reported as itself rather than as a negative age.
const ahead = ago(at(-600), now);
assert.doesNotMatch(ahead, /ago|just now/, `expected a date, got ${ahead}`);

// A value that is not a time is passed through, so a malformed field shows as
// what it is instead of "Invalid Date".
assert.equal(ago('not-a-time', now), 'not-a-time');

// --- duration ----------------------------------------------------------
// The unit follows the value, so every threshold is a boundary.
assert.equal(duration(0), '0 µs');
assert.equal(duration(87), '87 µs');
assert.equal(duration(999), '999 µs');
assert.equal(duration(1000), '1.0 ms');
assert.equal(duration(4183), '4.2 ms');
assert.equal(duration(9999), '10.0 ms');
assert.equal(duration(10000), '10 ms');
assert.equal(duration(142374), '142 ms');
assert.equal(duration(999999), '1000 ms');
assert.equal(duration(1000000), '1.00 s');
assert.equal(duration(2500000), '2.50 s');

// A missing or nonsensical duration is stated as unknown rather than printed
// as a plausible "0 µs" — a run that reported no timing did not take no time.
assert.equal(duration(-1), '—');
assert.equal(duration(NaN), '—');
assert.equal(duration(Infinity), '—');

console.log('time formatting: all assertions passed');
