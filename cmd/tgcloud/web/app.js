// tgcloud web app
const API = '';
let ws = null;
let accounts = [];
let selectedId = null;

// --- API helpers ---
async function api(path, opts = {}) {
  const res = await fetch(API + path, {
    headers: { 'Content-Type': 'application/json', ...opts.headers },
    ...opts,
  });
  return res.json();
}

// --- Init ---
document.addEventListener('DOMContentLoaded', () => {
  connectWS();
  loadAccounts();
  loadStatus();
  bindEvents();
});

function bindEvents() {
  document.getElementById('btn-add').onclick = () => openModal('modal-add');
  document.getElementById('btn-add-confirm').onclick = addAccount;
  document.getElementById('btn-code-confirm').onclick = submitCode;
  document.getElementById('btn-pass-confirm').onclick = submitPassword;
  document.getElementById('btn-close-detail').onclick = closeDetail;
}

// --- WebSocket ---
function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(`${proto}//${location.host}/ws`);

  ws.onopen = () => {
    document.getElementById('ws-status').className = 'ws-online';
  };

  ws.onclose = () => {
    document.getElementById('ws-status').className = 'ws-offline';
    setTimeout(connectWS, 3000);
  };

  ws.onmessage = (e) => {
    const msg = JSON.parse(e.data);
    switch (msg.event) {
      case 'account_created':
      case 'login_done':
      case 'account_deleted':
        loadAccounts();
        loadStatus();
        break;
      case 'login_error':
        alert('Login error: ' + msg.data.error);
        loadAccounts();
        break;
    }
  };
}

// --- Account list ---
async function loadAccounts() {
  const r = await api('/api/accounts');
  if (!r.ok) return;
  accounts = r.data;
  renderAccounts();
}

function renderAccounts() {
  const el = document.getElementById('account-list');
  if (accounts.length === 0) {
    el.innerHTML = '<div class="empty"><p>No accounts yet</p></div>';
    return;
  }
  el.innerHTML = accounts.map(a => {
    const cls = a.status === 'online' ? 'status-online' :
      a.status === 'connecting' ? 'status-connecting' :
      a.status === 'error' ? 'status-error' : 'status-offline';
    return `
      <div class="account-card" onclick="selectAccount(${a.id})" data-id="${a.id}"
           style="${selectedId === a.id ? 'border-color:var(--accent)' : ''}">
        <div class="phone">
          <span class="status-dot ${cls}"></span>
          ${a.phone}
        </div>
        <div class="meta">
          <span>${esc(a.first_name || a.username || '-')}</span>
          <span>${a.status}</span>
          ${a.flood_wait > 0 ? `<span style="color:var(--red)">FW:${a.flood_wait}s</span>` : ''}
        </div>
      </div>`;
  }).join('');
}

// --- Detail ---
async function selectAccount(id) {
  selectedId = id;
  renderAccounts();

  const a = accounts.find(x => x.id === id);
  if (!a) return;

  document.getElementById('detail-panel').style.display = 'block';
  document.getElementById('detail-title').textContent = a.phone;

  const ops = await api(`/api/operations?account_id=${id}`);
  const opList = (ops.ok && ops.data || []).map(o => `
    <div class="operation-item">
      <span class="type">${o.type}</span>
      <span style="color:var(--yellow)">${o.status}</span>
      <div class="meta">${new Date(o.created_at).toLocaleString()}</div>
    </div>
  `).join('') || '<div class="empty"><p>No operations</p></div>';

  document.getElementById('detail-content').innerHTML = `
    <div class="detail-section">
      <h3>Info</h3>
      <div class="detail-grid">
        <div class="detail-item"><div class="label">Status</div>${a.status}</div>
        <div class="detail-item"><div class="label">Name</div>${esc(a.first_name) || '-'}</div>
        <div class="detail-item"><div class="label">Username</div>${esc(a.username) || '-'}</div>
        <div class="detail-item"><div class="label">Proxy</div>${esc(a.proxy) || 'none'}</div>
      </div>
    </div>
    <div class="detail-section">
      <h3>Actions</h3>
      <button class="btn btn-primary" onclick="startLogin(${a.id})">Login</button>
      <button class="btn btn-danger" onclick="deleteAccount(${a.id})">Delete</button>
    </div>
    <div class="detail-section">
      <h3>Operations</h3>
      ${opList}
    </div>
  `;
}

function closeDetail() {
  selectedId = null;
  document.getElementById('detail-panel').style.display = 'none';
  renderAccounts();
}

// --- Login ---
async function startLogin(id) {
  const r = await api(`/api/accounts/${id}/login`, { method: 'POST' });
  if (!r.ok) { alert(r.error); return; }
  const a = accounts.find(x => x.id === id);
  document.getElementById('login-phone').textContent = a.phone;
  openModal('modal-login');
}

async function submitCode() {
  const code = document.getElementById('login-code').value;
  if (!code) return;
  const r = await api(`/api/accounts/${selectedId}/code`, {
    method: 'POST',
    body: JSON.stringify({ code }),
  });
  if (!r.ok) {
    // Might need 2FA
    if (r.error && r.error.includes('password')) {
      closeModal('modal-login');
      const a = accounts.find(x => x.id === selectedId);
      document.getElementById('pass-phone').textContent = a.phone;
      openModal('modal-password');
      return;
    }
    alert(r.error);
    return;
  }
  closeModal('modal-login');
  document.getElementById('login-code').value = '';
  loadAccounts();
}

async function submitPassword() {
  const password = document.getElementById('pass-input').value;
  if (!password) return;
  const r = await api(`/api/accounts/${selectedId}/password`, {
    method: 'POST',
    body: JSON.stringify({ password }),
  });
  if (!r.ok) { alert(r.error); return; }
  closeModal('modal-password');
  document.getElementById('pass-input').value = '';
  loadAccounts();
}

// --- Add / Delete ---
async function addAccount() {
  const phone = document.getElementById('add-phone').value;
  const proxy = document.getElementById('add-proxy').value;
  if (!phone) return;
  const r = await api('/api/accounts', {
    method: 'POST',
    body: JSON.stringify({ phone, proxy }),
  });
  if (!r.ok) { alert(r.error); return; }
  closeModal('modal-add');
  document.getElementById('add-phone').value = '';
  document.getElementById('add-proxy').value = '';
  loadAccounts();
}

async function deleteAccount(id) {
  if (!confirm('Delete this account?')) return;
  const r = await api(`/api/accounts/${id}`, { method: 'DELETE' });
  if (!r.ok) { alert(r.error); return; }
  if (selectedId === id) closeDetail();
  loadAccounts();
}

// --- Status ---
async function loadStatus() {
  const r = await api('/api/status');
  if (r.ok) {
    document.getElementById('sys-stats').textContent =
      `Online: ${r.data.online_accounts}/${r.data.total_accounts}`;
  }
}

// --- Modal helpers ---
function openModal(id) { document.getElementById(id).style.display = 'flex'; }
function closeModal(id) { document.getElementById(id).style.display = 'none'; }
function esc(s) { return (s || '').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }
