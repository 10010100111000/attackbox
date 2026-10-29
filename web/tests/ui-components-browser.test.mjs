import test, { before, after } from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { uiFixture } from './helpers/ui-fixture.mjs'
import { chooseOption } from './helpers/select.mjs'

let fixture, browser
before(async () => {
  fixture = await uiFixture()
  browser = await chromium.launch({
    executablePath: process.env.CHROMIUM_PATH || '/usr/bin/chromium',
    args: ['--no-sandbox']
  })
})
after(async () => {
  await browser?.close()
  await fixture?.close()
})

async function visit(path = '/') {
  const page = await browser.newPage()
  page.setDefaultTimeout(7000)
  await page.goto(fixture.base + path)
  return page
}

test('admin page styles preserve shared button colors, sizing and status semantics in both themes', async () => {
  const page = await visit()
  try {
    await page.locator('#outside').waitFor()
    for (const theme of ['graphite', 'midnight']) {
      await page.evaluate(theme => (document.documentElement.dataset.theme = theme), theme)
      const styles = await page.evaluate(() =>
        Object.fromEntries(
          ['outside', 'users-primary', 'reference-primary', 'users-danger'].map(id => {
            const style = getComputedStyle(document.getElementById(id))
            return [
              id,
              {
                color: style.color,
                background: style.backgroundColor,
                font: style.fontSize,
                height: style.height
              }
            ]
          })
        )
      )
      assert.notEqual(styles.outside.color, styles.outside.background)
      assert.deepEqual(styles['users-primary'], styles.outside)
      assert.deepEqual(styles['reference-primary'], styles.outside)
      assert.notEqual(styles['users-danger'].color, styles.outside.color)
    }
    assert.equal(await page.locator('#error-notice').getAttribute('role'), 'alert')
    assert.equal(await page.locator('#success-notice').getAttribute('role'), 'status')
  } finally {
    await page.close()
  }
})

test('disabled and loading links cannot navigate or invoke handlers by mouse, keyboard or scripted clicks', async () => {
  const page = await visit()
  try {
    await page.locator('#download').waitFor()
    assert.equal(await page.locator('#download').getAttribute('download'), 'sample.md')
    assert.equal(await page.locator('#download').getAttribute('href'), '/download.md')
    await page.evaluate(() => window.setFixtureBusy(true))
    for (const id of ['internal', 'download', 'loading-link']) {
      const link = page.locator('#' + id)
      assert.equal(await link.getAttribute('href'), null)
      assert.equal(await link.getAttribute('aria-disabled'), 'true')
      assert.equal(await link.getAttribute('tabindex'), '-1')
      await link.click({ force: true })
      await link.focus()
      await page.keyboard.press('Enter')
      await link.evaluate(el => el.click())
    }
    assert.equal(await page.locator('#native').isDisabled(), true)
    assert.equal(await page.locator('#fieldset-native').isDisabled(), true)
    assert.equal(
      await page.locator('#fieldset-native').evaluate(el => getComputedStyle(el).opacity),
      '0.5'
    )
    assert.equal(await page.getByRole('button', { name: '显示密码' }).isDisabled(), true)
    assert.equal(await page.evaluate(() => window.activations), 0)
    assert.equal(page.url(), fixture.base + '/')
    await page.evaluate(() => window.setFixtureBusy(false))
    await page.locator('#internal').click()
    await page.waitForURL('**/destination')
    assert.equal(await page.evaluate(() => window.activations), 1)
  } finally {
    await page.close()
  }
})

