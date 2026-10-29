import test from 'node:test'
import assert from 'node:assert/strict'
import { createHash, randomUUID } from 'node:crypto'
import { createJiti } from 'jiti'
import { chromium } from 'playwright'
import { chromiumOptions } from './helpers/browser.mjs'
import { learningFixture } from './helpers/learning-fixture.mjs'
import { chooseOption } from './helpers/select.mjs'

const enabled = !!process.env.CONTENT_TEST_DATABASE_URL && !!process.env.CONTENT_TEST_BASE_URL

test(
  'Git cover sync, public access, homepage filters, failed-image fallback and removal',
  { skip: !enabled, timeout: 90000 },
  async () => {
    process.env.DATABASE_URL = process.env.CONTENT_TEST_DATABASE_URL
    const f = await learningFixture()
    const jiti = createJiti(import.meta.url)
    const sync = await jiti.import('../server/services/content-sync.ts')
    const database = await jiti.import('../server/db/index.ts')
    const previousFetch = globalThis.$fetch
    const title = `Cover Module ${randomUUID().slice(0, 8)}`
    const filename = `${title}.md`
    const config = { repository: 'fixture/covers', branch: 'main', directory: 'courses' }
    const image = Buffer.from(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=',
      'base64'
    )
    const source = cover =>
      Buffer.from(
        `---\nsummary: Learn with a real repository cover\ndifficulty: advanced\naccess: public\n${cover ? 'cover: images/cover.png\n' : ''}chapters:\n  First:\n    durationMinutes: 20\n---\n# First\n\nContent\n\n\`\`\`quiz\nprompt: Type hello\nanswer: hello\n\`\`\`\n`
      )
    let markdown = source(true)
    const blobSha = bytes =>
      createHash('sha1').update(`blob ${bytes.length}\0`).update(bytes).digest('hex')
    const imageSha = blobSha(image)
    let id, browser
    // Mock only GitHub transport. Exercise the current sync service, image store,
    // database and public asset endpoints rather than the retired preview API.
    globalThis.$fetch = async url => {
      const prefix = 'https://api.github.com/repos/fixture/covers/'
      assert.ok(url.startsWith(prefix), `unexpected GitHub request: ${url}`)
      const endpoint = url.slice(prefix.length)
      if (endpoint === 'commits/main') return { sha: 'a'.repeat(40) }
      if (endpoint === `git/commits/${'a'.repeat(40)}`) return { tree: { sha: 'root' } }
      if (endpoint === 'git/trees/root')
        return { tree: [{ path: 'courses', type: 'tree', sha: 'courses' }] }
      if (endpoint === 'git/trees/courses?recursive=1')
        return {
          tree: [
            {
              path: filename,
              type: 'blob',
              mode: '100644',
              sha: blobSha(markdown),
              size: markdown.length
            },
            {
              path: 'images/cover.png',
              type: 'blob',
              mode: '100644',
              sha: imageSha,
              size: image.length
            }
          ]
        }
      if (endpoint === `git/blobs/${blobSha(markdown)}`)
        return { encoding: 'base64', content: markdown.toString('base64') }
      if (endpoint === `git/blobs/${imageSha}`)
        return { encoding: 'base64', content: image.toString('base64') }
      throw new Error(`unexpected GitHub endpoint: ${endpoint}`)
    }
    try {
      const result = await sync.syncRepositoryModules(config, '', f.admin.id, {
        sourcePath: filename
      })
      assert.equal(result.synced.length, 1)
      assert.equal(result.synced[0].action, 'created')
      assert.equal(result.synced[0].downloadedImages, 1)
      id = result.synced[0].moduleId
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
      assert.deepEqual(Buffer.from(await asset.arrayBuffer()), image)
      const cards = (await f.request('/api/modules')).data
      assert.equal(cards.find(m => m.id === id).durationMinutes, 20)
      await f.createModule('publish', { title: 'Beginner Module', difficulty: 'beginner' })
      browser = await chromium.launch(chromiumOptions())
      const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(f.base + '/learn')
      const card = page
        .locator('.card')
        .filter({ has: page.locator('.card-title', { hasText: title }) })
      const img = card.locator('img.module-cover')
      await img.waitFor()
      await page.waitForFunction(() => {
        const img = document.querySelector('.card img.module-cover')
        return img?.complete && img.naturalWidth > 0
      })
      assert.ok((await page.locator('.feature-banner').boundingBox()).height < 190)
      await chooseOption(page, page.getByRole('combobox', { name: '筛选难度' }), '高阶实战')
      assert.equal(await page.locator('.card').count(), 1)
      await chooseOption(page, page.getByRole('combobox', { name: '模块排序' }), '最近更新')
      await chooseOption(page, page.getByRole('combobox', { name: '筛选难度' }), '全部难度')
      assert.equal(await page.locator('.card').count(), 2)
      await page.getByRole('searchbox', { name: '搜索模块' }).fill(title)
      assert.equal(await page.locator('.card').count(), 1)
      await img.evaluate(el => el.dispatchEvent(new Event('error')))
      await img.waitFor({ state: 'detached' })
      assert.equal(await card.locator('.card-art').count(), 1)
      const learner = await f.account()
      await page
        .context()
        .addCookies([{ name: 'attackbox_session', value: learner.token, url: f.base }])
      await page.goto(`${f.base}/learn/modules/${detail.module.slug}/${detail.chapters[0].slug}`)
      const answer = page.locator('.answer-input')
      const check = page.getByRole('button', { name: '校验答案', exact: true })
      await answer.waitFor()
      assert.equal((await answer.boundingBox()).height, (await check.boundingBox()).height)
      await answer.fill('incorrect')
      await check.click()
      await page.locator('.answer-feedback.error').waitFor()
      assert.equal(await answer.getAttribute('aria-invalid'), 'true')
      await answer.fill('hello')
      assert.equal(await answer.getAttribute('aria-invalid'), 'false')
      await check.click()
      await page.locator('.answer-feedback.success').waitFor()
      assert.deepEqual(errors, [])
      markdown = source(false)
      const next = await sync.syncRepositoryModules(config, '', f.admin.id, {
        sourcePath: filename
      })
      assert.equal(next.synced[0].moduleId, id)
      assert.equal(next.synced[0].action, 'updated')
      assert.equal(
        (await f.request(`/api/admin/modules/${id}`, { actor: f.admin })).data.module.coverAssetId,
        null
      )
      assert.equal((await fetch(f.base + href)).status, 404)
    } finally {
      globalThis.$fetch = previousFetch
      await browser?.close()
      await f.sql`delete from audit_logs where actor_user_id=${f.admin.id}`
      if (id) await f.sql`delete from modules where id=${id}`
      await f.sql`delete from assets where uploaded_by=${f.admin.id}`
      await f.cleanup()
      await database.client.end()
    }
  }
)
