function getToken() {
  return localStorage.getItem('attackbox_token') || ''
}

function setToken(token) {
  if (token) {
    localStorage.setItem('attackbox_token', token)
  } else {
    localStorage.removeItem('attackbox_token')
  }
}

async function request(url, options = {}) {
  const headers = {
    'Content-Type': 'application/json',
    ...options.headers,
  }
  const token = getToken()
  if (token && !headers['Authorization']) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const resp = await fetch(url, {
    credentials: 'include',
    headers,
    ...options,
  })
  const data = await resp.json().catch(() => ({}))
  if (!resp.ok) {
    const error = new Error(data.error || `请求失败 (${resp.status})`)
    error.status = resp.status
    throw error
  }
  return data
}

export const api = {
  getToken,
  setToken,

  sendVerificationCode: (email) =>
    request('/api/auth/send-verification-code', {
      method: 'POST',
      body: JSON.stringify({ email }),
    }),

  register: (email, code, password) =>
    request('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, code, password }),
    }),

  login: (email, password) =>
    request('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({
        email: email.trim(),
        password,
      }),
    }),

  updateUsername: (username) =>
    request('/api/user/username', {
      method: 'POST',
      body: JSON.stringify({ username }),
    }),

  logout: async () => {
    try {
      await request('/api/logout', { method: 'POST' })
    } finally {
      setToken('')
    }
  },

  me: () => request('/api/me'),

  changePassword: (oldPassword, newPassword) =>
    request('/api/user/password', {
      method: 'POST',
      body: JSON.stringify({ oldPassword, newPassword }),
    }),

  startSandbox: (signal) => request('/api/sandbox/start', { method: 'POST', body: '{}', signal }),

  stopSandbox: (sandboxId = '') =>
    request('/api/sandbox/stop', {
      method: 'POST',
      body: JSON.stringify({ sandbox_id: sandboxId }),
    }),

  getActiveSandbox: (params = '', signal) => {
    const qs = typeof params === 'string' && params ? (params.startsWith('?') ? params : `?${params}`) : ''
    return request(`/api/sandbox/active${qs}`, { signal })
  },
}

// Both viewers use the same idempotent start. A concurrent start may still be
// creating; wait for that instance instead of allocating another environment.
export async function ensureSandbox(signal) {
  try {
    return await api.startSandbox(signal)
  } catch (error) {
    if (error.status !== 409) throw error
    for (let attempt = 0; attempt < 60; attempt++) {
      const active = await api.getActiveSandbox('', signal)
      if (active.active) return active
      if (active.state && active.state !== 'creating') throw error
      await new Promise((resolve, reject) => {
        if (signal?.aborted) return reject(new DOMException('Aborted', 'AbortError'))
        const abort = () => {
          clearTimeout(timer)
          reject(new DOMException('Aborted', 'AbortError'))
        }
        const timer = setTimeout(() => {
          signal?.removeEventListener('abort', abort)
          resolve()
        }, 500)
        signal?.addEventListener('abort', abort, { once: true })
      })
    }
    throw new Error('沙箱仍在启动，请稍后重试')
  }
}