test('route editing freezes controls during save, prevents duplicate requests and protects failed changes', async () => {
  const page = await visit('/admin/routes/existing')
  try {
    const title = page.getByRole('textbox').first()
    await title.fill('Submitted route')
    let release,
      requests = 0,
      submitted
    await page.route('**/api/admin/routes/existing', async route => {
      if (route.request().method() !== 'PUT') return route.continue()
      requests++
      submitted = route.request().postDataJSON()
      await new Promise(resolve => {
        release = resolve
      })
      await route.fulfill({ json: { data: { id: 'existing' } } })
    })
    const save = page.getByRole('button', { name: '保存为草稿' })
    await save.click()
    await page.waitForFunction(() => document.querySelector('fieldset')?.disabled)
    assert.equal(await title.isDisabled(), true)
    assert.equal(await save.isDisabled(), true)
    await save.evaluate(el => el.click())
    assert.equal(requests, 1)
    assert.equal(submitted.title, 'Submitted route')
    release()
    await page.getByRole('status').filter({ hasText: '已保存为草稿' }).waitFor()
    assert.equal(await page.getByText('未保存修改', { exact: true }).count(), 0)

    await title.fill('Unsaved change')
    await page.route('**/api/admin/routes/existing', route =>
      route.fulfill({ status: 409, json: { message: '路线已被修改' } })
    )
    await save.click()
    await page.getByRole('alert').filter({ hasText: '路线已被修改' }).waitFor()
    assert.equal(await title.inputValue(), 'Unsaved change')
    await page.evaluate(() => window.fixtureRouter.push('/destination'))
    assert.equal(new URL(page.url()).pathname, '/admin/routes/existing')
    assert.equal(await page.evaluate(() => window.confirmCalls), 1)
  } finally {
    await page.close()
  }
})

test('creating a route navigates to its saved editor without a discard confirmation', async () => {
  const page = await visit('/admin/routes/new')
  try {
    await page.getByRole('textbox').first().fill('New route')
    await page.route('**/api/admin/routes', route =>
      route.fulfill({ json: { data: { id: 'created' } } })
    )
    await page.getByRole('button', { name: '保存为草稿' }).click()
    await page.waitForURL('**/admin/routes/created')
    assert.equal(await page.evaluate(() => window.confirmCalls), 0)
  } finally {
    await page.close()
  }
})

test('notification failures render alerts and successful retries render success messages', async () => {
  const page = await visit('/admin/notifications')
  try {
    await page.getByLabel('标题', { exact: true }).fill('An update')
    await page.getByLabel('正文', { exact: true }).fill('Course content')
    await page.route('**/api/admin/notifications', route =>
      route.request().method() === 'POST'
        ? route.fulfill({ status: 503, json: { message: '发布失败，稍后再试' } })
        : route.continue()
    )
    await page.getByRole('button', { name: '发布通知', exact: true }).click()
    await page.getByRole('alert').filter({ hasText: '发布失败，稍后再试' }).waitFor()
    assert.equal(await page.getByLabel('标题', { exact: true }).inputValue(), 'An update')
    await page.unroute('**/api/admin/notifications')
    await page.route('**/api/admin/notifications', route =>
      route.request().method() === 'POST' ? route.fulfill({ json: { data: {} } }) : route.continue()
    )
    await page.getByRole('button', { name: '发布通知', exact: true }).click()
    await page.getByRole('status').filter({ hasText: '通知已发布' }).waitFor()
    assert.equal(await page.getByRole('alert').count(), 0)
  } finally {
    await page.close()
  }
})

