import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtemp, mkdir, writeFile, rm } from 'node:fs/promises'
import path from 'node:path'
import os from 'node:os'
import { createJiti } from 'jiti'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'
import { chooseOption } from './helpers/select.mjs'
test(
  'Git cover import, public access, homepage filters, failed-image fallback and removal',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 90000 },
  async () => {
    process.env.DATABASE_URL = process.env.CONTENT_TEST_DATABASE_URL
    const f = await learningFixture(),
      jiti = createJiti(import.meta.url),
      sync = await jiti.import('../server/services/content-sync.ts'),
      database = await jiti.import('../server/db/index.ts')
    const root = await mkdtemp(path.join(os.tmpdir(), 'cover-import-'))
    let id, browser
    const source = cover =>
      `---\nsummary: Learn with a real repository cover\ndifficulty: advanced\naccess: public\n${cover ? 'cover: images/cover.png\n' : ''}chapters:\n  First:\n    durationMinutes: 20\n---\n# First\n\nContent\n`
    try {
      await mkdir(path.join(root, 'images'))
      await writeFile(
        path.join(root, 'images/cover.png'),
        Buffer.from(
          'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=',
          'base64'
        )
      )
      await writeFile(path.join(root, 'Cover Module.md'), source(true))
      const preview = await sync.createLocalContentPreview(root, f.admin.id)
      assert.equal(preview.modules[0].preview.cover, 'images/cover.png')
      id = (await sync.applyContentPreview(preview.id, f.admin.id)).moduleIds[0]
      const detail = (await f.request(`/api/admin/modules/${id}`, { actor: f.admin })).data
      assert.ok(detail.module.coverAssetId)
      await f.request(`/api/admin/modules/${id}/publish`, {
        actor: f.admin,
        method: 'POST',
        body: { lockVersion: detail.module.lockVersion }
      })
      const href = `/api/modules/${detail.module.slug}/assets/${detail.module.coverAssetId}`
      const asset = await fetch(f.base + href)
      assert.equal(asset.status, 200)
      assert.equal(asset.headers.get('content-type'), 'image/png')
      const cards = (await f.request('/api/modules')).data
      assert.equal(cards.find(m => m.id === id).durationMinutes, 20)
      await f.createModule('publish', { title: 'Beginner Module', difficulty: 'beginner' })
      browser = await chromium.launch({
        executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
      await page.goto(f.base + '/learn')
      await page.locator('.card-title').filter({ hasText: 'Cover Module' }).waitFor()
      const img = page.locator('.card img.module-cover')
      await img.waitFor()
      assert.ok(await img.evaluate(el => el.complete && el.naturalWidth > 0))
      assert.ok((await page.locator('.feature-banner').boundingBox()).height < 190)
      await chooseOption(page, page.getByRole('combobox', { name: '筛选难度' }), '高阶实战')
      assert.equal(await page.locator('.card').count(), 1)
      await chooseOption(page, page.getByRole('combobox', { name: '模块排序' }), '最近更新')
      await chooseOption(page, page.getByRole('combobox', { name: '筛选难度' }), '全部难度')
      assert.equal(await page.locator('.card').count(), 2)
      await page.screenshot({ path: '/tmp/attackbox-learn-discovery.png', fullPage: true })
      await img.evaluate(el => el.dispatchEvent(new Event('error')))
      await page.waitForTimeout(100)
      assert.equal(await page.locator('.card img.module-cover').count(), 0)
      await writeFile(path.join(root, 'Cover Module.md'), source(false))
      const next = await sync.createLocalContentPreview(root, f.admin.id)
      await sync.applyContentPreview(next.id, f.admin.id)
      assert.equal(
        (await f.request(`/api/admin/modules/${id}`, { actor: f.admin })).data.module.coverAssetId,
        null
      )
      assert.equal((await fetch(f.base + href)).status, 404)
    } finally {
      await browser?.close()
      await f.sql`delete from audit_logs where actor_user_id=${f.admin.id}`
      if (id) await f.sql`delete from modules where id=${id}`
      await f.sql`delete from assets where uploaded_by=${f.admin.id}`
      await f.cleanup()
      await database.client.end()
      await rm(root, { recursive: true, force: true })
    }
  }
)
