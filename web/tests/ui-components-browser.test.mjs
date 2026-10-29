import test, { before, after } from 'node:test'
import assert from 'node:assert/strict'
import { chromium } from 'playwright'
import { uiFixture } from './helpers/ui-fixture.mjs'

let fixture, browser
before(async () => {
  fixture = await uiFixture()
  browser = await chromium.launch({ executablePath: '/usr/bin/chromium', args: ['--no-sandbox'] })
})
after(async () => { await browser?.close();await fixture?.close() })

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
      await page.evaluate(theme => document.documentElement.dataset.theme = theme, theme)
      const styles = await page.evaluate(() => Object.fromEntries(['outside', 'users-primary', 'reference-primary', 'users-danger'].map(id => {
        const style = getComputedStyle(document.getElementById(id))
        return [id, { color: style.color, background: style.backgroundColor, font: style.fontSize, height: style.height }]
      })))
      assert.notEqual(styles.outside.color, styles.outside.background)
      assert.deepEqual(styles['users-primary'], styles.outside)
      assert.deepEqual(styles['reference-primary'], styles.outside)
      assert.notEqual(styles['users-danger'].color, styles.outside.color)
    }
    assert.equal(await page.locator('#error-notice').getAttribute('role'), 'alert')
    assert.equal(await page.locator('#success-notice').getAttribute('role'), 'status')
  } finally { await page.close() }
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
    assert.equal(await page.locator('#fieldset-native').evaluate(el => getComputedStyle(el).opacity), '0.5')
    assert.equal(await page.getByRole('button', { name: '显示密码' }).isDisabled(), true)
    assert.equal(await page.evaluate(() => window.activations), 0)
    assert.equal(page.url(), fixture.base + '/')
    await page.evaluate(() => window.setFixtureBusy(false))
    await page.locator('#internal').click()
    await page.waitForURL('**/destination')
    assert.equal(await page.evaluate(() => window.activations), 1)
  } finally { await page.close() }
})

test('route editing freezes controls during save, prevents duplicate requests and protects failed changes', async () => {
  const page = await visit('/admin/routes/existing')
  try {
    const title = page.getByRole('textbox').first()
    await title.fill('Submitted route')
    let release, requests = 0, submitted
    await page.route('**/api/admin/routes/existing', async route => {
      if (route.request().method() !== 'PUT') return route.continue()
      requests++
      submitted = route.request().postDataJSON()
      await new Promise(resolve => { release = resolve })
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
    await page.route('**/api/admin/routes/existing', route => route.fulfill({ status: 409, json: { message: '路线已被修改' } }))
    await save.click()
    await page.getByRole('alert').filter({ hasText: '路线已被修改' }).waitFor()
    assert.equal(await title.inputValue(), 'Unsaved change')
    await page.evaluate(() => window.fixtureRouter.push('/destination'))
    assert.equal(new URL(page.url()).pathname, '/admin/routes/existing')
    assert.equal(await page.evaluate(() => window.confirmCalls), 1)
  } finally { await page.close() }
})

test('creating a route navigates to its saved editor without a discard confirmation', async () => {
  const page = await visit('/admin/routes/new')
  try {
    await page.getByRole('textbox').first().fill('New route')
    await page.route('**/api/admin/routes', route => route.fulfill({ json: { data: { id: 'created' } } }))
    await page.getByRole('button', { name: '保存为草稿' }).click()
    await page.waitForURL('**/admin/routes/created')
    assert.equal(await page.evaluate(() => window.confirmCalls), 0)
  } finally { await page.close() }
})

test('notification failures render alerts and successful retries render success messages', async () => {
  const page = await visit('/admin/notifications')
  try {
    await page.getByLabel('标题', { exact: true }).fill('An update')
    await page.getByLabel('正文', { exact: true }).fill('Course content')
    await page.route('**/api/admin/notifications', route => route.request().method() === 'POST'
      ? route.fulfill({ status: 503, json: { message: '发布失败，稍后再试' } }) : route.continue())
    await page.getByRole('button', { name: '发布通知', exact: true }).click()
    await page.getByRole('alert').filter({ hasText: '发布失败，稍后再试' }).waitFor()
    assert.equal(await page.getByLabel('标题', { exact: true }).inputValue(), 'An update')
    await page.unroute('**/api/admin/notifications')
    await page.route('**/api/admin/notifications', route => route.request().method() === 'POST'
      ? route.fulfill({ json: { data: {} } }) : route.continue())
    await page.getByRole('button', { name: '发布通知', exact: true }).click()
    await page.getByRole('status').filter({ hasText: '通知已发布' }).waitFor()
    assert.equal(await page.getByRole('alert').count(), 0)
  } finally { await page.close() }
})
