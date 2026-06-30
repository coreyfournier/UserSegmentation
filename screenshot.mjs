import { chromium } from 'playwright';
import { createServer } from 'http';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const SHOTS = join(__dirname, 'docs', 'screenshots');
const BASE = 'http://localhost:5173';
const API  = 'http://localhost:8080';

async function poll(url, timeout = 30_000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    try {
      await fetch(url);
      return;
    } catch {
      await new Promise(r => setTimeout(r, 600));
    }
  }
  throw new Error(`Timed out waiting for ${url}`);
}

async function main() {
  console.log('Waiting for servers…');
  await Promise.all([
    poll(`${API}/v1/health`),
    poll(BASE),
  ]);
  console.log('Servers ready.');

  const browser = await chromium.launch();
  const page = await browser.newPage();
  await page.setViewportSize({ width: 1400, height: 900 });

  // ── helper ──────────────────────────────────────────────────────────────────
  async function evaluate({ subject, layers, ctx }) {
    await page.goto(`${BASE}/testing`);
    await page.waitForLoadState('networkidle');

    // Subject key
    await page.locator('input[placeholder="user-123"]').fill(subject);

    // Layer filter — click each desired layer checkbox (all start unchecked = all layers run,
    // checking any means "only these")
    for (const name of (layers ?? [])) {
      await page.locator('label').filter({ hasText: name })
        .locator('input[type="checkbox"]').click();
    }

    // Switch to JSON mode and fill context
    await page.locator('button').filter({ hasText: 'JSON' }).click();
    const ta = page.locator('textarea');
    await ta.fill(JSON.stringify(ctx, null, 2));
    await ta.press('Tab'); // trigger onBlur to parse

    // Evaluate
    await page.locator('button.btn-primary').click();
    await page.waitForSelector('[class*="layerName"]', { timeout: 12_000 });
    await page.waitForTimeout(200); // let render settle
  }
  // ────────────────────────────────────────────────────────────────────────────

  // ── 1. EWA Compliance Eligibility ──────────────────────────────────────────
  console.log('Screenshotting EWA compliance result…');
  await evaluate({
    subject: 'emp-001',
    layers: ['ewa-eligibility'],
    ctx: { EarnedWages: 1200, StateCap: 500, DaysWorked: 10, RequestedAmount: 100 },
  });
  await page.screenshot({ path: join(SHOTS, 'ewa-compliance-result.png') });

  // ── 2. EWA Risk Scoring ─────────────────────────────────────────────────────
  console.log('Screenshotting EWA risk scoring result…');
  await evaluate({
    subject: 'emp-002',
    layers: ['ewa-risk'],
    ctx: {
      Signals: [
        { weight: 0.8,  score: 0.3,  age_sec: 3600, tau_sec: 86400 },
        { weight: -1.2, score: -0.5, age_sec: 7200, tau_sec: 3600  },
      ],
      w0: -3.0, Fee: 5, AchCost: 1, Lambda: 0.5, Alpha: 0.40, NetPay: 1300,
    },
  });
  await page.screenshot({ path: join(SHOTS, 'ewa-risk-result.png') });

  // ── 3. Expression Help Panel ─────────────────────────────────────────────────
  console.log('Screenshotting expression help panel…');
  await page.goto(`${BASE}/layers/ewa-risk/segments/ewa-risk`);
  await page.waitForLoadState('networkidle');
  await page.waitForSelector('[class*="editor"]', { timeout: 10_000 });
  // Scroll down to the Configuration section so expressions are visible
  await page.locator('summary').filter({ hasText: 'Expression Reference' }).scrollIntoViewIfNeeded();
  await page.locator('summary').filter({ hasText: 'Expression Reference' }).click();
  await page.waitForTimeout(400);
  await page.screenshot({ path: join(SHOTS, 'expression-help-panel.png') });

  await browser.close();
  console.log('All screenshots saved to docs/screenshots/');
}

main().catch(err => { console.error(err); process.exit(1); });
