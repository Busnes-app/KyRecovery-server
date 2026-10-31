import { test, expect, enter, tab, account } from './fixtures.mjs';

test('navigation, account and dialog actions remain reachable at the shipped sizes', async ({ page, vault }) => {
  await enter(page, vault);
  const nav = page.getByRole('navigation', { name: 'Primary' });
  await expect(nav).toHaveCount(1);
  await tab(page, 'Appearance');
  if (page.viewportSize().width >= 900) {
    const box = await nav.boundingBox();
    expect(box.x).toBe(0);
    const region = await page.locator('.app-sidebar').boundingBox();
    expect(region.x).toBe(0);
    expect(region.width).toBe(240);
    const account = await page.locator('.sidebar-foot').boundingBox();
    expect(account.x).toBeLessThan(240);
    expect(account.y).toBeGreaterThan(box.y);
    await page.setViewportSize({ width: 390, height: 844 });
  }
  await expect.poll(() => nav.locator('[aria-current="page"]').evaluate(el => {
    const item = el.getBoundingClientRect(), row = el.parentElement.getBoundingClientRect();
    return item.left >= row.left - 1 && item.right <= row.right + 1;
  })).toBe(true);
  await tab(page, 'Offsite Replication');
  await page.getByRole('button', { name: 'Add target', exact: true }).click();
  const dialog = page.locator('#replication-modal');
  for (const name of ['Test connection', 'Cancel', 'Save target']) {
    const button = dialog.getByRole('button', { name, exact: true });
    const box = await button.boundingBox();
    expect(box.height).toBeGreaterThanOrEqual(40);
    expect(box.y).toBeGreaterThanOrEqual(0);
    expect(box.y + box.height).toBeLessThanOrEqual(page.viewportSize().height);
  }
  await expect(dialog.locator('.modal-body')).toHaveCSS('overflow-y', 'auto');
  await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(dialog).toBeHidden();
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBe(0);
});


test('dialogs contain keyboard focus, close with Escape and restore their triggers', async ({ page, vault }) => {
  await enter(page, vault);
  for (const [destination, name, id] of [
    ['Paired Products', 'New pairing code', 'pairing-gen-modal'],
    ['Custodians & Quorum', 'Add custodian', 'custodian-modal'],
    ['Offsite Replication', 'Add target', 'replication-modal'],
  ]) {
    await tab(page, destination);
    const trigger = page.getByRole('button', { name, exact: true });
    await trigger.click();
    const dialog = page.locator('#' + id);
    await expect(dialog).toHaveRole('dialog');
    const close = dialog.getByRole('button', { name: 'Close dialog', exact: true });
    await expect(close).toBeFocused();
    await page.keyboard.press('Shift+Tab');
    // Chromium may focus browser chrome at a native-dialog boundary; page controls stay inert.
    expect(await dialog.evaluate(el => !document.hasFocus() || el.contains(document.activeElement))).toBe(true);
    await page.keyboard.press('Tab');
    await expect(close).toBeFocused();
    await page.locator('nav.nav-tabs button').first().evaluate(el => el.focus());
    await expect(close).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(trigger).toBeFocused();
    await trigger.click();
    for (const button of await dialog.locator('.modal-footer button').all()) {
      expect((await button.boundingBox()).height).toBe(40);
    }
    await dialog.getByRole('button', { name: 'Cancel', exact: true }).click();
    await expect(trigger).toBeFocused();
  }
  for (const [action, id, triggerId] of [
    ['Change password', 'change-password-modal', 'btn-change-pass-header'],
    ['SSO pairing', 'sso-config-modal', 'btn-sso-pair-header'],
  ]) {
    await account(page, action);
    const dialog = page.locator('#' + id);
    await expect(dialog.getByRole('button', { name: 'Close dialog', exact: true })).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(page.locator('#' + triggerId)).toBeFocused();
  }
});


test('empty restore, retention help and provider-required labels match available tasks', async ({ page, vault }) => {
  await enter(page, vault);
  await tab(page, 'Backups');
  await expect(page.locator('#restore-empty')).toBeVisible();
  await expect(page.locator('#restore-controls')).toBeHidden();
  await tab(page, 'Capsules');
  const save = page.getByRole('button', { name: 'Save retention', exact: true });
  const box = await save.boundingBox();
  expect(box.y + box.height).toBeLessThanOrEqual(page.viewportSize().height);
  const help = page.locator('#retention-help');
  await expect(help.locator('p')).toBeHidden();
  await help.locator('summary').focus();
  await page.keyboard.press('Enter');
  await expect(help.locator('p')).toBeVisible();
  await expect(help).toContainText('Manual purge can remove the last backup');
  await page.keyboard.press('Enter');
  await expect(help.locator('p')).toBeHidden();
  await tab(page, 'Offsite Replication');
  await page.getByRole('button', { name: 'Add target', exact: true }).click();
  for (const preset of ['s3', 'r2', 'minio', 'sftp', 'smb', 'local']) {
    await page.locator('#repl-preset').selectOption(preset);
    await expect(page.locator('#repl-endpoint-label')).toContainText('(required)');
    await expect(page.locator('#repl-endpoint')).toHaveJSProperty('required', true);
  }
  await page.locator('#replication-modal').getByRole('button', { name: 'Cancel', exact: true }).click();
});

test('ledger layouts display untrusted values as text and keep all fields reachable', async ({ page, vault }) => {
  await enter(page, vault);
  const payload = '<img src=x onerror="window.auditMarkupExecuted=true">';
  const hash = 'a'.repeat(64);
  await page.route('**/api/audit', route => route.fulfill({ json: [{
    sequence_num: 1, action: payload, actor: payload, target_id: payload,
    event_hash: hash, created_at: '2026-10-27T12:00:00Z',
  }] }));
  await page.reload();
  await tab(page, 'Audit Ledger');
  const rendered = page.locator(page.viewportSize().width <= 720 ? '#audit-phone-list' : '#audit-table-body');
  await expect(rendered).toBeVisible();
  await expect(rendered).toContainText(payload);
  await expect(rendered.locator('img, script')).toHaveCount(0);
  expect(await page.evaluate(() => window.auditMarkupExecuted)).toBeUndefined();
  if (page.viewportSize().width <= 720) {
    for (const label of ['Actor', 'Recorded', 'Target', 'Hash']) await expect(rendered).toContainText(label);
    await expect(rendered).toContainText(hash);
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBe(0);
});

test('shared sign-in card and the licence link opening the About dialog', async ({ page, vault }) => {
  await page.goto(vault.url);
  await expect(page.getByRole('heading', { level: 1 })).toHaveText('KyRecovery');
  await expect(page.locator('.ky-auth-help')).toContainText('initial-admin-password');
  await expect(page.locator('.ky-auth-sso')).toBeHidden();
  await expect(page.getByRole('button', { name: 'Sign in', exact: true })).toBeVisible();
  await enter(page, vault);
  const link = page.getByRole('button', { name: /^© \d{4} · MIT License$/ });
  await link.click();
  const about = page.getByRole('dialog', { name: 'About KyRecovery' });
  await expect(about).toBeVisible();
  await expect(about.locator('.ky-about-version')).toHaveText(/^v\d+\.\d+\.\d+/);
  await expect(about.getByRole('textbox', { name: 'MIT License' })).toHaveValue(/^MIT License\n\nCopyright/);
  await about.getByRole('button', { name: 'Close', exact: true }).click();
  await expect(about).toBeHidden();
  await expect(link).toBeFocused();
});
