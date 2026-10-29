import test from 'node:test'
import assert from 'node:assert/strict'
import path from 'node:path'
import net from 'node:net'
import { spawn } from 'node:child_process'
import { once } from 'node:events'
import { learningFixture } from './helpers/learning-fixture.mjs'

test(
  'node policy survives a fresh Nitro process and remains editable with optimistic versions',
  {
    skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.CONTENT_TEST_BASE_URL,
    timeout: 30000
  },
  async () => {
    const f = await learningFixture()
    let child, original, currentVersion
    try {
      original = (await f.request('/api/admin/nodes', { actor: f.admin })).data.nodes[0].settings
      const saved = (
        await f.request('/api/admin/nodes/test-node/settings', {
          actor: f.admin,
          method: 'PUT',
          body: { ...original, label: '持久化验证', draining: true, memoryMb: 768 }
        })
      ).data
      currentVersion = saved.lockVersion
      const reservation = net.createServer()
      await new Promise(resolve => reservation.listen(0, '127.0.0.1', resolve))
      const port = reservation.address().port
      await new Promise(resolve => reservation.close(resolve))
      child = spawn(process.execPath, ['.output/server/index.mjs'], {
        cwd: path.resolve(import.meta.dirname, '..'),
        env: {
          ...process.env,
          DATABASE_URL: process.env.CONTENT_TEST_DATABASE_URL,
          HOST: '127.0.0.1',
          PORT: String(port),
          SANDBOX_CONTROL_ENABLED: 'true',
          SANDBOX_SCHEDULING_ENABLED: 'false',
          SANDBOX_CLUSTER_SECRET: 'persistence-test-cluster-identity-0000',
          SANDBOX_SIGNING_SEED_HEX: '22'.repeat(32),
          SANDBOX_NODES_JSON:
            '[{"id":"test-node","label":"Startup default","publicDataUrl":"ws://127.0.0.1:18443"}]'
        },
        stdio: ['ignore', 'pipe', 'pipe']
      })
      let loaded
      for (let attempt = 0; attempt < 100; attempt++) {
        if (child.exitCode !== null) throw new Error('Fresh Nitro process failed to start')
        try {
          const response = await fetch(`http://127.0.0.1:${port}/api/admin/nodes`, {
            headers: { cookie: `attackbox_session=${f.admin.token}` }
          })
          if (response.status === 200) {
            loaded = (await response.json()).data
            break
          }
        } catch {
          /* 清理失败不影响断言结果 */
        }
        await new Promise(resolve => setTimeout(resolve, 100))
      }
      assert.ok(loaded, 'fresh server should become ready')
      assert.equal(loaded.nodes[0].label, '持久化验证')
      assert.equal(loaded.nodes[0].settings.memoryMb, 768)
      assert.equal(loaded.nodes[0].settings.draining, true)
      assert.equal(loaded.nodes[0].settings.lockVersion, saved.lockVersion)
      assert.equal(loaded.schedulingEnabled, false)
    } finally {
      if (child && child.exitCode === null) {
        const exited = once(child, 'exit')
        child.kill('SIGTERM')
        await exited
      }
      if (original && currentVersion)
        await f.request('/api/admin/nodes/test-node/settings', {
          actor: f.admin,
          method: 'PUT',
          body: { ...original, lockVersion: currentVersion }
        })
      await f.cleanup()
    }
  }
)
