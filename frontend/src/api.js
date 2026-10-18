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

export const api = {
  login: (username, password) =>
    request('/api/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  register: (username, password) =>
    request('/api/register', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),

  logout: () => request('/api/logout', { method: 'POST' }),

  me: () => request('/api/me'),

  changePassword: (oldPassword, newPassword) =>
    request('/api/user/password', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword }),
    }),

  toggleCli: () => request('/api/env/cli/toggle', { method: 'POST' }),

  toggleGui: () => request('/api/env/gui/toggle', { method: 'POST' }),

  destroyEnv: () => request('/api/env/destroy', { method: 'POST' }),
}
