// Staged UI captures against a disposable KyRecovery with synthetic data only.
// From the repository root, with a directory holding @playwright/test 1.63 and
// its Chromium (for example a product's web/ after `npm ci`) in CAPTURE_PLAYWRIGHT:
//   CAPTURE_PLAYWRIGHT=/path/to/web node docs/ui-fleet-2026-10-26/capture.mjs <stage nn> <before|after>
// The server has no bind flag (Compose limits it to loopback), so this run uses a random
// admin password and an empty scratch vault that is deleted afterwards.
import { createHash, randomBytes } from 'node:crypto';
import { createServer } from 'node:http';
import { createRequire } from 'node:module';
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import { networkInterfaces, release, tmpdir, type } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync, spawn } from 'node:child_process';

const [stage, phase] = process.argv.slice(2);
if (!/^\d\d$/.test(stage ?? '') || !['before', 'after'].includes(phase)) throw new Error('usage: capture.mjs <nn> <before|after>');
const root = fileURLToPath(new URL('../../', import.meta.url));
const here = fileURLToPath(new URL('.', import.meta.url));
const pw = createRequire(join(process.env.CAPTURE_PLAYWRIGHT ?? root, 'package.json'));
const { chromium } = pw('@playwright/test');
const port = process.env.CAPTURE_PORT ?? '5494';
const base = `http://127.0.0.1:${port}`;
const password = randomBytes(18).toString('base64url');

const dir = await mkdtemp(join(tmpdir(), 'kyrecovery-capture-'));
const binary = join(dir, 'kyrecovery');
execFileSync('go', ['build', '-o', binary, './cmd/kyrecovery'], { cwd: root, stdio: 'inherit' });
const server = spawn(binary, ['serve', '--port', port, '--data-dir', join(dir, 'data')], {
  cwd: dir, stdio: ['ignore', 'ignore', 'ignore'],
  env: { PATH: process.env.PATH, HOME: dir, KY_ADMIN_INITIAL_PASSWORD: password, KYRECOVERY_COOKIE_SECURE: 'false' },
});
const cleanup = async () => { server.kill('SIGTERM'); await new Promise(r => server.once('exit', r)); await rm(dir, { recursive: true, force: true }); };

function sha(bytes) { return createHash('sha256').update(bytes).digest('hex'); }
function git(...args) { return execFileSync('git', ['-C', root, ...args]); }
// Hash of uncommitted source, excluding this evidence directory.
function worktreeHash() {
  const evidence = ':(exclude)docs/ui-fleet-2026-10-26';
  const tracked = git('diff', 'HEAD', '--binary', '--', '.', evidence);
  const untracked = git('ls-files', '--others', '--exclude-standard', '-z', '--', '.', evidence).toString().split('\0').filter(Boolean);
  if (!tracked.length && !untracked.length) return null;
  const h = createHash('sha256').update(tracked);
  for (const file of untracked.sort()) h.update(file).update(execFileSync('cat', [join(root, file)]));
  return h.digest('hex');
}

// Tabs are client state. At baseline the only theme control is the header select on Overview.
// Product theme adapter facts used by the behaviour checks.
const openAppearance = async page => {
  await page.goto('/');
  await page.locator('nav.nav-tabs').getByRole('button', { name: 'Appearance', exact: true }).click();
  await page.locator('[data-theme-picker] input[type=radio]').first().waitFor();
};
const storageKey = 'kyrecovery-theme';
const navItem = 'nav.nav-tabs .ky-nav-item';
const inkVar = '--ky-ink-strong';
const storedFor = id => id;
const systemStored = 'system';
const savedChecks = [['light', 'busnes-light', ''], ['dark', 'busnes-dark', ''], ['ocean', 'ocean', '#e0f2fb']];

const surfaces = {
  overview: { tab: 'Overview' },
  appearance: { tab: phase === 'before' ? 'Overview' : 'Appearance' },
  // Fleet review F6: a tab opened by an Overview link, not by the tab row (a click scrolls the row itself).
  ...(stage >= '05' && { 'replication-via-overview': { action: 'View replication', tab: 'Offsite Replication' } }),
};

