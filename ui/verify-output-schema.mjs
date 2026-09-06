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
  FIELD_TYPES,
  isTemplateField,
  placeholderFor,
  validateOutputField,
  supportsOutputSchema,
  fieldCoverage,
  outputValueRows,
  availableOutputFields,
  renameOutputKey,
  matchingComputedField,
  unfilledComputedMatches,
} = await import(
  pathToFileURL(join(out, 'components', 'schema', 'outputSchemaRules.js')).href
);

// Pinned so the schema editor and the values editor — both of which import
// this instead of declaring their own copy — stay in lockstep with each
// other and with the engine's FieldType set.
assert.deepEqual(FIELD_TYPES, ['string', 'number', 'boolean', 'array', 'object']);

// A string field's value is a template; every other type is an expression.
assert.equal(isTemplateField({ type: 'string' }), true);
assert.equal(isTemplateField({ type: 'number' }), false);
assert.equal(isTemplateField({ type: 'boolean' }), false);
assert.equal(isTemplateField({ type: 'array' }), false);
assert.equal(isTemplateField({ type: 'object' }), false);

// The placeholder names the authoring shape for each: template vs expression.
assert.match(placeholderFor({ type: 'string' }), /\$\{/);
assert.match(placeholderFor({ type: 'number' }), /expression/);
assert.match(placeholderFor({ type: 'boolean' }), /expression/);
assert.match(placeholderFor({ type: 'array' }), /expression/);
assert.match(placeholderFor({ type: 'object' }), /expression/);

// A lookup-bound field must exist and must agree with the table's key type.
const tables = [{ id: 'sev', name: 'severity', keyType: 'string', entries: [] }];
assert.match(validateOutputField('s', { type: 'string', lookup: 'nope' }, tables), /nope/);
assert.match(validateOutputField('s', { type: 'number', lookup: 'sev' }, tables), /keyType|key type/);
assert.equal(validateOutputField('s', { type: 'string', lookup: 'sev' }, tables), null);

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
  { authored: 1, total: 2, segmentLevel: false, overridesAuthored: 0, overridesTotal: 0, defaultUnauthored: false },
);
assert.deepEqual(
  fieldCoverage({ ...seg, outputs: { cat: 'y' } }, schema, 'cat'),
  { authored: 1, total: 2, segmentLevel: true, overridesAuthored: 0, overridesTotal: 0, defaultUnauthored: false },
);
// An empty segment-level value does not count — the engine treats it as unauthored.
assert.deepEqual(
  fieldCoverage({ ...seg, outputs: { cat: '' } }, schema, 'cat'),
  { authored: 1, total: 2, segmentLevel: false, overridesAuthored: 0, overridesTotal: 0, defaultUnauthored: false },
);
// Nor does an empty rule-level value — key presence alone must not count as authoring it.
assert.deepEqual(
  fieldCoverage(
    { ...seg, rules: [...seg.rules, { ruleName: 'd', outputs: { cat: '' }, condition: { field: 'f', operator: 'eq', value: 4 } }] },
    schema,
    'cat',
  ),
  { authored: 1, total: 3, segmentLevel: false, overridesAuthored: 0, overridesTotal: 0, defaultUnauthored: false },
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
  { authored: 1, total: 2, segmentLevel: false, overridesAuthored: 1, overridesTotal: 2, defaultUnauthored: false },
);
// A segment-level value still covers everything, overrides included.
assert.deepEqual(
  fieldCoverage({ ...withOverrides, outputs: { cat: 'y' } }, schema, 'cat'),
  { authored: 1, total: 2, segmentLevel: true, overridesAuthored: 1, overridesTotal: 2, defaultUnauthored: false },
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
  { authored: 1, total: 1, segmentLevel: false, overridesAuthored: 0, overridesTotal: 0, defaultUnauthored: true },
);
// No default declared: rule values alone are enough.
assert.equal(fieldCoverage({ ...ruleSeg, default: '' }, schema, 'cat').defaultUnauthored, false);
assert.equal(fieldCoverage({ ...ruleSeg, default: undefined }, schema, 'cat').defaultUnauthored, false);
// A checklist never reads Default, so a stray one there is inert.
assert.equal(fieldCoverage({ ...seg, default: 'stray' }, schema, 'cat').defaultUnauthored, false);

// The default authors its own values now, so a value there settles it — this
// is the case that used to demand a segment-level value and, with it, the same
// value on every rule that matched.
assert.equal(
  fieldCoverage({ ...ruleSeg, defaultOutputs: { cat: '"x"' } }, schema, 'cat').defaultUnauthored,
  false,
);
// An empty string is not a value: evaluateOutputs skips it exactly as it skips
// an absent key.
assert.equal(
  fieldCoverage({ ...ruleSeg, defaultOutputs: { cat: '' } }, schema, 'cat').defaultUnauthored,
  true,
);