test(
  'module writes wait for the latest version and failed reads cannot repeat a completed mutation',
  { timeout: 30000 },
  async () => {
    const page = await visit('/admin/modules/detail')
    let releaseRead, signalRead
    let readStarted = new Promise(resolve => {
      signalRead = resolve
    })
    try {
      await page.getByRole('button', { name: '运行配置', exact: true }).click()
      assert.equal(
        await page.getByLabel('前置学习模块').evaluate(el => getComputedStyle(el).backgroundImage),
        'none'
      )
      await page.getByLabel('前置学习模块').selectOption('module-one')
      let writes = 0,
        publicationVersion,
        failRead = false
      const detail = {
        module: {
          id: 'detail',
          title: 'Module detail',
          slug: 'module',
          status: 'unpublished',
          lockVersion: 1,
          defaultTargetTemplateId: null,
          learningOutcomes: []
        },
        chapters: [],
        prerequisites: [{ id: 'module-one' }]
      }
      await page.route('**/api/admin/modules/detail/runtime', async route => {
        writes++
        assert.equal(route.request().postDataJSON().lockVersion, 0)
        await route.fulfill({ json: { data: { module: detail.module } } })
      })
      await page.route('**/api/admin/modules/detail', async route => {
        if (failRead) return route.fulfill({ status: 503, json: { message: 'read unavailable' } })
        await new Promise(resolve => {
          releaseRead = resolve
          signalRead()
        })
        await route.fulfill({ json: { data: detail } })
      })
      const save = page.getByRole('button', { name: '保存运行配置', exact: true })
      await save.click()
      await page.waitForFunction(
        () => document.querySelector('.runtime-settings fieldset')?.disabled
      )
      assert.equal(
        await page.getByRole('button', { name: '同步最新内容', exact: true }).isDisabled(),
        true
      )
      await page.waitForTimeout(50)
      assert.equal(writes, 1)
      await page.waitForFunction(
        () => document.querySelector('.runtime-settings fieldset')?.disabled
      )
      await readStarted
      releaseRead()
      await page.getByRole('status').filter({ hasText: '运行配置已保存' }).waitFor()

      failRead = true
      await page.route('**/api/admin/modules/detail/publish', async route => {
        publicationVersion = route.request().postDataJSON().lockVersion
        await route.fulfill({ json: { data: { id: 'detail' } } })
      })
      await page.getByRole('button', { name: '发布', exact: true }).click()
      await page.getByRole('alert').filter({ hasText: '操作已完成，但最新数据读取失败' }).waitFor()
      assert.equal(publicationVersion, 1)
      assert.equal(await page.getByRole('button', { name: '发布', exact: true }).isDisabled(), true)
      assert.equal(await save.isDisabled(), true)
      assert.equal(writes, 1)

      failRead = false
      detail.module.status = 'published'
      detail.module.lockVersion = 2
      releaseRead = null
      readStarted = new Promise(resolve => {
        signalRead = resolve
      })
      await page.getByRole('button', { name: '重新读取', exact: true }).click()
      await readStarted
      releaseRead()
      await page.getByRole('button', { name: '下架', exact: true }).waitFor()
      assert.equal(await page.getByRole('button', { name: '下架', exact: true }).isEnabled(), true)
    } finally {
      releaseRead?.()
      await page.close()
    }
  }
)

test('repository panel closing protects unsaved credentials and failed saves retain the input', async () => {
  const page = await visit('/admin/references')
  try {
    const toggle = page.getByRole('button', { name: '仓库设置', exact: true })
    await toggle.click()
    const repository = page.getByLabel('仓库', { exact: true })
    await repository.fill('author/new-reference')
    await page.getByLabel('只读 Token', { exact: true }).fill('fixture-token')
    await toggle.click()
    assert.equal(await page.evaluate(() => window.confirmCalls), 1)
    assert.equal(await repository.inputValue(), 'author/new-reference')
    assert.equal(
      await page.getByRole('button', { name: '清除 Token', exact: true }).isDisabled(),
      true
    )
    await page.route('**/api/admin/references/repository', route =>
      route.fulfill({ status: 409, json: { message: '仓库配置已变化' } })
    )
    await page.getByRole('button', { name: '保存仓库', exact: true }).click()
    await page.getByRole('alert').filter({ hasText: '仓库配置已变化' }).waitFor()
    assert.equal(await page.getByLabel('只读 Token', { exact: true }).inputValue(), 'fixture-token')
    await page.getByRole('button', { name: '取消修改', exact: true }).click()
    await toggle.click()
    assert.equal(await page.getByLabel('仓库', { exact: true }).count(), 0)
  } finally {
    await page.close()
  }
})

test('runtime policy protects route updates and document exits, and releases the guard after saving', async () => {
  const page = await visit('/admin/runtime-policy')
  try {
    await page.getByLabel('使用时长', { exact: true }).first().fill('90')
    assert.equal(
      await page.evaluate(() => {
        const event = new Event('beforeunload', { cancelable: true })
        window.dispatchEvent(event)
        return event.defaultPrevented
      }),
      true
    )
    await page.evaluate(() => window.fixtureRouter.push('/admin/runtime-policy?tab=other'))
    assert.equal(new URL(page.url()).search, '')
    assert.equal(await page.evaluate(() => window.confirmCalls), 1)
    await page.route('**/api/admin/runtime-policy', route =>
      route.fulfill({
        json: {
          data: {
            sandboxTtlMinutes: 90,
            sandboxIdleMinutes: 10,
            targetTtlMinutes: 60,
            lockVersion: 1
          }
        }
      })
    )
    await page.getByRole('button', { name: '保存设置', exact: true }).click()
    await page.getByRole('status').filter({ hasText: '运行策略已保存' }).waitFor()
    assert.equal(
      await page.evaluate(() => {
        const event = new Event('beforeunload', { cancelable: true })
        window.dispatchEvent(event)
        return event.defaultPrevented
      }),
      false
    )
    await page.evaluate(() => window.fixtureRouter.push('/destination'))
    await page.waitForURL('**/destination')
  } finally {
    await page.close()
  }
})

