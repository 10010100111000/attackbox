async function request(url, options = {}) {
  const resp = await fetch(url, {
    headers: {
      'Content-Type': 'application/json',
      ...options.headers,
    },
    ...options,
  })
  const data = await resp.json().catch(() => ({}))
  if (!resp.ok) {
    throw new Error(data.error || `请求失败 (${resp.status})`)
  }
  return data
}

export const adminApi = {
  getMetrics: () => request('/api/admin/metrics'),

  getUsers: () => request('/api/admin/users'),

  createUser: (username, password, role) =>
    request('/api/admin/users/create', {
      method: 'POST',
      body: JSON.stringify({ username, password, role }),
    }),

  toggleUser: (username) =>
    request('/api/admin/users/toggle', {
      method: 'POST',
      body: JSON.stringify({ username }),
    }),

  resetPassword: (username, password) =>
    request('/api/admin/users/reset-pass', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  deleteUser: (username) =>
    request('/api/admin/users/delete', {
      method: 'POST',
      body: JSON.stringify({ username }),
    }),

  saveBroadcast: (enabled, message) =>
    request('/api/admin/broadcast', {
      method: 'POST',
      body: JSON.stringify({ enabled, message }),
    }),

  getSandboxes: () => request('/api/admin/sandboxes'),

  releaseSandbox: (user, boxId) =>
    request('/api/admin/sandboxes/release', {
      method: 'POST',
      body: JSON.stringify({ user, box_id: boxId }),
    }),

  getAuditLogs: (params = {}) => {
    const qs = new URLSearchParams()
    if (params.limit) qs.set('limit', params.limit)
    if (params.offset !== undefined) qs.set('offset', params.offset)
    if (params.event_type) qs.set('event_type', params.event_type)
    if (params.search) qs.set('search', params.search)
    return request(`/api/admin/audit?${qs.toString()}`)
  },

  getSettings: () => request('/api/admin/settings'),

  updateSettings: (settings) =>
    request('/api/admin/settings', {
      method: 'POST',
      body: JSON.stringify(settings),
    }),

  getOAuthSettings: () => request('/api/admin/oauth/settings'),

  updateOAuthSettings: (data) =>
    request('/api/admin/oauth/settings', {
      method: 'POST',
      body: JSON.stringify(data),
    }),

  getResendSettings: () => request('/api/admin/resend/settings'),

  updateResendSettings: (data) =>
    request('/api/admin/resend/settings', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
}
