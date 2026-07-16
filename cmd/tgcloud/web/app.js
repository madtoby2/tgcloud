// tgcloud web app
const API = '';
let ws = null, accounts = [], selectedId = null, opPollTimer = null;

// --- API ---
async function api(path, opts = {}) {
  const res = await fetch(API + path, {
    headers: { 'Content-Type': 'application/json', ...opts.headers }, ...opts,
  });
  return res.json();
}

// --- Init ---
document.addEventListener('DOMContentLoaded', () => {
  connectWS(); loadAccounts(); loadStatus(); bindEvents(); pollOps();
});

function bindEvents() {
  document.getElementById('btn-add').onclick = () => openModal('modal-add');
  document.getElementById('btn-add-confirm').onclick = addAccount;
  document.getElementById('btn-code-confirm').onclick = submitCode;
  document.getElementById('btn-pass-confirm').onclick = submitPassword;
  document.getElementById('btn-close-detail').onclick = closeDetail;
  document.getElementById('btn-tool-submit').onclick = submitTool;

  // Tabs
  document.querySelectorAll('.tab').forEach(t => {
    t.onclick = () => switchTab(t.dataset.tab);
  });

  // Tool cards
  document.querySelectorAll('.tool-card').forEach(c => {
    c.onclick = () => openTool(c.dataset.tool);
  });

  // Overlay close
  document.getElementById('detail-overlay').onclick = closeDetail;
}

// --- Tabs ---
function switchTab(name) {
  document.querySelectorAll('.tab').forEach(t => t.classList.toggle('active', t.dataset.tab === name));
  document.querySelectorAll('.tab-content').forEach(c => c.style.display = 'none');
  document.getElementById('tab-' + name).style.display = 'block';
  if (name === 'ops') loadOps();
}

// --- WebSocket ---
function connectWS() {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(`${proto}//${location.host}/ws`);
  ws.onopen = () => document.getElementById('ws-status').className = 'ws-online';
  ws.onclose = () => { document.getElementById('ws-status').className = 'ws-offline'; setTimeout(connectWS, 3000); };
  ws.onmessage = (e) => {
    const m = JSON.parse(e.data);
    if (['account_created','login_done','account_deleted','operation_created'].includes(m.event)) {
      loadAccounts(); loadStatus();
    }
    if (m.event === 'login_error') alert('Login error: ' + m.data.error);
  };
}

// --- Accounts ---
async function loadAccounts() {
  const r = await api('/api/accounts'); if (!r.ok) return;
  accounts = r.data; renderAccounts(); loadOpAccounts();
}

function renderAccounts() {
  const el = document.getElementById('account-list');
  if (!accounts.length) { el.innerHTML = '<div class="empty"><p>No accounts yet</p></div>'; return; }
  el.innerHTML = accounts.map(a => {
    const cls = {online:'status-online',connecting:'status-connecting',error:'status-error'}[a.status] || 'status-offline';
    const sel = selectedId === a.id ? ' selected' : '';
    return `<div class="account-card${sel}" onclick="selectAccount(${a.id})" data-id="${a.id}">
      <div class="phone"><span class="status-dot ${cls}"></span>${a.phone}</div>
      <div class="meta">
        <span>${esc(a.first_name || a.username || '-')}</span><span>${a.status}</span>
        ${a.flood_wait > 0 ? `<span class="fw-badge">FW ${a.flood_wait}s</span>` : ''}
      </div></div>`;
  }).join('');
}

async function selectAccount(id) {
  selectedId = id; renderAccounts();
  const a = accounts.find(x => x.id === id); if (!a) return;
  document.getElementById('detail-overlay').style.display = 'block';
  document.getElementById('detail-panel').classList.add('open');
  document.getElementById('detail-title').textContent = a.phone;
  const ops = await api(`/api/operations?account_id=${id}`);
  const opHtml = (ops.ok && ops.data || []).slice(0, 10).map(renderOpItem).join('') || '<div class="empty">No operations</div>';
  document.getElementById('detail-content').innerHTML = `
    <div class="detail-section"><h3>Info</h3>
      <div class="detail-grid">
        <div class="detail-item"><div class="label">Status</div>${a.status}</div>
        <div class="detail-item"><div class="label">Name</div>${esc(a.first_name) || '-'}</div>
        <div class="detail-item"><div class="label">Username</div>${esc(a.username) || '-'}</div>
        <div class="detail-item"><div class="label">Proxy</div>${esc(a.proxy) || 'none'}</div>
        <div class="detail-item"><div class="label">FloodWait</div>${a.flood_wait}s</div>
      </div></div>
    <div class="detail-section"><h3>Actions</h3>
      <button class="btn btn-primary" onclick="startLogin(${a.id})">Login</button>
      <button class="btn btn-danger" onclick="deleteAccount(${a.id})">Delete</button></div>
    <div class="detail-section"><h3>Recent Ops</h3>${opHtml}</div>`;
}

