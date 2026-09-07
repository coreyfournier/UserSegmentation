/**
 * Pins the property that makes a context field typeable.
 *
 * A controlled text box shows `display(parse(text))`. If that composition ever
 * returns something other than what was typed, the box rewrites itself under
 * the cursor and the value becomes unreachable — which has now happened three
 * times in this UI, most recently on a boolean where typing the "t" of "true"
 * produced "false" and locked the field there. So the property is asserted
 * over every prefix of a realistic entry, not just the finished value.
 *
 *   npm run verify:context-values
 */
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, 'node_modules', '.context-value-check');

mkdirSync(out, { recursive: true });
execFileSync(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['tsc', 'src/utils/parse.ts', '--outDir', out,
   '--module', 'esnext', '--target', 'es2022', '--moduleResolution', 'bundler'],
  { cwd: here, stdio: 'inherit', shell: process.platform === 'win32' }
);
writeFileSync(join(out, 'package.json'), '{"type":"module"}');

const { parseContextValue, displayContextValue } =
  await import(pathToFileURL(join(out, 'parse.js')).href);

/** Every prefix of `text`, as typed one character at a time from empty. */
const prefixes = (text) => Array.from({ length: text.length + 1 }, (_, i) => text.slice(0, i));

const typeable = (type, text) => {
  for (const p of prefixes(text)) {
    const shown = displayContextValue(type, parseContextValue(type, p));
    assert.equal(shown, p, `typing ${JSON.stringify(text)} as ${type}: after ${JSON.stringify(p)} the box would show ${JSON.stringify(shown)}`);
  }
};

// Strings pass through untouched, punctuation and all.
typeable('string', 'ACME, Inc.');
typeable('string', 'true');
typeable(undefined, 'no declared type');

// Numbers and arrays do NOT survive the round trip, and these assertions say
// so rather than wishing otherwise — each one is a keystroke that would rewrite
// the box under the cursor:
//
//   "a,"    -> ["a"]  -> "a"    the array separator, gone as it is typed
//   "0.0"   -> 0      -> "0"    every decimal with a zero after the point
//   "1.50"  -> 1.5    -> "1.5"  a trailing zero, unreachable
//
// ContextEditor therefore shows what was typed and keeps the parsed value
// beside it. If anyone makes these lossless, delete the drafts along with them
// — but until then, removing the drafts brings the bug straight back.
assert.equal(displayContextValue('array', parseContextValue('array', 'a,')), 'a');
assert.equal(displayContextValue('number', parseContextValue('number', '0.0')), '0');
assert.equal(displayContextValue('number', parseContextValue('number', '1.50')), '1.5');

// The finished value really is a number, not the text of one.
assert.equal(parseContextValue('number', '42'), 42);
assert.equal(parseContextValue('number', '-3.5'), -3.5);
assert.equal(parseContextValue('number', ''), '');
assert.equal(parseContextValue('number', 'abc'), 'abc');

// Arrays parse to their elements, with the separator's whitespace ignored.
assert.deepEqual(parseContextValue('array', 'a, b'), ['a', 'b']);
assert.deepEqual(parseContextValue('array', 'a,b'), ['a', 'b']);
assert.deepEqual(parseContextValue('array', ''), []);
assert.equal(displayContextValue('array', ['a', 'b']), 'a, b');

// A boolean has no text parse at all — the editor gives it a picker. If one is
// ever reintroduced, this is the case that must not regress.
assert.equal(parseContextValue('boolean', 't'), 't');
assert.equal(parseContextValue('boolean', 'true'), 'true');

// An absent value is an empty box, never the string "undefined".
assert.equal(displayContextValue('string', undefined), '');
assert.equal(displayContextValue('number', undefined), '');
assert.equal(displayContextValue('array', undefined), '');

console.log('context field values: all assertions passed');