test('Compose entry changes commit only after parsing and preserve the previous configuration on failure', async () => {
  const page = await visit('/admin/target-templates/existing')
  try {
    const entry = page.getByRole('combobox', { name: /^部署入口/ })
    await entry.waitFor()
    await page.route('**/api/admin/lab-bundles/bundle?file=alternate.yml', route =>
      route.fulfill({ status: 400, json: { message: 'Compose 文件解析失败' } })
    )
    await chooseOption(page, entry, 'alternate.yml')
    await page.getByRole('alert').filter({ hasText: 'Compose 文件解析失败' }).waitFor()
    assert.equal(await entry.innerText(), 'compose.yml')
    assert.equal(await page.getByLabel('端口', { exact: true }).inputValue(), '8080')
    assert.equal(
      await page.evaluate(() => {
        const event = new Event('beforeunload', { cancelable: true })
        window.dispatchEvent(event)
        return event.defaultPrevented
      }),
      false
    )
    await page.unroute('**/api/admin/lab-bundles/bundle?file=alternate.yml')
    await page.route('**/api/admin/lab-bundles/bundle?file=alternate.yml', route =>
      route.fulfill({
        json: {
          data: {
            id: 'bundle',
            hash: 'hash1',
            composeFiles: ['compose.yml', 'alternate.yml'],
            files: 2,
            services: [
              {
                name: 'web',
                image: 'example/web',
                networks: ['default'],
                ports: [{ port: 9090, protocol: 'tcp' }]
              }
            ],
            networks: ['default'],
            variables: [],
            missingFiles: [],
            issues: []
          }
        }
      })
    )
    await chooseOption(page, entry, 'alternate.yml')
    await page.waitForFunction(() => document.querySelector('input[type=number]')?.value === '9090')
    assert.equal(await entry.innerText(), 'alternate.yml')
    assert.equal(
      await page.getByRole('button', { name: '保存模板', exact: true }).isEnabled(),
      true
    )
  } finally {
    await page.close()
  }
})

test('module actions reserve consistent columns and homepage controls use shared button styling', async () => {
  const page = await visit('/admin/modules')
  try {
    const detailLinks = page.locator('.module-detail-link')
    await detailLinks.first().waitFor()
    const boxes = await detailLinks.evaluateAll(links =>
      links.map(link => ({
        x: link.getBoundingClientRect().x,
        width: link.getBoundingClientRect().width
      }))
    )
    assert.equal(boxes.length, 2)
    assert.deepEqual(boxes[0], boxes[1])
    assert.equal(
      await page
        .locator('.module-preview-link')
        .evaluate(el => getComputedStyle(el).gridColumnStart),
      '3'
    )
    const primary = page.getByRole('link', { name: '检查 Git 更新', exact: true })
    assert.equal(
      await primary.evaluate(
        el => getComputedStyle(el).color !== getComputedStyle(el).backgroundColor
      ),
      true
    )
    await page.goto(fixture.base + '/admin/learn-home')
    const save = page.getByRole('button', { name: '保存首页', exact: true })
    await save.waitFor()
    await page.getByLabel('标题 *', { exact: true }).fill('Updated recommendation')
    assert.equal(await save.isEnabled(), true)
    assert.equal(
      await save.evaluate(
        el => getComputedStyle(el).color !== getComputedStyle(el).backgroundColor
      ),
      true
    )
    await page.getByRole('button', { name: '添加推荐', exact: true }).click()
    assert.equal(await page.locator('.slide-row').count(), 2)
  } finally {
    await page.close()
  }
})
