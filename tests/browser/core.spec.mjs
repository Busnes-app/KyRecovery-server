import { test, expect, signIn, enter, tab, settings, account, ceremony } from './fixtures.mjs';
import { mkdir } from 'node:fs/promises';
import { join } from 'node:path';

test('real login, cookies, reload and logout enforce the session', async ({ page, context, vault }) => {
  await signIn(page, vault, 'wrong-password');
  await expect(page.locator('#login-error-box')).toBeVisible();
  await expect(page.locator('#dashboard-view')).toBeHidden();
  await enter(page, vault);
  const cookies = await context.cookies();
  expect(cookies.find(c => c.name === 'kyrecovery_session')).toMatchObject({ httpOnly: true, sameSite: 'Lax' });
  const cookie = cookies.map(c => `${c.name}=${c.value}`).join('; ');
  const valid = await fetch(vault.url + '/api/auth/me', { headers: { Cookie: cookie } });
  expect((await valid.json()).authenticated).toBe(true);
  await page.reload();
  await expect(page.locator('#dashboard-view')).toBeVisible();
  await account(page, 'Sign out');
  await expect(page.locator('#login-gateway-view')).toBeVisible();
  const replay = await fetch(vault.url + '/api/auth/me', { headers: { Cookie: cookie } });
  expect((await replay.json()).authenticated).not.toBe(true);
});

test('password validation and replacement revoke another existing session', async ({ page, browser, vault }) => {
  await enter(page, vault);
  const other = await browser.newContext();
  try {
    expect((await other.request.post(vault.url + '/api/auth/login/local', { data: { username: 'admin', password: vault.password } })).ok()).toBe(true);
    await account(page, 'Change password');
    await page.locator('#pass-current').fill(vault.password);
    await page.locator('#pass-new').fill('Replacement-Test-Password-71!');
    await page.locator('#pass-confirm').fill('Mismatched-Test-Password-71!');
    await page.getByRole('button', { name: 'Update password' }).click();
    await expect(page.locator('#change-pass-error')).toContainText('do not match');
    expect((await (await other.request.get(vault.url + '/api/auth/me')).json()).authenticated).toBe(true);
    await page.locator('#pass-confirm').fill('Replacement-Test-Password-71!');
    await page.locator('#pass-current').fill('wrong-current-password');
    await page.getByRole('button', { name: 'Update password' }).click();
    await expect(page.locator('#change-pass-error')).toContainText('incorrect');
    await page.locator('#pass-current').fill(vault.password);
    page.once('dialog', dialog => dialog.accept());
    await page.getByRole('button', { name: 'Update password' }).click();
    await expect(page.locator('#change-password-modal')).toBeHidden();
    expect((await (await other.request.get(vault.url + '/api/auth/me')).json()).authenticated).not.toBe(true);
    expect((await page.request.post(vault.url + '/api/auth/login/local', { data: { username: 'admin', password: vault.password } })).status()).toBe(401);
    await page.reload();
    await expect(page.locator('#dashboard-view')).toBeVisible();
  } finally { await other.close(); }
});

test('custodian values persist as literal text and update the directory count', async ({ page, vault }) => {
  await enter(page, vault);
  await tab(page, 'Custodians & Quorum');
  await page.getByRole('button', { name: 'Add custodian' }).click();
  const name = 'Alice <img src=x onerror="window.injected=true">';
  await page.locator('#custodian-name').fill(name);
  await page.locator('#custodian-email').fill('alice@example.invalid');
  await page.getByRole('button', { name: 'Register custodian', exact: true }).click();
  await expect(page.locator('#custodians-table-body')).toContainText(name);
  await expect(page.locator('#custodians-table-body img')).toHaveCount(0);
  expect(await page.evaluate(() => window.injected)).toBeUndefined();
  await page.reload();
  await tab(page, 'Custodians & Quorum');
  await expect(page.locator('#custodians-table-body')).toContainText('alice@example.invalid');
  await expect(page.locator('#metric-custodians')).toHaveText('1');
  const stored = await (await page.request.get(vault.url + '/api/custodians')).json();
  expect(stored).toHaveLength(1);
  expect(stored[0]).toMatchObject({ name, email: 'alice@example.invalid' });
});

