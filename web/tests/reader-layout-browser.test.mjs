import test from 'node:test'
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { chromium } from 'playwright'
import { chromiumOptions } from './helpers/browser.mjs'
import { learningFixture } from './helpers/learning-fixture.mjs'

const enabled = !!process.env.CONTENT_TEST_DATABASE_URL && !!process.env.CONTENT_TEST_BASE_URL
const paragraph = '阅读资料、理解上下文，再亲手验证结论。'.repeat(35)
const body =
  `# 第一部分\n\n${paragraph}\n\n## 起点\n\n${paragraph}\n\n### 安装\n\n${paragraph}\n\n#### 高级设置\n\n${paragraph}\n\n## 验证\n\n${paragraph}\n\n# 第二部分\n\n## 验证\n\n${paragraph}\n\n\`\`\`bash\n${'command-with-a-long-argument-'.repeat(30)}\n\`\`\`\n\n` +
  Array(8).fill(paragraph).join('\n\n') +
  '\n\n## 附记\n\nEnd of document.'
async function seedReference(f, source = body) {
  const id = randomUUID(),
    slug = `reader-${id.slice(0, 8)}`,
    title = '公开阅读资料'
  await f.sql`insert into reference_documents (id,slug,title,body_markdown,status,source_origin,source_path,source_commit,manifest,created_by,published_at) values(${id},${slug},${title},${source},'published',${id},'reader.md','fixture','{}',${f.admin.id},now())`
  await f.publishReference(id)
  return { id, path: `/references/${slug}`, title }
}

test(
  'GitHub typography applies to runtime and precompiled readers independently of the OS theme',
  { skip: !enabled, timeout: 60000 },
  async () => {
    const f = await learningFixture()
    const source =
      '## DNSSEC\n\n正文包含 **重点**、[参考链接](https://example.org) 和 `DNSKEY`。\n\n' +
      '| 记录类型 | 描述 |\n| :--- | ---: |\n| RRSIG | 存储单个资源记录的数字签名 |\n| DNSKEY | 发布公钥 |\n| DS | 标识签名密钥 |\n| NSEC3 | 提供经过身份验证的否定响应 |\n\n' +
      '- 先理解\n  - 再验证\n\n1. 阅读\n2. 实践\n\n' +
      '> 普通引用\n\n> [!NOTE]\n> 这里的 **强调** 应当保持行内显示。\n\n' +
      '### 命令示例\n\n```javascript\nconst record = "DNSKEY"\nconsole.log(record)\n```\n\n' +
      `| 命令 |\n| --- |\n| \`${'long-command-argument-'.repeat(40)}\` |`
    let reference, browser
    try {
      reference = await seedReference(f, source)
      const mod = await f.createModule('publish', {
        chapters: [
          {
            title: '排版验收',
            slug: 'typography',
            access: 'public',
            bodyMarkdown: source,
            questions: []
          }
        ]
      })
      browser = await chromium.launch(chromiumOptions())
      for (const theme of ['graphite', 'midnight']) {
        const context = await browser.newContext({
          viewport: { width: 1440, height: 1000 },
          colorScheme: theme === 'graphite' ? 'dark' : 'light',
          permissions: ['clipboard-read', 'clipboard-write']
        })
        await context.addInitScript(
          theme => localStorage.setItem('attackbox-appearance', theme),
          theme
        )
        const page = await context.newPage(),
          errors = []
        page.on('pageerror', error => errors.push(error.message))
        page.setDefaultTimeout(8000)
        for (const path of [reference.path, `/learn/modules/${mod.module.slug}/typography`]) {
          await page.goto(f.base + path)
          await page.waitForFunction(() => !!document.getElementById('__nuxt')?.__vue_app__)
          const markdown = page.locator('.reader-article > .markdown-body')
          await markdown.locator('table').last().scrollIntoViewIfNeeded()
          const styles = await markdown.evaluate(el => {
            const style = selector => getComputedStyle(el.querySelector(selector))
            const tables = el.querySelectorAll('table')
            const table = tables[tables.length - 1]
            return {
              color: getComputedStyle(el).color,
              background: getComputedStyle(el).backgroundColor,
              scheme: getComputedStyle(el).colorScheme,
              fontSize: getComputedStyle(el).fontSize,
              cellBorder: style('td').borderTopWidth,
              cellPadding: style('td').paddingLeft,
              right: style('tbody td:nth-child(2)').textAlign,
              bullet: style('ul').listStyleType,
              number: style('ol').listStyleType,
              inlineBold: style('.callout p strong').display,
              label: style('.callout > strong').display,
              keyword: style('.hljs-keyword').color,
              code: style('pre code').color,
              tableScroll: table.scrollWidth > table.clientWidth,
              fits: table.getBoundingClientRect().right <= el.getBoundingClientRect().right + 1,
              pageFits: document.documentElement.scrollWidth <= innerWidth
            }
          })
          assert.equal(
            styles.color,
            theme === 'graphite' ? 'rgb(39, 38, 43)' : 'rgb(220, 224, 232)'
          )
          assert.equal(styles.background, 'rgba(0, 0, 0, 0)')
          assert.equal(styles.scheme, theme === 'graphite' ? 'light' : 'dark')
          assert.equal(styles.fontSize, '17px')
          assert.equal(styles.cellBorder, '1px')
          assert.equal(styles.cellPadding, '13px', 'table spacing comes from the GitHub stylesheet')
          assert.equal(styles.right, 'right')
          assert.equal(styles.bullet, 'disc')
          assert.equal(styles.number, 'decimal')
          assert.equal(styles.inlineBold, 'inline')
          assert.equal(styles.label, 'block')
          assert.notEqual(styles.keyword, styles.code, 'highlight.js tokens have a visible theme')
          assert.ok(styles.tableScroll && styles.fits && styles.pageFits)
          await markdown.locator('.copy-code-btn').click()
          assert.match(await page.evaluate(() => navigator.clipboard.readText()), /const record/)
          await page.setViewportSize({ width: 390, height: 844 })
          await markdown.locator('table').last().scrollIntoViewIfNeeded()
          assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
          await page.setViewportSize({ width: 1440, height: 1000 })
          if (path === reference.path) {
            await page.getByLabel('资料正文', { exact: true }).evaluate(el => {
              el.scrollTop = 0
            })
            await page.screenshot({ path: `/tmp/attackbox-markdown-${theme}.png` })
          }
        }
        assert.deepEqual(errors, [])
        await context.close()
      }
    } finally {
      await browser?.close()
      if (reference) await f.sql`delete from reference_documents where id=${reference.id}`
      await f.cleanup()
    }
  }
)

