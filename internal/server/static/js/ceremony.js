// This file drives the only page that ever sees the recovery private key. The key and the
// share strings stay in this tab: the only request it makes carries the three fields the
// import endpoint accepts, named one by one below.
const esc = v => String(v ?? '').replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

const ready = (async () => {
  const go = new Go();
  const { instance } = await WebAssembly.instantiateStreaming(fetch('/static/wasm/ceremony.wasm'), go.importObject);
  go.run(instance); // main blocks on select{}; kyCeremony is registered by the time this yields
})();

let result = null;

// A store that already holds a key makes this a rotation. The page names the key it will
// replace, and the import says which key that was, so a tab left open across another
// rotation is refused rather than replacing a key its operator never saw.
let replaces = null;
fetch('/api/recovery-key').then(res => (res.ok ? res.json() : null)).then(k => {
  if (!k || !k.key_id) return;
  replaces = k.key_id;
  document.getElementById('current-key-id').textContent = k.key_id;
  document.getElementById('rotation-notice').hidden = false;
}).catch(() => {});

document.getElementById('ceremony-form').addEventListener('submit', async ev => {
  ev.preventDefault();
  await ready;
  const k = Number(document.getElementById('k').value), n = Number(document.getElementById('n').value);
  const r = globalThis.kyCeremony(k, n);
  const err = document.getElementById('error');
  if (r.error) { err.textContent = r.error; err.hidden = false; return; }
  err.hidden = true;
  result = r;
  document.getElementById('key-id').textContent = r.key_id;
  document.getElementById('card-list').innerHTML = r.shares.map((s, i) => `
    <div class="panel card">
      <h3>Custodian card ${i + 1} of ${n} &mdash; ${k} needed</h3>
      <p>Recovery key <code>${esc(r.key_id)}</code></p>
      <p>Share <code class="share">${esc(s)}</code></p>
      <p>Custodian: ____________________ &nbsp; Date: ${new Date().toISOString().slice(0, 10)}</p>
    </div>`).join('');
  document.getElementById('printed-n').textContent = n;
  document.getElementById('cards').hidden = false;
  document.getElementById('ceremony-form').hidden = true;
});

// Import waits for the cards: an imported key whose cards were never printed leaves every
// product paired to a key nobody can open. The tick unlocks once the print dialog opened.
let imported = false;
const printed = document.getElementById('printed');
document.getElementById('print-btn').addEventListener('click', () => {
  window.print();
  if (!imported) printed.disabled = false;
});
printed.addEventListener('change', () => {
  document.getElementById('import-btn').disabled = imported || !printed.checked;
});

document.getElementById('import-btn').addEventListener('click', async ev => {
  if (!result) return;
  // One import per key: a second click would be refused and overwrite the success line.
  const button = ev.currentTarget;
  button.disabled = true;
  printed.disabled = true; // unticking and re-ticking must not re-enable Import mid-request
  const status = document.getElementById('import-status');
  // Only these fields are sent: the public half, the topology and, for a rotation, the
  // key being replaced. The shares are never in this request.
  const fields = { public_key: result.public_key_b64, threshold: result.threshold, total_shares: result.total_shares };
  if (replaces) fields.replaces_key_id = replaces;
  let res;
  try {
    res = await fetch(replaces ? '/api/recovery-key/rotate' : '/api/recovery-key', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(fields),
    });
  } catch {
    // Nothing reached the store: the same cards can be imported again.
    status.textContent = 'Import failed: the server could not be reached. Try again.';
    button.disabled = false;
    printed.disabled = false;
    return;
  }
  const body = await res.json().catch(() => ({}));
  if (!res.ok) { status.textContent = `Import failed: ${body.error || res.status}`; button.disabled = false; printed.disabled = false; return; }
  imported = true;
  printed.disabled = true;
  const repair = (body.products_to_repair || []).map(p => `${p.service_name} (${p.app_name})`).join(', ');
  status.textContent = replaces
    ? `Rotated to key ${body.key_id}; ${body.replaces_key_id} is now previous. Re-pair ${repair || 'each product'} and confirm this key ID there, then close this tab.`
    : `Imported key ${body.key_id}. Hand out the cards, then close this tab.`;
});
