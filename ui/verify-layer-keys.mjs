/**
 * Pins the TypeScript layer-key rules against the same table the Go tests use
 * (TestValidateLayerKey / TestDeriveLayerKey), so the two copies of the rule
 * cannot drift apart silently. The duplication is deliberate — the form has to
 * reject a key while it is being typed — and this is what keeps it honest.
 *
 *   npm run verify:layer-keys
 */
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const out = join(here, 'node_modules', '.layer-key-check');

mkdirSync(out, { recursive: true });
execFileSync(
  process.platform === 'win32' ? 'npx.cmd' : 'npx',
  ['tsc', 'src/components/layers/layerKeyRules.ts', '--outDir', out,
   '--module', 'esnext', '--target', 'es2022', '--moduleResolution', 'bundler'],
  { cwd: here, stdio: 'inherit', shell: process.platform === 'win32' }
);
writeFileSync(join(out, 'package.json'), '{"type":"module"}');

const { validateLayerKey, deriveLayerKey } =
  await import(pathToFileURL(join(out, 'layerKeyRules.js')).href);

for (const k of ['ewaRisk', 'EwaRisk', 'ewa_risk', '_private', 'a', 'layer2', 'Value', 'async']) {
  assert.equal(validateLayerKey(k), '', `${k} should be valid`);
}

const bad = {
  '': 'key is required',
  'ewa-risk': 'key may contain only letters, digits and underscores',
  'CT Rule': 'key may contain only letters, digits and underscores',
  '2024rollout': 'key cannot start with a digit',
  class: 'key is a C# reserved word',
  default: 'key is a C# reserved word',
  string: 'key is a C# reserved word',
};
for (const [k, want] of Object.entries(bad)) {
  assert.equal(validateLayerKey(k), want, `key ${JSON.stringify(k)}`);
}

const derive = {
  'base-tier': 'baseTier',
  'CT Rule': 'ctRule',
  'Risk Rating': 'riskRating',
  'ct-fee-override': 'ctFeeOverride',
  'ewa-eligibility': 'ewaEligibility',
  'ewa-risk': 'ewaRisk',
  experiments: 'experiments',
  dSDsd: 'dSDsd',
  RiskRating: 'riskRating',
  'T&A Gates': 'tAGates',
  'Balance Diagnosis': 'balanceDiagnosis',
  '2024 rollout': '_2024Rollout',
  class: 'classLayer',
  '---': '',
  '': '',
};
for (const [name, want] of Object.entries(derive)) {
  assert.equal(deriveLayerKey(name), want, `deriveLayerKey(${JSON.stringify(name)})`);
}

// Whatever the derivation produces must pass validation, or the form would
// suggest a key that cannot be saved.
for (const n of ['base-tier', 'CT Rule', '2024 rollout', 'class', '  spaced  ', '9', '___']) {
  const k = deriveLayerKey(n);
  if (k === '') continue;
  assert.equal(validateLayerKey(k), '', `derived ${k} from ${n} must be valid`);
}

console.log('layer key rules OK');