test('retention validates fields, persists values and refuses a cross-origin write', async ({ page, vault }) => {
  await enter(page, vault);
  await tab(page, 'Capsules');
  await expect(page.locator('#retention-status')).toContainText('Keep forever');
  const writes = [];
  page.on('request', request => { if (request.method() === 'POST' && request.url().endsWith('/api/retention')) writes.push(request.postDataJSON()); });
  await page.locator('#retention-days').fill('-1');
  await page.getByRole('button', { name: 'Save retention' }).click();
  expect(writes).toHaveLength(0);
  for (const [field, value] of [['days', '7'], ['weeks', '4'], ['months', '12']]) await page.locator('#retention-' + field).fill(value);
  await page.getByRole('button', { name: 'Save retention' }).click();
  await expect(page.locator('#retention-status')).toContainText('All for 7 days; weekly for 4 weeks; monthly for 12 months');
  expect(writes).toEqual([{ days: 7, weeks: 4, months: 12 }]);
  const denied = await page.request.post(vault.url + '/api/retention', {
    headers: { Origin: 'https://untrusted.invalid', 'Sec-Fetch-Site': 'cross-site' }, data: { days: 1, weeks: 0, months: 0 },
  });
  expect(denied.status()).toBe(403);
  await page.reload();
  await tab(page, 'Capsules');
  await expect(page.locator('#retention-days')).toHaveValue('7');
  await expect(page.locator('#retention-weeks')).toHaveValue('4');
  await expect(page.locator('#retention-months')).toHaveValue('12');
});

test('actual WASM ceremony refuses invalid topology, imports only public fields and rotates explicitly', async ({ page, vault }) => {
  await enter(page, vault);
  await expect(page.locator('#overview-key-action')).toHaveClass('btn btn-primary');
  await expect(page.locator('.evidence-row').first()).toHaveAttribute('id', 'overview-key-row');
  await page.goto(vault.url + '/admin/ceremony');
  await page.locator('#k').fill('4');
  await page.locator('#n').fill('3');
  await page.getByRole('button', { name: 'Generate and split' }).click();
  await expect(page.locator('#error')).toBeVisible();
  await expect(page.locator('#card-list .share')).toHaveCount(0);
  const first = await ceremony(page, vault);
  const second = await ceremony(page, vault);
  expect(second).not.toBe(first);
  const keys = await (await page.request.get(vault.url + '/api/recovery-keys')).json();
  expect(keys.find(k => k.key_id === first).status).toBe('previous');
  expect(keys.find(k => k.key_id === second).status).toBe('current');
  await page.goto(vault.url);
  await expect(page.locator('#overview-backups-action')).toHaveClass('btn btn-primary');
  await expect(page.locator('.evidence-row').first()).toHaveAttribute('id', 'overview-backups-row');
  await tab(page, 'Custodians & Quorum');
  const previous = page.locator('#recovery-keys-body tr').filter({ hasText: first });
  page.once('dialog', dialog => dialog.accept());
  await previous.getByRole('button', { name: 'Retire', exact: true }).click();
  await expect(previous).toContainText('retired');
});

