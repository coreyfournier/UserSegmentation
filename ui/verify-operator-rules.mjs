/**
 * Checks the operator/type rules the editor uses to keep an author from
 * building config the engine will refuse — the rules behind
 *
 *   operator "gte" not compatible with type "boolean" for field "X"
 *
 * which is what a save reports when they are not applied.
 *
 *   npm run verify:operator-rules
 */
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, 'node_modules', '.operator-rule-check');

mkdirSync(out, { recursive: true });
execFileSync(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['tsc', 'src/components/rules/operatorRules.ts', '--outDir', out,
   '--module', 'esnext', '--target', 'es2022', '--moduleResolution', 'bundler'],
  { cwd: here, stdio: 'inherit', shell: process.platform === 'win32' }
);
writeFileSync(join(out, 'package.json'), '{"type":"module"}');

// operatorRules imports OPERATOR_TYPES — a value, not a type, so the emitted
// module really does import ../../api/types at runtime. tsc writes that
// specifier without an extension, which a bundler resolves and node does not,
// so the emitted files get the extension added. The other verify scripts do
// not need this: their modules import only types, which are erased.
const emitted = join(out, 'components', 'rules', 'operatorRules.js');
writeFileSync(
  emitted,
  readFileSync(emitted, 'utf8').replace("'../../api/types'", "'../../api/types.js'"),
);

const { operatorSupports, segmentRetypeBreaks, layerRetypeBreaks, describeBreak } =
  await import(pathToFileURL(join(out, 'components', 'rules', 'operatorRules.js')).href);

// --- operatorSupports ---

// The reported case: a comparison operator on a boolean.
assert.equal(operatorSupports('gte', 'boolean'), false);
assert.equal(operatorSupports('gt', 'boolean'), false);
// What a boolean does admit.
assert.equal(operatorSupports('eq', 'boolean'), true);
assert.equal(operatorSupports('neq', 'boolean'), true);
assert.equal(operatorSupports('is_null', 'boolean'), true);
// And on a number, where the same operator is fine — which is how the config
// gets written before the field is retyped.
assert.equal(operatorSupports('gte', 'number'), true);
// An unknown field type claims nothing: the editor must not flag a condition
// on a field it cannot see the declaration for.
assert.equal(operatorSupports('gte', undefined), true);

// --- what a retype breaks ---

const seg = {
  id: 'ct-fee',
  strategy: 'rule',
  computed: [{ name: 'HasMaxFeeBeenReached', type: 'number', formula: '10 >= 1' }],
  rules: [
    {
      ruleName: 'fee-waived',
      condition: { field: 'HasMaxFeeBeenReached', operator: 'gte', value: 1 },
    },
    { ruleName: 'unrelated', condition: { field: 'amount', operator: 'gte', value: 5 } },
  ],
};

const breaks = segmentRetypeBreaks(seg, 'HasMaxFeeBeenReached', 'boolean');
assert.equal(breaks.length, 1, 'only the condition on the retyped field breaks');
assert.deepEqual(breaks[0], {
  segment: 'ct-fee',
  rule: 'fee-waived',
  operator: 'gte',
});
assert.equal(describeBreak(breaks[0]), 'segment "ct-fee" rule "fee-waived" uses gte');

// Retyping to something the operator still admits breaks nothing.
assert.deepEqual(segmentRetypeBreaks(seg, 'HasMaxFeeBeenReached', 'number'), []);

// Nested conditions count: only a leaf names a field, and a leaf can be at any
// depth inside an And/Or.
const nested = {
  id: 's',
  rules: [{
    ruleName: 'outer',
    operator: 'And',
    rules: [{ ruleName: 'inner', condition: { field: 'flag', operator: 'gt', value: 1 } }],
  }],
};
assert.equal(segmentRetypeBreaks(nested, 'flag', 'boolean').length, 1);

// Overrides and the dispatch predicate are conditions too.
const others = {
  id: 's',
  overrides: [{ ruleName: 'ov', condition: { field: 'flag', operator: 'lte', value: 1 } }],
  when: { ruleName: 'applies', condition: { field: 'flag', operator: 'gt', value: 0 } },
};
assert.equal(segmentRetypeBreaks(others, 'flag', 'boolean').length, 2);

// A cross-layer reference takes its type from the other layer, so retyping a
// local field of the same name must not flag it.
const crossLayer = {
  id: 's',
  rules: [{ ruleName: 'r', condition: { field: 'layer:flag', operator: 'gte', value: 1 } }],
};
assert.deepEqual(segmentRetypeBreaks(crossLayer, 'layer:flag', 'boolean'), []);

// The layer-wide form is the per-segment one across every segment.
assert.equal(layerRetypeBreaks([seg, nested], 'HasMaxFeeBeenReached', 'boolean').length, 1);
assert.equal(layerRetypeBreaks([], 'flag', 'boolean').length, 0);

console.log('operator rules OK');
