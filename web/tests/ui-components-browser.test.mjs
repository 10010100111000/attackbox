import test, { before, after, afterEach } from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { chromiumOptions } from './helpers/browser.mjs'
import { uiFixture } from './helpers/ui-fixture.mjs'
import { chooseOption } from './helpers/select.mjs'

let fixture, browser
const pageErrors = []
afterEach(() => assert.deepEqual(pageErrors.splice(0), [], 'unexpected browser runtime errors'))
before(async () => {
  fixture = await uiFixture()
  browser = await chromium.launch(chromiumOptions())
})
after(async () => {
  await browser?.close()
  await fixture?.close()
})

async function visit(path = '/') {
  const page = await browser.newPage()
  page.on('pageerror', error => pageErrors.push(error.message))
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

test('bare inputs preserve DOM placement, native events, numeric modifiers and accessible field metadata', async () => {
  const page = await visit('/controls')
  try {
    const input = page.getByRole('textbox', { name: 'Inline input', exact: true })
    await input.waitFor()
    assert.equal(await page.locator('.inline-row > input').count(), 1)
    assert.equal(await input.evaluate(el => el.getBoundingClientRect().width), 160)
    assert.equal(await input.getAttribute('aria-describedby'), 'external-help')
    assert.equal(await input.getAttribute('maxlength'), '10')
    const values = async () => JSON.parse(await page.locator('#control-values').textContent())
    await input.fill('hello')
    await input.press('Tab')
    let state = await values()
    assert.equal(state.text, 'hello')
    assert.equal(state.events.input, 1)
    assert.equal(state.events.change, 1)
    assert.equal(state.events.blur, 1)
    assert.ok(state.events.keydown > 0)
    await input.evaluate(el => {
      el.dispatchEvent(new CompositionEvent('compositionstart', { bubbles: true }))
      el.value = 'ni'
      el.dispatchEvent(new InputEvent('input', { bubbles: true, isComposing: true }))
    })
    assert.equal((await values()).text, 'hello')
    await input.evaluate(el => {
      el.value = '你好'
      el.dispatchEvent(new CompositionEvent('compositionend', { bubbles: true }))
    })
    assert.equal((await values()).text, '你好')
    await page.locator('#focus-input').click()
    assert.equal(await input.evaluate(el => el === document.activeElement), true)
    await page.locator('#toggle-error').click()
    assert.equal(await input.getAttribute('aria-invalid'), 'true')
    assert.equal(await input.getAttribute('aria-describedby'), 'external-help')
    const enhanced = page.getByLabel('Enhanced password', { exact: true })
    assert.equal(
      await enhanced.getAttribute('aria-describedby'),
      'external-help enhanced-password-error'
    )
    assert.equal(await page.locator('#enhanced-password-error').textContent(), 'Invalid')
    assert.equal(await page.getByRole('button', { name: '显示密码', exact: true }).count(), 1)
    await page.getByRole('button', { name: '显示密码', exact: true }).click()
    assert.equal(await enhanced.getAttribute('type'), 'text')
    assert.equal(
      await page.getByLabel('Bare password', { exact: true }).getAttribute('type'),
      'password'
    )
    const number = page.getByRole('spinbutton', { name: 'Numeric input' })
    await number.fill('8')
    assert.equal((await values()).amount, 8)
    await number.fill('')
    assert.equal((await values()).amount, '')
    assert.equal(await number.evaluate(el => el.validity.valueMissing), true)
    await number.fill('11')
    assert.equal(await number.evaluate(el => el.validity.rangeOverflow), true)
    await page.getByLabel('Trimmed input', { exact: true }).fill('  hello  ')
    assert.equal((await values()).trimmed, 'hello')
    const deferred = page.getByLabel('Deferred input', { exact: true })
    await deferred.fill('wait for change')
    assert.equal((await values()).deferred, '')
    await deferred.press('Tab')
    assert.equal((await values()).deferred, 'wait for change')
  } finally {
    await page.close()
  }
})

test('checkboxes support label and keyboard activation, form values and disabled fieldsets; separators retain semantics', async () => {
  const page = await visit('/controls')
  try {
    const checkbox = page.getByRole('checkbox', { name: 'Accept terms' })
    await checkbox.waitFor()
    assert.equal(await checkbox.isChecked(), false)
    await page.getByText('Accept terms', { exact: true }).click()
    assert.equal(await checkbox.isChecked(), true)
    assert.equal(
      await page.evaluate(() =>
        new FormData(document.querySelector('#control-form')).get('accepted')
      ),
      'yes'
    )
    await checkbox.focus()
    await page.keyboard.press('Space')
    assert.equal(await checkbox.isChecked(), false)
    assert.equal(
      await page.evaluate(() =>
        new FormData(document.querySelector('#control-form')).get('accepted')
      ),
      null
    )
    await page.locator('#disable-controls').click()
    assert.equal(await checkbox.isDisabled(), true)
    assert.equal(await page.getByLabel('Disabled input').isDisabled(), true)
    assert.equal(await checkbox.evaluate(el => getComputedStyle(el).opacity), '0.5')
    await checkbox.evaluate(el => el.click())
    assert.equal(await checkbox.isChecked(), false)
    assert.equal(await page.locator('#horizontal-separator').getAttribute('role'), 'separator')
    assert.equal(
      await page.locator('#vertical-separator').getAttribute('aria-orientation'),
      'vertical'
    )
    assert.equal(await page.locator('#decorative-separator').getAttribute('role'), 'none')
    assert.equal(await page.locator('#labeled-separator').textContent(), 'Other methods')
    assert.equal(await page.locator('#labeled-separator [role="none"]').count(), 2)
  } finally {
    await page.close()
  }
})

test('migrated homepage settings submit booleans and numeric values without changing label or grid structure', async () => {
  const page = await visit('/admin/learn-home')
  try {
    let submitted
    await page.route('**/api/admin/learn-home', route => {
      if (route.request().method() !== 'PUT') return route.continue()
      submitted = route.request().postDataJSON()
      return route.fulfill({ json: { data: { ...submitted, lockVersion: 1 } } })
    })
    const enabled = page.getByRole('checkbox', { name: '显示推荐区' })
    await enabled.waitFor()
    assert.equal((await page.locator('.fields > label > input').count()) > 0, true)
    await enabled.uncheck()
    await page.getByRole('checkbox', { name: '自动轮播' }).check()
    await page.getByRole('spinbutton', { name: /间隔/ }).fill('12')
    await page.getByRole('button', { name: '保存首页', exact: true }).click()
    await page.getByRole('status').filter({ hasText: '首页已更新' }).waitFor()
    assert.equal(submitted.enabled, false)
    assert.equal(submitted.autoplay, true)
    assert.equal(submitted.intervalSeconds, 12)
  } finally {
    await page.close()
  }
})

test('admin list pages own their headers, search fields, tables and actions without loading user-page CSS', async () => {
  const page = await visit('/admin/challenges')
  try {
    for (const theme of ['graphite', 'midnight']) {
      const headings = []
      for (const [path, title, create, href] of [
        ['/admin/challenges', '挑战管理', '新建挑战', '/admin/challenges/new'],
        ['/admin/routes', '学习路线', '新建路线', '/admin/routes/new'],
        ['/admin/target-templates', '靶场模板', '新建模板', '/admin/target-templates/new'],
        ['/admin/users', '用户管理', null, null]
      ]) {
        await page.goto(fixture.base + path)
        await page.evaluate(theme => (document.documentElement.dataset.theme = theme), theme)
        await page.getByRole('heading', { name: title, exact: true }).waitFor()
        await page.locator('tbody tr').waitFor()
        const metrics = await page.locator('.admin-list-page').evaluate(el => {
          const heading = el.querySelector('.page-heading'),
            h1 = heading.querySelector('h1')
          const table = el.querySelector('table'),
            panel = table.parentElement
          const titleStyle = getComputedStyle(h1),
            headerStyle = getComputedStyle(heading)
          const thStyle = getComputedStyle(table.querySelector('th'))
          return {
            font: titleStyle.fontSize,
            weight: titleStyle.fontWeight,
            line: titleStyle.lineHeight,
            headerDisplay: headerStyle.display,
            tableFont: getComputedStyle(table).fontSize,
            cellPadding: thStyle.padding,
            border: getComputedStyle(panel).borderTopWidth,
            headerWidth: heading.getBoundingClientRect().width,
            panelWidth: panel.getBoundingClientRect().width
          }
        })
        headings.push([metrics.font, metrics.weight, metrics.line])
        assert.equal(metrics.font, '24px')
        assert.equal(metrics.headerDisplay, 'flex')
        assert.equal(metrics.tableFont, '12px')
        assert.equal(metrics.cellPadding, '16px 20px')
        assert.equal(metrics.border, '1px')
        assert.equal(metrics.headerWidth, metrics.panelWidth)
        if (create) {
          const action = page.getByRole('link', { name: create, exact: true })
          assert.equal(await action.getAttribute('href'), href)
          assert.equal(await action.evaluate(el => getComputedStyle(el).height), '32px')
          assert.equal(
            await action.evaluate(
              el => getComputedStyle(el).color !== getComputedStyle(el).backgroundColor
            ),
            true
          )
          assert.equal(await page.getByRole('link', { name: '编辑', exact: true }).count(), 1)
        }
      }
      assert.deepEqual(headings, Array(4).fill(headings[0]))
    }
    await page.goto(fixture.base + '/admin/challenges')
    await page.getByRole('textbox', { name: '搜索挑战' }).fill('no matching challenge')
    await page.getByText('暂无挑战', { exact: true }).waitFor()
    assert.equal(await page.locator('tbody tr').count(), 0)
    await page.getByRole('textbox', { name: '搜索挑战' }).fill('Web')
    await page.locator('tbody tr').waitFor()
  } finally {
    await page.close()
  }
})

test('homepage editing previews and saves generic image campaigns, without terminal decoration fields', async () => {
  const page = await visit('/admin/learn-home')
  try {
    let submitted
    await page.route('**/api/admin/learn-home', route => {
      if (route.request().method() !== 'PUT') return route.continue()
      submitted = route.request().postDataJSON()
      return route.fulfill({ json: { data: { ...submitted, lockVersion: 1 } } })
    })
    await page.getByLabel('标题 *', { exact: true }).fill('Workshop campaign')
    await page.getByLabel('图片地址', { exact: false }).fill('/banner-fixture.svg')
    await page.getByLabel('宣传文案', { exact: true }).fill('Meet and learn together')
    assert.equal(
      await page
        .getByLabel('宣传文案', { exact: true })
        .evaluate(el => getComputedStyle(el).padding),
      '8px 12px'
    )
    await page
      .getByLabel('跳转链接', { exact: true })
      .fill('https://sponsor.example/campaign?from=learn')
    await page.getByLabel('按钮文字', { exact: true }).fill('')
    await page.locator('.preview .banner-image').waitFor()
    assert.equal(await page.locator('.banner-art, .terminal-art').count(), 0)
    assert.equal(await page.locator('.preview h1').textContent(), 'Workshop campaign')
    assert.equal(await page.locator('.preview .banner-link').textContent(), '了解更多')
    await page.getByRole('button', { name: '保存首页', exact: true }).click()
    await page.getByRole('status').filter({ hasText: '首页已更新' }).waitFor()
    assert.equal(submitted.slides[0].imageUrl, '/banner-fixture.svg')
    assert.equal(submitted.slides[0].href, 'https://sponsor.example/campaign?from=learn')
    assert.equal('command' in submitted.slides[0], false)
    assert.equal('caption' in submitted.slides[0], false)
    await page.getByLabel('图片地址', { exact: false }).fill('')
    await page.getByLabel('跳转链接', { exact: true }).fill('')
    await page.locator('.preview .banner-image').waitFor({ state: 'detached' })
    assert.equal(await page.locator('.preview .banner-link').count(), 0)
  } finally {
    await page.close()
  }
})

test('image carousel supports external links, plain announcements, image failure and pausing autoplay', async () => {
  const page = await visit('/carousel')
  try {
    await page.locator('.banner-image').waitFor()
    assert.equal(
      await page.getByRole('link', { name: '了解活动' }).getAttribute('href'),
      'https://sponsor.example/campaign?from=learn'
    )
    for (const theme of ['graphite', 'midnight']) {
      await page.evaluate(theme => (document.documentElement.dataset.theme = theme), theme)
      assert.equal(
        await page.locator('.feature-banner h1').evaluate(el => getComputedStyle(el).color),
        'rgb(255, 255, 255)'
      )
    }
    await page.clock.install()
    await page.mouse.move(1200, 700)
    await page.evaluate(() => window.setCarouselAutoplay(true))
    await page.clock.fastForward(5100)
    await page.getByRole('heading', { name: 'Text announcement' }).waitFor()
    assert.equal(await page.locator('.banner-image, .banner-link').count(), 0)
    await page.locator('.feature-banner').hover()
    await page.clock.fastForward(10000)
    assert.equal(await page.locator('.feature-banner h1').textContent(), 'Text announcement')
    await page.mouse.move(1200, 700)
    await page.clock.fastForward(5100)
    await page.getByRole('heading', { name: 'Image fallback' }).waitFor()
    await page.locator('.banner-image').waitFor({ state: 'detached' })
    assert.equal(
      await page.locator('.feature-banner').evaluate(el => el.classList.contains('has-image')),
      false
    )
    assert.equal(await page.getByRole('link', { name: '了解更多' }).getAttribute('href'), '/learn')
    await page.getByRole('button', { name: '暂停轮播' }).click()
    await page.clock.fastForward(10000)
    assert.equal(await page.locator('.feature-banner h1').textContent(), 'Image fallback')
  } finally {
    await page.close()
  }
})
