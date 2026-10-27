import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'
import { sandboxWorker } from './helpers/sandbox-worker.mjs'
test(
  'global runtime policy: admin UI, persistence, conflicts and new-instance deadlines',
  {
    skip:
      !process.env.CONTENT_TEST_DATABASE_URL ||
      !process.env.NODE_TEST_CLUSTER_SECRET,
    timeout: 90000,
  },
  async () => {
    const f = await learningFixture()
    let worker, browser, user, other, target
    const policy = '/api/admin/runtime-policy',
      admin = f.admin
    try {
      worker = await sandboxWorker(
        f.base,
        process.env.NODE_TEST_CLUSTER_SECRET,
        { composeLabs: true },
      )
      user = await f.account()
      other = await f.account()
      const original = (await f.request(policy, { actor: admin })).data
      assert.equal(original.sandboxTtlMinutes, 60)
      assert.equal(original.sandboxIdleMinutes, 10)
      assert.equal(original.targetTtlMinutes, 60)
      await f.request(policy, { status: 401 })
      await f.request(policy, { actor: user, status: 403 })
      await f.request(policy, {
        actor: user,
        method: 'PUT',
        body: original,
        status: 403,
      })
      await f.request(policy, {
        actor: admin,
        method: 'PUT',
        body: { ...original, sandboxIdleMinutes: 61 },
        status: 400,
      })
      const first = (
        await f.request('/api/sandbox/start', {
          actor: user,
          method: 'POST',
          body: {},
        })
      ).data
      const create = worker.requests.find((r) => r.type === 'create_box')
      assert.equal(create.payload.ttl_seconds, 3600)
      assert.equal(create.payload.idle_seconds, 600)
      browser = await chromium.launch({
        executablePath: '/usr/bin/chromium',
        args: ['--no-sandbox'],
      })
      const ctx = await browser.newContext({
        viewport: { width: 1440, height: 1000 },
      })
      await ctx.addCookies([
        { name: 'attackbox_session', value: admin.token, url: f.base },
      ])
      const page = await ctx.newPage()
      await page.goto(f.base + '/admin/runtime-policy')
      await page.locator('#sandbox-ttl').fill('90')
      await page.locator('#sandbox-idle').fill('15')
      await page.locator('#target-ttl').fill('180')
      await page.getByRole('button', { name: '保存设置', exact: true }).click()
      await page.getByText('运行策略已保存', { exact: true }).waitFor()
      await page.reload()
      await page.locator('#sandbox-ttl').waitFor()
      assert.equal(await page.locator('#sandbox-ttl').inputValue(), '90')
      await f.request(policy, {
        actor: admin,
        method: 'PUT',
        body: original,
        status: 409,
      })
      const unchanged = (
        await f.request('/api/sandbox/start', {
          actor: user,
          method: 'POST',
          body: {},
        })
      ).data
      assert.equal(unchanged.expiresAt, first.expiresAt)
      await f.request('/api/sandbox/start', {
        actor: other,
        method: 'POST',
        body: {},
      })
      const next = worker.requests.filter((r) => r.type === 'create_box').at(-1)
      assert.equal(next.payload.ttl_seconds, 5400)
      assert.equal(next.payload.idle_seconds, 900)
      target = await f.seedTarget({ enabled: true }, worker.boot)
      const mod = await f.createModule()
      await f.sql`update modules set default_target_template_id=${target.id} where id=${mod.module.id}`
      const scope = {
        kind: 'chapter',
        moduleSlug: mod.module.slug,
        chapterSlug: 'intro',
        action: 'start',
      }
      const send = async (body) => {
        const r = await fetch(f.base + '/api/targets', {
          method: 'POST',
          headers: {
            cookie: `attackbox_session=${user.token}`,
            'content-type': 'application/json',
          },
          body: JSON.stringify(body),
        })
        const j = await r.json()
        assert.equal(r.status, 200, JSON.stringify(j))
        return j
      }
      const started = await send(scope)
      assert.ok(
        Math.abs(Date.parse(started.expiresAt) - Date.now() - 180 * 60000) <
          10000,
      )
      await page.screenshot({
        path: '/tmp/attackbox-runtime-policy.png',
        animations: 'disabled',
      })
      const stored = (await f.request(policy, { actor: admin })).data
      await f.request(policy, {
        actor: admin,
        method: 'PUT',
        body: { ...stored, targetTtlMinutes: 30 },
      })
      assert.equal((await send(scope)).expiresAt, started.expiresAt)
      await send({ id: started.id, action: 'stop' })
      for (const account of [user, other]) {
        const status = (
          await f.request('/api/sandbox/status', { actor: account })
        ).data
        if (status.instance)
          await f.request('/api/sandbox/stop', {
            actor: account,
            method: 'POST',
            body: { sandboxId: status.instance.id },
          })
      }
    } finally {
      await browser?.close()
      await worker?.close()
      if (user)
        await f.sql`delete from target_instances where user_id=${user.id}`
      await f.sql`delete from runtime_policies where id='global'`
      await f.cleanup()
    }
  },
)
