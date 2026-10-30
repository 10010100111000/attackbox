import test from 'node:test'
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { chromium, request } from 'playwright'
import { chromiumOptions } from './helpers/browser.mjs'
import { learningFixture } from './helpers/learning-fixture.mjs'

const enabled = !!process.env.CONTENT_TEST_DATABASE_URL && !!process.env.CONTENT_TEST_BASE_URL
const creative = () => ({
  name: 'Community workshop',
  title: '公开安全工作坊',
  description: '学习与交流',
  imageUrl: '',
  href: 'https://sponsor.example/workshop',
  cta: '了解工作坊',
  sponsor: 'Community',
  placements: ['learn_catalog', 'reference_footer'],
  enabled: false,
  weight: 3,
  startsAt: null,
  endsAt: null,
  lockVersion: 0
})
async function eventually(check) {
  const until = Date.now() + 8000
  while (Date.now() < until) {
    if (await check()) return
    await new Promise(resolve => setTimeout(resolve, 30))
  }
  assert.fail('expected the asynchronous statistic to be recorded')
}

test(
  'operations API enforces admin access, delivery windows, visit ownership, deduplication, attribution, limits and retention',
  { skip: !enabled, timeout: 60000 },
  async () => {
    const f = await learningFixture(),
      ids = [],
      visits = []
    let guest, other
    try {
      const baseline = (await f.request('/api/admin/analytics?days=7', { actor: f.admin })).data
        .totals
      const user = await f.account()
      await f.request('/api/admin/analytics', { status: 401 })
      await f.request('/api/admin/analytics', { actor: user, status: 403 })
      await f.request('/api/admin/advertisements', {
        actor: user,
        method: 'POST',
        body: creative(),
        status: 403
      })
      const ad = (
        await f.request('/api/admin/advertisements', {
          actor: f.admin,
          method: 'POST',
          body: creative()
        })
      ).data
      ids.push(ad.id)
      assert.equal(ad.lockVersion, 1)
      assert.equal((await f.request('/api/ads/learn_catalog')).data, null)
      await f.request(`/api/admin/advertisements/${ad.id}`, {
        actor: f.admin,
        method: 'PUT',
        body: { ...ad, weight: 0 },
        status: 400
      })
      const live = (
        await f.request(`/api/admin/advertisements/${ad.id}`, {
          actor: f.admin,
          method: 'PUT',
          body: { ...ad, enabled: true }
        })
      ).data
      await f.request(`/api/admin/advertisements/${ad.id}`, {
        actor: f.admin,
        method: 'PUT',
        body: { ...ad, enabled: true },
        status: 409
      })
      assert.equal((await f.request('/api/ads/learn_catalog')).data.id, ad.id)
      assert.equal((await f.request('/api/ads/reference_footer')).data.id, ad.id)
      const scheduled = (
        await f.request('/api/admin/advertisements', {
          actor: f.admin,
          method: 'POST',
          body: { ...creative(), enabled: true, startsAt: '2099-01-01T00:00:00Z' }
        })
      ).data
      ids.push(scheduled.id)
      for (let i = 0; i < 4; i++)
        assert.equal((await f.request('/api/ads/learn_catalog')).data.id, ad.id)
      guest = await request.newContext({ baseURL: f.base, extraHTTPHeaders: { Origin: f.base } })
      other = await request.newContext({ baseURL: f.base, extraHTTPHeaders: { Origin: f.base } })
      const visit = {
        id: randomUUID(),
        path: '/learn',
        referrerHost: 'www.google.com',
        source: '',
        medium: '',
        campaign: ''
      }
      visits.push(visit.id)
      assert.equal((await guest.post('/api/analytics/visits', { data: visit })).status(), 200)
      assert.equal((await guest.post('/api/analytics/visits', { data: visit })).status(), 200)
      assert.equal(
        (
          await guest.post('/api/analytics/visits', {
            data: { ...visit, path: '/references/book' }
          })
        ).status(),
        409
      )
      assert.equal((await other.post('/api/analytics/visits', { data: visit })).status(), 409)
      const cookies = (await guest.storageState()).cookies
      assert.equal(cookies.find(c => c.name === 'attackbox_visitor').httpOnly, true)
      const [stored] =
        await f.sql`select visitor_key,path,source from analytics_visits where id=${visit.id}`
      assert.notEqual(stored.visitor_key, cookies.find(c => c.name === 'attackbox_visitor').value)
      assert.equal(stored.source, 'google.com')
      assert.equal(
        (
          await guest.post('/api/analytics/visits', {
            data: { ...visit, id: randomUUID(), path: '/auth?password=secret' }
          })
        ).status(),
        400
      )
      assert.equal(
        (
          await guest.post('/api/analytics/visits', {
            data: visit,
            headers: { Origin: 'https://elsewhere.example' }
          })
        ).status(),
        403
      )
      assert.equal(
        (
          await guest.post('/api/analytics/visits', { data: { excess: 'x'.repeat(5000) } })
        ).status(),
        413
      )
      const click = { visitId: visit.id, adId: ad.id, placement: 'learn_catalog', kind: 'click' }
      await Promise.all(
        Array.from({ length: 4 }, () =>
          guest.post('/api/analytics/advertisements', { data: click })
        )
      )
      await other.post('/api/analytics/advertisements', { data: click })
      await guest.post('/api/analytics/advertisements', {
        data: { ...click, placement: 'reference_footer' }
      })
      const mod = await f.createModule('publish')
      const reading = {
        ...visit,
        id: randomUUID(),
        path: `/learn/modules/${mod.module.slug}/intro`,
        source: 'community',
        medium: 'post',
        campaign: 'launch'
      }
      visits.push(reading.id)
      assert.equal((await guest.post('/api/analytics/visits', { data: reading })).status(), 200)
      assert.equal(
        (await guest.post('/api/analytics/read', { data: { visitId: reading.id } })).status(),
        200
      )
      assert.equal(
        (await guest.post('/api/analytics/read', { data: { visitId: reading.id } })).status(),
        200
      )
      assert.equal(
        (await other.post('/api/analytics/read', { data: { visitId: reading.id } })).status(),
        409
      )
      assert.equal(
        (await guest.post('/api/analytics/read', { data: { visitId: visit.id } })).status(),
        409
      )
      const ignored = await f.request('/api/analytics/visits', {
        actor: f.admin,
        method: 'POST',
        body: { ...visit, id: randomUUID() }
      })
      assert.equal(ignored.response.status, 200)
      let report = (await f.request('/api/admin/analytics?days=7', { actor: f.admin })).data
      assert.equal(report.totals.views, baseline.views + 2)
      assert.equal(report.totals.visitors, baseline.visitors + 1)
      assert.equal(report.totals.reads, baseline.reads + 1)
      assert.equal(report.totals.impressions, baseline.impressions + 1)
      assert.equal(report.totals.clicks, baseline.clicks + 1)
      assert.equal(
        report.sources.find(s => s.source === 'community' && s.campaign === 'launch').campaign,
        'launch'
      )
      assert.equal(report.advertisements.find(a => a.id === ad.id).name, ad.name)
      await f.request(`/api/admin/advertisements/${ad.id}`, {
        actor: f.admin,
        method: 'DELETE',
        body: { lockVersion: ad.lockVersion },
        status: 409
      })
      await f.request(`/api/admin/advertisements/${ad.id}`, {
        actor: f.admin,
        method: 'DELETE',
        body: { lockVersion: live.lockVersion }
      })
      report = (await f.request('/api/admin/analytics?days=7', { actor: f.admin })).data
      assert.equal(
        report.totals.clicks,
        baseline.clicks + 1,
        'campaign deletion retains event facts'
      )
      assert.equal((await f.request('/api/ads/learn_catalog')).data, null)
      await f.request('/api/admin/analytics?days=999', { actor: f.admin, status: 400 })
      const old = randomUUID()
      visits.push(old)
      await f.sql`insert into analytics_visits(id,visitor_key,path,source,medium,campaign,channel,created_at) values(${old},'expired','/learn','old','','','direct',now()-interval '91 days')`
      const cleanup = { ...visit, id: randomUUID() }
      visits.push(cleanup.id)
      await guest.post('/api/analytics/visits', { data: cleanup })
      assert.equal((await f.sql`select id from analytics_visits where id=${old}`).length, 0)
      await f.sql`update analytics_budgets set attempts=600,window_at=now()`
      assert.equal(
        (
          await guest.post('/api/analytics/visits', { data: { ...visit, id: randomUUID() } })
        ).status(),
        429
      )
    } finally {
      await guest?.dispose()
      await other?.dispose()
      if (visits.length) await f.sql`delete from analytics_visits where id=any(${visits}::uuid[])`
      if (ids.length) await f.sql`delete from advertisements where id=any(${ids}::uuid[])`
      await f.sql`delete from analytics_budgets`
      await f.cleanup()
    }
  }
)