// --- output value rows -------------------------------------------------
const rowSchema = {
  severity: { type: 'string', required: true },
  category: { type: 'string', required: true },
  title:    { type: 'string' },
  notes:    { type: 'string' },
};

// Required fields always appear, authored or not. Optional ones only when
// authored. Orphans — keys with no declaration — appear flagged, because a
// stale value is otherwise invisible while still breaking the save.
let rows = outputValueRows(rowSchema, { title: 'T', ghost: 'G' });
assert.deepEqual(rows.map(r => r.name), ['category', 'severity', 'ghost', 'title']);
assert.deepEqual(rows.map(r => r.required), [true, true, false, false]);
assert.deepEqual(rows.map(r => r.orphaned), [false, false, true, false]);
assert.equal(rows.find(r => r.name === 'severity').value, '');
assert.equal(rows.find(r => r.name === 'title').value, 'T');
assert.equal(rows.find(r => r.name === 'ghost').field, undefined);

// No outputs at all still shows the two required fields.
assert.deepEqual(outputValueRows(rowSchema, undefined).map(r => r.name), ['category', 'severity']);

// An empty schema with an authored key is all orphans.
assert.deepEqual(outputValueRows({}, { x: '1' }).map(r => r.orphaned), [true]);

// --- the dropdown's options -------------------------------------------
// A field already used by ANOTHER row is not offered; the row's own current
// selection always is, or the select would have no matching option.
assert.deepEqual(
  availableOutputFields(rowSchema, { severity: 'C', title: 'T' }, 'title'),
  ['category', 'notes', 'title'],
);
assert.deepEqual(
  availableOutputFields(rowSchema, {}, ''),
  ['category', 'notes', 'severity', 'title'],
);
// An orphan's own name is offered so the select can display it.
assert.ok(availableOutputFields(rowSchema, { ghost: 'G' }, 'ghost').includes('ghost'));

// --- re-pointing a row -------------------------------------------------
// The value follows the rename, the old key goes, order is irrelevant.
assert.deepEqual(renameOutputKey({ severty: 'Critical' }, 'severty', 'severity'),
                 { severity: 'Critical' });
// Renaming onto a name already present overwrites it — the caller prevents
// this via availableOutputFields, but the function must not corrupt the map.
assert.deepEqual(renameOutputKey({ a: '1', b: '2' }, 'a', 'b'), { b: '1' });
// Renaming the only key to itself is a no-op, not a delete.
assert.deepEqual(renameOutputKey({ a: '1' }, 'a', 'a'), { a: '1' });
// Emptying yields undefined so the key is omitted from JSON.
assert.equal(renameOutputKey({ a: '1' }, 'a', ''), undefined);
assert.equal(renameOutputKey(undefined, 'a', 'b'), undefined);

console.log('output schema rules OK');

// --- computed field matching -------------------------------------------
// A suggestion the editor acts on when told to, never something the engine
// resolves: what it fills in is an ordinary expression written into the config.

const computed = [
  { name: 'TransferFee', type: 'number', formula: 'amount * 0.02' },
  { name: 'Tier', type: 'string', formula: '"gold"' },
];

// Name and type both agree.
assert.equal(
  matchingComputedField('TransferFee', { type: 'number' }, computed)?.name,
  'TransferFee',
);

// Name agrees, type does not. Wiring these together would emit the number's
// string form as though that had been intended.
assert.equal(matchingComputedField('TransferFee', { type: 'string' }, computed), undefined);

// No computed field of that name at all.
assert.equal(matchingComputedField('Severity', { type: 'string' }, computed), undefined);

// Nothing to match against.
assert.equal(matchingComputedField('TransferFee', { type: 'number' }, []), undefined);
assert.equal(matchingComputedField('TransferFee', { type: 'number' }, undefined), undefined);
// An undeclared field cannot match: there is no type to agree with.
assert.equal(matchingComputedField('TransferFee', undefined, computed), undefined);

// The set a "fill these in" action would write, which excludes anything
// already authored — filling those would overwrite an author's own value.
const matchSchema = {
  TransferFee: { type: 'number' },
  Tier: { type: 'string' },
  Severity: { type: 'string' },
};
assert.deepEqual(
  unfilledComputedMatches(matchSchema, computed, {}).map((c) => c.name).sort(),
  ['Tier', 'TransferFee'],
);
assert.deepEqual(
  unfilledComputedMatches(matchSchema, computed, { TransferFee: '0' }).map((c) => c.name),
  ['Tier'],
);
assert.deepEqual(unfilledComputedMatches(matchSchema, [], {}), []);
assert.deepEqual(unfilledComputedMatches(undefined, computed, {}), []);

console.log('computed field matching OK');
