// Every value below comes from the database, and some of it was typed by someone
// who never authenticated. Two contexts, two escapes:
//   esc   — HTML text and ordinary attribute values.
//   escJs — a value that lands inside a JS string literal inside an attribute,
//           i.e. onclick="fn('...')". HTML-escaping alone is not enough there:
//           the browser decodes the entity before parsing the handler as script,
//           so the quote has to be neutralised for JS first and for HTML second.
const esc = v => String(v ?? '').replace(/[&<>"']/g, c =>
  ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));

const escJs = v => esc(String(v ?? '').replace(/[\\'"\r\n]/g, c =>
  '\\' + ({ '\r': 'r', '\n': 'n' }[c] || c)));

document.addEventListener('DOMContentLoaded', () => {
  initTabs();
  document.getElementById('tab-backups').addEventListener('click', onBackupsClick);
  loadAuthUser();
  loadReadiness();
  loadCapsules();
  loadPairing();
  loadCustodians();
  loadRecoveryKey();
  loadReplication();
  loadAudit();

  // Polling for freshness
  setInterval(() => {
    loadReadiness();
    loadReplication();
  }, 10000);
});

// Phones scroll the tab row sideways: keep the current tab in view, also after Overview links.
// Only the row scrolls; the row is not sticky, so scrollIntoView would also move the page.
function revealTab(tab) {
  const row = tab.parentElement.getBoundingClientRect(), item = tab.getBoundingClientRect();
  if (item.left < row.left) tab.parentElement.scrollLeft -= row.left - item.left;
  else if (item.right > row.right) tab.parentElement.scrollLeft += item.right - row.right;
  if (item.top < row.top) tab.parentElement.scrollTop -= row.top - item.top;
  else if (item.bottom > row.bottom) tab.parentElement.scrollTop += item.bottom - row.bottom;
}

// Tab Navigation
function initTabs() {
  const tabs = document.querySelectorAll('.tab-btn');
  tabs.forEach(tab => {
    tab.addEventListener('click', () => {
      tabs.forEach(t => { t.classList.remove('active'); t.removeAttribute('aria-current'); });
      document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

      tab.classList.add('active');
      tab.setAttribute('aria-current', 'page');
      const target = document.getElementById(tab.dataset.tab);
      if (target) target.classList.add('active');
      revealTab(tab);
    });
  });
  const revealCurrent = () => {
    const current = document.querySelector('.tab-btn[aria-current="page"]');
    if (current) revealTab(current);
  };
  window.addEventListener('resize', revealCurrent);
  document.fonts.ready.then(revealCurrent);
}

// 1. Readiness & System Status
async function loadReadiness() {
  try {
    const res = await fetch('/api/readiness');
    if (!res.ok) return;
    const data = await res.json();

    document.getElementById('overview-audit-status').textContent = data.audit_append_disabled === true ? 'Appends blocked' : data.audit_append_disabled === false ? 'Accepting entries' : 'Unknown';
    const capsuleCount = Number(data.capsule_count) || 0;
    document.getElementById('metric-capsules').textContent = capsuleCount;
    document.getElementById('metric-custodians').textContent = Number(data.custodian_count) || 0;

    // The pill states a fact the store can check. It must never imply a verified
    // restore: nothing here can open a capsule.
    const statusPill = document.getElementById('system-status-pill');
    const dot = '<span class="dot"></span> ';
    statusPill.className = capsuleCount > 0 ? 'status-pill ready' : 'status-pill warning';
    statusPill.innerHTML = capsuleCount > 0
      ? dot + capsuleCount + ' capsule' + (capsuleCount === 1 ? '' : 's') + ' stored'
      : dot + 'No capsules stored';
  } catch (err) {
    console.error('Error fetching readiness:', err);
  }
}

// 2. Capsules List
async function loadCapsules() {
  const tbody = document.getElementById('capsules-table-body');
  if (!tbody) return;

  try {
    const res = await fetch('/api/capsules');
    if (!res.ok) return;
    const capsules = await res.json() || [];

    if (capsules.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" style="text-align:center; color: var(--text-dim);">No capsules yet. Pair a product and it will deposit its own sealed backups here.</td></tr>';
      return;
    }

    tbody.innerHTML = capsules.map(c => `
      <tr>
        <td><code>${esc(c.id)}</code></td>
        <td><strong>${esc(c.service_name)}</strong></td>
        <td><span class="status-pill ready">${esc(c.threshold)} of ${esc(c.total_shares)} Shares</span></td>
        <td>${(c.size_bytes / 1024).toFixed(1)} KB</td>
        <td><code title="${esc(c.payload_hash)}">${esc(String(c.payload_hash).substring(0, 12))}...</code></td>
        <td>${new Date(c.created_at).toLocaleString()}</td>
        <td>
          <a class="btn btn-secondary btn-sm" href="/api/capsules/${encodeURIComponent(c.id)}/download" download>Download</a>
        </td>
      </tr>
    `).join('');
  } catch (err) {
    console.error('Error fetching capsules:', err);
  }
}

// 3. Custodians
async function loadCustodians() {
  const tbody = document.getElementById('custodians-table-body');
  if (!tbody) return;

  try {
    const res = await fetch('/api/custodians');
    if (!res.ok) return;
    const list = await res.json() || [];

    if (list.length === 0) {
      tbody.innerHTML = '<tr><td colspan="4" style="text-align:center; color: var(--text-dim);">No custodians yet.</td></tr>';
      return;
    }

    tbody.innerHTML = list.map(c => `
      <tr>
        <td><strong>${esc(c.name)}</strong></td>
        <td>${esc(c.email)}</td>
        <td><code>${esc(c.fingerprint)}</code></td>
        <td>${new Date(c.created_at).toLocaleString()}</td>
      </tr>
    `).join('');
  } catch (err) {
    console.error('Error fetching custodians:', err);
  }
}

// The public halves of every ceremony's keypair. The server never has more than these, so
// this is the whole status the dashboard can show. The current key is what products pin and
// deposits must be sealed to; a previous key still opens the capsules sealed to it.
async function loadRecoveryKey() {
  const el = document.getElementById('recovery-key-status');
  // The overview caption states the real quorum, which the ceremony fixes; the
  // custodian count next to it is a directory and must never be read as one.
  const quorum = document.getElementById('metric-custodians-sub');
  if (!el) return;
  try {
    const res = await fetch('/api/recovery-keys');
    if (!res.ok) return;
    const keys = await res.json() || [];
    const k = keys.find(x => x.status === 'current');
    // After the first import the ceremony page rotates the key; say so on the way in.
    const link = document.getElementById('ceremony-link');
    if (link) link.textContent = k ? 'Rotate recovery key' : 'Run recovery key ceremony';
    if (!k) {
      el.textContent = 'No recovery key \u2014 run the ceremony';
      if (quorum) quorum.textContent = 'No ceremony run yet, so the recovery key cannot be reconstructed.';
      return;
    }
    if (quorum) quorum.textContent = `${k.threshold} of ${k.total_shares} custodian cards reconstruct the recovery key.`;
    el.innerHTML = '<div style="overflow-x: auto;"><table class="data-table"><thead><tr><th>Key ID</th><th>Status</th><th>Cards</th><th>Capsules</th><th></th><th></th></tr></thead><tbody id="recovery-keys-body"></tbody></table></div>';
    document.getElementById('recovery-keys-body').innerHTML = keys.map(x => `<tr>
      <td><code>${esc(x.key_id)}</code>${x.status === 'previous' && (x.pinned_products || []).length ? `<br><small>Still pinned by ${esc(x.pinned_products.map(p => `${p.service_name} (${p.app_name})`).join(', '))}; re-pair them.</small>` : ''}</td>
      <td>${esc(x.status)}</td><td>${esc(x.threshold)} of ${esc(x.total_shares)}</td><td>${esc(x.capsule_count)}</td>
      <td>${x.status === 'retired' ? `retired by ${esc(x.retired_by)} on ${esc(new Date(x.retired_at).toLocaleString())}` : `imported by ${esc(x.imported_by)} on ${esc(new Date(x.imported_at).toLocaleString())}`}</td>
      <td>${x.status === 'previous' && isAdminUser ? `<button class="btn btn-secondary btn-sm" onclick="retireRecoveryKey('${escJs(x.key_id)}')">Retire</button>` : ''}</td>
    </tr>`).join('');
  } catch (err) {
    console.error('Error fetching recovery keys:', err);
  }
}

// Retiring says the key's cards are no longer needed. The server refuses while any retained
// capsule is sealed to the key and names them; this shows that list rather than guessing.
async function retireRecoveryKey(keyId) {
  if (!confirm(`Retire recovery key ${keyId}?\n\nRetire only once no backup you might restore is sealed to it, including offsite copies that local retention no longer lists. Its cards cannot open anything sealed to the current key.`)) return;
  try {
    const res = await fetch('/api/recovery-key/retire', {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ key_id: keyId }),
    });
    const body = await res.json().catch(() => ({}));
    if (!res.ok) {
      const caps = (body.capsules || []).map(c => `${c.id} (${c.service_name}, ${new Date(c.deposited_at).toLocaleString()})`);
      alert(`Not retired: ${body.error || res.status}${caps.length ? '\n\nCapsules still sealed to this key:\n' + caps.join('\n') : ''}`);
      return;
    }
    loadRecoveryKey();
  } catch (err) {
    alert(`Retire failed: ${err.message}`);
  }
}

// 5. Audit Ledger
async function loadAudit() {
  const tbody = document.getElementById('audit-table-body');
  if (!tbody) return;

  try {
    const res = await fetch('/api/audit');
    if (!res.ok) return;
    const events = await res.json() || [];

    if (events.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" style="text-align:center; color: var(--text-dim);">No ledger entries yet.</td></tr>';
      return;
    }

    tbody.innerHTML = events.map(e => `
      <tr>
        <td><strong>#${esc(e.sequence_num)}</strong></td>
        <td><code>${esc(e.action)}</code></td>
        <td>${esc(e.actor)}</td>
        <td><code>${esc(e.target_id)}</code></td>
        <td><code title="${esc(e.event_hash)}">${esc(String(e.event_hash).substring(0, 14))}...</code></td>
        <td>${new Date(e.created_at).toLocaleString()}</td>
      </tr>
    `).join('');
  } catch (err) {
    console.error('Error fetching audit:', err);
  }
}

// Verify Chain
async function verifyAuditChain() {
  const badge = document.getElementById('chain-verify-status');
  badge.textContent = 'Verifying cryptographic chain...';
  badge.style.color = 'var(--text-muted)';

  try {
    const res = await fetch('/api/audit/verify', { method: 'POST' });
    const data = await res.json();
    if (data.valid) {
      badge.textContent = `✓ Cryptographic Chain Verified (${data.count} events)`;
      badge.style.color = 'var(--accent-green)';
    } else {
      badge.textContent = `✗ Chain Broken: ${data.error || 'verification failed'}`;
      badge.style.color = 'var(--accent-red)';
    }
  } catch (err) {
    badge.textContent = 'Chain verification error';
    badge.style.color = 'var(--accent-red)';
  }
}

// Custodian Add
function openCustodianModal() {
  document.getElementById('custodian-modal').classList.add('open');
}
function closeCustodianModal() {
  document.getElementById('custodian-modal').classList.remove('open');
}
async function submitCustodian(e) {
  e.preventDefault();
  const name = document.getElementById('custodian-name').value;
  const email = document.getElementById('custodian-email').value;

  try {
    const res = await fetch('/api/custodians', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, email })
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed to add custodian');
    closeCustodianModal();
    loadCustodians();
    loadReadiness();
  } catch (err) {
    alert('Error: ' + err.message);
  }
}

// 6. Pairing Management
async function loadPairing() {
  const tbody = document.getElementById('pairing-table-body');
  if (!tbody) return;

  try {
    const res = await fetch('/api/pairing/list');
    if (!res.ok) return;
    const list = await res.json() || [];

    if (list.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" style="text-align:center; color: var(--text-dim);">No paired products yet. Create a pairing code and enter it in a Ky Suite app.</td></tr>';
      return;
    }

    tbody.innerHTML = list.map(p => `
      <tr>
        <td><strong>${esc(p.app_name)}</strong></td>
        <td><code>${esc(p.service_name)}</code></td>
        <td>
          ${p.status === 'pending'
            ? `<span style="color: var(--text-dim); font-size: 12px;">Code shown once, when generated</span>`
            : `<code style="color: var(--text-dim);">${esc(p.id)}</code>`}
        </td>
        <td>
          <span class="status-pill ${p.status === 'paired' ? 'ready' : (p.status === 'pending' ? 'warning' : 'danger')}">
            <span class="dot"></span> ${esc(String(p.status))}
          </span>
        </td>
        <td>${p.last_backup_at ? new Date(p.last_backup_at).toLocaleString() : '<span style="color: var(--text-dim);">Never</span>'}</td>
        <td>
          <div style="font-size: 12px;">${p.status === 'paired' && p.paired_at ? new Date(p.paired_at).toLocaleString() : 'Expires ' + new Date(p.expires_at).toLocaleTimeString()}</div>
        </td>
        <td class="admin-only" style="display:none; text-align:right;">
          ${p.status === 'revoked' ? `<button class="btn btn-secondary btn-sm clear-pairing" data-id="${esc(p.id)}" data-app-name="${esc(p.app_name)}">Clear</button>` : `<button class="btn btn-danger btn-sm revoke-pairing" data-id="${esc(p.id)}" data-app-name="${esc(p.app_name)}">Revoke</button>`}
        </td>
      </tr>
    `).join('');
    // Bound here rather than inline: app_name is chosen by whoever claims a code, so it
    // never goes inside a script string.
    tbody.querySelectorAll('.clear-pairing').forEach(btn => {
      btn.addEventListener('click', () => clearPairing(btn.dataset.id, btn.dataset.appName));
    });
    tbody.querySelectorAll('.revoke-pairing').forEach(btn => {
      btn.addEventListener('click', () => revokePairing(btn.dataset.id, btn.dataset.appName));
    });
    applyAdminOnly();
  } catch (err) {
    console.error('Error fetching pairing list:', err);
  }
}

// Revoking ends a product's deposits: its token is refused from the next request on. The
// capsules it already deposited stay. The product side keeps its pairing record until its
// admin unpairs there, so tell them.
async function revokePairing(id, appName) {
  if (!confirm(`Revoke ${appName}'s pairing?\n\nIts deposits will be refused from now on. Capsules already held are kept. Unpair on the product's Disaster recovery screen as well, or its scheduled backups will keep failing.`)) return;
  try {
    const res = await fetch('/api/pairing/revoke', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      alert(body.error || `Revoke failed (HTTP ${res.status})`);
      return;
    }
    await loadPairing();
    if (typeof loadAudit === 'function') loadAudit();
  } catch (err) {
    console.error('Error revoking pairing:', err);
  }
}

