import test from 'node:test'
import assert from 'node:assert/strict'
import {
  parseLabRepository,
  parseLabToken,
  resolveLabCommit,
  readLabSnapshot,
  githubReader
} from '../server/labs/github.ts'
const commit = 'a'.repeat(40),
  root = 'b'.repeat(40),
  folder = 'c'.repeat(40),
  blob = 'd'.repeat(40)
function fixture(
  entries = [{ path: 'compose.yml', type: 'blob', mode: '100644', sha: blob, size: 3 }]
) {
  const calls = []
  const read = async path => {
    calls.push(path)
    if (path === 'commits/feature%2Flab') return { sha: commit }
    if (path === `git/commits/${commit}`) return { tree: { sha: root } }
    if (path === `git/trees/${root}`) return { tree: [{ path: 'labs', type: 'tree', sha: folder }] }
    if (path === `git/trees/${folder}?recursive=1`) return { tree: entries }
    if (path === `git/blobs/${blob}`)
      return { encoding: 'base64', content: Buffer.from('abc').toString('base64') }
    throw Error('unexpected ' + path)
  }
  return { calls, read }
}
test('Git source pins commit and keeps selected directory relative paths', async () => {
  const config = parseLabRepository({
      repository: 'https://github.com/example/labs.git',
      branch: 'feature/lab',
      directory: 'labs/'
    }),
    f = fixture()
  assert.deepEqual(config, { repository: 'example/labs', branch: 'feature/lab', directory: 'labs' })
  assert.equal(await resolveLabCommit(config, f.read), commit)
  const files = await readLabSnapshot(config, commit, f.read)
  assert.equal(files.get('compose.yml').toString(), 'abc')
  assert.ok(f.calls.includes(`git/commits/${commit}`))
  assert.ok(!f.calls.some(p => p.includes('HEAD')))
})
test('Git rejects external URLs, unsafe paths, links, submodules and oversized snapshots before blob download', async () => {
  for (const repository of ['https://evil.example/a/b', 'http://github.com/a/b', 'a/b/c', '../a'])
    assert.throws(() => parseLabRepository({ repository }))
  for (const directory of ['../lab', '/etc', 'a/../../b', 'a\\b'])
    assert.throws(() => parseLabRepository({ repository: 'a/b', directory }))
  assert.throws(() => parseLabToken('token\nsecret'))
  const config = { repository: 'a/b', branch: 'main', directory: 'labs' }
  for (const patch of [
    { path: '../bad' },
    { type: 'commit', mode: '160000' },
    { mode: '120000' },
    { size: 4 * 1024 * 1024 + 1 }
  ]) {
    const f = fixture([
      { path: 'compose.yml', type: 'blob', mode: '100644', sha: blob, size: 3, ...patch }
    ])
    await assert.rejects(readLabSnapshot(config, commit, f.read))
    assert.ok(!f.calls.some(p => p.startsWith('git/blobs/')))
  }
  const tooMany = fixture(
    Array.from({ length: 513 }, (_, i) => ({
      path: `${i}.txt`,
      type: 'blob',
      mode: '100644',
      sha: blob,
      size: 3
    }))
  )
  await assert.rejects(readLabSnapshot(config, commit, tooMany.read))
  const truncated = async p =>
    p.startsWith('git/commits/') ? { tree: { sha: root } } : { truncated: true, tree: [] }
  await assert.rejects(readLabSnapshot({ ...config, directory: '' }, commit, truncated))
})
test('Git transport uses fixed HTTPS API, optional credentials, rejects redirects and does not leak token', async () => {
  const original = globalThis.fetch,
    calls = []
  try {
    globalThis.fetch = async (url, options) => {
      calls.push({ url, options })
      return new Response(JSON.stringify({ sha: commit }), { status: 200 })
    }
    const config = { repository: 'example/labs', branch: 'main', directory: '' }
    await githubReader(config, '', AbortSignal.timeout(1000))('commits/main')
    assert.equal(calls[0].url, 'https://api.github.com/repos/example/labs/commits/main')
    assert.equal(calls[0].options.redirect, 'error')
    assert.equal(calls[0].options.headers.Authorization, undefined)
    await githubReader(config, 'private-value', AbortSignal.timeout(1000))('commits/main')
    assert.equal(calls[1].options.headers.Authorization, 'Bearer private-value')
    globalThis.fetch = async () => new Response('private-value', { status: 403 })
    await assert.rejects(
      githubReader(config, 'private-value', AbortSignal.timeout(1000))('commits/main'),
      e => !e.message.includes('private-value')
    )
  } finally {
    globalThis.fetch = original
  }
})
test('Git credentials are bound to source snapshot and isolated from deployment variables', async () => {
  const { sealVariables, openVariables } = await import('../server/labs/credentials.ts'),
    previous = process.env.CONTENT_CREDENTIALS_KEY
  try {
    process.env.CONTENT_CREDENTIALS_KEY = 'ab'.repeat(32)
    const cipher = sealVariables('git:bundle-a', { token: 'private-test-token' })
    assert.ok(!cipher.includes('private-test-token'))
    assert.equal(openVariables('git:bundle-a', cipher).token, 'private-test-token')
    assert.throws(() => openVariables('bundle-a', cipher))
    assert.throws(() => openVariables('git:bundle-b', cipher))
  } finally {
    if (previous === undefined) delete process.env.CONTENT_CREDENTIALS_KEY
    else process.env.CONTENT_CREDENTIALS_KEY = previous
  }
})
