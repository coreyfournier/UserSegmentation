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

// A literal must parse as its declared type.
assert.equal(validateLiteralValue({ type: 'number' }, '3'), null);
assert.match(validateLiteralValue({ type: 'number' }, 'high'), /number/);
assert.equal(validateLiteralValue({ type: 'boolean' }, 'true'), null);
assert.match(validateLiteralValue({ type: 'boolean' }, 'yes'), /boolean/);
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
// a segment-level value as covering everything.
const seg = {
  id: 's',
  strategy: 'checklist',
  outputSchema: { cat: { type: 'string' } },
  rules: [
    { ruleName: 'a', outputs: { cat: 'x' }, condition: { field: 'f', operator: 'eq', value: 1 } },
    { ruleName: 'b', condition: { field: 'f', operator: 'eq', value: 2 } },
    { ruleName: 'c', enabled: false, condition: { field: 'f', operator: 'eq', value: 3 } },
  ],
};
assert.deepEqual(fieldCoverage(seg, 'cat'), { authored: 1, total: 2, segmentLevel: false });
assert.deepEqual(
  fieldCoverage({ ...seg, outputs: { cat: 'y' } }, 'cat'),
  { authored: 1, total: 2, segmentLevel: true },
);
// An empty segment-level value does not count — the engine treats it as unauthored.
assert.deepEqual(
  fieldCoverage({ ...seg, outputs: { cat: '' } }, 'cat'),
  { authored: 1, total: 2, segmentLevel: false },
);

console.log('output schema rules OK');
