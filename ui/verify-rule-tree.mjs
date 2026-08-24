/**
 * Checks the rule-tree move logic. There is no JS test runner in this project,
 * so this compiles the one module that carries real regression risk and asserts
 * against it with node's built-in assert.
 *
 *   npm run verify:rules
 */
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, 'node_modules', '.rule-tree-check');

mkdirSync(out, { recursive: true });
execFileSync(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['tsc', 'src/components/rules/ruleTree.ts', '--outDir', out,
   '--module', 'esnext', '--target', 'es2022', '--moduleResolution', 'bundler'],
  { cwd: here, stdio: 'inherit' }
);
// Emitted .js needs this marker to be loaded as ESM.
writeFileSync(join(out, 'package.json'), '{"type":"module"}');

const { moveRule, canDrop } = await import(
  pathToFileURL(join(out, 'components', 'rules', 'ruleTree.js')).href
);

const leaf = (name) => ({ ruleName: name, condition: { field: name, operator: 'eq', value: 1 } });
const group = (name, ...kids) => ({ ruleName: name, operator: 'And', rules: kids });

/** Renders the tree as "a,G(b,c)" so failures read clearly. */
const show = (rules) =>
  rules.map((r) => (r.rules ? `${r.ruleName}(${show(r.rules)})` : r.ruleName)).join(',');

let passed = 0;
const check = (label, actual, expected) => {
  assert.equal(actual, expected, `${label}\n  expected: ${expected}\n  actual:   ${actual}`);
  passed++;
};
const is = (label, actual, expected) => {
  assert.equal(actual, expected, label);
  passed++;
};

// Reordering among siblings.
{
  const t = [leaf('a'), leaf('b'), leaf('c')];
  check('first to end', show(moveRule(t, [0], [3])), 'b,c,a');
  check('last to front', show(moveRule(t, [2], [0])), 'c,a,b');
  check('middle up', show(moveRule(t, [1], [0])), 'b,a,c');
}

// A check into a group — the case that previously required rebuilding it.
{
  const t = [leaf('a'), group('G', leaf('x'))];
  check('into group, at end', show(moveRule(t, [0], [1, 1])), 'G(x,a)');
  check('into group, at front', show(moveRule(t, [0], [1, 0])), 'G(a,x)');
}

// And back out.
{
  const t = [group('G', leaf('x'), leaf('y')), leaf('a')];
  check('out of group, to end', show(moveRule(t, [0, 0], [2])), 'G(y),a,x');
  check('out of group, to front', show(moveRule(t, [0, 1], [0])), 'y,G(x),a');
}

// Group to group.
{
  const t = [group('G1', leaf('x')), group('G2', leaf('y'))];
  check('across groups', show(moveRule(t, [0, 0], [1, 1])), 'G1(),G2(y,x)');
  check('across groups, to front', show(moveRule(t, [1, 0], [0, 0])), 'G1(y,x),G2()');
}

// Empty groups, including one with no rules array at all.
{
  check('into empty group', show(moveRule([leaf('a'), group('E')], [0], [1, 0])), 'E(a)');
  const noArray = [leaf('a'), { ruleName: 'E', operator: 'And' }];
  check('into group lacking a rules array', show(moveRule(noArray, [0], [1, 0])), 'E(a)');
}

// A group carries its subtree with it.
{
  const t = [group('G1', leaf('x'), leaf('y')), group('G2', leaf('z'))];
  check('group keeps children', show(moveRule(t, [0], [1, 1])), 'G2(z,G1(x,y))');
}

// Arbitrary depth.
{
  const t = [group('G1', group('G2', leaf('deep'))), leaf('a')];
  check('deep leaf out to root', show(moveRule(t, [0, 0, 0], [2])), 'G1(G2()),a,deep');
  check('root leaf down to depth 2', show(moveRule(t, [1], [0, 0, 1])), 'G1(G2(deep,a))');
}

// Illegal moves return the original array untouched.
{
  const t = [group('G', leaf('x')), leaf('a')];
  is('no dropping into own subtree', moveRule(t, [0], [0, 0]), t);
  is('no dropping onto itself', moveRule(t, [0], [0]), t);
  is('no dropping just after itself', moveRule(t, [0], [1]), t);

  is('canDrop: self-nesting rejected', canDrop(t, [0], [0, 0]), false);
  is('canDrop: own subtree rejected', canDrop(t, [0], [0, 1]), false);
  is('canDrop: leaf into group allowed', canDrop(t, [1], [0, 0]), true);
  is('canDrop: leaves hold no children', canDrop([leaf('a'), leaf('b')], [0], [1, 0]), false);
}

// The caller's array is never mutated.
{
  const t = [group('G', leaf('x')), leaf('a')];
  const before = show(t);
  moveRule(t, [1], [0, 0]);
  check('input left untouched', show(t), before);
}

console.log(`rule tree: ${passed} assertions passed`);