function closeDetail() {
  selectedId = null; renderAccounts();
  document.getElementById('detail-overlay').style.display = 'none';
  document.getElementById('detail-panel').classList.remove('open');
}

// --- Login ---
async function startLogin(id) {
  const r = await api(`/api/accounts/${id}/login`, { method: 'POST' }); if (!r.ok) { alert(r.error); return; }
  const a = accounts.find(x => x.id === id);
  document.getElementById('login-phone').textContent = a.phone; openModal('modal-login');
}
async function submitCode() {
  const code = document.getElementById('login-code').value; if (!code) return;
  const r = await api(`/api/accounts/${selectedId}/code`, { method: 'POST', body: JSON.stringify({ code }) });
  if (!r.ok) { alert(r.error); return; }
  closeModal('modal-login'); document.getElementById('login-code').value = ''; loadAccounts();
}
async function submitPassword() {
  const pw = document.getElementById('pass-input').value; if (!pw) return;
  const r = await api(`/api/accounts/${selectedId}/password`, { method: 'POST', body: JSON.stringify({ password: pw }) });
  if (!r.ok) { alert(r.error); return; }
  closeModal('modal-password'); document.getElementById('pass-input').value = ''; loadAccounts();
}
async function addAccount() {
  const phone = document.getElementById('add-phone').value;
  const proxy = document.getElementById('add-proxy').value; if (!phone) return;
  const r = await api('/api/accounts', { method: 'POST', body: JSON.stringify({ phone, proxy }) });
  if (!r.ok) { alert(r.error); return; }
  closeModal('modal-add'); document.getElementById('add-phone').value = ''; loadAccounts();
}
async function deleteAccount(id) {
  if (!confirm('Delete?')) return;
  await api(`/api/accounts/${id}`, { method: 'DELETE' });
  if (selectedId === id) closeDetail(); loadAccounts(); loadStatus();
}

// --- Operations ---
function loadOpAccounts() {
  const sel = document.getElementById('op-account');
  sel.innerHTML = '<option value="">All</option>' + accounts.map(a => `<option value="${a.id}">${a.phone} (${a.status})</option>`).join('');
}
async function loadOps() {
  const aid = document.getElementById('op-account').value;
  const r = await api(`/api/operations?account_id=${aid}`);
  document.getElementById('op-list').innerHTML = r.ok && r.data.length
    ? r.data.map(renderOpItem).join('') : '<div class="empty"><p>No operations</p></div>';
}
function renderOpItem(o) {
  const cls = {done:'op-done',failed:'op-failed',running:'op-running'}[o.status] || 'op-pending';
  let result = '';
  try { if (o.result && o.result !== '{}') result = JSON.stringify(JSON.parse(o.result), null, 2); } catch(e) {}
  return `<div class="op-item">
    <div><span class="op-type">${o.type}</span>
      <span class="op-meta">#${o.id} ${new Date(o.created_at).toLocaleString()}</span></div>
    <div style="display:flex;align-items:center;gap:8px">
      <span class="op-status ${cls}">${o.status}</span>
      ${o.status==='running'?`<button class="btn btn-sm btn-danger" onclick="cancelOp(${o.id})">✕</button>`:''}
      ${result?`<button class="btn btn-sm" onclick="this.nextElementSibling.style.display=this.nextElementSibling.style.display==='block'?'none':'block'">📋</button><pre class="result-json" style="display:none">${esc(result)}</pre>`:''}
    </div></div>`;
}
async function cancelOp(id) { await api(`/api/operations/${id}/cancel`, { method: 'POST' }); loadOps(); }
function pollOps() { opPollTimer = setInterval(loadOps, 5000); }

// --- Tools ---
let currentTool = null;
function openTool(name) {
  currentTool = name;
  const titles = {
    send_message:'Send Message', join_group:'Join Groups', invite_users:'Invite Users',
    farming:'Farming', scrape_members:'Scrape Members', phone_filter:'Phone Filter',
    search_groups:'Search Groups', clone_channel:'Clone Channel', boost:'Boost'
  };
  document.getElementById('tool-title').textContent = titles[name] || name;

  let body = '';
  switch (name) {
    case 'send_message':
      body = formLabel('Targets','targets_text','@username1, @username2') +
             formArea('Message','msg','Type your message...') +
             formInput('Interval (sec)','interval','5','number');
      break;
    case 'join_group':
      body = formArea('Links','links','t.me/username\nhttps://t.me/+abc123hash\n@other_group');
      break;
    case 'invite_users':
      body = formInput('Channel','channel','@target_channel') +
             formArea('Users','users','@user1\n@user2\n@user3') +
             formInput('Max per account','max_users','50','number');
      break;
    case 'farming':
      body = formArea('Groups','groups','@group1\n@group2') +
             formArea('Messages (one per line)','messages','Hello everyone!\nGood morning\nNice to meet you all') +
             formInput('Loops','loops','3','number') +
             formInput('Min Delay (sec)','min_delay','30','number') +
             formInput('Max Delay (sec)','max_delay','90','number');
      break;
    case 'scrape_members':
      body = formInput('Group','group','@target_group') +
             formInput('Limit','limit','200','number');
      break;
    case 'phone_filter':
      body = formArea('Phones (one per line)','phones','+8613800138000\n+8613900139000');
      break;
    case 'search_groups':
      body = formInput('Query','query','keyword') +
             formInput('Limit','limit','50','number');
      break;
    case 'clone_channel':
      body = formInput('Source','source','@source_channel') +
             formInput('Target','target','@target_channel') +
             formInput('Limit','limit','100','number') +
             `<div class="form-group"><label>Filter</label>
              <select id="clone_filter"><option value="">All</option>
              <option value="media_only">Media only</option>
              <option value="text_only">Text only</option></select></div>`;
      break;
    case 'boost':
      body = formInput('Channel','channel','@target_channel') +
             `<div class="form-group"><label><input type="checkbox" id="boost_subscribe" checked style="width:auto;margin-right:8px"> Subscribe (join channel)</label></div>` +
             formInput('View Posts','view_posts','20','number') +
             formInput('Interval (sec)','interval','5','number');
      break;
  }
  body += formSelect();
  document.getElementById('tool-body').innerHTML = body;
  openModal('modal-tool');
}

