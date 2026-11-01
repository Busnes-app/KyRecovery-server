import { test, expect, enter, tab, settings, account } from './fixtures.mjs';

test('navigation, account and dialog actions remain reachable at the shipped sizes', async ({ page, vault }) => {
  await enter(page, vault);
  const nav = page.getByRole('navigation', { name: 'Primary' });
  await expect(nav).toHaveCount(1);
  await settings(page, 'Appearance');
  if (page.viewportSize().width > 720) {
    const box = await nav.boundingBox();
    const region = await page.locator('.ky-sidebar').boundingBox();
    expect(region.x).toBe(0);
    expect(region.width).toBe(240);
    // Nothing between the brand row and the first destination; content starts at the shared inset.
    expect(Math.round((await nav.getByRole('button').first().boundingBox()).y)).toBe(88);
    expect((await page.locator('#tab-settings .ky-page-head').boundingBox()).x).toBe(272);
    // The user block and licence line close the sidebar.
    const account = await page.locator('.ky-sidebar-foot').boundingBox();
    expect(account.x).toBeLessThan(240);
    expect(account.y).toBeGreaterThan(box.y + box.height);
    expect(Math.abs(account.y + account.height - (region.y + region.height - 16))).toBeLessThan(1);
    await page.setViewportSize({ width: 390, height: 844 });
  }
  // Phones: an icon-only Sign out in the header row, no user block.
  await expect(page.locator('.ky-mobile-head').getByRole('button', { name: 'Sign out', exact: true })).toBeVisible();
  await expect(page.locator('.ky-user')).toBeHidden();
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
    await page.getByRole('navigation', { name: 'Primary' }).getByRole('button').first().evaluate(el => el.focus());
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
    ['Set up SSO', 'sso-config-modal', 'btn-sso-pair-header'],
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

test('Settings holds Account and Appearance as tabs with their own hash; Sign out is an icon', async ({ page, vault }) => {
  await enter(page, vault);
  const nav = page.getByRole('navigation', { name: 'Primary' });
  // Admin console: Settings is a navigation item, and Account and Appearance are not.
  await expect(nav.getByRole('button')).toHaveText(['Overview', 'Backups', 'Capsules', 'Paired Products', 'Custodians & Quorum', 'Offsite Replication', 'Audit Ledger', 'Settings']);
  if (page.viewportSize().width > 720) {
    const user = page.locator('.ky-user');
    await expect(user.getByRole('button', { name: 'Sign out', exact: true })).toHaveAttribute('title', 'Sign out');
    await expect(user).not.toContainText('Sign out');
    await expect(user.getByRole('link')).toHaveCount(0);
  } else {
    await expect(page.locator('.ky-mobile-head')).not.toContainText('Sign out');
  }
  await tab(page, 'Settings');
  await expect(page).toHaveURL(/#settings\/account$/);
  await expect(nav.getByRole('button', { name: 'Settings', exact: true })).toHaveAttribute('aria-current', 'page');
  const sections = page.getByRole('navigation', { name: 'Settings sections' });
  await expect(sections.getByRole('link')).toHaveText(['Account', 'Appearance']);
  await expect(sections.getByRole('link', { name: 'Account' })).toHaveAttribute('aria-current', 'page');
  await expect(page.getByRole('region', { name: 'Account', exact: true })).toBeVisible();
  await sections.getByRole('link', { name: 'Appearance' }).click();
  await expect(page).toHaveURL(/#settings\/appearance$/);
  await expect(page.getByRole('region', { name: 'Account', exact: true })).toBeHidden();
  await tab(page, 'Capsules');
  await expect(page).not.toHaveURL(/#/);
  // Back returns to the section, and Forward to the page after it.
  await page.goBack();
  await expect(page.getByRole('region', { name: 'Appearance', exact: true })).toBeVisible();
  await expect(sections.getByRole('link', { name: 'Appearance' })).toHaveAttribute('aria-current', 'page');
  await page.goBack();
  await expect(page.getByRole('region', { name: 'Account', exact: true })).toBeVisible();
  await page.goForward();
  await page.goForward();
  await expect(nav.getByRole('button', { name: 'Capsules', exact: true })).toHaveAttribute('aria-current', 'page');
  // The bare #account alias lands on the Account section.
  await page.goto(vault.url + '/#account');
  await expect(page).toHaveURL(/#settings\/account$/);
  await expect(page.getByRole('region', { name: 'Account', exact: true })).toBeVisible();
});
