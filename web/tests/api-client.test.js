// web/tests/api-client.test.js
import test from 'node:test'
import assert from 'node:assert/strict'
import { ApiError, apiErrorMessage, apiStatusCode, apiData, apiFetch } from '../utils/api.ts'

test('apiErrorMessage prefers the server message, then the fallback', () => {
  assert.equal(apiErrorMessage({ data: { message: '模板读取失败' } }, '兜底'), '模板读取失败')
  assert.equal(
    apiErrorMessage({ statusCode: 400, data: { statusMessage: '参数无效' } }, '兜底'),
    '参数无效'
  )
  assert.equal(apiErrorMessage(new ApiError('保存失败'), '兜底'), '保存失败')
  assert.equal(apiErrorMessage('直接给出的说明', '兜底'), '直接给出的说明')
  assert.equal(apiErrorMessage(null, '兜底'), '兜底')
  assert.equal(apiErrorMessage(undefined, '兜底'), '兜底')
  assert.equal(apiErrorMessage(new Error('fetch failed'), '兜底'), '兜底')
  assert.equal(apiErrorMessage({ data: { message: '   ' } }, '兜底'), '兜底')
})

test('apiStatusCode reads ApiError and ofetch-style failures', () => {
  assert.equal(apiStatusCode(new ApiError('x', { statusCode: 403 })), 403)
  assert.equal(apiStatusCode({ statusCode: 404 }), 404)
  assert.equal(apiStatusCode({ status: 502 }), 502)
  assert.equal(apiStatusCode(new Error('network')), 0)
})

test('apiFetch disables retries by default and forwards overrides', async () => {
  const calls = []
  const original = globalThis.$fetch
  globalThis.$fetch = async (url, options) => {
    calls.push({ url, options })
    return { ok: true }
  }
  try {
    const body = await apiFetch('/api/example', { method: 'POST' })
    assert.deepEqual(body, { ok: true })
    assert.equal(calls.length, 1)
    assert.equal(calls[0].options.retry, false)
    assert.equal(calls[0].options.method, 'POST')
  } finally {
    globalThis.$fetch = original
  }
})

test('apiFetch normalizes failures into ApiError', async () => {
  const original = globalThis.$fetch
  globalThis.$fetch = async () => {
    throw { statusCode: 409, statusMessage: 'Conflict', data: { message: '版本已过期,请重新读取' } }
  }
  try {
    await assert.rejects(
      () => apiFetch('/api/example'),
      error => {
        assert.ok(error instanceof ApiError)
        assert.equal(error.statusCode, 409)
        assert.equal(error.message, '版本已过期,请重新读取')
        assert.equal(error.statusMessage, 'Conflict')
        return true
      }
    )
  } finally {
    globalThis.$fetch = original
  }
})

test('apiData unwraps the data envelope and rejects malformed bodies', async () => {
  const original = globalThis.$fetch
  try {
    globalThis.$fetch = async () => ({ data: { id: 'abc' } })
    assert.deepEqual(await apiData('/api/example'), { id: 'abc' })

    globalThis.$fetch = async () => ({ ok: true })
    await assert.rejects(
      () => apiData('/api/example'),
      error => error instanceof ApiError && error.message === '接口返回格式无效'
    )
  } finally {
    globalThis.$fetch = original
  }
})
