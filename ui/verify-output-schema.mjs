/**
 * Checks the output-schema rules. There is no JS test runner in this project,
 * so this compiles the one module that carries real regression risk and
 * asserts against it with node's built-in assert.
 *
 *   npm run verify:output-schema
 */
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, 'node_modules', '.output-schema-check');

mkdirSync(out, { recursive: true });
execFileSync(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['tsc', 'src/components/schema/outputSchemaRules.ts', '--outDir', out,
   '--module', 'esnext', '--target', 'es2022', '--moduleResolution', 'bundler'],
  { cwd: here, stdio: 'inherit', shell: process.platform === 'win32' }
);
// Emitted .js needs this marker to be loaded as ESM.
writeFileSync(join(out, 'package.json'), '{"type":"module"}');

const {
  allowedTypesForMode,
  validateOutputField,
  validateLiteralValue,
  supportsOutputSchema,
  fieldCoverage,
} = await import(
  pathToFileURL(join(out, 'components', 'schema', 'outputSchemaRules.js')).href
);

// A template always produces a string, so only string may be declared.
assert.deepEqual(allowedTypesForMode('template'), ['string']);

// A literal cannot express a collection.
assert.deepEqual(allowedTypesForMode('literal'), ['string', 'number', 'boolean']);

// An expression returns a typed value, so anything goes.
assert.deepEqual(
  allowedTypesForMode('expression'),
  ['string', 'number', 'boolean', 'array', 'object'],
);

// Mode/type mismatches are rejected, with the mode named.
assert.match(
  validateOutputField('rank', { type: 'number', eval: 'template' }, []),
  /template/,
);
assert.equal(validateOutputField('title', { type: 'string', eval: 'template' }, []), null);
assert.match(validateOutputField('bag', { type: 'object' }, []), /expression/);
assert.equal(validateOutputField('bag', { type: 'object', eval: 'expression' }, []), null);

// A lookup-bound field must exist and must agree with the table's key type.
const tables = [{ id: 'sev', name: 'severity', keyType: 'string', entries: [] }];
assert.match(validateOutputField('s', { type: 'string', lookup: 'nope' }, tables), /nope/);
assert.match(validateOutputField('s', { type: 'number', lookup: 'sev' }, tables), /keyType|key type/);
assert.equal(validateOutputField('s', { type: 'string', lookup: 'sev' }, tables), null);

// A literal must parse as its declared type, by Go's rules — not JS's.
assert.equal(validateLiteralValue({ type: 'number' }, '3'), null);
assert.equal(validateLiteralValue({ type: 'number' }, '-3.5'), null);
assert.equal(validateLiteralValue({ type: 'number' }, '1e3'), null);
assert.equal(validateLiteralValue({ type: 'number' }, '+7'), null);
assert.match(validateLiteralValue({ type: 'number' }, 'high'), /number/);
// Number() would accept all four of these; strconv.ParseFloat rejects them,
// so the editor must too or the save fails after the editor said it was fine.
assert.match(validateLiteralValue({ type: 'number' }, ' 42'), /number/);
assert.match(validateLiteralValue({ type: 'number' }, '42 '), /number/);
assert.match(validateLiteralValue({ type: 'number' }, '0x10'), /number/);
assert.match(validateLiteralValue({ type: 'number' }, '  '), /number/);
// Range matters too: ParseFloat(_, 64) errors with ErrRange on overflow, and
// the engine treats any non-nil error as a rejection. Underflow is NOT an
// error there, so 1e-999 must still be accepted.
assert.match(validateLiteralValue({ type: 'number' }, '1e999'), /number/);
assert.match(validateLiteralValue({ type: 'number' }, '-1e999'), /number/);
assert.match(validateLiteralValue({ type: 'number' }, '1e309'), /number/);
assert.equal(validateLiteralValue({ type: 'number' }, '1e-999'), null);
assert.equal(validateLiteralValue({ type: 'number' }, '1e308'), null);
// strconv.ParseBool accepts twelve spellings, not two.
for (const ok of ['1', 't', 'T', 'TRUE', 'true', 'True', '0', 'f', 'F', 'FALSE', 'false', 'False']) {
  assert.equal(validateLiteralValue({ type: 'boolean' }, ok), null, `boolean ${ok} should be accepted`);
}
assert.match(validateLiteralValue({ type: 'boolean' }, 'yes'), /boolean/);
assert.match(validateLiteralValue({ type: 'boolean' }, 'TrUe'), /boolean/);
assert.equal(validateLiteralValue({ type: 'string' }, 'anything'), null);
// Only literal mode is checked — a template or expression is not a literal.
assert.equal(validateLiteralValue({ type: 'number', eval: 'expression' }, 'a + b'), null);
assert.equal(validateLiteralValue({ type: 'number', eval: 'template' }, '${x}'), null);
// An empty value is "not authored", not an invalid literal.
assert.equal(validateLiteralValue({ type: 'number' }, ''), null);

