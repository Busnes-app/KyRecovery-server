// Run with node scripts/test-overview.mjs; no browser or dependencies required.
import { strict as assert } from 'node:assert';
import { readFileSync } from 'node:fs';
import { runInNewContext } from 'node:vm';
const appSource = readFileSync(new URL('../internal/server/static/js/app.js', import.meta.url), 'utf8');
const page = readFileSync(new URL('../internal/server/static/index.html', import.meta.url), 'utf8');
const ids = [...page.matchAll(/\sid=["']([^"']+)["']/g)].map(match => match[1]);
for (const id of ['overview-audit-status', 'metric-capsules', 'metric-custodians', 'system-status-pill']) {
  assert.equal(ids.filter(value => value === id).length, 1, `dashboard must contain exactly one ${id}`);
}

function runtime(body) {
  const elements = new Map(ids.map(id => {
    let text = '';
    return [id, {
      get textContent() { return text; },
      set textContent(value) { text = String(value ?? ''); },
      innerHTML: '', className: '',
    }];
  }));
  const state = { body, errors: [], jsonCalls: 0 };
  const context = {
    document: { addEventListener() {}, getElementById: id => elements.get(id) ?? null },
    console: { error: (...args) => state.errors.push(args) },
    fetch: async url => {
      assert.equal(url, '/api/readiness');
      if (state.networkError) throw new Error('synthetic network failure');
      return {
        ok: state.ok !== false,
        json: async () => {
          state.jsonCalls++;
          if (state.jsonError) throw new Error('synthetic JSON failure');
          return state.body;
        },
      };
    },
  };
  runInNewContext(appSource, context);
  return { context, elements, state };
}

for (const [flag, expected] of [
  [true, 'Appends blocked'], [false, 'Accepting entries'], [undefined, 'Unknown'],
  [null, 'Unknown'], ['false', 'Unknown'], [0, 'Unknown'],
]) {
  const { context, elements, state } = runtime({ capsule_count: 3, custodian_count: 5, audit_append_disabled: flag });
  await context.loadReadiness();
  assert.equal(elements.get('overview-audit-status').textContent, expected);
  assert.equal(elements.get('metric-capsules').textContent, '3');
  assert.equal(elements.get('metric-custodians').textContent, '5');
  assert.equal(state.errors.length, 0);
}

for (const [count, expectedCount, expected] of [
  [0, 0, 'No capsules stored'], [1, 1, '1 capsule stored'], [2, 2, '2 capsules stored'],
  ['3', 3, '3 capsules stored'], [undefined, 0, 'No capsules stored'],
  ['invalid', 0, 'No capsules stored'], ['<img src=x onerror=alert(1)>', 0, 'No capsules stored'],
]) {
  const { context, elements, state } = runtime({ capsule_count: count });
  await context.loadReadiness();
  const pill = elements.get('system-status-pill');
  assert.equal(elements.get('metric-capsules').textContent, String(expectedCount));
  assert.equal(pill.innerHTML, '<span class="dot"></span> ' + expected);
  assert.equal(pill.className, expected === 'No capsules stored' ? 'status-pill warning' : 'status-pill ready');
  assert.equal(elements.get('metric-custodians').textContent, '0');
  assert.equal(elements.get('overview-audit-status').textContent, 'Unknown');
  assert.equal(state.errors.length, 0);
}

for (const failure of ['http', 'network', 'json']) {
  const { context, elements, state } = runtime({});
  state.ok = failure !== 'http';
  state.networkError = failure === 'network';
  state.jsonError = failure === 'json';
  await context.loadReadiness();
  assert.equal(state.jsonCalls, failure === 'json' ? 1 : 0);
  assert.equal(state.errors.length, failure === 'http' ? 0 : 1);
  assert.equal(elements.get('system-status-pill').innerHTML, '');

  // A later valid response must recover without inventing restore evidence.
  Object.assign(state, { ok: true, networkError: false, jsonError: false,
    body: { capsule_count: 1, custodian_count: 2, audit_append_disabled: false } });
  await context.loadReadiness();
  assert.equal(elements.get('overview-audit-status').textContent, 'Accepting entries');
  assert.equal(elements.get('metric-capsules').textContent, '1');
  assert.equal(elements.get('metric-custodians').textContent, '2');
  assert(!elements.get('system-status-pill').innerHTML.includes('restore'));
}
// Evidence rows: every missing piece gets an explicit state and a next action.
{
  const { context } = runtime({});
  const day = new Date(Date.now() - 86400e3).toISOString();
  const report = (services, counts) => ({ report: { services, counts } });
  const svc = { instances: [{ newest: { deposited_at: day } }] };
  const cases = [
    [context.backupsEvidence(undefined), 'checking'],
    [context.backupsEvidence(null), 'unavailable'],
    [context.backupsEvidence(report([], {})), 'missing', 'tab-pairing'],
    [context.backupsEvidence(report([svc, svc], { ok: 1, corrupt: 1 })), 'failing', 'tab-backups'],
    [context.backupsEvidence(report([svc, svc], { ok: 1, stale: 1 })), 'attention', 'tab-backups'],
    [context.backupsEvidence(report([svc, svc], { ok: 1, pending: 1 })), 'ok', 'tab-backups'],
    [context.keyEvidence(undefined), 'checking'],
    [context.keyEvidence(null), 'unavailable'],
    [context.keyEvidence([], 3), 'missing', 'tab-custodians'],
    [context.keyEvidence([{ status: 'current', threshold: 3, total_shares: 5 }], 3), 'attention', 'tab-custodians'],
    [context.keyEvidence([{ status: 'current', threshold: 3, total_shares: 5 }], 5), 'ok', 'tab-custodians'],
    [context.keyEvidence([{ status: 'current', threshold: 3, total_shares: 5 }, { status: 'previous', pinned_products: [{}] }], 5), 'attention'],
    [context.copiesEvidence(undefined), 'checking'],
    [context.copiesEvidence(null), 'unavailable'],
    [context.copiesEvidence([]), 'missing', 'tab-replication'],
    [context.copiesEvidence([{ status: 'error', last_sync_at: day }, { status: 'active' }]), 'failing'],
    [context.copiesEvidence([{ status: 'active' }]), 'attention'],
    [context.copiesEvidence([{ status: 'active', last_sync_at: day }]), 'ok'],
  ];
  for (const [[state, summary, label, go], want, target] of cases) {
    assert.equal(state, want, summary);
    assert(summary && label, `${want} needs a summary and an action`);
    if (target) assert.equal(go, target);
    assert(!/restor(ed|able)|verified restore/i.test(summary), `evidence must not claim a restore: ${summary}`);
  }
  assert.match(context.keyEvidence([{ status: 'current', threshold: 3, total_shares: 5 }], 5)[1], /^3 of 5 cards/);
}
console.log('Overview audit, count, failure, recovery and evidence states passed.');
