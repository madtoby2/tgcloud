const BASE = ''

async function request(path, options = {}) {
  const url = BASE + path
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json', ...options.headers },
    ...options,
  })
  const json = await res.json()
  if (!json.ok) throw new Error(json.error || 'request failed')
  return json.data
}

export const api = {
  // Dashboard
  dashboard: () => request('/api/dashboard'),
  status: () => request('/api/status'),

  // Accounts
  listAccounts: (params = {}) => {
    const q = new URLSearchParams(params).toString()
    return request('/api/accounts' + (q ? '?' + q : ''))
  },
  getAccount: (id) => request('/api/accounts/' + id),
  addAccount: (data) => request('/api/accounts', { method: 'POST', body: JSON.stringify(data) }),
  updateAccount: (id, data) => request('/api/accounts/' + id, { method: 'PUT', body: JSON.stringify(data) }),
  deleteAccount: (id) => request('/api/accounts/' + id, { method: 'DELETE' }),
  loginAccount: (id) => request('/api/accounts/' + id + '/login', { method: 'POST' }),
  submitCode: (id, code) => request('/api/accounts/' + id + '/code', { method: 'POST', body: JSON.stringify({ code }) }),
  submitPassword: (id, pass) => request('/api/accounts/' + id + '/password', { method: 'POST', body: JSON.stringify({ password: pass }) }),
  logoutAccount: (id) => request('/api/accounts/' + id + '/logout', { method: 'POST' }),
  checkStatus: (id) => request('/api/accounts/' + id + '/status-check', { method: 'POST' }),
  batchStatusCheck: (ids) => request('/api/accounts/batch/status-check', { method: 'POST', body: JSON.stringify({ ids }) }),
  batchDeleteWaste: () => request('/api/accounts/batch/delete-waste', { method: 'POST' }),

  // Categories
  listCategories: () => request('/api/categories'),
  createCategory: (data) => request('/api/categories', { method: 'POST', body: JSON.stringify(data) }),
  updateCategory: (id, data) => request('/api/categories/' + id, { method: 'PUT', body: JSON.stringify(data) }),
  deleteCategory: (id) => request('/api/categories/' + id, { method: 'DELETE' }),

  // Channels / Groups
  listChannels: () => request('/api/channels'),
  syncChannels: (id) => request('/api/channels/sync', { method: 'POST', body: JSON.stringify({ account_id: id }) }),
  listGroups: () => request('/api/groups'),
  syncGroups: (id) => request('/api/groups/sync', { method: 'POST', body: JSON.stringify({ account_id: id }) }),

  // Operations
  listOperations: (params) => {
    const q = new URLSearchParams(params).toString()
    return request('/api/operations' + (q ? '?' + q : ''))
  },
  createOperation: (data) => request('/api/operations', { method: 'POST', body: JSON.stringify(data) }),
  cancelOperation: (id) => request('/api/operations/' + id + '/cancel', { method: 'POST' }),

  // Batch Tasks
  listBatchTasks: () => request('/api/batch-tasks'),
  createBatchTask: (data) => request('/api/batch-tasks', { method: 'POST', body: JSON.stringify(data) }),
  startBatchTask: (id) => request('/api/batch-tasks/' + id + '/start', { method: 'POST' }),
  pauseBatchTask: (id) => request('/api/batch-tasks/' + id + '/pause', { method: 'POST' }),
  cancelBatchTask: (id) => request('/api/batch-tasks/' + id + '/cancel', { method: 'POST' }),

  // Scheduled Tasks
  listScheduledTasks: () => request('/api/scheduled-tasks'),
  createScheduledTask: (data) => request('/api/scheduled-tasks', { method: 'POST', body: JSON.stringify(data) }),
  updateScheduledTask: (id, data) => request('/api/scheduled-tasks/' + id, { method: 'PUT', body: JSON.stringify(data) }),
  deleteScheduledTask: (id) => request('/api/scheduled-tasks/' + id, { method: 'DELETE' }),
  toggleScheduledTask: (id) => request('/api/scheduled-tasks/' + id + '/toggle', { method: 'POST' }),

  // Import/Export
  importTelethon: (data) => request('/api/import/telethon', { method: 'POST', body: JSON.stringify(data) }),
  importTData: (data) => request('/api/import/tdata', { method: 'POST', body: JSON.stringify(data) }),
  exportTelethon: (id) => request('/api/export/' + id + '/telethon'),
}

// WebSocket
export function connectWS(onMessage) {
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const ws = new WebSocket(proto + '//' + location.host + '/ws')
  ws.onmessage = (e) => {
    try {
      const msg = JSON.parse(e.data)
      onMessage(msg.event, msg.data)
    } catch {}
  }
  ws.onclose = () => {
    setTimeout(() => connectWS(onMessage), 3000)
  }
  return ws
}
