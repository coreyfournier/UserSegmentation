/**
 * Pins how a resolved output value is rendered in a test result, against the
 * shapes the engine actually emits.
 *
 * Worth a test because the two interesting shapes are not obvious from the
 * type — an output value is `unknown`, and a lookup-bound field arrives as the
 * whole table entry rather than as its key. Before this the panel printed the
 * raw JSON of both.
 *
 *   npm run verify:output-format
 */
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, 'node_modules', '.output-format-check');

mkdirSync(out, { recursive: true });
execFileSync(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['tsc', 'src/components/testing/outputFormat.ts', '--outDir', out,
   '--module', 'esnext', '--target', 'es2022', '--moduleResolution', 'bundler'],
  { cwd: here, stdio: 'inherit', shell: process.platform === 'win32' }
);
writeFileSync(join(out, 'package.json'), '{"type":"module"}');

const { formatOutput } = await import(pathToFileURL(join(out, 'outputFormat.js')).href);

// Scalars pass through as themselves, false and 0 included — a falsy value is
// a value, and blanking it would misreport the record.
assert.equal(formatOutput('Warning'), 'Warning');
assert.equal(formatOutput(4), '4');
assert.equal(formatOutput(0), '0');
assert.equal(formatOutput(false), 'false');
assert.equal(formatOutput(true), 'true');

// Absent stays blank rather than printing "undefined" into the table.
assert.equal(formatOutput(undefined), '');
assert.equal(formatOutput(null), '');

// A lookup-bound field resolves to the whole entry. The key is the value; the
// label is shown only when the table gives one, and `order` never is — that is
// the table's ordering, not a fact about this subject.
assert.equal(formatOutput({ key: 'DataSync', value: '' }), 'DataSync');
assert.equal(formatOutput({ key: 'Critical', order: 0, value: '' }), 'Critical');
assert.equal(formatOutput({ key: 'Critical', order: 0, value: 'Needs attention now' }),
             'Critical — Needs attention now');
assert.equal(formatOutput({ key: 3, value: 'Third' }), '3 — Third');

// A signal list, exactly as balanceDiagnosis emits it.
assert.equal(
  formatOutput([
    { Name: 'EmployeeStatus', Value: 'Failed' },
    { Name: 'EmployeeCount', Value: 5 },
  ]),
  'EmployeeStatus=Failed, EmployeeCount=5',
);

// A plain array of scalars.
assert.equal(formatOutput(['a', 'b']), 'a, b');
assert.equal(formatOutput([]), '');

// Anything else is still shown rather than swallowed — JSON is a poor display
// but an honest one, and a shape nobody anticipated must not vanish.
assert.equal(formatOutput({ some: 'thing' }), '{"some":"thing"}');

console.log('output value formatting OK');