async function submitTool() {
  const accountId = parseInt(document.getElementById('tool-account-select').value) || 0;
  if (!accountId) { alert('Select an account'); return; }

  const getVal = id => document.getElementById(id)?.value || '';
  const getList = id => getVal(id).split('\n').map(s => s.trim()).filter(Boolean);

  let params = {};
  switch (currentTool) {
    case 'send_message':
      params = { targets: getList('targets_text'), message: getVal('msg'), interval: parseInt(getVal('interval'))||5 }; break;
    case 'join_group': params = { links: getList('links') }; break;
    case 'invite_users':
      params = { channel: getVal('channel'), users: getList('users'), max_users: parseInt(getVal('max_users'))||50 }; break;
    case 'farming':
      params = { groups: getList('groups'), messages: getList('messages'), loops: parseInt(getVal('loops'))||3, min_delay: parseInt(getVal('min_delay'))||30, max_delay: parseInt(getVal('max_delay'))||90 }; break;
    case 'scrape_members': params = { group: getVal('group'), limit: parseInt(getVal('limit'))||200 }; break;
    case 'phone_filter': params = { phones: getList('phones') }; break;
    case 'search_groups': params = { query: getVal('query'), limit: parseInt(getVal('limit'))||50 }; break;
    case 'clone_channel':
      const filter = document.getElementById('clone_filter')?.value || '';
      params = { source: getVal('source'), target: getVal('target'), limit: parseInt(getVal('limit'))||100 };
      if (filter === 'media_only') params.media_only = true;
      if (filter === 'text_only') params.text_only = true;
      break;
    case 'boost':
      params = {
        channel: getVal('channel'),
        subscribe: document.getElementById('boost_subscribe')?.checked || false,
        view_posts: parseInt(getVal('view_posts'))||20,
        interval: parseInt(getVal('interval'))||5
      };
      break;
    default: return;
  }

  const r = await api('/api/operations', {
    method: 'POST',
    body: JSON.stringify({ account_id: accountId, type: currentTool, params }),
  });
  if (!r.ok) { alert(r.error); return; }
  closeModal('modal-tool');
  switchTab('ops');
  loadOps();
}

// --- Helpers ---
function formLabel(label, id, placeholder) {
  return `<div class="form-group"><label>${label}</label><input id="${id}" placeholder="${placeholder||''}"></div>`;
}
function formInput(label, id, placeholder, type) {
  return `<div class="form-group"><label>${label}</label><input id="${id}" type="${type||'text'}" placeholder="${placeholder||''}"></div>`;
}
function formArea(label, id, placeholder) {
  return `<div class="form-group"><label>${label}</label><textarea id="${id}" placeholder="${placeholder||''}"></textarea></div>`;
}
function formSelect() {
  const opts = accounts.filter(a => a.status === 'online').map(a => `<option value="${a.id}">${a.phone} (${a.first_name||a.username||''})</option>`).join('');
  return `<div class="form-group"><label>Account (online only)</label><select id="tool-account-select"><option value="">-- Select --</option>${opts}</select></div>`;
}

async function loadStatus() {
  const r = await api('/api/status');
  if (r.ok) document.getElementById('sys-stats').textContent = `Online: ${r.data.online_accounts}/${r.data.total_accounts}`;
}

// --- Modal ---
function openModal(id) { document.getElementById(id).style.display = 'flex'; }
function closeModal(id) { document.getElementById(id).style.display = 'none'; }
function esc(s) { return (s||'').replace(/</g,'&lt;').replace(/>/g,'&gt;'); }

// --- Keyboard ---
document.addEventListener('keydown', e => { if (e.key === 'Escape') closeDetail(); });