async function clearPairing(id, appName) {
  if (!confirm(`Clear ${appName}'s revoked registration from this list? Existing backups and audit history will be kept.`)) return;
  try {
    const res = await fetch('/api/pairing/clear', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id })
    });
    await loadPairing();
    loadAudit();
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      alert(body.error || `Clear failed (HTTP ${res.status})`);
    }
  } catch (err) {
    alert('Could not clear the revoked registration. Refresh and try again.');
  }
}

let isAdminUser = false;
function applyAdminOnly() {
  document.querySelectorAll('.admin-only').forEach(el => { el.style.display = isAdminUser ? '' : 'none'; });
}

function openPairingGenModal() {
  document.getElementById('pairing-gen-modal').classList.add('open');
}

function closePairingGenModal() {
  document.getElementById('pairing-gen-modal').classList.remove('open');
}

async function submitGeneratePairing(e) {
  e.preventDefault();
  const serviceName = document.getElementById('pairing-service').value;
  const appName = document.getElementById('pairing-app-name').value;
  const ttl = parseInt(document.getElementById('pairing-ttl').value, 10);

  try {
    const res = await fetch('/api/pairing/generate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ service_name: serviceName, app_name: appName, ttl_minutes: ttl })
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed generating pairing code');

    closePairingGenModal();

    // Show display modal
    document.getElementById('display-pairing-code').textContent = data.pairing_code;
    document.getElementById('display-pairing-expires').textContent = `${ttl} minutes (${new Date(data.expires_at).toLocaleTimeString()})`;
    document.getElementById('display-pairing-url').textContent = window.location.origin;
    document.getElementById('pairing-code-display-modal').classList.add('open');

    loadPairing();
    loadAudit();
  } catch (err) {
    alert('Error generating pairing code: ' + err.message);
  }
}

function closePairingDisplayModal() {
  document.getElementById('pairing-code-display-modal').classList.remove('open');
}

// 7. Authentication State Management & SSO Gateway
let currentUser = null;
let ssoConfig = null;

async function loadAuthUser() {
  try {
    const [meRes, ssoRes] = await Promise.all([
      fetch('/api/auth/me'),
      fetch('/api/auth/sso/config')
    ]);

    const data = meRes.ok ? await meRes.json() : { authenticated: false };
    ssoConfig = ssoRes.ok ? await ssoRes.json() : null;

    const nameEl = document.getElementById('user-display-name');
    const roleEl = document.getElementById('user-role-badge');
    const btn = document.getElementById('btn-auth-action');
    const btnSSOPair = document.getElementById('btn-sso-pair-header');
    const btnChangePass = document.getElementById('btn-change-pass-header');
    const loginGateway = document.getElementById('login-gateway-view');
    const dashboardView = document.getElementById('dashboard-view');
    const ssoActiveSection = document.getElementById('sso-active-section');
    document.body.classList.toggle('signed-in', !!(data.authenticated && data.user));

    // Show SSO sign-in button only if SSO is paired & active
    if (ssoActiveSection) {
      ssoActiveSection.style.display = (ssoConfig && ssoConfig.enabled && ssoConfig.issuer_url) ? 'block' : 'none';
    }

    if (data.authenticated && data.user) {
      currentUser = data.user;
      if (nameEl) nameEl.textContent = data.user.name || data.user.email || data.user.username;
      if (roleEl) {
        roleEl.textContent = data.user.role || 'operator';
        roleEl.className = `status-pill ${data.user.role === 'admin' ? 'ready' : 'warning'}`;
      }
      if (btn) {
        btn.textContent = 'Sign Out';
        btn.className = 'btn btn-secondary btn-sm';
      }
      if (btnSSOPair) btnSSOPair.style.display = data.user.role === 'admin' ? 'inline-flex' : 'none';
      isAdminUser = data.user.role === 'admin';
      applyAdminOnly();
      if (btnChangePass) btnChangePass.style.display = 'inline-flex';

      if (loginGateway) loginGateway.style.display = 'none';
      if (dashboardView) dashboardView.style.display = 'flex';

      // Load protected dashboard data
      loadReadiness();
      loadBackups();
      loadCapsules();
      loadRetention();
      loadPairing();
      loadCustodians();
      loadAudit();
      loadReplication();
      loadRecoveryKey();
    } else {
      currentUser = null;
      if (nameEl) nameEl.textContent = 'Unauthenticated';
      if (roleEl) {
        roleEl.textContent = 'Guest';
        roleEl.className = 'status-pill danger';
      }
      if (btn) {
        btn.textContent = 'Sign In';
        btn.className = 'btn btn-primary btn-sm';
      }
      if (btnSSOPair) btnSSOPair.style.display = 'none';
      if (btnChangePass) btnChangePass.style.display = 'none';

      if (loginGateway) loginGateway.style.display = 'block';
      if (dashboardView) dashboardView.style.display = 'none';
    }
  } catch (err) {
    console.error('Error fetching auth user:', err);
  }
}

async function submitLocalLogin(e) {
  e.preventDefault();
  const username = document.getElementById('login-username').value.trim();
  const password = document.getElementById('login-password').value;
  const errBox = document.getElementById('login-error-box');
  const btn = document.getElementById('btn-local-login-submit');

  errBox.style.display = 'none';
  btn.disabled = true;
  btn.textContent = 'Verifying credentials...';

  try {
    const res = await fetch('/api/auth/login/local', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, password })
    });

    const data = await res.json();
    if (!res.ok) {
      throw new Error(data.error || 'Authentication failed');
    }

    // Success
    document.getElementById('login-password').value = '';
    await loadAuthUser();
  } catch (err) {
    errBox.textContent = err.message;
    errBox.style.display = 'block';
  } finally {
    btn.disabled = false;
    btn.textContent = 'Sign In as Local Admin';
  }
}

function handleLogin() {
  window.location.href = '/api/auth/login';
}

async function handleLogout() {
  try {
    await fetch('/api/auth/logout', { method: 'POST' });
    currentUser = null;
    await loadAuthUser();
  } catch (err) {
    console.error('Logout error:', err);
  }
}

function handleAuthAction() {
  if (currentUser) {
    handleLogout();
  } else {
    const loginGateway = document.getElementById('login-gateway-view');
    if (loginGateway) {
      loginGateway.scrollIntoView({ behavior: 'smooth' });
      document.getElementById('login-password').focus();
    }
  }
}

// 7b. SSO Pairing & Config Modal
async function openSSOConfigModal() {
  const statusBox = document.getElementById('sso-test-status');
  if (statusBox) statusBox.style.display = 'none';

  try {
    const res = await fetch('/api/auth/sso/config');
    if (res.ok) {
      const cfg = await res.json();
      document.getElementById('sso-enabled-check').checked = !!cfg.enabled;
      document.getElementById('sso-issuer-input').value = cfg.issuer_url || '';
      document.getElementById('sso-client-id-input').value = cfg.client_id || 'kyrecovery-server';
      document.getElementById('sso-client-secret-input').value = cfg.client_secret || '';
      document.getElementById('sso-redirect-input').value = cfg.redirect_url || (window.location.origin + '/api/auth/callback');
      document.getElementById('sso-admin-email-input').value = cfg.admin_email || '';
    }
  } catch (err) {
    console.error('Error fetching SSO config:', err);
  }

  document.getElementById('sso-config-modal').classList.add('open');
}