test(
  'reference reader uses the available width and supports persistent sidebar collapse, nested headings, anchors and mobile navigation',
  { skip: !enabled, timeout: 60000 },
  async () => {
    const f = await learningFixture()
    let reference, browser
    try {
      reference = await seedReference(f)
      const response = await fetch(f.base + reference.path)
      const html = await response.text()
      assert.equal(response.status, 200)
      assert.ok(html.includes('高级设置'), 'public document content is server rendered')
      browser = await chromium.launch(chromiumOptions())
      const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
      page.setDefaultTimeout(8000)
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(f.base + reference.path)
      const scroll = page.getByLabel('资料正文', { exact: true })
      const nav = page.getByRole('navigation', { name: '本篇目录' })
      await nav.getByRole('link', { name: '高级设置', exact: true }).waitFor()
      let gets = 0
      page.on('request', request => {
        if (
          request.method() === 'GET' &&
          new URL(request.url()).pathname.startsWith('/api/references/')
        )
          gets++
      })
      await nav.getByRole('link', { name: '高级设置', exact: true }).click()
      await page.waitForFunction(() => decodeURIComponent(location.hash) === '#高级设置')
      await page.waitForFunction(
        () =>
          document.querySelector('.reader-sidebar a[aria-current="location"]')?.textContent ===
          '高级设置'
      )
      const beforeFold = await scroll.evaluate(el => el.scrollTop)
      await nav.getByRole('button', { name: '收起起点的子目录', exact: true }).click()
      assert.equal(await nav.getByRole('link', { name: '安装', exact: true }).isVisible(), false)
      assert.equal(
        await nav.getByRole('link', { name: '高级设置', exact: true }).isVisible(),
        false
      )
      assert.equal(await scroll.evaluate(el => el.scrollTop), beforeFold)
      assert.equal(
        await nav
          .getByRole('link', { name: '起点', exact: true })
          .evaluate(el => el.classList.contains('contains-active')),
        true
      )
      await page.getByRole('button', { name: '收起资料目录', exact: true }).click()
      assert.equal(
        await page.evaluate(() => localStorage.getItem('attackbox-reader-outline-collapsed')),
        'true'
      )
      await page.reload()
      await page.getByRole('button', { name: '展开资料目录', exact: true }).waitFor()
      await page.getByRole('button', { name: '展开资料目录', exact: true }).click()
      const afterReload = gets
      for (const width of [1280, 1440, 1920]) {
        await page.setViewportSize({ width, height: 1000 })
        const before = await page.locator('.reader-article > .markdown-body').boundingBox()
        const right = before.x + before.width
        assert.ok(
          before.width >= width - 248 - 120,
          'text uses the reader column without the old 960px ceiling'
        )
        await page.getByRole('button', { name: '收起资料目录', exact: true }).click()
        const after = await page.locator('.reader-article > .markdown-body').boundingBox()
        assert.ok(
          after.width >= before.width + 190,
          'collapsing expands the text instead of shifting a fixed-width article'
        )
        assert.ok(Math.abs(after.x + after.width - right) < 2)
        await page.getByRole('button', { name: '展开资料目录', exact: true }).click()
      }
      await nav.getByRole('link', { name: '第二部分', exact: true }).click()
      await page.waitForFunction(() => decodeURIComponent(location.hash) === '#第二部分')
      await page.locator('h1[id="第二部分"]').waitFor()
      assert.equal(gets, afterReload, 'outline navigation never refetches the database detail API')
      assert.equal(await page.evaluate(() => window.scrollY), 0)
      await nav.getByRole('link', { name: '附记', exact: true }).click()
      await page.waitForFunction(
        () =>
          document.querySelector('.reader-sidebar a[aria-current="location"]')?.textContent ===
          '附记'
      )
      assert.ok(
        (await page.locator('h2[id="附记"]').boundingBox()).y > (await scroll.boundingBox()).y + 24,
        'short final sections cannot align to the scroll viewport top'
      )
      await page.setViewportSize({ width: 390, height: 844 })
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      await page.getByRole('button', { name: '打开资料目录', exact: true }).click()
      const dialog = page.getByRole('dialog', { name: '资料目录' })
      await dialog.getByRole('button', { name: '收起起点的子目录', exact: true }).click()
      assert.equal(await dialog.getByRole('link', { name: '安装', exact: true }).isVisible(), false)
      await dialog.getByRole('button', { name: '展开起点的子目录', exact: true }).click()
      await dialog.getByRole('link', { name: '高级设置', exact: true }).click()
      await dialog.waitFor({ state: 'hidden' })
      await page.waitForFunction(() => decodeURIComponent(location.hash) === '#高级设置')
      assert.equal(await page.evaluate(() => window.scrollY), 0)
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      if (reference) await f.sql`delete from reference_documents where id=${reference.id}`
      await f.cleanup()
    }
  }
)