try {
  const browser = await chromium.launch();
  const setup = await browser.newContext({ baseURL: base });
  for (let i = 0; ; i++) {
    try { await setup.request.get('/'); break; } catch (e) { if (i > 100) throw e; await new Promise(r => setTimeout(r, 100)); }
  }
  const login = await setup.request.post('/api/auth/login/local', { data: { username: 'admin', password } });
  if (!login.ok()) throw new Error(`login: ${login.status()}`);
  const storage = await setup.storageState();
  await setup.close();

  const out = join(here, 'screenshots');
  await mkdir(out, { recursive: true });
  const shots = [];
  for (const theme of ['light', 'dark']) {
    for (const [device, viewport] of [['desktop', { width: 1280, height: 900 }], ['mobile', { width: 390, height: 844 }]]) {
      const ctx = await browser.newContext({ baseURL: base, colorScheme: theme, viewport, deviceScaleFactor: 1, storageState: storage, serviceWorkers: 'block' });
      const page = await ctx.newPage();
      for (const [surface, s] of Object.entries(surfaces)) {
        await page.goto('/');
        await page.locator('#dashboard-view').waitFor();
        if (s.action) await page.getByRole('button', { name: s.action }).click();
        else await page.locator('nav.nav-tabs').getByRole('button', { name: s.tab, exact: true }).click();
        await page.waitForLoadState('networkidle');
        await page.evaluate(() => document.fonts.ready);
        await page.waitForTimeout(300);
        const file = `kyrecovery-${stage}-${phase}-${surface}-${device}-${theme}.png`;
        const bytes = await page.screenshot({ path: join(out, file), fullPage: true });
        shots.push({ file, sha256: sha(bytes), surface, tab: s.tab, device, viewport, theme_source: `OS ${theme} (prefers-color-scheme), no saved choice` });
      }
      await ctx.close();
    }
  }
  // Behaviour checks in the real app (after phase only): each records pass/fail, none aborts.
  const checks = [];
  if (phase === 'after') {
    const ctx = await browser.newContext({ baseURL: base, colorScheme: 'light', viewport: { width: 1280, height: 900 }, storageState: storage });
    const page = await ctx.newPage();
    const errors = [];
    page.on('console', m => { if (m.type() === 'error') errors.push(m.text()); });
    page.on('pageerror', e => errors.push(e.message));
    const check = async (name, fn) => {
      try { checks.push({ name, pass: true, detail: (await fn()) ?? '' }); } catch (e) { checks.push({ name, pass: false, detail: String(e.message ?? e).split('\n')[0] }); }
    };
    const lum = c => { const k = c.startsWith('color(') ? 1 : 255; const [r, g, b] = c.match(/[\d.]+/g).slice(0, 3).map(v => Number(v) / k).map(v => v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4); return 0.2126 * r + 0.7152 * g + 0.0722 * b; };
    const expectEq = (got, want, what) => { if (got !== want) throw new Error(`${what}: got ${got}, want ${want}`); return `${what} = ${got}`; };
    const prop = name => page.evaluate(n => document.documentElement.style.getPropertyValue(n), name);
    const radios = () => page.locator('[data-theme-picker] input[type=radio]');
    await openAppearance(page);
    await check('all catalog themes plus Match system render as native radios', async () => expectEq(await radios().count(), await page.evaluate(async () => (await import('/static/ky-ui/themes.js')).THEME_CATALOG.length + 1), 'radio count'));
    await check('no saved choice checks Match system and follows the OS', async () => {
      expectEq(await page.locator('input[value=system]').isChecked(), true, 'system checked');
      return expectEq(await page.evaluate(() => document.documentElement.dataset.theme), 'busnes-light', 'data-theme');
    });
    for (const [saved, id, inkStrong] of savedChecks) {
      await check(`legacy saved value ${JSON.stringify(saved)} resolves to ${id} unchanged`, async () => {
        await page.evaluate(([k, v]) => localStorage.setItem(k, v), [storageKey, saved]);
        await page.reload();
        await openAppearance(page);
        expectEq(await page.locator(`input[value="${id}"]`).isChecked(), true, `${id} checked`);
        if (inkStrong) expectEq(await prop(inkVar), inkStrong, inkVar);
        return expectEq(await page.evaluate(k => localStorage.getItem(k), storageKey), saved, 'stored value untouched');
      });
    }
    await check('keyboard arrows move the choice and persist it', async () => {
      await page.locator('input[value=ocean]').click(); // focuses even when already checked
      await page.keyboard.press('ArrowRight');
      expectEq(await page.locator('input[value=coffee]').isChecked(), true, 'coffee checked');
      expectEq(await page.locator('input[value=coffee]').evaluate(el => el === document.activeElement), true, 'focus on coffee');
      const outline = await page.locator('label.ky-appearance-option', { has: page.locator('input[value=coffee]') }).evaluate(el => getComputedStyle(el).outlineStyle);
      expectEq(outline, 'solid', 'focus outline');
      return expectEq(await page.evaluate(k => localStorage.getItem(k), storageKey), storedFor('coffee'), 'saved');
    });
    await check('a light named theme keeps light nav hover and selected surfaces', async () => {
      await page.locator('input[value=sky]').check();
      const item = page.locator(navItem).first();
      await item.hover();
      await page.waitForTimeout(300); // nav colours transition
      const [bg, ink] = await item.evaluate(el => [getComputedStyle(el).backgroundColor, getComputedStyle(el).color]);
      if (!(lum(bg) > 0.6 && lum(ink) < 0.2)) throw new Error(`hover ${bg} on ${ink}`);
      const sel = await page.locator(`${navItem}[aria-current="page"]`).evaluate(el => getComputedStyle(el).backgroundColor);
      if (!(lum(sel) > 0.5)) throw new Error(`selected ${sel}`);
      return `hover ${bg} / ${ink}; selected ${sel}`;
    });
    await check('Match system clears the named palette and the saved choice', async () => {
      await page.locator('input[value=system]').check();
      expectEq(await prop('--ky-bg'), '', 'inline --ky-bg');
      expectEq(await page.evaluate(k => localStorage.getItem(k), storageKey), systemStored, 'saved');
      return expectEq(await page.evaluate(() => getComputedStyle(document.body).backgroundColor), 'rgb(248, 246, 240)', 'body background');
    });
    await check('the picker is in sync with another tab', async () => {
      const other = await ctx.newPage();
      await openAppearance(other);
      await other.locator('[data-theme-picker] input[value=forest]').check();
      await page.waitForFunction(() => document.querySelector('[data-theme-picker] input[value=forest]')?.checked);
      await other.close();
      return 'forest checked after storage event';
    });
    await check('phone: an Overview link reveals its tab without moving the page or focus', async () => {
      const phone = await browser.newContext({ baseURL: base, colorScheme: 'light', viewport: { width: 390, height: 844 }, storageState: storage });
      const p = await phone.newPage();
      try {
        await p.goto('/');
        await p.locator('#dashboard-view').waitFor();
        // Test-only height, so the page cannot clamp its scroll: any movement comes from the tab change.
        await p.addStyleTag({ content: 'main { min-height: 400vh; }' });
        const link = p.getByRole('button', { name: 'View replication' });
        await link.scrollIntoViewIfNeeded();
        const before = await p.evaluate(() => scrollY);
        await link.click();
        await p.waitForTimeout(300);
        const r = await p.evaluate(() => {
          const nav = document.querySelector('nav.nav-tabs'), box = nav.getBoundingClientRect();
          const cur = nav.querySelector('[aria-current="page"]'), item = cur.getBoundingClientRect();
          return { tab: cur.textContent.trim(), inView: item.left >= box.left - 1 && item.right <= box.right + 1, y: scrollY,
            max: document.documentElement.scrollHeight - innerHeight, navFocused: !!document.activeElement?.closest('nav') };
        });
        expectEq(r.tab, 'Offsite Replication', 'current tab');
        expectEq(r.inView, true, 'current tab in view');
        expectEq(r.y, Math.max(0, Math.min(before, r.max)), 'page scroll');
        expectEq(r.navFocused, false, 'focus in tab row');
        return `scrollY ${before} -> ${r.y}`;
      } finally { await phone.close(); }
    });
    await check('no console errors or CSP violations', async () => { if (errors.length) throw new Error(errors.join(' | ')); return 'none'; });
    await ctx.close();
  }

  const manifest = {
    product: 'kyrecovery-server', stage, phase, captured_at: new Date().toISOString(),
    source_commit: git('rev-parse', 'HEAD').toString().trim(), uncommitted_source_sha256: worktreeHash(),
    os: `${type()} ${release()}`, driver: `Playwright ${pw('@playwright/test/package.json').version} Chromium ${browser.version()}, device scale 1, service workers blocked`,
    server: 'kyrecovery built from this worktree into a temporary directory, scratch data dir, random per-run admin password; removed after capture',
    fixture: 'synthetic: fresh vault, bootstrap admin only; no ceremony, recovery key, paired products, capsules or replication (warning states are real)',
    shots,
    checks,
  };
  await writeFile(join(here, `manifest-${stage}-${phase}.json`), JSON.stringify(manifest, null, 2) + '\n');
  await browser.close();
  console.log(`${shots.length} captures, ${checks.filter(c => c.pass).length}/${checks.length} checks passed, manifest-${stage}-${phase}.json`);
  for (const c of checks.filter(c => !c.pass)) console.log(`FAIL ${c.name}: ${c.detail}`);
} finally {
  await cleanup();
}
