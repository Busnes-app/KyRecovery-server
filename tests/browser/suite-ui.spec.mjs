import { test, expect, enter, tab, account } from './fixtures.mjs';

test('navigation, account and dialog actions remain reachable at the shipped sizes', async ({ page, vault }) => {
  await enter(page, vault);
  const nav = page.getByRole('navigation', { name: 'Primary' });
  await expect(nav).toHaveCount(1);
  await tab(page, 'Appearance');
  if (page.viewportSize().width >= 900) {
    const box = await nav.boundingBox();
    expect(box.x).toBe(0);
    expect(box.width).toBe(240);
    const account = await page.locator('#user-auth-area').boundingBox();
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
  for (const name of ['Test Connection', 'Cancel', 'Save Target']) {
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
    ['Custodians & Quorum', '+ Register Custodian', 'custodian-modal'],
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
    ['Password', 'change-password-modal', 'btn-change-pass-header'],
    ['SSO pairing', 'sso-config-modal', 'btn-sso-pair-header'],
  ]) {
    await account(page, action);
    const dialog = page.locator('#' + id);
    await expect(dialog.getByRole('button', { name: 'Close dialog', exact: true })).toBeFocused();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    const returned = page.viewportSize().width < 900 ? page.getByRole('button', { name: 'Account', exact: true }) : page.locator('#' + triggerId);
    await expect(returned).toBeFocused();
  }
});
