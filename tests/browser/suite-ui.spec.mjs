import { test, expect, enter, tab } from './fixtures.mjs';

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
