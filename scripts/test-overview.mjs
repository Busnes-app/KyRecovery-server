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
      innerHTML: '', className: '', dataset: {},
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
  assert.equal(pill.textContent, expected);
  assert.equal(pill.innerHTML, '', 'the pill text is never parsed as markup');
  assert.equal(pill.dataset.kyStatus, expected === 'No capsules stored' ? 'warning' : undefined);
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
  assert.equal(elements.get('system-status-pill').textContent, '');

  // A later valid response must recover without inventing restore evidence.
  Object.assign(state, { ok: true, networkError: false, jsonError: false,
    body: { capsule_count: 1, custodian_count: 2, audit_append_disabled: false } });
  await context.loadReadiness();
  assert.equal(elements.get('overview-audit-status').textContent, 'Accepting entries');
  assert.equal(elements.get('metric-capsules').textContent, '1');
  assert.equal(elements.get('metric-custodians').textContent, '2');
  assert(!elements.get('system-status-pill').textContent.includes('restore'));
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
  for (const [states, want] of [
    [['attention', 'ok', 'missing'], 2], [['ok', 'failing', 'missing'], 1], [['ok', 'attention', 'unavailable'], 1],
    [['ok', 'ok', 'ok'], -1], [['checking', 'ok', 'checking'], -1], [['missing', 'missing', 'attention'], 0],
  ]) assert.equal(context.leadRow(states), want, states.join());
  assert.match(context.keyEvidence([{ status: 'current', threshold: 3, total_shares: 5 }], 5)[1], /^3 of 5 cards/);
}
console.log('Overview audit, count, failure, recovery and evidence states passed.');

// Freshness history: gaps per source, the late line at interval + 10%, a bounded row, escaped names.
{
  const { context } = runtime({});
  const H = 3600e3, now = Date.now();
  const at = h => new Date(now - h * H).toISOString();
  const row = context.historyRow({
    service_name: '<b>kynotes</b>', state: 'stale', expected_seconds: 24 * 3600,
    instances: [{ state: 'ok', newest: { deposited_at: at(30) } }],
    repos: [{ flags: ['stale'], last_snapshot_at: at(47) }],
    runs: [
      { at: at(100), source: 'a', channel: 'capsule' }, { at: at(98), source: 'u/r', channel: 'restic' },
      { at: at(76), source: 'a', channel: 'capsule' }, { at: at(48), source: 'a', channel: 'capsule' },
      { at: at(47), source: 'u/r', channel: 'restic', corrupt: true }, { at: at(30), source: 'a', channel: 'capsule' },
    ],
  });
  const tones = [...row.matchAll(/data-tone="(\w+)"/g)].map(m => m[1]);
  // a: 24 h, 28 h (late), 18 h; u/r: 51 h but corrupt; now: u/r has waited 47 h (late).
  assert.deepEqual(tones, ['success', 'warning', 'danger', 'success', 'warning']);
  assert.equal((row.match(/history-now/g) || []).length, 1);
  // The stale source leads in the caption and the summary, never a fresher "Last" figure.
  assert.match(row, /aria-label="&lt;b&gt;kynotes&lt;\/b&gt;: 4 gaps between backups, 1 late, 1 corrupt\. Restic 47 h ago, capsule 30 h ago; due every 24 h\."/);
  assert.match(row, /history-fresh"><span>Restic 47 h ago<\/span> · <span>capsule 30 h ago<\/span><\/span><span class="history-due">Every 24 h · 1 late</);
  assert(!row.includes('Last '), 'a stale row never shows a bare Last figure');
  assert(!row.includes('<b>'), 'service names are escaped');
  const long = context.historyRow({ service_name: 'z', state: 'stale', expected_seconds: 24 * 3600,
    runs: [{ at: at(200), source: 'a', channel: 'capsule' }, { at: at(1), source: 'a', channel: 'capsule' }] });
  assert.match(long, /data-tone="warning" style="--h:1\.000"/, 'heights cap at the top of the track');

  const many = Array.from({ length: 30 }, (_, i) => ({ at: at(60 - i), source: 'a', channel: 'capsule' }));
  const full = context.historyRow({ service_name: 'x', state: 'ok', expected_seconds: 3600, runs: many,
    instances: [{ state: 'ok', newest: { deposited_at: at(31) } }], repos: [{ flags: [], last_snapshot_at: at(32) }] });
  assert.match(full, /history-fresh"><span>Last 31 h ago</, 'healthy rows show the newest backup');
  assert.equal((full.match(/class="history-bar[" ]/g) || []).length, 30, '29 gaps and the wait, within 31 slots');
  assert.match(context.historyRow({ service_name: 'y', state: 'never', expected_seconds: 3600, runs: [], instances: [{ state: 'never' }] }),
    /history-fresh"><span>Capsule never</);
  assert.match(context.historyRow({ service_name: 'p', state: 'pending', expected_seconds: 3600, runs: [], instances: [{ state: 'pending' }] }),
    /aria-label="p: No backups yet; due every 1 h\."/);
}
