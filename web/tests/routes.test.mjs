import test from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'
test(
  'routes: atomic publication, ordering, isolation, progress, stale edits and browser editor',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 120000 },
  async () => {
    const f = await learningFixture()
    const routes = []
    let browser
    const req = async (path, options) => (await f.request(path, options)).data
    try {
      const user = await f.account(),
        a = await f.createModule(),
        b = await f.createModule(),
        draft = await f.createModule('draft')
      await f.request('/api/admin/routes', { status: 401 })
      await f.request('/api/admin/routes', { actor: user, status: 403 })
      const save = (id, body, status = 200) =>
        req(id ? `/api/admin/routes/${id}` : '/api/admin/routes', {
          actor: f.admin,
          method: id ? 'PUT' : 'POST',
          body,
          status
        })
      const payload = {
        title: '路线 ' + a.module.id,
        summary: '从模块到学习计划',
        intent: 'draft',
        moduleIds: [a.module.id, b.module.id]
      }
      const r = await save(null, payload)
      routes.push(r.id)
      await f.request(`/api/routes/${r.id}`, { status: 404 })
      let d = await req(`/api/admin/routes/${r.id}`, { actor: f.admin })
      await save(
        r.id,
        { ...payload, intent: 'publish', moduleIds: [draft.module.id], lockVersion: d.lockVersion },
        409
      )
      await save(
        r.id,
        { ...payload, moduleIds: [a.module.id, a.module.id], lockVersion: d.lockVersion },
        400
      )
      await save(
        r.id,
        {
          ...payload,
          moduleIds: [a.module.id, a.module.id.toUpperCase()],
          lockVersion: d.lockVersion
        },
        400
      )
      await save(
        r.id,
        { ...payload, intent: 'publish', moduleIds: [], lockVersion: d.lockVersion },
        400
      )
      await save(r.id, {
        ...payload,
        intent: 'publish',
        moduleIds: [b.module.id, a.module.id],
        lockVersion: d.lockVersion
      })
      await save(r.id, { ...payload, lockVersion: d.lockVersion }, 409)
      d = await req(`/api/routes/${r.id}`)
      assert.deepEqual(
        d.modules.map(m => m.id),
        [b.module.id, a.module.id]
      )
      assert.equal(d.completedChapters, 0)
      assert.ok(!JSON.stringify(d).includes('bodyMarkdown'))
      await f.sql`insert into chapter_progress(user_id,chapter_id,completed_at) values(${user.id},${a.chapters[0].chapterId},now())`
      assert.equal((await req(`/api/routes/${r.id}`, { actor: user })).completedChapters, 1)
      assert.equal((await req(`/api/routes/${r.id}`)).completedChapters, 0)
      const edit = await req(`/api/admin/modules/${a.module.id}`, { actor: f.admin })
      await f.request(`/api/admin/modules/${a.module.id}`, {
        actor: f.admin,
        method: 'DELETE',
        body: { lockVersion: edit.module.lockVersion },
        status: 409
      })
      await f.sql`update modules set status='unpublished' where id=${b.module.id}`
      d = await req(`/api/routes/${r.id}`)
      assert.deepEqual(
        d.modules.map(m => m.id),
        [a.module.id]
      )
      await f.sql`update modules set status='unpublished' where id=${a.module.id}`
      await f.request(`/api/routes/${r.id}`, { status: 404 })
      assert.ok(!(await req('/api/routes')).some(x => x.id === r.id))
      await f.sql`update modules set status='published' where id in (${a.module.id},${b.module.id})`
      browser = await chromium.launch({
        executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const ctx = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await ctx.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
      const p = await ctx.newPage()
      await p.goto(`${f.base}/admin/routes/${r.id}`)
      await p.getByRole('heading', { name: '编辑路线', exact: true }).waitFor()
      await p.locator('input[maxlength="160"]').fill(payload.title + ' 编辑')
      await p.getByRole('button', { name: `上移 ${a.module.title}`, exact: true }).click()
      await p.getByRole('button', { name: '保存并发布', exact: true }).click()
      await p.getByText('已保存并发布', { exact: true }).waitFor()
      d = await req(`/api/routes/${r.id}`)
      assert.equal(d.modules[0].id, a.module.id)
      await p.goto(`${f.base}/learn/routes/${r.id}`)
      await p.getByRole('heading', { name: payload.title + ' 编辑', exact: true }).waitFor()
      assert.equal(await p.locator('.route-module').count(), 2)
      await p.goto(`${f.base}/learn?tab=routes`)
      await p.locator(`a[href="/learn/routes/${r.id}"]`).waitFor()
      await p.screenshot({ path: '/tmp/attackbox-routes.png', fullPage: true })
      await p.goto(`${f.base}/admin/routes/new`)
      await p.getByRole('heading', { name: '新建路线', exact: true }).waitFor()
      assert.equal(
        (await req('/api/admin/routes', { actor: f.admin })).length,
        1,
        'opening a new editor does not create a draft'
      )
      await p.locator('input[maxlength="160"]').fill('浏览器新建路线')
      await p.getByRole('button').filter({ hasText: a.module.title }).click()
      const saving = p.waitForResponse(
        x => new URL(x.url()).pathname === '/api/admin/routes' && x.request().method() === 'POST'
      )
      await p.getByRole('button', { name: '保存并发布', exact: true }).click()
      const created = (await (await saving).json()).data
      routes.push(created.id)
      await p.waitForURL(`**/admin/routes/${created.id}`)
      assert.equal((await req(`/api/routes/${created.id}`)).moduleCount, 1)
      d = await req(`/api/admin/routes/${r.id}`, { actor: f.admin })
      await save(r.id, { ...payload, intent: 'draft', lockVersion: d.lockVersion })
      await f.request(`/api/routes/${r.id}`, { status: 404 })
      const list = await req('/api/routes')
      assert.ok(!list.some(x => x.id === r.id))
      d = await req(`/api/admin/routes/${r.id}`, { actor: f.admin })
      await f.request(`/api/admin/routes/${r.id}`, {
        actor: f.admin,
        method: 'DELETE',
        body: { lockVersion: d.lockVersion - 1 },
        status: 409
      })
      await f.request(`/api/admin/routes/${r.id}`, {
        actor: f.admin,
        method: 'DELETE',
        body: { lockVersion: d.lockVersion }
      })
      assert.equal((await f.sql`select * from chapter_progress where user_id=${user.id}`).length, 1)
    } finally {
      await browser?.close()
      for (const id of routes) await f.sql`delete from learning_routes where id=${id}`
      await f.cleanup()
    }
  }
)