test(
  'admin creates reusable sponsorships; public visits, visible exposures, clicks and reader qualification appear in the dashboard',
  { skip: !enabled, timeout: 90000 },
  async () => {
    const f = await learningFixture(),
      source = `community-${randomUUID().slice(0, 8)}`,
      reference = randomUUID()
    let browser, adId
    try {
      const baseline = (await f.request('/api/admin/analytics', { actor: f.admin })).data.totals
      await f.sql`insert into reference_documents(id,slug,title,body_markdown,status,source_origin,source_path,source_commit,manifest,created_by,published_at) values(${reference},${reference},'运营阅读测试',${'## 阅读\n\n' + '阅读与理解。'.repeat(2400)},'published',${reference},'operations.md','fixture','{}',${f.admin.id},now())`
      browser = await chromium.launch(chromiumOptions())
      const admin = await browser.newContext({ viewport: { width: 1600, height: 1100 } })
      await admin.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
      await admin.route('https://images.example/**', route =>
        route.fulfill({
          contentType: 'image/svg+xml',
          body: '<svg xmlns="http://www.w3.org/2000/svg" width="320" height="192"><rect width="320" height="192" fill="#28566a"/></svg>'
        })
      )
      const editor = await admin.newPage(),
        errors = []
      editor.setDefaultTimeout(8000)
      editor.on('pageerror', e => errors.push(e.message))
      await editor.goto(f.base + '/admin/advertisements/new')
      await editor.getByLabel('内部名称 *', { exact: true }).fill('上线赞助测试')
      await editor.getByLabel('展示标题 *', { exact: true }).fill('一起参加安全工作坊')
      await editor.getByLabel('赞助方', { exact: true }).fill('Community')
      await editor.getByLabel('图片地址', { exact: true }).fill('https://images.example/ad.svg')
      await editor
        .getByLabel('跳转链接 *', { exact: true })
        .fill('https://sponsor.example/workshop')
      await editor.getByRole('checkbox', { name: '启用展示', exact: true }).check()
      await editor.getByRole('checkbox', { name: '参考资料文末', exact: true }).check()
      await editor.getByRole('button', { name: '保存广告', exact: true }).click()
      await editor.waitForURL(/\/admin\/advertisements\/[a-f0-9-]{36}$/)
      adId = new URL(editor.url()).pathname.split('/').at(-1)
      await editor.reload()
      assert.equal(
        await editor.getByLabel('展示标题 *', { exact: true }).inputValue(),
        '一起参加安全工作坊'
      )
      assert.equal(
        await editor.getByRole('checkbox', { name: '参考资料文末', exact: true }).isChecked(),
        true
      )
      await editor.screenshot({ path: '/tmp/attackbox-ad-editor.png', fullPage: true })
      const guest = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await guest.route('https://images.example/**', route =>
        route.fulfill({
          contentType: 'image/svg+xml',
          body: '<svg xmlns="http://www.w3.org/2000/svg" width="320" height="192"><rect width="320" height="192" fill="#28566a"/></svg>'
        })
      )
      await guest.route('https://sponsor.example/**', route =>
        route.fulfill({ contentType: 'text/html', body: 'Sponsor destination' })
      )
      const page = await guest.newPage()
      page.setDefaultTimeout(8000)
      page.on('pageerror', e => errors.push(e.message))
      await page.clock.install()
      await page.goto(
        `${f.base}/learn?utm_source=${source}&utm_medium=post&utm_campaign=launch&password=not-recorded`
      )
      const banner = page.locator('[data-placement="learn_catalog"]')
      await banner.waitFor()
      await banner.scrollIntoViewIfNeeded()
      await eventually(
        async () =>
          (await f.sql`select id from analytics_visits where source=${source}`).length === 1
      )
      await page.clock.fastForward(1200)
      await eventually(
        async () =>
          (await f.sql`select kind from ad_events where ad_id=${adId} and kind='impression'`)
            .length === 1
      )
      const popupPromise = page.waitForEvent('popup')
      await banner.getByRole('link', { name: '了解更多' }).click()
      const popup = await popupPromise
      await popup.waitForLoadState()
      assert.equal(popup.url(), 'https://sponsor.example/workshop')
      await popup.close()
      await eventually(
        async () =>
          (await f.sql`select kind from ad_events where ad_id=${adId} and kind='click'`).length ===
          1
      )
      await page.goto(`${f.base}/references/${reference}`)
      const scroll = page.getByLabel('资料正文', { exact: true })
      await page.getByRole('heading', { name: '运营阅读测试', exact: true }).waitFor()
      await eventually(
        async () =>
          (await f.sql`select id from analytics_visits where source=${source}`).length === 2
      )
      await scroll.evaluate(el => {
        el.scrollTop = (el.scrollHeight - el.clientHeight) * 0.4
      })
      await page.clock.fastForward(20000)
      await page.evaluate(() => {
        Object.defineProperty(document, 'hidden', { configurable: true, get: () => true })
        document.dispatchEvent(new Event('visibilitychange'))
      })
      await page.clock.fastForward(40000)
      assert.equal(
        (
          await f.sql`select read_at from analytics_visits where source=${source} and read_at is not null`
        ).length,
        0
      )
      await page.evaluate(() => {
        Object.defineProperty(document, 'hidden', { configurable: true, get: () => false })
        document.dispatchEvent(new Event('visibilitychange'))
      })
      await page.clock.fastForward(10500)
      await eventually(
        async () =>
          (
            await f.sql`select read_at from analytics_visits where source=${source} and read_at is not null`
          ).length === 1
      )
      assert.equal(
        (await f.sql`select * from analytics_visits where source=${source}`).some(row =>
          JSON.stringify(row).includes('not-recorded')
        ),
        false
      )
      const footer = page.locator('[data-placement="reference_footer"]')
      await footer.scrollIntoViewIfNeeded()
      await page.clock.fastForward(1200)
      await eventually(
        async () =>
          (await f.sql`select kind from ad_events where ad_id=${adId} and kind='impression'`)
            .length === 2
      )
      await editor.goto(f.base + '/admin/analytics')
      await editor.getByText(source, { exact: true }).waitFor()
      await editor.screenshot({ path: '/tmp/attackbox-analytics-dashboard.png', fullPage: true })
      const report = (await f.request('/api/admin/analytics', { actor: f.admin })).data
      assert.equal(report.sources.find(row => row.source === source).views, 2)
      assert.equal(report.sources.find(row => row.source === source).visitors, 1)
      assert.equal(report.totals.reads, baseline.reads + 1)
      assert.equal(
        report.advertisements.find(row => row.id === adId && row.placement === 'reference_footer')
          .impressions,
        1
      )
      await editor.goto(`${f.base}/admin/advertisements/${adId}`)
      await editor.getByRole('checkbox', { name: '启用展示' }).uncheck()
      await editor.getByRole('button', { name: '保存广告', exact: true }).click()
      await editor.getByRole('status').filter({ hasText: '广告已保存' }).waitFor()
      await page.goto(f.base + '/learn')
      await page.locator('.page-tabs').waitFor()
      assert.equal(await page.locator('.ad-slot').count(), 0)
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      await f.sql`delete from analytics_visits where source=${source}`
      if (adId) await f.sql`delete from advertisements where id=${adId}`
      await f.sql`delete from reference_documents where id=${reference}`
      await f.sql`delete from analytics_budgets`
      await f.cleanup()
    }
  }
)