function closeSSOConfigModal() {
  document.getElementById('sso-config-modal').classList.remove('open');
}

async function testSSOConnection() {
  const statusBox = document.getElementById('sso-test-status');
  const issuer = document.getElementById('sso-issuer-input').value.trim();

  if (!issuer) {
    alert('Please enter a KySignOn Issuer URL to test.');
    return;
  }

  statusBox.style.display = 'block';
  statusBox.style.background = 'var(--bg-input)';
  statusBox.style.color = 'var(--accent-cyan)';
  statusBox.textContent = 'Contacting KySignOn authority at ' + issuer + '...';

  try {
    const res = await fetch('/api/auth/sso/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ issuer_url: issuer })
    });
    const data = await res.json();
    if (data.success) {
      statusBox.style.background = 'var(--accent-green-dim)';
      statusBox.style.color = 'var(--accent-green)';
      statusBox.textContent = '✓ ' + data.message;
    } else {
      statusBox.style.background = 'var(--accent-red-dim)';
      statusBox.style.color = 'var(--accent-red)';
      statusBox.textContent = '✗ ' + (data.error || 'Connection failed');
    }
  } catch (err) {
    statusBox.style.background = 'var(--accent-red-dim)';
    statusBox.style.color = 'var(--accent-red)';
    statusBox.textContent = '✗ Network test error: ' + err.message;
  }
}

async function submitSSOConfig(e) {
  e.preventDefault();
  const payload = {
    enabled: document.getElementById('sso-enabled-check').checked,
    issuer_url: document.getElementById('sso-issuer-input').value.trim(),
    client_id: document.getElementById('sso-client-id-input').value.trim(),
    client_secret: document.getElementById('sso-client-secret-input').value.trim(),
    redirect_url: document.getElementById('sso-redirect-input').value.trim(),
    admin_email: document.getElementById('sso-admin-email-input').value.trim()
  };

  try {
    const res = await fetch('/api/auth/sso/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed saving SSO settings');

    closeSSOConfigModal();
    alert('✓ KySignOn SSO configuration saved successfully!');
    await loadAuthUser();
  } catch (err) {
    alert('SSO Config Error: ' + err.message);
  }
}

// 7c. Change Local Password Modal
function openChangePasswordModal() {
  document.getElementById('pass-current').value = '';
  document.getElementById('pass-new').value = '';
  document.getElementById('pass-confirm').value = '';
  document.getElementById('change-pass-error').style.display = 'none';
  document.getElementById('change-password-modal').classList.add('open');
}

function closeChangePasswordModal() {
  document.getElementById('change-password-modal').classList.remove('open');
}

