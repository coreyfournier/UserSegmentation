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
  // shell:true on Windows — Node 22 rejects execFileSync against npx.cmd with
  // EINVAL otherwise, which silently made this whole verifier unrunnable.
  { cwd: here, stdio: 'inherit', shell: process.platform === 'win32' }
);
// Emitted .js needs this marker to be loaded as ESM.
writeFileSync(join(out, 'package.json'), '{"type":"module"}');

const { moveRule, canDrop, toGroup, toLeaf } = await import(
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


// --- converting a check to a group and back ----------------------------
// The conversion exists because a predicate is capped at one root: there is no
// second slot to add a group into and drag the check across, so without this a
// single condition could never become an And/Or.
{
  const check = {
    ruleName: 'ProductCheck',
    enabled: true,
    condition: { field: 'ProductType', operator: 'eq', value: 'T&A' },
  };

  // The condition survives as the group's first child. Losing it would mean
  // retyping what the author had already written, which is the whole reason
  // they would not use the conversion.
  const grouped = toGroup(check, 'And');
  assert.equal(grouped.operator, 'And');
  assert.equal(grouped.condition, undefined);
  assert.deepEqual(grouped.rules, [{ ruleName: '', condition: check.condition }]);
  // Everything that identifies the node stays on the node that keeps its place.
  assert.equal(grouped.ruleName, 'ProductCheck');
  assert.equal(grouped.enabled, true);
  // The original is untouched — the editor holds it until onChange lands.
  assert.deepEqual(check.condition, { field: 'ProductType', operator: 'eq', value: 'T&A' });

  // Switching operator on an existing group is just the operator.
  const or = toGroup(grouped, 'Or');
  assert.equal(or.operator, 'Or');
  assert.deepEqual(or.rules, grouped.rules);

  // An empty group converts back to a blank check.
  const empty = { ruleName: 'g', operator: 'And', rules: [] };
  const backToCheck = toLeaf(empty);
  assert.equal(backToCheck.operator, undefined);
  assert.equal(backToCheck.rules, undefined);
  assert.deepEqual(backToCheck.condition, { field: '', operator: 'eq', value: '' });

  // A group with children refuses: absorbing several conditions into one is
  // not something the conversion can do honestly, so it deletes nothing. The
  // picker disables the option too — this is the second line of defence.
  const populated = { ruleName: 'g', operator: 'And', rules: [check] };
  assert.deepEqual(toLeaf(populated), populated);

  passed += 10;
}

console.log(`rule tree: ${passed} assertions passed`);
