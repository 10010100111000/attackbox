import test from 'node:test'
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { chromium } from 'playwright'
import { chromiumOptions } from './helpers/browser.mjs'
import { learningFixture } from './helpers/learning-fixture.mjs'

test(
  'static reference chunks hydrate once, scroll on demand, jump directly, retry, restore anchors and withdraw with publication',
  {
    skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.CONTENT_TEST_BASE_URL,
    timeout: 60000
  },
  async () => {
    const f = await learningFixture(),
      id = randomUUID(),
      path = `/references/${id}`
    let browser
    try {
      const body = Array.from(
        { length: 60 },
        (_, i) =>
          `# Chapter ${i + 1}\n\nBODY-MARKER-${i + 1}-END\n\n${Array(20).fill('Learning material. '.repeat(30)).join('\n\n')}`
      ).join('\n\n')
      await f.sql`insert into reference_documents(id,slug,title,body_markdown,status,source_origin,source_path,source_commit,manifest,created_by,published_at) values(${id},${id},'Long reference',${body},'unpublished',${id},'long.md','fixture','{}',${f.admin.id},now())`
      await f.request('/api' + path, { status: 404 })
      const meta = await f.publishReference(id)
      const first = (await f.request('/api' + path)).data
      assert.equal(first.meta.chunks.length, 60)
      assert.equal(first.meta.outline.length, 60)
      assert.equal('bodyMarkdown' in first, false)
      assert.ok(JSON.stringify(first).length < body.length / 10)
      assert.ok(first.html.includes('BODY-MARKER-1-END'))
      assert.ok(!first.html.includes('BODY-MARKER-60-END'))
      const staticMeta = await (await fetch(f.base + `/static-refs/${id}/meta.json`)).json()
      assert.equal('html' in staticMeta, false)
      assert.equal(staticMeta.revision, meta.revision)
      const last = await fetch(f.base + meta.chunks[59].url)
      assert.equal(last.status, 200)
      assert.match(last.headers.get('content-type'), /text\/html/)
      assert.ok((await last.text()).includes('BODY-MARKER-60-END'))
      const ssr = await (await fetch(f.base + path)).text()
      assert.ok(ssr.includes('BODY-MARKER-1-END') && !ssr.includes('BODY-MARKER-60-END'))
      browser = await chromium.launch(chromiumOptions())
      const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
      page.setDefaultTimeout(8000)
      const errors = [],
        requests = []
      page.on('pageerror', error => errors.push(error.message))
      page.on('request', request => {
        if (new URL(request.url()).pathname.startsWith('/static-refs/'))
          requests.push(new URL(request.url()).pathname)
      })
      await page.goto(f.base + path)
      const markdown = page.locator('.reader-article > .markdown-body')
      await markdown.getByText('BODY-MARKER-1-END', { exact: true }).waitFor()
      assert.equal(requests.length, 0, 'hydration does not re-fetch static content')
      const scroll = page.getByLabel('资料正文', { exact: true })
      await scroll.evaluate(el => {
        el.scrollTop = el.scrollHeight
      })
      await markdown.getByText('BODY-MARKER-2-END', { exact: true }).waitFor()
      assert.deepEqual(requests, [meta.chunks[1].url])
      assert.equal(await markdown.locator('.reference-chunk').count(), 2)
      const nav = page.getByRole('navigation', { name: '本篇目录' })
      await nav.getByRole('link', { name: 'Chapter 60', exact: true }).click()
      await markdown.getByText('BODY-MARKER-60-END', { exact: true }).waitFor()
      assert.deepEqual(
        requests,
        [meta.chunks[1].url, meta.chunks[59].url],
        'distant jumps do not download the skipped book'
      )
      assert.equal(await markdown.locator('.reference-chunk').count(), 1)
      assert.equal(new URL(page.url()).search, '', 'there is no pagination route')
      await page.route('**/chunk-29.html', route =>
        route.fulfill({ status: 503, body: 'fixture unavailable' })
      )
      await nav.getByRole('link', { name: 'Chapter 30', exact: true }).click()
      await page.getByRole('button', { name: '重试', exact: true }).waitFor()
      assert.ok(await markdown.getByText('BODY-MARKER-60-END', { exact: true }).isVisible())
      await page.unroute('**/chunk-29.html')
      await page.getByRole('button', { name: '重试', exact: true }).click()
      await markdown.getByText('BODY-MARKER-30-END', { exact: true }).waitFor()
      await nav.getByRole('link', { name: 'Chapter 60', exact: true }).click()
      await page.reload()
      await markdown.getByText('BODY-MARKER-60-END', { exact: true }).waitFor()
      await f.sql`update reference_documents set body_markdown=${'# Updated edition\n\nUPDATED-BODY'} where id=${id}`
      const replacement = await f.publishReference(id)
      await nav.getByRole('link', { name: 'Chapter 50', exact: true }).click()
      await page.getByRole('button', { name: '重新打开', exact: true }).waitFor()
      await page.getByRole('button', { name: '重新打开', exact: true }).click()
      await markdown.getByText('UPDATED-BODY', { exact: true }).waitFor()
      assert.notEqual(replacement.revision, meta.revision)
      await page.screenshot({ path: '/tmp/attackbox-static-reference.png' })
      assert.deepEqual(errors, [])
      const [row] = await f.sql`select lock_version from reference_documents where id=${id}`
      await f.request(`/api/admin/references/${id}`, {
        actor: f.admin,
        method: 'PUT',
        body: { intent: 'unpublish', lockVersion: row.lock_version }
      })
      await f.request('/api' + path, { status: 404 })
      assert.equal((await fetch(f.base + meta.chunks[59].url)).status, 404)
      assert.equal((await fetch(f.base + replacement.chunks[0].url)).status, 404)
      assert.equal((await fetch(f.base + `/static-refs/${id}/meta.json`)).status, 404)
    } finally {
      await browser?.close()
      await f.cleanup()
    }
  }
)