test(
  'learning and reference readers share text geometry while retaining chapter locks, quizzes and nested section navigation',
  { skip: !enabled, timeout: 60000 },
  async () => {
    const f = await learningFixture()
    let reference, browser
    try {
      reference = await seedReference(f)
      const learner = await f.account()
      const mod = await f.createModule('publish', {
        chapters: [
          {
            title: '阅读与实践',
            slug: 'reader',
            access: 'public',
            bodyMarkdown: body.replace('# 第一部分\n\n', ''),
            questions: [{ promptMarkdown: '## 题目提示小节\n\n输入 hello', answer: 'hello' }]
          },
          {
            title: '后续章节',
            slug: 'locked',
            access: 'authenticated',
            bodyMarkdown: '## 后续内容',
            questions: []
          }
        ]
      })
      browser = await chromium.launch(chromiumOptions())
      const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } })
      await context.addCookies([{ name: 'attackbox_session', value: learner.token, url: f.base }])
      const page = await context.newPage()
      page.setDefaultTimeout(8000)
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(f.base + reference.path)
      await page
        .getByRole('navigation', { name: '本篇目录' })
        .getByRole('link', { name: '安装', exact: true })
        .waitFor()
      const referenceBox = await page.locator('.reader-article > .markdown-body').boundingBox()
      await page.goto(`${f.base}/learn/modules/${mod.module.slug}/reader`)
      const nav = page.getByRole('navigation', { name: '本章小节' })
      await nav.getByRole('link', { name: '高级设置', exact: true }).waitFor()
      const chapterBox = await page.locator('.reader-article > .markdown-body').boundingBox()
      assert.equal(chapterBox.x, referenceBox.x)
      assert.equal(chapterBox.width, referenceBox.width)
      assert.equal(
        await page
          .locator('.reader-article > .markdown-body')
          .evaluate(el => getComputedStyle(el).fontSize),
        '17px'
      )
      assert.equal(await nav.getByRole('link', { name: '题目提示小节', exact: true }).count(), 0)
      assert.equal(await page.getByRole('button', { name: /后续章节/ }).isDisabled(), true)
      await nav.getByRole('button', { name: '收起起点的子目录', exact: true }).click()
      await page.getByRole('button', { name: '收起模块目录', exact: true }).click()
      await page.getByRole('button', { name: '展开模块目录', exact: true }).click()
      assert.equal(
        await nav.getByRole('link', { name: '安装', exact: true }).isVisible(),
        false,
        'hiding the sidebar preserves nested folds'
      )
      await nav.getByRole('button', { name: '展开起点的子目录', exact: true }).click()
      await nav.getByRole('link', { name: '高级设置', exact: true }).click()
      await page.waitForFunction(() => decodeURIComponent(location.hash) === '#高级设置')
      const top = await page.locator('h4[id="高级设置"]').boundingBox(),
        bounds = await page.getByLabel('章节正文', { exact: true }).boundingBox()
      assert.ok(Math.abs(top.y - bounds.y - 24) <= 2)
      await page.locator('.answer-input').fill('hello')
      await page.getByRole('button', { name: '校验答案', exact: true }).click()
      await page.locator('.answer-feedback.success').waitFor()
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      if (reference) await f.sql`delete from reference_documents where id=${reference.id}`
      await f.cleanup()
    }
  }
)
