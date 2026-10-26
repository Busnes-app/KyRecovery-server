import { execFileSync, spawnSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { createRequire } from 'node:module';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../../', import.meta.url));
const fixtureRoot = mkdtempSync(join(tmpdir(), 'kyrec-browser-'));
try {
  const binary = join(fixtureRoot, 'kyrecovery');
  execFileSync('go', ['build', '-o', binary, './cmd/kyrecovery'], { cwd: root, stdio: 'inherit' });
  const require = createRequire(import.meta.url);
  const result = spawnSync(process.execPath, [require.resolve('@playwright/test/cli'), 'test', ...process.argv.slice(2)], {
    cwd: fileURLToPath(new URL('.', import.meta.url)), stdio: 'inherit',
    env: { ...process.env, KY_BROWSER_BINARY: binary, KY_BROWSER_ROOT: fixtureRoot },
  });
  if (result.error) throw result.error;
  process.exitCode = result.status ?? 1;
} finally {
  rmSync(fixtureRoot, { recursive: true, force: true });
}
