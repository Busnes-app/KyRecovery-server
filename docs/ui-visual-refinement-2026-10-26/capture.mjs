// Visual refinement captures against a disposable KyRecovery with synthetic data only.
// From the repository root, with a directory holding @playwright/test and its Chromium
// (for example a product's web/ after `npm ci`) in CAPTURE_PLAYWRIGHT:
//   CAPTURE_PLAYWRIGHT=/path/to/web node docs/ui-visual-refinement-2026-10-26/capture.mjs <stage nn> <before|after>
// The server has no bind flag (Compose limits it to loopback), so this run uses a random
// admin password and an empty scratch vault that is deleted afterwards.
import { createHash, randomBytes } from 'node:crypto';
import { createRequire } from 'node:module';
import { connect } from 'node:net';
import { mkdtemp, mkdir, rm, writeFile } from 'node:fs/promises';
import { release, tmpdir, type } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync, spawn } from 'node:child_process';

const [stage, phase] = process.argv.slice(2);
if (!/^\d\d$/.test(stage ?? '') || !['before', 'after'].includes(phase)) throw new Error('usage: capture.mjs <nn> <before|after>');
const evidenceDir = 'docs/ui-visual-refinement-2026-10-26';
const root = fileURLToPath(new URL('../../', import.meta.url));
const here = fileURLToPath(new URL('.', import.meta.url));
const pw = createRequire(join(process.env.CAPTURE_PLAYWRIGHT ?? root, 'package.json'));
const { chromium } = pw('@playwright/test');
const port = process.env.CAPTURE_PORT ?? '5391';
const base = `http://127.0.0.1:${port}`;
const password = randomBytes(18).toString('base64url');

// Refuse to start when another process holds a port: the run would capture someone else's server.
for (const p of [port]) {
  const busy = await new Promise(r => { const s = connect(+p, '127.0.0.1', () => { s.destroy(); r(true); }); s.on('error', () => r(false)); });
  if (busy) throw new Error(`127.0.0.1:${p} is in use by another process; set CAPTURE_PORT`);
}
const dir = await mkdtemp(join(tmpdir(), 'kyrecovery-capture-'));
const binary = join(dir, 'kyrecovery');
execFileSync('go', ['build', '-o', binary, './cmd/kyrecovery'], { cwd: root, stdio: 'inherit' });
const server = spawn(binary, ['serve', '--port', port, '--data-dir', join(dir, 'data')], {
  cwd: dir, stdio: ['ignore', 'ignore', 'ignore'],
  env: { PATH: process.env.PATH, HOME: dir, KY_ADMIN_INITIAL_PASSWORD: password, KYRECOVERY_COOKIE_SECURE: 'false' },
});
const exited = new Promise(r => server.once('exit', r));
const cleanup = async () => { server.kill('SIGTERM'); await exited; await rm(dir, { recursive: true, force: true }); };

function sha(bytes) { return createHash('sha256').update(bytes).digest('hex'); }
function git(...args) { return execFileSync('git', ['-C', root, ...args]); }
// Hash of uncommitted source, excluding this evidence directory.
function worktreeHash() {
  const evidence = `:(exclude)${evidenceDir}`;
  const tracked = git('diff', 'HEAD', '--binary', '--', '.', evidence);
  const untracked = git('ls-files', '--others', '--exclude-standard', '-z', '--', '.', evidence).toString().split('\0').filter(Boolean);
  if (!tracked.length && !untracked.length) return null;
  const h = createHash('sha256').update(tracked);
  for (const file of untracked.sort()) h.update(file).update(execFileSync('cat', [join(root, file)]));
  return h.digest('hex');
}

