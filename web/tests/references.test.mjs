import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import { unlink } from 'node:fs/promises'
import pathModule from 'node:path'
import { createJiti } from 'jiti'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'

test(
  'references: private Git import, preview/publish, local images, SSR, permissions and version conflicts',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL, timeout: 120000 },
  async () => {
    process.env.DATABASE_URL = process.env.CONTENT_TEST_DATABASE_URL
    const f = await learningFixture(),
      jiti = createJiti(import.meta.url)
    const service = await jiti.import('../server/services/references.ts'),
      repository = await jiti.import('../server/services/content-repository.ts'),
      database = await jiti.import('../server/db/index.ts')
    const oldFetch = globalThis.$fetch,
      token = 'fake-private-token-' + crypto.randomUUID(),
      calls = []
    let browser,
      id,
      version = 0,
      source =
        '---\nsummary: 用 Markdown 管理参考资料\n---\n\n## 开始\n\n公开资料正文。 ![示意图](images/test.png)\n\n## 深入\n\n' +
        '阅读与思考。'.repeat(200) +
        '\n\n[外部资料](https://example.org)\n\n<script data-reference-xss>bad()</script>',
      commit = 'c'.repeat(40),
      missing = false
    let image = Buffer.from(
      'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=',
      'base64'
    )
    const sha = value => crypto.createHash('sha1').update(value).digest('hex')
    const root = 'a'.repeat(40),
      subtree = 'b'.repeat(40)
    globalThis.$fetch = async (url, options) => {
      const u = new URL(url)
      assert.equal(u.hostname, 'api.github.com')
      calls.push({ path: u.pathname, auth: options.headers.Authorization })
      const suffix = u.pathname.split('/').slice(4).join('/')
      if (suffix === 'commits/main') return { sha: commit }
      if (suffix === `git/commits/${commit}`) return { tree: { sha: root } }
      if (suffix === `git/trees/${root}`)
        return { tree: [{ path: 'references', type: 'tree', mode: '040000', sha: subtree }] }
      if (suffix === `git/trees/${subtree}`)
        return {
          tree: [
            ...(!missing
              ? [
                  {
                    path: '指南.md',
                    type: 'blob',
                    mode: '100644',
                    sha: sha(source),
                    size: Buffer.byteLength(source)
                  }
                ]
              : []),
            { path: 'README.md', type: 'blob', mode: '100644', sha: 'e'.repeat(40), size: 10 },
            {
              path: 'images/test.png',
              type: 'blob',
              mode: '100644',
              sha: sha(image),
              size: image.length
            }
          ]
        }
      if (suffix === `git/blobs/${sha(source)}`)
        return { encoding: 'base64', content: Buffer.from(source).toString('base64') }
      if (suffix === `git/blobs/${sha(image)}`)
        return { encoding: 'base64', content: image.toString('base64') }
      throw Error('Unexpected mocked GitHub path: ' + suffix)
    }
    try {
      const learner = await f.account(),
        settingsPath = '/api/admin/references/repository'
      for (const path of ['/api/admin/references', settingsPath]) {
        await f.request(path, { status: 401 })
        await f.request(path, { actor: learner, status: 403 })
      }
      for (const path of ['/api/admin/references/scan', '/api/admin/references/import']) {
        await f.request(path, { method: 'POST', body: {}, status: 401 })
        await f.request(path, { actor: learner, method: 'POST', body: {}, status: 403 })
      }
      assert.equal((await f.request('/api/references')).data.total, 0)
      await f.request('/api/references?page=-1', { status: 400 })
      await f.request(settingsPath, {
        actor: f.admin,
        method: 'PUT',
        body: {
          repository: 'owner/private-references',
          branch: 'main',
          directory: 'references',
          token,
          version: 0
        }
      })
      let settings = (await f.request(settingsPath, { actor: f.admin })).data
      assert.equal(settings.version, 1)
      assert.equal(settings.hasToken, true)
      assert.ok(!JSON.stringify(settings).includes(token))
      assert.notEqual(
        (await f.sql`select token_cipher from content_repository where id=2`)[0].token_cipher,
        token
      )
      await repository.saveContentRepository({
        repository: 'owner/private-references',
        branch: 'main',
        directory: 'learning',
        token,
        version: 0
      })
      await assert.rejects(
        () =>
          repository.saveContentRepository(
            { repository: 'owner/private-references', directory: '', branch: 'main', version: 1 },
            2
          ),
        e => e.statusCode === 400
      )
      await assert.rejects(
        () =>
          repository.saveContentRepository({
            repository: 'owner/private-references',
            directory: '',
            branch: 'main',
            version: 1
          }),
        e => e.statusCode === 400
      )
      let scan = await service.scanReferences()
      assert.equal(scan.items.length, 1)
      assert.equal(scan.items[0].state, 'new')
      const importBody = () => ({
        sourcePath: '指南.md',
        lockVersion: version,
        commit: scan.commit,
        version: scan.version
      })
      browser = await chromium.launch({
        executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
        args: ['--no-sandbox']
      })
      const admin = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await admin.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
      const editor = await admin.newPage(),
        errors = []
      editor.on('pageerror', e => errors.push(e.message))
      await editor.route('**/api/admin/references/scan', async route =>
        route.fulfill({ json: { data: await service.scanReferences() } })
      )
      await editor.route('**/api/admin/references/import', async route => {
        const result = await service.importReference(f.admin.id, route.request().postDataJSON())
        id = result.id
        await route.fulfill({ json: { data: result } })
      })
      await editor.goto(f.base + '/admin/references')
      await editor.getByRole('button', { name: '扫描仓库', exact: true }).click()
      await editor.getByRole('button', { name: '导入', exact: true }).click()
      await editor.getByRole('link', { name: '管理', exact: true }).waitFor()
      assert.equal(new URL(editor.url()).pathname, '/admin/references')
      await editor.getByRole('link', { name: '管理', exact: true }).click()
      await editor.waitForURL('**/admin/references/*')
      await editor.getByRole('button', { name: '发布资料', exact: true }).waitFor()
      let detail = (await f.request('/api/admin/references/' + id, { actor: f.admin })).data
      version = detail.lockVersion
      assert.equal(detail.status, 'unpublished')
      assert.equal(detail.title, '指南')
      const path = '/references/' + detail.slug,
        publicApi = '/api' + path
      await f.request(publicApi, { status: 404 })
      await f.request('/api/admin/references/' + id, { status: 401 })
      await f.request('/api/admin/references/' + id, { actor: learner, status: 403 })
      assert.equal('bodyMarkdown' in detail, false)
      const [{ asset_id: assetId }] =
        await f.sql`select asset_id from reference_images where reference_id=${id}`
      const adminImage = `/api/admin/references/${id}/assets/${assetId}`
      await f.request(adminImage, { status: 401 })
      await f.request(adminImage, { actor: learner, status: 403 })
      assert.equal(
        (
          await fetch(f.base + adminImage, {
            headers: { cookie: `attackbox_session=${f.admin.token}` }
          })
        ).status,
        200
      )
      assert.equal((await fetch(f.base + publicApi + '/assets/' + assetId)).status, 404)
      const csrf = await fetch(f.base + '/api/admin/references/' + id, {
        method: 'PUT',
        headers: {
          cookie: `attackbox_session=${f.admin.token}`,
          origin: 'https://other.invalid',
          'content-type': 'application/json'
        },
        body: JSON.stringify({ intent: 'publish', lockVersion: version })
      })
      assert.equal(csrf.status, 403)
      await editor.screenshot({
        path: '/tmp/attackbox-reference-admin-preview.png',
        animations: 'disabled'
      })
      await editor.getByRole('button', { name: '发布资料', exact: true }).click()
      await editor.getByRole('button', { name: '下架', exact: true }).waitFor()
      detail = (await f.request('/api/admin/references/' + id, { actor: f.admin })).data
      version = detail.lockVersion
      await f.request('/api/admin/references/' + id, {
        actor: f.admin,
        method: 'PUT',
        body: { intent: 'unpublish', lockVersion: version - 1 },
        status: 409
      })
      // Recover a lost public pointer without re-parsing Markdown, then reconcile
      // an interrupted withdrawal from the authoritative database state.
      const beforeRecovery = (await f.request(publicApi)).data
      await unlink(pathModule.join(process.env.REFERENCE_PUBLIC_DIR, beforeRecovery.meta.slug))
      await service.reconcileReferencePublications()
      assert.equal((await f.request(publicApi)).data.meta.id, id)
      await f.sql`update reference_documents set status='unpublished' where id=${id}`
      await service.reconcileReferencePublications()
      await f.request(publicApi, { status: 404 })
      await f.sql`update reference_documents set status='published' where id=${id}`
      await service.reconcileReferencePublications()
      const publicDoc = (await f.request(publicApi)).data
      for (const value of [
        token,
        'sourceOrigin',
        'sourcePath',
        'sourceCommit',
        'manifest',
        'lockVersion',
        'owner/private-references'
      ])
        assert.ok(!JSON.stringify(publicDoc).includes(value))
      assert.equal((await f.request('/api/references')).data.items[0].id, id)
      const htmlResponse = await fetch(f.base + path),
        html = await htmlResponse.text()
      assert.equal(htmlResponse.status, 200)
      assert.ok(html.includes('公开资料正文'))
      assert.ok(html.includes('<h2 id="开始">'))
      assert.ok(!html.includes('owner/private-references'))
      assert.ok(!html.includes(token))
      const imageUrl = `/api/references/${publicDoc.meta.slug}/assets/${assetId}`,
        asset = await fetch(f.base + imageUrl)
      assert.equal(asset.status, 200)
      assert.equal(asset.headers.get('x-content-type-options'), 'nosniff')
      assert.equal(asset.headers.get('content-type'), 'image/png')
      assert.deepEqual(Buffer.from(await asset.arrayBuffer()), image)
      const guest = await browser.newContext({ viewport: { width: 1440, height: 1000 } }),
        page = await guest.newPage()
      page.on('pageerror', e => errors.push(e.message))
      await page.goto(f.base + '/references')
      await page
        .getByRole('link', { name: '指南 用 Markdown 管理参考资料 阅读', exact: true })
        .click()
      await page.waitForURL('**' + path)
      assert.equal(await page.locator('script[data-reference-xss]').count(), 0)
      assert.equal(await page.getByRole('button', { name: '启动环境', exact: true }).count(), 0)
      assert.equal(await page.locator('.quiz-card').count(), 0)
      await page
        .getByRole('navigation', { name: '本篇目录' })
        .getByRole('link', { name: '深入', exact: true })
        .click()
      await page.waitForFunction(() => decodeURIComponent(location.hash) === '#深入')
      assert.equal(await page.evaluate(() => window.scrollY), 0)
      await page.screenshot({ path: '/tmp/attackbox-reference-reader.png', animations: 'disabled' })
      await page.evaluate(() => (document.documentElement.dataset.theme = 'midnight'))
      await page.screenshot({
        path: '/tmp/attackbox-reference-reader-midnight.png',
        animations: 'disabled'
      })
      await page.setViewportSize({ width: 390, height: 844 })
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      // Ordinary tutorial Markdown links point to the same public document and section.
      const module = await f.createModule('publish', {
        chapters: [
          {
            title: '引用资料',
            slug: 'reference',
            access: 'public',
            bodyMarkdown: `参考：[指南](${path}#${encodeURIComponent('深入')})`,
            questions: []
          }
        ]
      })
      await page.goto(`${f.base}/learn/modules/${module.module.slug}/reference`)
      await page
        .locator('.reader-article .markdown-body')
        .getByRole('link', { name: '指南', exact: true })
        .click()
      await page.waitForURL('**' + path + '#*')
      await page.getByRole('heading', { name: '指南', exact: true }).waitFor()
      scan = await service.scanReferences()
      assert.equal(scan.items[0].state, 'unchanged')
      const unchanged = await service.importReference(f.admin.id, importBody())
      assert.equal(unchanged.unchanged, true)
      assert.equal((await f.request(publicApi)).data.meta.id, id)
      image = Buffer.concat([image, Buffer.from([0])])
      commit = 'd'.repeat(40)
      scan = await service.scanReferences()
      assert.equal(scan.items[0].state, 'changed')
      await service.importReference(f.admin.id, importBody())
      detail = (await f.request('/api/admin/references/' + id, { actor: f.admin })).data
      assert.equal(detail.slug, publicDoc.meta.slug)
      assert.equal(detail.status, 'unpublished')
      assert.equal(detail.lockVersion, version + 1)
      await f.request(publicApi, { status: 404 })
      assert.equal((await fetch(f.base + imageUrl)).status, 404)
      version = detail.lockVersion
      await assert.rejects(
        () => service.importReference(f.admin.id, { ...importBody(), lockVersion: version - 1 }),
        e => e.statusCode === 409
      )
      const stale = scan.commit
      commit = 'f'.repeat(40)
      await assert.rejects(
        () => service.importReference(f.admin.id, { ...importBody(), commit: stale }),
        e => e.statusCode === 409
      )
      scan = await service.scanReferences()
      await f.request('/api/admin/references/' + id, {
        actor: f.admin,
        method: 'PUT',
        body: { intent: 'publish', lockVersion: version }
      })
      version++
      source = '---\nsummary: updated\n---\n\n## 新内容\n\n资料更新'
      scan = await service.scanReferences()
      const imports = await Promise.allSettled([
        service.importReference(f.admin.id, importBody()),
        service.importReference(f.admin.id, importBody())
      ])
      assert.equal(imports.filter(r => r.status === 'fulfilled').length, 1)
      assert.equal(imports.find(r => r.status === 'rejected').reason.statusCode, 409)
      detail = (await f.request('/api/admin/references/' + id, { actor: f.admin })).data
      version = detail.lockVersion
      missing = true
      scan = await service.scanReferences()
      assert.equal(scan.items[0].state, 'missing')
      assert.equal(
        (await f.request('/api/admin/references/' + id, { actor: f.admin })).data.title,
        '指南'
      )
      await f.request('/api/admin/references/' + id, {
        actor: f.admin,
        method: 'DELETE',
        body: { lockVersion: version - 1 },
        status: 409
      })
      await f.request('/api/admin/references/' + id, {
        actor: f.admin,
        method: 'DELETE',
        body: { lockVersion: version }
      })
      await f.request(publicApi, { status: 404 })
      assert.equal(
        (await fetch(f.base + path)).status,
        404,
        'unpublished/deleted SSR routes use HTTP 404'
      )
      assert.ok(calls.every(call => call.auth === `Bearer ${token}`))
      await f.request(settingsPath, {
        actor: f.admin,
        method: 'PUT',
        body: { ...settings, clearToken: true }
      })
      calls.length = 0
      await service.scanReferences()
      assert.ok(
        calls.every(call => call.auth === undefined),
        'public repos do not send an empty Bearer header'
      )
      assert.deepEqual(errors, [])
    } finally {
      globalThis.$fetch = oldFetch
      await browser?.close()
      await f.sql`delete from reference_documents where created_by=${f.admin.id}`
      await f.sql`delete from assets where uploaded_by=${f.admin.id}`
      await f.sql`delete from content_repository where id in (1,2)`
      await f.cleanup()
      await database.client.end()
    }
  }
)