// Strategies that emit no record cannot carry an output schema.
assert.equal(supportsOutputSchema('checklist'), true);
assert.equal(supportsOutputSchema('rule'), true);
assert.equal(supportsOutputSchema('static'), false);
assert.equal(supportsOutputSchema('percentage'), false);

// Coverage counts reporting rules only, ignores disabled ones, and reports
// a segment-level value as covering everything. The schema the field is
// declared in lives on the layer now, so it is passed alongside the
// segment rather than read off it.
const schema = { cat: { type: 'string' } };
const seg = {
  id: 's',
  strategy: 'checklist',
  rules: [
    { ruleName: 'a', outputs: { cat: 'x' }, condition: { field: 'f', operator: 'eq', value: 1 } },
    { ruleName: 'b', condition: { field: 'f', operator: 'eq', value: 2 } },
    { ruleName: 'c', enabled: false, condition: { field: 'f', operator: 'eq', value: 3 } },
  ],
};
assert.deepEqual(
  fieldCoverage(seg, schema, 'cat'),
  { authored: 1, total: 2, segmentLevel: false, overridesAuthored: 0, overridesTotal: 0, defaultNeedsSegmentValue: false },
);
assert.deepEqual(
  fieldCoverage({ ...seg, outputs: { cat: 'y' } }, schema, 'cat'),
  { authored: 1, total: 2, segmentLevel: true, overridesAuthored: 0, overridesTotal: 0, defaultNeedsSegmentValue: false },
);
// An empty segment-level value does not count — the engine treats it as unauthored.
assert.deepEqual(
  fieldCoverage({ ...seg, outputs: { cat: '' } }, schema, 'cat'),
  { authored: 1, total: 2, segmentLevel: false, overridesAuthored: 0, overridesTotal: 0, defaultNeedsSegmentValue: false },
);
// Nor does an empty rule-level value — key presence alone must not count as authoring it.
assert.deepEqual(
  fieldCoverage(
    { ...seg, rules: [...seg.rules, { ruleName: 'd', outputs: { cat: '' }, condition: { field: 'f', operator: 'eq', value: 4 } }] },
    schema,
    'cat',
  ),
  { authored: 1, total: 3, segmentLevel: false, overridesAuthored: 0, overridesTotal: 0, defaultNeedsSegmentValue: false },
);

// Overrides count toward coverage too — enabled ones only, and per-item values
// cannot be authored on them in this UI, so a shortfall always needs the
// segment-level value.
const withOverrides = {
  ...seg,
  overrides: [
    { ruleName: 'o1', outputs: { cat: 'x' }, condition: { field: 'f', operator: 'eq', value: 1 } },
    { ruleName: 'o2', condition: { field: 'f', operator: 'eq', value: 2 } },
    { ruleName: 'o3', enabled: false, condition: { field: 'f', operator: 'eq', value: 3 } },
  ],
};
assert.deepEqual(
  fieldCoverage(withOverrides, schema, 'cat'),
  { authored: 1, total: 2, segmentLevel: false, overridesAuthored: 1, overridesTotal: 2, defaultNeedsSegmentValue: false },
);
// A segment-level value still covers everything, overrides included.
assert.deepEqual(
  fieldCoverage({ ...withOverrides, outputs: { cat: 'y' } }, schema, 'cat'),
  { authored: 1, total: 2, segmentLevel: true, overridesAuthored: 1, overridesTotal: 2, defaultNeedsSegmentValue: false },
);

// A rule segment with a non-empty default reads no rule values on that path —
// only a segment-level value can satisfy a required field there.
const ruleSeg = {
  id: 's2',
  strategy: 'rule',
  default: 'fallback',
  rules: [
    { ruleName: 'a', outputs: { cat: 'x' }, condition: { field: 'f', operator: 'eq', value: 1 } },
  ],
};
assert.deepEqual(
  fieldCoverage(ruleSeg, schema, 'cat'),
  { authored: 1, total: 1, segmentLevel: false, overridesAuthored: 0, overridesTotal: 0, defaultNeedsSegmentValue: true },
);
// No default declared: rule values alone are enough.
assert.equal(fieldCoverage({ ...ruleSeg, default: '' }, schema, 'cat').defaultNeedsSegmentValue, false);
assert.equal(fieldCoverage({ ...ruleSeg, default: undefined }, schema, 'cat').defaultNeedsSegmentValue, false);
// A checklist never reads Default, so it never needs the segment-level value
// on that account, even if a stray default is present.
assert.equal(fieldCoverage({ ...seg, default: 'stray' }, schema, 'cat').defaultNeedsSegmentValue, false);

console.log('output schema rules OK');
