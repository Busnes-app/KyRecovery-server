// Run with node scripts/test-overview.mjs; no browser or dependencies required.
import { strict as assert } from 'node:assert';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
const elements = new Map();
const document = { addEventListener() {}, getElementById(id) { if (!elements.has(id)) elements.set(id, {}); return elements.get(id); } };
const context = { document, console, fetch: async () => ({ ok: true, json: async () => context.response }) };
runInNewContext(readFileSync(new URL('../internal/server/static/js/app.js', import.meta.url), 'utf8'), context);
for (const [flag, expected] of [[true, 'Appends blocked'], [false, 'Accepting entries'], [undefined, 'Unknown']]) {
  context.response = { capsule_count: 3, custodian_count: 5, audit_append_disabled: flag };
  await context.loadReadiness();
  assert.equal(elements.get('overview-audit-status').textContent, expected);
  assert.equal(elements.get('metric-capsules').textContent, 3);
  assert.equal(elements.get('metric-custodians').textContent, 5);
}
assert(!elements.get('system-status-pill').innerHTML.includes('restore'));
console.log('Overview evidence states passed.');