async function submitChangePassword(e) {
  e.preventDefault();
  const oldPassword = document.getElementById('pass-current').value;
  const newPassword = document.getElementById('pass-new').value;
  const confirmPassword = document.getElementById('pass-confirm').value;
  const errBox = document.getElementById('change-pass-error');

  if (newPassword !== confirmPassword) {
    errBox.textContent = 'New password and confirmation do not match.';
    errBox.style.display = 'block';
    return;
  }

  try {
    const res = await fetch('/api/auth/password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ old_password: oldPassword, new_password: newPassword })
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed changing password');

    closeChangePasswordModal();
    alert('✓ Password updated successfully!');
  } catch (err) {
    errBox.textContent = err.message;
    errBox.style.display = 'block';
  }
}

// 9. Offsite Replication Management
async function loadReplication() {
  await Promise.all([loadReplicationTargets(), loadReplicationLogs()]);
}

async function loadReplicationTargets() {
  const tbody = document.getElementById('replication-targets-table-body');
  if (!tbody) return;

  try {
    const res = await fetch('/api/replication/targets');
    if (!res.ok) return;
    const list = await res.json() || [];

    if (list.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" style="text-align:center; color: var(--text-dim);">No replication targets yet. Add one and every new capsule is copied offsite.</td></tr>';
      return;
    }

    tbody.innerHTML = list.map(t => `
      <tr>
        <td><strong>${esc(t.name)}</strong></td>
        <td><span class="pill-badge">${esc(String(t.type))}</span></td>
        <td>
          <code style="font-size: 12px;">${esc(t.type === 's3' ? (t.bucket + ' (' + t.region + ')') : t.type === 'smb' ? (t.endpoint + '/' + t.bucket) : t.endpoint)}</code>
        </td>
        <td>
          <span class="status-pill ${t.auto_sync ? 'ready' : 'warning'}">
            <span class="dot"></span> ${t.auto_sync ? 'ENABLED' : 'MANUAL'}
          </span>
        </td>
        <td>
          <span class="status-pill ${t.status === 'active' ? 'ready' : (t.status === 'error' ? 'danger' : 'warning')}">
            <span class="dot"></span> ${esc(String(t.status))}
          </span>
        </td>
        <td>${t.last_sync_at ? new Date(t.last_sync_at).toLocaleString() : '<span style="color: var(--text-dim);">Never</span>'}</td>
        <td>
          <button class="btn btn-secondary btn-sm" onclick="syncAllToTarget('${escJs(t.id)}')">Sync All</button>
          <button class="btn btn-sm" style="color: var(--accent-red); margin-left: 4px;" onclick="deleteReplicationTarget('${escJs(t.id)}')">Delete</button>
        </td>
      </tr>
    `).join('');
  } catch (err) {
    console.error('Error loading replication targets:', err);
  }
}

async function loadReplicationLogs() {
  const tbody = document.getElementById('replication-logs-table-body');
  if (!tbody) return;

  try {
    const res = await fetch('/api/replication/logs');
    if (!res.ok) return;
    const list = await res.json() || [];

    if (list.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" style="text-align:center; color: var(--text-dim);">No transfers yet.</td></tr>';
      return;
    }

    tbody.innerHTML = list.map(l => `
      <tr>
        <td><span style="font-size: 12px;">${new Date(l.created_at).toLocaleString()}</span></td>
        <td><code>${esc(l.target_id)}</code></td>
        <td><code>${esc(l.capsule_id)}</code></td>
        <td>${(l.bytes_transferred / 1024).toFixed(1)} KB</td>
        <td>${Number(l.duration_ms) || 0} ms</td>
        <td>
          <span class="status-pill ${l.status === 'success' ? 'ready' : 'danger'}">
            <span class="dot"></span> ${esc(String(l.status))}
          </span>
        </td>
      </tr>
    `).join('');
  } catch (err) {
    console.error('Error loading replication logs:', err);
  }
}

function openReplicationModal() {
  applyReplicationPreset();
  document.getElementById('repl-test-status').style.display = 'none';
  document.getElementById('replication-modal').classList.add('open');
}

function closeReplicationModal() {
  document.getElementById('replication-modal').classList.remove('open');
}

function applyReplicationPreset() {
  const preset = document.getElementById('repl-preset').value;
  const s3Fields = document.getElementById('repl-s3-fields');
  const endpointLabel = document.getElementById('repl-endpoint-label');
  const endpointInput = document.getElementById('repl-endpoint');
  const show = (id, on) => { document.getElementById(id).style.display = on ? '' : 'none'; };
  const label = (id, text) => { document.getElementById(id).textContent = text; };

  show('repl-host-key-group', preset === 'sftp');
  show('repl-smb-warning', preset === 'smb');
  show('repl-bucket-group', preset !== 'sftp');
  show('repl-region-group', preset !== 'sftp' && preset !== 'smb');
  label('repl-bucket-label', preset === 'smb' ? 'Share Name' : 'Bucket Name');
  label('repl-access-key-label', preset === 'sftp' || preset === 'smb' ? 'Username' : 'Access Key ID');
  label('repl-secret-key-label', preset === 'sftp' ? 'Password or PEM Private Key' : preset === 'smb' ? 'Password' : 'Secret Access Key');
  label('repl-prefix-label', preset === 'sftp' || preset === 'smb' ? 'Remote Directory' : 'Prefix / Subdirectory');
  document.getElementById('repl-secret-key').placeholder = preset === 'sftp' ? 'Password, or paste a PEM private key' : preset === 'smb' ? 'Password' : 'Secret access key';
  document.getElementById('repl-access-key').placeholder = preset === 'sftp' ? 'user' : preset === 'smb' ? 'user or DOMAIN\\user' : 'Access key';
  // The field is prefilled with capsules/, which would override a directory
  // pasted in an SMB path; leave it blank there and say where it comes from.
  const prefixInput = document.getElementById('repl-prefix');
  if (preset === 'smb') {
    if (prefixInput.value === 'capsules/') prefixInput.value = '';
    prefixInput.placeholder = 'from the host path, or a directory in the share';
  } else {
    if (prefixInput.value === '') prefixInput.value = 'capsules/';
    prefixInput.placeholder = 'capsules/';
  }

  if (preset === 'local') {
    s3Fields.style.display = 'none';
    endpointLabel.textContent = 'Destination Local Directory / Mount Path';
    endpointInput.placeholder = '/mnt/cold-storage/kyrecovery-vault';
  } else if (preset === 'sftp') {
    s3Fields.style.display = 'block';
    endpointLabel.textContent = 'SSH Host (host or host:port)';
    endpointInput.placeholder = 'nas.lan:22';
  } else if (preset === 'smb') {
    s3Fields.style.display = 'block';
    endpointLabel.textContent = 'SMB Host (host, host:port, or //host/share/dir)';
    endpointInput.placeholder = 'nas.lan  or  //nas.lan/backups/kyrecovery';
    document.getElementById('repl-bucket').placeholder = 'backups';
    document.getElementById('repl-access-key').placeholder = 'user or DOMAIN\\user';
  } else {
    s3Fields.style.display = 'block';
    endpointLabel.textContent = 'S3 Endpoint URL';
    if (preset === 'r2') {
      endpointInput.placeholder = 'https://<account-id>.r2.cloudflarestorage.com';
      document.getElementById('repl-region').value = 'auto';
    } else if (preset === 'minio') {
      endpointInput.placeholder = 'http://localhost:9000';
      document.getElementById('repl-region').value = 'us-east-1';
    } else {
      endpointInput.placeholder = 'https://s3.us-east-1.amazonaws.com';
      document.getElementById('repl-region').value = 'us-east-1';
    }
  }
}

function getReplicationTargetPayload() {
  const preset = document.getElementById('repl-preset').value;
  const type = ['local', 'sftp', 'smb'].includes(preset) ? preset : 's3';

  return {
    name: document.getElementById('repl-name').value.trim(),
    type: type,
    endpoint: document.getElementById('repl-endpoint').value.trim(),
    bucket: document.getElementById('repl-bucket') ? document.getElementById('repl-bucket').value.trim() : '',
    region: document.getElementById('repl-region') ? document.getElementById('repl-region').value.trim() : 'us-east-1',
    access_key: document.getElementById('repl-access-key') ? document.getElementById('repl-access-key').value.trim() : '',
    secret_key: document.getElementById('repl-secret-key') ? document.getElementById('repl-secret-key').value.trim() : '',
    // For SMB a blank directory lets a pasted //host/share/dir supply it.
    prefix: document.getElementById('repl-prefix').value.trim() || (type === 'smb' ? '' : 'capsules/'),
    host_key: document.getElementById('repl-host-key').value.trim(),
    auto_sync: document.getElementById('repl-auto-sync').checked
  };
}

async function testReplicationConnection() {
  const statusBox = document.getElementById('repl-test-status');
  statusBox.style.display = 'block';
  statusBox.style.background = 'var(--bg-input)';
  statusBox.style.color = 'var(--accent-cyan)';
  statusBox.textContent = 'Testing connection and write permissions...';

  const payload = getReplicationTargetPayload();

  try {
    const res = await fetch('/api/replication/targets/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json();
    if (data.success) {
      statusBox.style.color = 'var(--accent-green)';
      statusBox.textContent = '✓ ' + data.message;
    } else if (data.host_key) {
      // Not trusted yet: show the fingerprint and let the operator confirm it.
      document.getElementById('repl-host-key').value = data.host_key;
      statusBox.style.color = 'var(--accent-yellow, #e0b040)';
      statusBox.textContent = 'Server host key ' + data.host_key + ' filled in. Check it matches the server (ssh-keygen -lf) and test again.';
    } else {
      statusBox.style.color = 'var(--accent-red)';
      statusBox.textContent = '✗ ' + (data.error || 'Connection failed');
    }
  } catch (err) {
    statusBox.style.color = 'var(--accent-red)';
    statusBox.textContent = '✗ Test failed: ' + err.message;
  }
}

async function submitReplicationTarget(e) {
  e.preventDefault();
  const payload = getReplicationTargetPayload();

  try {
    const res = await fetch('/api/replication/targets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.error || 'Failed saving target');
    }

    closeReplicationModal();
    loadReplication();
    loadAudit();
  } catch (err) {
    alert('Failed adding replication target: ' + err.message);
  }
}

async function deleteReplicationTarget(id) {
  if (!confirm(`Delete replication target ${id}?`)) return;

  try {
    const res = await fetch(`/api/replication/targets/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('Failed deleting target');
    loadReplication();
    loadAudit();
  } catch (err) {
    alert('Delete error: ' + err.message);
  }
}

// The result is page content in a live region, so it is announced and stays readable.
async function syncAllToTarget(targetId) {
  const status = document.getElementById('replication-status');
  const say = msg => { status.textContent = msg; status.hidden = false; };
  try {
    const res = await fetch('/api/capsules');
    const capsules = await res.json() || [];
    if (capsules.length === 0) {
      say('No capsules available to sync.');
      return;
    }

    say(`Syncing ${capsules.length} capsules…`);
    let failed = 0;
    for (const c of capsules) {
      const r = await fetch('/api/replication/sync', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ capsule_id: c.id, target_id: targetId })
      });
      if (!r.ok) failed++;
    }

    say(failed
      ? `Replicated ${capsules.length - failed} of ${capsules.length} capsules; ${failed} failed. The replication log lists each result.`
      : `Replicated ${capsules.length} capsules to the offsite target.`);
    loadReplication();
    loadAudit();
  } catch (err) {
    say('Sync error: ' + err.message);
  }
}

// 10. Snapshot Diff Inspector
async function openDiffModal() {
  const baseSelect = document.getElementById('diff-base-select');
  const targetSelect = document.getElementById('diff-target-select');
  baseSelect.innerHTML = '<option value="">Loading...</option>';
  targetSelect.innerHTML = '<option value="">Loading...</option>';

  try {
    const res = await fetch('/api/capsules');
    const capsules = await res.json() || [];
    if (capsules.length < 2) {
      alert('You need at least 2 deposited capsules to run a version diff comparison.');
      return;
    }

    const optionsHTML = capsules.map(c => `
      <option value="${esc(c.id)}">${esc(c.id)} (${esc(c.service_name)}, ${(c.size_bytes / 1024).toFixed(1)} KB, ${new Date(c.created_at).toLocaleTimeString()})</option>
    `).join('');

    baseSelect.innerHTML = optionsHTML;
    targetSelect.innerHTML = optionsHTML;

    // Set defaults: base = older, target = newest
    if (capsules.length >= 2) {
      baseSelect.selectedIndex = 1;
      targetSelect.selectedIndex = 0;
    }

    document.getElementById('diff-results-container').style.display = 'none';
    document.getElementById('diff-modal').classList.add('open');
  } catch (err) {
    alert('Error loading capsules: ' + err.message);
  }
}

function closeDiffModal() {
  document.getElementById('diff-modal').classList.remove('open');
}

async function executeSnapshotDiff() {
  const baseID = document.getElementById('diff-base-select').value;
  const targetID = document.getElementById('diff-target-select').value;

  if (!baseID || !targetID) {
    alert('Please select both a base and target capsule to compare.');
    return;
  }
  if (baseID === targetID) {
    alert('Please select two distinct capsule versions to compare.');
    return;
  }

  try {
    const res = await fetch(`/api/capsules/diff?base=${encodeURIComponent(baseID)}&target=${encodeURIComponent(targetID)}`);
    if (!res.ok) {
      const errData = await res.json();
      throw new Error(errData.error || 'Failed generating version diff');
    }
    const report = await res.json();

    const driftBadgeEl = document.getElementById('diff-drift-badge');
    driftBadgeEl.textContent = report.identical_payload ? 'IDENTICAL' : 'DRIFT DETECTED';
    driftBadgeEl.style.color = report.identical_payload ? 'var(--accent-green)' : 'var(--accent-amber)';

    const fmt = n => (Number(n) > 0 ? '+' : '') + (Number(n) || 0);
    document.getElementById('diff-quorum-delta').textContent = `${fmt(report.threshold_delta)} of ${fmt(report.total_shares_delta)}`;
    document.getElementById('diff-created-at').textContent = new Date(report.target_created_at).toLocaleString();

    document.getElementById('diff-results-container').style.display = 'block';
  } catch (err) {
    alert('Diff inspection error: ' + err.message);
  }
}





async function loadRetention() {
  const status = document.getElementById('retention-status');
  try {
    const res = await fetch('/api/retention');
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed loading retention');
    for (const field of ['days', 'weeks', 'months']) document.getElementById(`retention-${field}`).value = data[field];
    status.textContent = `${data.days === 0 ? 'Keep forever' : `All for ${data.days} days; weekly for ${data.weeks} weeks; monthly for ${data.months} months`}. ${data.expired_count} expired backups (${data.expired_bytes.toLocaleString()} bytes) eligible for manual purge.`;
    return data;
  } catch (err) {
    status.textContent = err.message;
    return null;
  }
}

async function saveRetention() {
  const policy = {};
  for (const field of ['days', 'weeks', 'months']) {
    const input = document.getElementById(`retention-${field}`);
    if (!input.reportValidity()) return;
    policy[field] = Number(input.value);
  }
  try {
    const res = await fetch('/api/retention', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(policy)
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed saving retention');
    await loadRetention();
    loadAudit();
  } catch (err) {
    document.getElementById('retention-status').textContent = err.message;
  }
}

async function purgeExpiredBackups() {
  const preview = await loadRetention();
  if (!preview || !preview.expired_count) return;
  if (!confirm(`Permanently purge local backups outside the saved policy (all: ${preview.days} days, weekly: ${preview.weeks} weeks, monthly: ${preview.months} months)? Currently ${preview.expired_count} backups qualify. This may remove a product's last backup. Offsite copies are unaffected.`)) return;
  try {
    const res = await fetch('/api/retention/purge', {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ days: preview.days, weeks: preview.weeks, months: preview.months })
    });
    const data = await res.json();
    await loadRetention();
    loadCapsules();
    loadReadiness();
    loadAudit();
    if (!res.ok) throw new Error(`${data.error || 'Purge failed'} Purged: ${data.purged_count || 0}.`);
    document.getElementById('retention-status').textContent += ` Purged ${data.purged_count} backups.`;
  } catch (err) {
    document.getElementById('retention-status').textContent = err.message;
  }
}

// 11. Backups: capsule and restic status from metadata, and the guided restore plan.
let backupsData = null;
const backupsOpen = new Set();
const backupsWarn = ' <span class="backups-warn" title="Observed cadence differs from the expected interval by more than 2×">⚠</span>';

const fmtTime = t => t ? new Date(t).toLocaleString() : '—';
const fmtDuration = s => {
  s = Math.abs(Number(s) || 0);
  return s < 3600 ? Math.round(s / 60) + ' min' : s < 172800 ? Math.round(s / 3600) + ' h' : Math.round(s / 86400) + ' d';
};
const fmtAgo = t => fmtDuration((Date.now() - new Date(t)) / 1000) + ' ago';
const fmtBytes = n => {
  n = Number(n) || 0;
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0;
  for (; n >= 1024 && i < units.length - 1; i++) n /= 1024;
  return (i ? n.toFixed(1) : n) + ' ' + units[i];
};
const newestTime = times => times.filter(Boolean).sort((a, b) => new Date(a) - new Date(b)).pop();
const capsuleState = c => c.status === 'corrupt' ? 'corrupt' : c.verified_at ? 'verified ' + fmtTime(c.verified_at) : 'stored';
const backupPill = state => ['corrupt', 'scan_error', 'missing'].includes(state) ? 'danger' : ['ok', 'pending'].includes(state) ? 'ready' : 'warning';
const stateLabel = state => String(state).replace('_', ' ');

function stateHtml(state, flags) {
  return `<span class="status-pill ${esc(backupPill(state))}"><span class="dot"></span> ${esc(stateLabel(state))}</span>` +
    (flags || []).filter(f => f !== state).map(f => ` <span class="pill-badge">${esc(stateLabel(f))}</span>`).join('');
}

async function loadBackups() {
  const status = document.getElementById('backups-status');
  try {
    const res = await fetch('/api/backups/status');
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed loading backup status');
    backupsData = data;
    renderBackups(data);
    status.textContent = `Updated ${new Date().toLocaleTimeString()}.`;
  } catch (err) {
    status.textContent = err.message;
  }
}

function renderBackups({ report, scan, samples }) {
  const services = report.services || [];
  const unassigned = report.unassigned || [];
  const count = state => Number((report.counts || {})[state]) || 0;
  const repos = services.flatMap(s => s.repos || []).concat(unassigned);
  const samplesFor = path => (samples || []).filter(x => x.path === path).sort((a, b) => a.day < b.day ? -1 : 1);

  document.getElementById('backups-healthy').textContent = count('ok');
  document.getElementById('backups-stale').textContent = count('stale') + count('never');
  document.getElementById('backups-corrupt').textContent = count('corrupt');
  document.getElementById('backups-scan').textContent = count('scan_error') + count('missing');
  document.getElementById('backups-storage').textContent = scan.enabled ? fmtBytes(repos.reduce((sum, r) => sum + (Number(r.size_bytes) || 0), 0)) : '--';

  const banner = document.getElementById('backups-banner');
  banner.hidden = !scan.enabled || report.scan_fresh;
  banner.textContent = `Restic results as of ${scan.last_success_at ? fmtTime(scan.last_success_at) : 'no successful scan yet'}.` +
    (scan.last_error ? ` Last scan error: ${scan.last_error}` : '');
  document.querySelectorAll('.restic-off-note').forEach(el => { el.hidden = scan.enabled; });

  document.getElementById('backups-table-body').innerHTML = services.length === 0
    ? `<tr><td colspan="6">No services yet. Pair a product or link a restic repository.<button type="button" class="btn btn-secondary empty-action" onclick="document.querySelector('[data-tab=tab-pairing]').click()">Paired products</button></td></tr>`
    : services.map(s => serviceRows(s, samplesFor)).join('');

  document.getElementById('backups-unassigned').innerHTML = unassigned.length === 0
    ? '<p class="backups-dim">No unassigned repositories.</p>'
    : unassigned.map(r => repoHtml(r, samplesFor(r.path))).join('');

  document.getElementById('backups-service-names').innerHTML = services.map(s => `<option value="${esc(s.service_name)}">`).join('');
  const select = document.getElementById('restore-service');
  const previous = select.value;
  select.innerHTML = services.map(s => `<option value="${esc(s.service_name)}">${esc(s.service_name)}</option>`).join('');
  if (services.some(s => s.service_name === previous)) select.value = previous;
}

function serviceRows(s, samplesFor) {
  const newest = (s.instances || []).map(i => i.newest).filter(Boolean)
    .sort((a, b) => new Date(b.deposited_at) - new Date(a.deposited_at))[0];
  const written = newestTime((s.repos || []).map(r => r.last_snapshot_at));
  const cadences = [...(s.instances || []), ...(s.repos || [])].map(x => x.cadence).filter(Boolean);
  const cadence = cadences.find(c => c.mismatch) || cadences[0];
  const channels = [s.expects_capsule && 'capsule', s.expects_restic && 'restic'].filter(Boolean).join(' + ') || 'none';
  const open = backupsOpen.has(s.service_name);
  const row = `<tr>
      <td><button class="btn btn-secondary btn-sm" data-action="toggle" data-service="${esc(s.service_name)}" aria-expanded="${open ? 'true' : 'false'}" aria-label="Details for ${esc(s.service_name)}">${open ? '▾' : '▸'}</button> <strong>${esc(s.service_name)}</strong></td>
      <td>${esc(channels)}</td>
      <td>${newest ? `${esc(fmtAgo(newest.deposited_at))}<div class="backups-dim">${esc(capsuleState(newest))}</div>` : `<span class="backups-dim">${s.expects_capsule ? 'never' : '—'}</span>`}</td>
      <td>${written ? esc('written ' + fmtAgo(written)) : `<span class="backups-dim">${s.expects_restic ? 'never' : '—'}</span>`}</td>
      <td>${isAdminUser
        ? `<input class="form-input backups-interval" type="number" min="0.25" step="any" required value="${Number(s.expected_seconds) / 3600}" aria-label="Expected interval for ${esc(s.service_name)}, hours"> h <button class="btn btn-secondary btn-sm" data-action="set-interval" data-service="${esc(s.service_name)}">Save</button>`
        : esc(fmtDuration(s.expected_seconds))}${cadence ? `<div class="backups-dim">observed ~${esc(fmtDuration(cadence.median_seconds))}</div>` : ''}</td>
      <td>` + stateHtml(s.state, s.flags) + (cadence && cadence.mismatch ? backupsWarn : '') + '</td></tr>';
  if (!open) return row;
  return row + '<tr class="backups-detail"><td colspan="6">' + instancesHtml(s.instances) +
    ((s.repos || []).length ? '<h4 class="form-label">Repositories</h4>' : '') +
    (s.repos || []).map(r => repoHtml(r, samplesFor(r.path))).join('') + '</td></tr>';
}

function instancesHtml(instances) {
  if (!instances || instances.length === 0) return '';
  return '<h4 class="form-label">Instances</h4><table class="data-table backups-sub"><thead><tr><th>Instance</th><th>Pairing</th><th>Capsules</th><th>Newest capsule</th><th>Cadence</th><th>Status</th></tr></thead><tbody>' +
    instances.map(i => `<tr>
      <td><strong>${esc(i.app_name)}</strong><div><code>${esc(i.id)}</code></div></td>
      <td>${esc(i.status)}</td>
      <td>${Number(i.capsule_count) || 0}</td>
      <td>${i.newest ? `<code>${esc(i.newest.id)}</code><div class="backups-dim">deposited ${esc(fmtAgo(i.newest.deposited_at))}, ${esc(capsuleState(i.newest))}</div>` : '<span class="backups-dim">none</span>'}</td>
      <td>${i.cadence ? esc('~' + fmtDuration(i.cadence.median_seconds)) + (i.cadence.mismatch ? backupsWarn : '') : '—'}</td>
      <td>` + stateHtml(i.state) + '</td></tr>').join('') + '</tbody></table>';
}

function repoHtml(r, samples) {
  const flags = r.flags || [];
  const newKeys = (Number(r.key_count) || 0) - (Number(r.acked_key_count) || 0);
  return `<div class="backups-repo">
      <div><code>${esc(r.path)}</code> ` + stateHtml(r.state, flags) + `</div>
      <div class="backups-facts">
        <span>${Number(r.snapshot_count) || 0} snapshots</span>
        <span>${r.last_snapshot_at ? esc('last written ' + fmtAgo(r.last_snapshot_at)) : 'nothing written yet'}</span>
        <span>${esc(fmtBytes(r.size_bytes))}</span>
        <span>${Number(r.key_count) || 0} keys</span>
        <span>${r.expected_seconds ? esc('expected every ' + fmtDuration(r.expected_seconds)) : 'no expected interval'}${r.cadence ? esc(', observed ~' + fmtDuration(r.cadence.median_seconds)) + (r.cadence.mismatch ? backupsWarn : '') : ''}</span>
        <span>${r.measured_at ? esc('measured ' + fmtTime(r.measured_at)) : 'not measured yet'}</span>
      </div>` +
    sparklines(samples) +
    (r.activity ? '<p class="backups-dim">Repository activity: a recent lock is present.</p>' : '') +
    (flags.includes('old_lock') ? `<p class="backups-alert">Old lock seen since ${esc(fmtTime(r.lock_seen_at))}. If no backup is running, clear it with <code>restic unlock</code>.</p>` : '') +
    (newKeys > 0 ? `<p class="backups-alert">${Number(newKeys)} key(s) added since the last acknowledgement. Check that you added them.` +
      (isAdminUser ? ` <button class="btn btn-secondary btn-sm" data-action="ack-key" data-repo="${esc(r.path)}" data-count="${Number(newKeys)}">Acknowledge</button>` : '') + '</p>' : '') +
    (r.missing_since ? `<p class="backups-alert">Missing from the restic directory since ${esc(fmtTime(r.missing_since))}.</p>` : '') +
    (r.scan_error ? `<p class="backups-alert">Scan error: ${esc(r.scan_error)}</p>` : '') +
    (isAdminUser ? `<div class="backups-link">
        <input class="form-input" list="backups-service-names" value="${esc(r.service_name)}" placeholder="service name" aria-label="Service for ${esc(r.path)}">
        <button class="btn btn-secondary btn-sm" data-action="link" data-repo="${esc(r.path)}">${r.service_name ? 'Save link' : 'Link'}</button>
        ${r.service_name ? '<span class="backups-dim">Clear the name to unlink.</span>' : ''}
      </div>` : '') +
    '</div>';
}

// One polyline per series, scaled to its own range: shape only, the caption carries the number.
function sparkline(values, label) {
  const min = Math.min(...values), range = Math.max(...values) - min || 1;
  const points = values.map((v, i) => (i * 100 / (values.length - 1)).toFixed(1) + ',' + (22 - (v - min) / range * 20).toFixed(1)).join(' ');
  return `<figure class="backups-spark"><svg viewBox="0 0 100 24" preserveAspectRatio="none" role="img" aria-label="${esc(label)}"><polyline points="${esc(points)}" vector-effect="non-scaling-stroke"/></svg><figcaption>${esc(label)}</figcaption></figure>`;
}

function sparklines(samples) {
  if (samples.length < 2) return '';
  const last = samples[samples.length - 1];
  return '<div class="backups-sparks">' +
    sparkline(samples.map(x => Number(x.size_bytes) || 0), `Size, 90 days: now ${fmtBytes(last.size_bytes)}`) +
    sparkline(samples.map(x => Number(x.snapshot_count) || 0), `Snapshots, 90 days: now ${Number(last.snapshot_count) || 0}`) +
    '</div>';
}

async function backupsPost(path, body) {
  const res = await fetch(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Request failed (HTTP ${res.status})`);
}

async function onBackupsClick(e) {
  const btn = e.target.closest('button[data-action]');
  if (!btn) return;
  const { action, service, repo } = btn.dataset;
  if (action === 'toggle') {
    if (!backupsOpen.delete(service)) backupsOpen.add(service);
    renderBackups(backupsData);
    return;
  }
  const input = btn.parentElement.querySelector('input');
  try {
    if (action === 'set-interval') {
      if (!input.reportValidity()) return;
      await backupsPost('/api/backups/expectations', { service_name: service, expected_interval_seconds: Math.round(Number(input.value) * 3600) });
    } else if (action === 'link') {
      await backupsPost('/api/backups/repos/link', { repo_path: repo, service_name: input.value.trim() });
    } else if (action === 'ack-key') {
      if (!confirm(`Acknowledge ${btn.dataset.count} new key(s) on ${repo}?\n\nA key you did not add can read and write this repository. Acknowledge only keys you added.`)) return;
      await backupsPost('/api/backups/repos/ack-key', { repo_path: repo });
    }
    await loadBackups();
    loadAudit();
  } catch (err) {
    alert(err.message);
  }
}

function resetRestorePickers() {
  for (const field of ['instance', 'repo']) {
    document.getElementById(`restore-${field}-field`).hidden = true;
    document.getElementById(`restore-${field}`).innerHTML = '';
  }
  document.getElementById('restore-plan').innerHTML = '';
  document.getElementById('restore-status').textContent = '';
}

async function planRestore() {
  const status = document.getElementById('restore-status');
  const out = document.getElementById('restore-plan');
  const q = new URLSearchParams({ service: document.getElementById('restore-service').value });
  for (const field of ['instance', 'repo']) {
    const v = document.getElementById(`restore-${field}`).value;
    if (v) q.set(field, v);
  }
  const at = document.getElementById('restore-at').value;
  if (at) q.set('at', new Date(at).toISOString());
  status.textContent = 'Finding restore point…';
  out.innerHTML = '';
  try {
    const res = await fetch(`/api/backups/restore-plan?${q}`);
    const data = await res.json();
    if (res.status === 400 && ['instance', 'repo'].includes(data.field) && Array.isArray(data.choices)) {
      const select = document.getElementById(`restore-${data.field}`);
      select.innerHTML = '<option value="">Choose…</option>' +
        data.choices.map(c => `<option value="${esc(c)}">${esc(choiceLabel(data.field, c))}</option>`).join('');
      document.getElementById(`restore-${data.field}-field`).hidden = false;
      select.focus();
      status.textContent = data.error;
      return;
    }
    if (!res.ok) throw new Error(data.error || 'Failed planning restore');
    status.textContent = '';
    out.innerHTML = restorePlanHtml(data);
  } catch (err) {
    status.textContent = err.message;
  }
}

function choiceLabel(field, id) {
  const inst = field === 'instance' && (backupsData ? backupsData.report.services || [] : [])
    .flatMap(s => s.instances || []).find(i => i.id === id);
  return inst ? `${inst.app_name} (${id})` : id;
}

function restorePlanHtml(p) {
  const canDownload = !!currentUser && (currentUser.role === 'admin' || currentUser.role === 'operator');
  const c = p.capsule, snap = p.snapshot, gap = Number(p.gap_seconds);
  return (p.skipped || []).map(k => `<p class="backups-alert danger"><strong>Skipped corrupt capsule</strong> <code>${esc(k.id)}</code>, deposited ${esc(fmtTime(k.deposited_at))}. Its stored bytes failed verification; the next older capsule is shown instead.</p>`).join('') +
    `<dl class="backups-plan">
      <dt>Service</dt><dd>${esc(p.service_name)}${p.instance ? esc(' / ' + choiceLabel('instance', p.instance)) : ''}, as of ${esc(fmtTime(p.at))}</dd>
      ${c ? `<dt>Capsule</dt><dd><code>${esc(c.id)}</code>${canDownload ? ` <a class="btn btn-secondary btn-sm" href="/api/capsules/${encodeURIComponent(c.id)}/download" download>Download</a>` : ''}</dd>
      <dt>Digest</dt><dd><code>${esc(c.digest)}</code></dd>
      <dt>Deposited</dt><dd>${esc(fmtTime(c.deposited_at))}</dd>
      <dt>Created</dt><dd>${esc(fmtTime(c.created_at))} (product-reported)</dd>
      <dt>Integrity</dt><dd>${esc(capsuleState(c))}</dd>
      <dt>Custodian cards</dt><dd>${p.recovery_key ? `${esc(p.recovery_key.threshold)} of the ${esc(p.recovery_key.total_shares)} cards printed with recovery key <code>${esc(p.recovery_key.key_id)}</code> (${esc(p.recovery_key.status)})` : `Cards printed with recovery key <code>${esc(c.recovery_key_id)}</code>`}</dd>` : '<dt>Capsule</dt><dd>No capsule deposited at or before this time.</dd>'}
      ${p.repo ? `<dt>Repository</dt><dd><code>${esc(p.repo)}</code></dd>` : ''}
      ${snap ? `<dt>Snapshot</dt><dd><code>${esc(snap.id)}</code></dd>
      <dt>Written at</dt><dd>${esc(fmtTime(snap.written_at))}${p.gap_seconds !== undefined ? esc(`, ${fmtDuration(gap)} ${gap < 0 ? 'before' : 'after'} the capsule was deposited`) : ''}</dd>` : (p.repo ? `<dt>Snapshot</dt><dd>${p.repo_missing ? 'Repository not found by the last scan.' : 'No snapshot written in this repository.'}</dd>` : '')}
    </dl>` +
    (p.warnings || []).map(w => `<p class="backups-alert">${esc(w)}</p>`).join('') +
    ((p.steps || []).length ? '<ol class="backups-steps">' + p.steps.map(st => `<li>${esc(st)}</li>`).join('') + '</ol>' : '');
}

