import { chromium } from 'playwright';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const SHOTS = join(__dirname, '..', 'docs', 'screenshots');
const BASE  = 'http://localhost:5173';
const API   = 'http://localhost:8080';

async function poll(url, timeout = 30_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    try { await fetch(url); return; } catch { /* retry */ }
    await new Promise(r => setTimeout(r, 600));
  }
  throw new Error(`Timed out waiting for ${url}`);
}

async function main() {
  console.log('Waiting for servers…');
  await Promise.all([poll(`${API}/v1/health`), poll(BASE)]);
  console.log('Servers ready.');

  const browser = await chromium.launch();
  const page    = await browser.newPage();
  await page.setViewportSize({ width: 1400, height: 900 });

  async function doEvaluate({ subject, layers, ctx }) {
    await page.goto(`${BASE}/testing`);
    await page.waitForLoadState('networkidle');
    await page.locator('input[placeholder="user-123"]').fill(subject);

    for (const name of (layers ?? [])) {
      await page.locator('label').filter({ hasText: name })
        .locator('input[type="checkbox"]').click();
    }

    await page.getByRole('button', { name: 'JSON' }).click();
    const ta = page.locator('textarea');
    await ta.fill(JSON.stringify(ctx, null, 2));
    await ta.press('Tab');

    await page.getByRole('button', { name: 'Evaluate' }).click();
    await page.waitForSelector('[class*="layerName"]', { timeout: 12_000 });
    await page.waitForTimeout(300);
  }

  // ── 0. CT Fee Override (Scenario C — partial fee edge case) ──────────────
  console.log('Screenshot: CT fee-partial result…');
  await doEvaluate({
    subject: 'batch-c',
    layers:  ['ct-fee-override'],
    ctx: {
      Employees: [
        { Id: 1234, State: 'CT', TransferSpendThisMonth: 20 },
        { Id: 1232, State: 'MD', TransferSpendThisMonth: 50 },
        { Id: 1888, State: 'CT', TransferSpendThisMonth: 8  },
      ],
    },
  });
  await page.screenshot({ path: join(SHOTS, 'ct-fee-result.png') });

  // ── 1. EWA Compliance Eligibility ─────────────────────────────────────────
  console.log('Screenshot: EWA compliance result…');
  await doEvaluate({
    subject: 'emp-001',
    layers:  ['ewa-eligibility'],
    ctx:     { EarnedWages: 1200, StateCap: 500, DaysWorked: 10, RequestedAmount: 100 },
  });
  await page.screenshot({ path: join(SHOTS, 'ewa-compliance-result.png') });

  // ── 2. EWA Risk Scoring ────────────────────────────────────────────────────
  console.log('Screenshot: EWA risk scoring result…');
  await doEvaluate({
    subject: 'emp-002',
    layers:  ['ewa-risk'],
    ctx: {
      Signals: [
        { weight: 0.8,  score: 0.3,  age_sec: 3600, tau_sec: 86400 },
        { weight: -1.2, score: -0.5, age_sec: 7200, tau_sec: 3600  },
      ],
      w0: -3.0, Fee: 5, AchCost: 1, Lambda: 0.5, Alpha: 0.40, NetPay: 1300,
    },
  });
  await page.screenshot({ path: join(SHOTS, 'ewa-risk-result.png') });

  // ── 3. Expression Help Panel ──────────────────────────────────────────────
  console.log('Screenshot: expression help panel…');
  await page.goto(`${BASE}/layers/ewa-risk/segments/ewa-risk`);
  await page.waitForLoadState('networkidle');
  await page.waitForSelector('[class*="editor"]', { timeout: 10_000 });
  await page.locator('summary').filter({ hasText: 'Expression Reference' }).scrollIntoViewIfNeeded();
  await page.locator('summary').filter({ hasText: 'Expression Reference' }).click();
  await page.waitForTimeout(400);
  await page.screenshot({ path: join(SHOTS, 'expression-help-panel.png') });

  await browser.close();
  console.log('All screenshots saved to docs/screenshots/');
}

main().catch(err => { console.error(err); process.exit(1); });