test('pairing requires ceremony, claims once, revokes and clears the registration', async ({ page, vault }) => {
  await enter(page, vault);
  await tab(page, 'Paired Products');
  await page.getByRole('button', { name: 'New pairing code' }).click();
  await page.locator('#pairing-service').fill('browser-fixture');
  await page.locator('#pairing-app-name').fill('Disposable Browser Client');
  await page.locator('#pairing-ttl').fill('15');
  await page.getByRole('button', { name: 'Generate pairing code' }).click();
  await expect(page.locator('#display-pairing-code')).toHaveText(/^\d{6}$/);
  const code = await page.locator('#display-pairing-code').textContent();
  const claim = { pairing_code: code, service_name: 'browser-fixture', app_name: 'Disposable Browser Client' };
  expect((await page.request.post(vault.url + '/api/pairing/claim', { data: claim })).status()).toBe(409);
  await ceremony(page, vault);
  const accepted = await page.request.post(vault.url + '/api/pairing/claim', { data: claim });
  expect(accepted.ok()).toBe(true);
  const token = (await accepted.json()).api_token;
  expect(typeof token).toBe('string');
  expect(token.length).toBeGreaterThan(20);
  const replay = await page.request.post(vault.url + '/api/pairing/claim', { data: claim });
  expect(replay.status()).toBe(409);
  expect((await replay.json()).error).toContain('already consumed');
  await page.goto(vault.url);
  await tab(page, 'Backups');
  await expect(page.locator('#restore-controls')).toBeVisible();
  await expect(page.locator('#restore-service')).toHaveValue('browser-fixture');
  await expect(page.locator('#restore-empty')).toBeHidden();
  const history = page.locator('#backups-history li').filter({ hasText: 'browser-fixture' });
  await expect(history.getByRole('img')).toHaveAccessibleName('browser-fixture: No backups yet; due every 26 h.');
  await expect(history.locator('.ky-status')).toHaveText('Pending');
  await tab(page, 'Paired Products');
  const row = page.locator('#pairing-table-body tr').filter({ hasText: 'Disposable Browser Client' });
  await expect(row.locator('.ky-status[data-ky-status="success"]')).toHaveText('Paired');
  await expect(page.locator('#pairing-table-body')).not.toContainText(token);
  page.once('dialog', dialog => dialog.accept());
  await row.getByRole('button', { name: 'Revoke' }).click();
  await expect(row.locator('.ky-status:not([data-ky-status])')).toHaveText('Revoked');
  const refused = await page.request.post(vault.url + '/api/backup/deposit', { headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/octet-stream' }, data: Buffer.from('not a capsule') });
  expect(refused.status()).toBe(401);
  page.once('dialog', dialog => dialog.accept());
  await row.getByRole('button', { name: 'Clear', exact: true }).click();
  await expect(row).toHaveCount(0);
  await tab(page, 'Backups');
  await page.locator('#tab-backups').getByRole('button', { name: 'Refresh', exact: true }).click();
  await expect(page.locator('#restore-controls')).toBeHidden();
  await expect(page.locator('#restore-empty')).toBeVisible();
});

test('local replication form tests an owned directory, persists and deletes its target', async ({ page, vault }) => {
  const destination = join(vault.directory, 'replica');
  await mkdir(destination);
  await enter(page, vault);
  await tab(page, 'Offsite Replication');
  await page.getByRole('button', { name: 'Add target' }).click();
  await page.locator('#repl-preset').selectOption('local');
  await page.locator('#repl-name').fill('Owned test destination');
  await page.locator('#repl-endpoint').fill(destination);
  await page.locator('#repl-auto-sync').uncheck();
  await page.getByRole('button', { name: 'Test connection', exact: true }).click();
  await expect(page.locator('#repl-test-status')).toContainText('✓');
  await page.getByRole('button', { name: 'Save target' }).click();
  const row = page.locator('#replication-targets-table-body tr').filter({ hasText: 'Owned test destination' });
  await expect(row.locator('.ky-status:not([data-ky-status])')).toHaveText('Manual');
  const targets = await (await page.request.get(vault.url + '/api/replication/targets')).json();
  expect(targets).toHaveLength(1);
  expect(targets[0]).toMatchObject({ name: 'Owned test destination', endpoint: destination, type: 'local', auto_sync: false });
  await page.reload();
  await tab(page, 'Offsite Replication');
  await expect(row).toBeVisible();
  page.once('dialog', dialog => dialog.accept());
  await row.getByRole('button', { name: 'Delete', exact: true }).click();
  await expect(row).toHaveCount(0);
});

test('Appearance follows the OS, persists across reload and synchronizes tabs', async ({ page, context, vault }) => {
  await page.emulateMedia({ colorScheme: 'dark' });
  await enter(page, vault);
  await settings(page, 'Appearance');
  const busnes = p => p.getByRole('group', { name: 'Busnes', exact: true });
  await expect(busnes(page).getByRole('radio', { name: 'Match system' })).toBeChecked();
  await expect(page.locator('html')).toHaveCSS('color-scheme', 'dark');
  await busnes(page).getByRole('radio', { name: 'Light' }).check();
  // The section's hash survives a reload.
  await page.reload();
  await expect(page).toHaveURL(/#settings\/appearance$/);
  await expect(busnes(page).getByRole('radio', { name: 'Light' })).toBeChecked();
  await expect(page.locator('html')).toHaveCSS('color-scheme', 'light');
  const other = await context.newPage();
  // The bare #appearance alias still opens the section.
  await other.goto(vault.url + '/#appearance');
  await expect(other).toHaveURL(/#settings\/appearance$/);
  await busnes(other).getByRole('radio', { name: 'Dark' }).check();
  await expect(page.locator('html')).toHaveCSS('color-scheme', 'dark');
  await other.close();
  const geometry = await page.evaluate(() => ({ width: innerWidth, content: document.documentElement.scrollWidth }));
  expect(geometry.content).toBeLessThanOrEqual(geometry.width + 1);
});
