/**
 * Checks the computed-field ordering rules: the reorder itself, and the
 * forward-reference detection that tells an author when a formula reads a
 * field declared below it.
 *
 *   npm run verify:computed-fields
 */
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, 'node_modules', '.computed-field-check');

mkdirSync(out, { recursive: true });
execFileSync(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['tsc', 'src/components/segments/computedFieldRules.ts', 'src/utils/move.ts', '--outDir', out,
   '--module', 'esnext', '--target', 'es2022', '--moduleResolution', 'bundler'],
  { cwd: here, stdio: 'inherit', shell: process.platform === 'win32' }
);
writeFileSync(join(out, 'package.json'), '{"type":"module"}');

// Nested because these modules have imports of their own, so tsc preserves the
// source tree under outDir rather than emitting flat.
const { forwardReferences } = await import(
  pathToFileURL(join(out, 'components', 'segments', 'computedFieldRules.js')).href,
);
const { moveItem } = await import(pathToFileURL(join(out, 'utils', 'move.js')).href);

const f = (name, formula = '') => ({ name, type: 'number', formula });
const names = (defs) => defs.map((d) => d.name).join(',');

// --- moveItem: shared, because more than one list's order is load-bearing ---

const three = [f('a'), f('b'), f('c')];

assert.equal(names(moveItem(three, 2, 0)), 'c,a,b', 'move last to first');
assert.equal(names(moveItem(three, 0, 2)), 'b,c,a', 'move first to last');
assert.equal(names(moveItem(three, 1, 0)), 'b,a,c', 'swap up');
assert.equal(names(moveItem(three, 1, 2)), 'a,c,b', 'swap down');

// The input is never mutated — the editor holds it as props.
const original = [f('a'), f('b')];
moveItem(original, 0, 1);
assert.equal(names(original), 'a,b', 'input must not be mutated');

// Out of range is a no-op, so a caller need not guard the ends.
assert.equal(moveItem(three, 0, -1), three, 'above the top is a no-op');
assert.equal(moveItem(three, 2, 3), three, 'below the bottom is a no-op');
assert.equal(moveItem(three, 1, 1), three, 'moving to its own slot is a no-op');
assert.equal(moveItem(three, 9, 0), three, 'unknown source is a no-op');

// --- forwardReferences ---

// The case that motivates the feature: a formula added first, referencing a
// field added afterwards.
const late = [f('total', 'base + bonus'), f('base', '10'), f('bonus', '5')];
const refs = forwardReferences(late);
assert.deepEqual(refs.get(0), ['base', 'bonus'], 'both later fields reported');
assert.equal(refs.size, 1, 'only the offending row is reported');

// Once reordered, nothing is flagged.
let fixed = moveItem(late, 0, 2);
assert.equal(names(fixed), 'base,bonus,total');
assert.equal(forwardReferences(fixed).size, 0, 'correct order reports nothing');

// A field reading one declared above it is fine.
assert.equal(forwardReferences([f('a', '1'), f('b', 'a * 2')]).size, 0);

// Word boundaries: a longer name that merely contains another is not a match.
assert.equal(
  forwardReferences([f('rate', 'baseRate * 2'), f('base', '1')]).size,
  0,
  '"base" inside "baseRate" is not a reference to "base"',
);

// A field referencing itself is not a forward reference — it is a different
// mistake, and not this hint's business.
assert.equal(forwardReferences([f('a', 'a + 1'), f('b', '2')]).size, 0);

// Unnamed rows (a half-typed new row) are skipped rather than matching everything.
assert.equal(forwardReferences([f('a', 'x + 1'), f('', '2')]).size, 0);

// An empty formula reads nothing.
assert.equal(forwardReferences([f('a', ''), f('b', '1')]).size, 0);

// A name holding regex metacharacters must not blow up the pattern.
assert.doesNotThrow(() => forwardReferences([f('a', 'x'), f('a.b(', '1')]));

console.log('computed field rules OK');
