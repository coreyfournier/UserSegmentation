import { chromium } from 'playwright';
const b = await chromium.launch();
const p = await b.newPage({ viewport: { width: 1280, height: 1200 } });
p.on('pageerror', e => console.log('  [PAGE ERROR] ' + e.message));
await p.route('**/v1/**', async (route) => {
  const u = new URL(route.request().url());
  const r = await route.fetch({ url: 'http://localhost:19300' + u.pathname + u.search });
  await route.fulfill({ response: r });
});
await p.goto('http://localhost:5173', { waitUntil: 'networkidle' });
await p.getByText('Lookups', { exact: true }).first().click();
await p.waitForTimeout(900);

const nameBox = () => p.locator('form:visible').first().locator('input').first();

// First open: type a name, then cancel.
await p.getByRole('button', { name: '+ Add Lookup' }).click();
await p.waitForTimeout(600);
await nameBox().fill('firstLookup');
console.log('1st open, typed:            ' + await nameBox().inputValue());
await p.getByRole('button', { name: 'Cancel' }).first().click();
await p.waitForTimeout(600);

// Second open: the box must be EMPTY, not carrying the previous value.
await p.getByRole('button', { name: '+ Add Lookup' }).click();
await p.waitForTimeout(600);
const carried = await nameBox().inputValue();
console.log('2nd open, name box value:   ' + JSON.stringify(carried));
console.log('form is clean (bug fixed):  ' + (carried === ''));
await b.close();