// Tabs are client state; the replication tab is opened by the Overview link (fleet review F6).
const surfaces = {
  overview: { tab: 'Overview' },
  'replication-via-overview': { action: 'View replication', tab: 'Offsite Replication' },
};
const open = async (page, s) => {
  await page.goto('/');
  await page.locator('#dashboard-view').waitFor();
  if (s.action) await page.getByRole('button', { name: s.action }).click();
  else await page.locator('nav.nav-tabs').getByRole('button', { name: s.tab, exact: true }).click();
  await page.waitForLoadState('networkidle');
  await page.evaluate(() => document.fonts.ready);
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
  let fonts = '';
  for (const theme of ['light', 'dark']) {
    for (const [device, viewport] of [['desktop', { width: 1280, height: 900 }], ['mobile', { width: 390, height: 844 }]]) {
      const ctx = await browser.newContext({ baseURL: base, colorScheme: theme, viewport, deviceScaleFactor: 1, storageState: storage, serviceWorkers: 'block' });
      const page = await ctx.newPage();
      for (const [surface, s] of Object.entries(surfaces)) {
        await open(page, s);
        fonts ||= await page.evaluate(() => `Space Grotesk loaded: ${document.fonts.check('15px "Space Grotesk"')}`);
        await page.waitForTimeout(300);
        const file = `kyrecovery-${stage}-${phase}-${surface}-${device}-${theme}.png`;
        const bytes = await page.screenshot({ path: join(out, file), fullPage: true });
        shots.push({ file, sha256: sha(bytes), surface, tab: s.tab, device, viewport, theme_source: `OS ${theme} (prefers-color-scheme), no saved choice` });
      }
      await ctx.close();
    }
  }

  // Behaviour checks in the real app, both phases: each records pass/fail, none aborts.
  const checks = [];
  const check = async (name, fn) => {
    try { checks.push({ name, pass: true, detail: (await fn()) ?? '' }); } catch (e) { checks.push({ name, pass: false, detail: String(e.message ?? e).split('\n')[0] }); }
  };
  const expectEq = (got, want, what) => { if (got !== want) throw new Error(`${what}: got ${got}, want ${want}`); return `${what} = ${got}`; };
  const errors = [];
  const contextFor = async viewport => {
    const ctx = await browser.newContext({ baseURL: base, colorScheme: 'light', viewport, storageState: storage });
    ctx.on('page', p => { p.on('console', m => { if (m.type() === 'error') errors.push(m.text()); }); p.on('pageerror', e => errors.push(e.message)); });
    return ctx;
  };
  const desktop = await contextFor({ width: 1280, height: 900 });
  const phone = await contextFor({ width: 390, height: 844 });
  const dp = await desktop.newPage();
  const pp = await phone.newPage();
  for (const [surface, s] of Object.entries(surfaces)) {
    await check(`desktop ${surface}: one visible h1`, async () => {
      await open(dp, s);
      return expectEq(await dp.locator('h1:visible').count(), 1, 'visible h1 count');
    });
  }
  await check('phone: header and tab row height', async () => {
    await open(pp, surfaces.overview);
    const h = await pp.evaluate(() => Math.round(document.querySelector('nav.nav-tabs').getBoundingClientRect().bottom));
    return `top of page to bottom of tab row: ${h}px`;
  });
  await check('phone: every account control is reachable', async () => {
    await open(pp, surfaces.overview);
    const toggle = pp.getByRole('button', { name: 'Account', exact: true });
    if (await toggle.isVisible()) await toggle.click();
    const seen = [];
    for (const name of ['SSO pairing', 'Password', 'Sign Out']) {
      const b = pp.getByRole('button', { name, exact: true });
      if (!(await b.isVisible())) throw new Error(`${name} not visible`);
      const box = await b.boundingBox();
      if (box.x < 0 || box.x + box.width > 390) throw new Error(`${name} outside the viewport`);
      seen.push(name);
    }
    await pp.keyboard.press('Escape');
    return `${seen.join(', ')} visible${(await toggle.count()) ? ' after opening Account' : ''}`;
  });
  await check('phone: replication empty states fit their card', async () => {
    await open(pp, surfaces['replication-via-overview']);
    const over = await pp.evaluate(() => [...document.querySelectorAll('#tab-replication td[colspan]')].map(td => {
      const card = td.closest('.panel').getBoundingClientRect();
      const r = document.createRange(); r.selectNodeContents(td);
      const text = r.getBoundingClientRect();
      return { text: td.textContent.trim().slice(0, 24), left: Math.round(text.left - card.left), right: Math.round(card.right - text.right) };
    }));
    const bad = over.filter(o => o.left < 0 || o.right < 0);
    if (bad.length) throw new Error(`clipped: ${JSON.stringify(bad)}`);
    return JSON.stringify(over);
  });
  await check('phone: an Overview link reveals its tab without moving the page or focus', async () => {
    const p = await phone.newPage();
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
    await p.close();
    expectEq(r.tab, 'Offsite Replication', 'current tab');
    expectEq(r.inView, true, 'current tab in view');
    expectEq(r.y, Math.max(0, Math.min(before, r.max)), 'page scroll');
    expectEq(r.navFocused, false, 'focus in tab row');
    return `scrollY ${before} -> ${r.y}`;
  });
  await check('no console errors or CSP violations', async () => { if (errors.length) throw new Error(errors.join(' | ')); return 'none'; });
  await desktop.close();
  await phone.close();

  const manifest = {
    product: 'kyrecovery-server', stage, phase, captured_at: new Date().toISOString(),
    source_commit: git('rev-parse', 'HEAD').toString().trim(), uncommitted_source_sha256: worktreeHash(),
    os: `${type()} ${release()}`, driver: `Playwright ${pw('@playwright/test/package.json').version} Chromium ${browser.version()}, device scale 1, service workers blocked`,
    fonts: `self-hosted Space Grotesk from /static/fonts; ${fonts}`,
    server: `kyrecovery built from this worktree into a temporary directory on 127.0.0.1:${port}, scratch data dir, random per-run admin password; removed after capture`,
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
