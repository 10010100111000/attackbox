import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import { createJiti } from 'jiti'
import { chromium } from 'playwright'
import { chromiumOptions } from './helpers/browser.mjs'
import { learningFixture } from './helpers/learning-fixture.mjs'

test(
  'reference management imports changes without navigation, preserves per-file failures, previews private compiled chunks and publishes a selection',
  {
    skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.CONTENT_TEST_BASE_URL,
    timeout: 90000
  },
  async () => {
    process.env.DATABASE_URL = process.env.CONTENT_TEST_DATABASE_URL
    const f = await learningFixture(),
      jiti = createJiti(import.meta.url)
    const service = await jiti.import('../server/services/references.ts'),
      database = await jiti.import('../server/db/index.ts')
    const previousFetch = globalThis.$fetch
    const paragraph = Array(20).fill('Read and understand. '.repeat(30)).join('\n\n')
    const docs = new Map([
      [
        'Alpha.md',
        `# Chapter A\n\n![Illustration](images/test.png)\n\nFIRST-MARKER\n\n${paragraph}\n\n# Chapter B\n\n${paragraph}\n\n# Chapter C\n\nLAST-MARKER\n\n${paragraph}`
      ],
      ['Broken.md', '---\nsummary: [invalid]\n---\n\n# Broken'],
      ['Omega.md', '# Omega chapter\n\nAnother reference.']
    ])
    const image = Buffer.from(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=',
      'base64'
    )
    const sha = value => crypto.createHash('sha1').update(value).digest('hex')
    let commit = 'c'.repeat(40),
      browser
    globalThis.$fetch = async url => {
      const endpoint = new URL(url).pathname.split('/').slice(4).join('/')
      if (endpoint === 'commits/main') return { sha: commit }
      if (endpoint === `git/commits/${commit}`) return { tree: { sha: 'a'.repeat(40) } }
      if (endpoint === `git/trees/${'a'.repeat(40)}`)
        return { tree: [{ path: 'references', type: 'tree', sha: 'b'.repeat(40) }] }
      if (endpoint === `git/trees/${'b'.repeat(40)}`)
        return {
          tree: [...docs]
            .map(([path, content]) => ({
              path,
              type: 'blob',
              mode: '100644',
              sha: sha(content),
              size: Buffer.byteLength(content)
            }))
            .concat([
              {
                path: 'images/test.png',
                type: 'blob',
                mode: '100644',
                sha: sha(image),
                size: image.length
              }
            ])
        }
      if (endpoint === `git/blobs/${sha(image)}`)
        return { encoding: 'base64', content: image.toString('base64') }
      for (const content of docs.values())
        if (endpoint === `git/blobs/${sha(content)}`)
          return { encoding: 'base64', content: Buffer.from(content).toString('base64') }
      throw Error('Unexpected Git fixture endpoint: ' + endpoint)
    }
    try {
      const learner = await f.account()
      await f.request('/api/admin/references/repository', {
        actor: f.admin,
        method: 'PUT',
        body: {
          repository: 'owner/admin-reference-fixture',
          branch: 'main',
          directory: 'references',
          token: '',
          version: 0
        }
      })
      browser = await chromium.launch(chromiumOptions())
      const context = await browser.newContext({ viewport: { width: 1600, height: 1000 } })
      await context.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
      const page = await context.newPage(),
        errors = []
      page.setDefaultTimeout(8000)
      page.on('pageerror', error => errors.push(error.message))
      await page.route('**/api/admin/references/scan', async route =>
        route.fulfill({ json: { data: await service.scanReferences() } })
      )
      await page.route('**/api/admin/references/import', async route => {
        try {
          await route.fulfill({
            json: {
              data: await service.importReference(f.admin.id, route.request().postDataJSON())
            }
          })
        } catch (error) {
          await route.fulfill({ status: error.statusCode || 500, json: { message: error.message } })
        }
      })
      await page.goto(f.base + '/admin/references')
      await page.getByRole('button', { name: '扫描仓库', exact: true }).click()
      await page.getByRole('button', { name: '导入全部变更（3）', exact: true }).click()
      const progress = page.getByLabel('批量处理进度')
      await page.getByText('导入完成', { exact: true }).waitFor()
      assert.match(await progress.textContent(), /成功\s*2/)
      assert.match(await progress.textContent(), /失败\s*1/)
      assert.equal(new URL(page.url()).pathname, '/admin/references')
      assert.equal(
        (await f.sql`select id from reference_documents where created_by=${f.admin.id}`).length,
        2
      )
      const [{ id, slug }] =
        await f.sql`select id,slug from reference_documents where title='Alpha' and created_by=${f.admin.id}`
      await f.request(`/api/admin/references/${id}/preview`, { status: 401 })
      await f.request(`/api/admin/references/${id}/preview`, { actor: learner, status: 403 })
      const preview = (await f.request(`/api/admin/references/${id}/preview`, { actor: f.admin }))
        .data
      assert.ok(!JSON.stringify(preview).includes('bodyMarkdown'))
      assert.equal(preview.meta.chunks.length, 3)
      assert.ok(preview.html.includes('FIRST-MARKER') && !preview.html.includes('LAST-MARKER'))
      const chunk = preview.meta.chunks[2].url
      await f.request(chunk, { status: 401 })
      await f.request(chunk, { actor: learner, status: 403 })
      const response = await fetch(f.base + chunk, {
        headers: { cookie: `attackbox_session=${f.admin.token}` }
      })
      assert.equal(response.status, 200)
      assert.ok((await response.text()).includes('LAST-MARKER'))
      assert.match(response.headers.get('cache-control'), /no-store/)
      assert.equal((await fetch(f.base + `/static-refs/${slug}/meta.json`)).status, 404)
      const [{ asset_id: assetId }] =
        await f.sql`select asset_id from reference_images where reference_id=${id}`
      const imageUrl = `/api/references/${slug}/assets/${assetId}`
      await f.request(imageUrl, { actor: learner, status: 404 })
      assert.equal(
        (
          await fetch(f.base + imageUrl, {
            headers: { cookie: `attackbox_session=${f.admin.token}` }
          })
        ).status,
        200
      )
      const popupPromise = page.waitForEvent('popup')
      await page
        .locator('tr')
        .filter({ hasText: 'Alpha.md' })
        .first()
        .getByRole('link', { name: '预览', exact: true })
        .click()
      const popup = await popupPromise
      popup.on('pageerror', error => errors.push(error.message))
      await popup.getByText('管理员预览', { exact: true }).waitFor()
      const markdown = popup.locator('.reader-article > .markdown-body')
      await markdown.getByText('FIRST-MARKER', { exact: true }).waitFor()
      await popup.waitForFunction(
        () => document.querySelector('.markdown-body img')?.naturalWidth > 0
      )
      await popup
        .getByRole('navigation', { name: '本篇目录' })
        .getByRole('link', { name: 'Chapter C', exact: true })
        .click()
      await markdown.getByText('LAST-MARKER', { exact: true }).waitFor()
      await popup.screenshot({ path: '/tmp/attackbox-reference-admin-reader.png' })
      await popup.close()
      docs.set('Broken.md', '# Fixed chapter\n\nFixed material.')
      commit = 'd'.repeat(40)
      await page.getByRole('button', { name: '扫描仓库', exact: true }).click()
      await page.getByRole('button', { name: '导入全部变更（1）', exact: true }).click()
      await page.getByRole('button', { name: '发布所选（3）', exact: true }).waitFor()
      const published =
        await f.sql`select id from reference_documents where created_by=${f.admin.id} and status='published'`
      assert.equal(published.length, 0, 'import does not publish')
      await page.getByRole('button', { name: '发布所选（3）', exact: true }).click()
      await page
        .getByRole('alertdialog')
        .getByRole('button', { name: '发布所选资料', exact: true })
        .click()
      await page.getByRole('button', { name: '发布所选（0）', exact: true }).waitFor()
      assert.equal(
        (
          await f.sql`select id from reference_documents where created_by=${f.admin.id} and status='published'`
        ).length,
        3
      )
      await f.request(`/api/references/${slug}`)
      await page.screenshot({ path: '/tmp/attackbox-reference-admin-batch.png', fullPage: true })
      assert.deepEqual(errors, [])
    } finally {
      globalThis.$fetch = previousFetch
      await browser?.close()
      await f.sql`delete from content_repository where id=2`
      await f.cleanup()
      await database.client.end()
    }
  }
)
