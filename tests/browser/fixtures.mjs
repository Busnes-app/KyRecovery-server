import { test as base, expect } from '@playwright/test';
import { spawn } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { once } from 'node:events';
import { mkdtemp, rm } from 'node:fs/promises';
import { createServer } from 'node:net';
import { join } from 'node:path';

export { expect };
export const test = base.extend({
  vault: async ({}, use, testInfo) => {
    if (!process.env.KY_BROWSER_ROOT || !process.env.KY_BROWSER_BINARY) throw new Error('Run npm test to build an owned server');
    const directory = await mkdtemp(join(process.env.KY_BROWSER_ROOT, 'test-'));
    const password = randomBytes(24).toString('base64url');
    const listener = createServer();
    listener.listen(0, '127.0.0.1');
    await once(listener, 'listening');
    const port = listener.address().port;
    await new Promise(resolve => listener.close(resolve));
    const url = `http://127.0.0.1:${port}`;
    const server = spawn(process.env.KY_BROWSER_BINARY, ['serve', '--port', String(port), '--data-dir', join(directory, 'data')], {
      cwd: directory, stdio: ['ignore', 'pipe', 'pipe'],
      env: { PATH: process.env.PATH, HOME: directory, GOMAXPROCS: '2', KY_ADMIN_INITIAL_PASSWORD: password, KYRECOVERY_COOKIE_SECURE: 'false' },
    });
    let log = '', launchError;
    server.on('error', error => { launchError = error; });
    for (const stream of [server.stdout, server.stderr]) stream.on('data', data => { log = (log + data).slice(-32_768); });
    try {
      await expect.poll(async () => {
        if (launchError) throw launchError;
        if (server.exitCode !== null) throw new Error(`Fixture exited: ${server.exitCode}`);
        try { return (await fetch(url + '/healthz', { signal: AbortSignal.timeout(1000) })).ok; } catch { return false; }
      }).toBe(true);
      await use({ url, password, directory });
    } finally {
      if (server.exitCode === null && server.signalCode === null && !launchError) {
        server.kill('SIGTERM');
        try { await once(server, 'exit', { signal: AbortSignal.timeout(5000) }); }
        catch { server.kill('SIGKILL'); await once(server, 'exit'); }
      }
      if (testInfo.status !== testInfo.expectedStatus) await testInfo.attach('server.log', { body: log, contentType: 'text/plain' });
      await rm(directory, { recursive: true, force: true });
    }
  },
});

export async function signIn(page, vault, password = vault.password) {
  await page.goto(vault.url);
  await page.locator('#login-username').fill('admin');
  await page.locator('#login-password').fill(password);
  await page.locator('#btn-local-login-submit').click();
}

export async function enter(page, vault) {
  await signIn(page, vault);
  await expect(page.locator('#dashboard-view')).toBeVisible();
}

export async function tab(page, name) {
  await page.locator('nav.nav-tabs').getByRole('button', { name, exact: true }).click();
}

// Sign out sits in the sidebar foot; password and SSO live on the Account page.
export async function account(page, name) {
  if (name === 'Sign out') return page.getByRole('button', { name, exact: true }).click();
  await tab(page, 'Account');
  await page.locator('#tab-account').getByRole('button', { name, exact: true }).click();
}

export async function ceremony(page, vault) {
  await page.goto(vault.url + '/admin/ceremony');
  await page.getByRole('button', { name: 'Generate and split' }).click();
  await expect(page.locator('#card-list .share')).toHaveCount(5);
  const keyID = await page.locator('#key-id').textContent();
  expect(keyID).toMatch(/^[0-9a-f]{64}$/);
  const request = page.waitForRequest(r => r.method() === 'POST' && /\/api\/recovery-key(?:\/rotate)?$/.test(r.url()));
  await page.getByRole('button', { name: 'Import public key into KyRecovery' }).click();
  const body = (await request).postDataJSON();
  const fields = ['public_key', 'threshold', 'total_shares'];
  if (body.replaces_key_id) fields.push('replaces_key_id');
  expect(Object.keys(body).sort()).toEqual(fields.sort());
  expect(body.threshold).toBe(3);
  expect(body.total_shares).toBe(5);
  expect(Buffer.from(body.public_key, 'base64')).toHaveLength(1216);
  await expect(page.locator('#import-status')).toHaveText(new RegExp('^(Imported key|Rotated to key) ' + keyID + '[.;]'));
  return keyID;
}
