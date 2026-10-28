import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import { existsSync, mkdirSync } from 'node:fs'
import { chromium } from 'playwright'
import { learningFixture } from './helpers/learning-fixture.mjs'

test(
  'personal center: real progress, navigation, profile edits, empty states and identity isolation',
  {
    skip:
      !process.env.CONTENT_TEST_DATABASE_URL ||
      !process.env.CONTENT_TEST_BASE_URL,
    timeout: 90000,
  },
  async () => {
    const f = await learningFixture(),
      moduleIds = [],
      challengeIds = []
    let browser
    const screenshots = process.env.PERSONAL_SCREENSHOT_DIR
    async function module(title, count) {
      const id = crypto.randomUUID(),
        slug = `personal-${id}`
      await f.sql`insert into modules(id,slug,title,status,created_by,published_at) values(${id},${slug},${title},'published',${f.admin.id},now())`
      moduleIds.push(id)
      const chapters = []
      for (let i = 1; i <= count; i++) {
        const [chapter] =
          await f.sql`insert into chapters(module_id,slug,position,title,access,body_markdown) values(${id},${'chapter-' + i},${i},${'章节 ' + i},'public','## Personal center fixture') returning id`
        chapters.push(chapter.id)
      }
      return { id, slug, chapters }
    }
    async function learn(user, mod, completed, hoursAgo = 0) {
      for (let i = 0; i < mod.chapters.length; i++) {
        if (i > completed) break
        await f.sql`insert into chapter_progress(user_id,chapter_id,last_visited_at,completed_at) values(${user.id},${mod.chapters[i]},now()-${hoursAgo}*interval '1 hour',${i < completed ? new Date() : null})`
      }
    }
    async function challenge(user, title, total, passed) {
      const [row] =
        await f.sql`insert into challenges(title,summary,body_markdown,category,difficulty,status) values(${title},'阶段式挑战','# Fixture','内网安全','advanced','published') returning id`
      challengeIds.push(row.id)
      for (let i = 1; i <= total; i++) {
        const [stage] =
          await f.sql`insert into challenge_stages(challenge_id,position,title) values(${row.id},${i},${'阶段 ' + i}) returning id`
        if (i <= passed)
          await f.sql`insert into challenge_completions(user_id,stage_id) values(${user.id},${stage.id})`
      }
      return row.id
    }
    async function snapshot(page, name) {
      if (!screenshots) return
      mkdirSync(screenshots, { recursive: true })
      await page.screenshot({
        path: `${screenshots}/${name}.png`,
        fullPage: true,
        animations: 'disabled',
      })
    }
    try {
      const a = await f.account(),
        b = await f.account(),
        empty = await f.account()
      await f.sql`update users set display_name='Lin' where id=${a.id}`
      const active = await module('Linux 权限与提权', 3),
        done = await module('Web 安全基础', 2),
        other = await module('另一个用户的专属记录', 2)
      await learn(a, active, 1, 1)
      await learn(a, done, 2)
      await learn(b, other, 1)
      const partial = await challenge(a, 'Active Directory 横向移动', 3, 1)
      await challenge(a, '容器逃逸初探', 2, 2)
      assert.equal(
        (await f.request('/api/me/learning', { actor: a })).data.length,
        2,
      )
      assert.equal(
        (await f.request('/api/me/challenges', { actor: a })).data.length,
        2,
      )
      await f.request('/api/me/profile', {
        method: 'PUT',
        body: { displayName: 'Unauthorized' },
        status: 401,
      })
      for (const displayName of ['   ', 'x'.repeat(65)])
        await f.request('/api/me/profile', {
          actor: a,
          method: 'PUT',
          body: { displayName },
          status: 400,
        })

      browser = await chromium.launch({
        executablePath:
          process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH ||
          (existsSync('/usr/bin/chromium') ? '/usr/bin/chromium' : undefined),
        args: ['--no-sandbox'],
      })
      const context = await browser.newContext({
        viewport: { width: 1440, height: 1000 },
      })
      await context.addCookies([
        { name: 'attackbox_session', value: a.token, url: f.base },
      ])
      const page = await context.newPage(),
        errors = []
      page.setDefaultTimeout(10000)
      page.on('pageerror', (error) => errors.push(error.message))
      const navigation = page.getByRole('navigation', {
        name: '个人中心',
        exact: true,
      })
      await page.goto(f.base + '/me')
      await page
        .locator('.personal-resume h2')
        .getByText('Linux 权限与提权', { exact: true })
        .waitFor()
      assert.equal(
        await page.locator('.personal-resume-button').getAttribute('href'),
        `/learn/modules/${active.slug}/chapter-2`,
      )
      assert.equal(await page.locator('.personal-record').count(), 2)
      assert.equal(
        await page
          .getByRole('progressbar', { name: 'Linux 权限与提权的章节进度' })
          .getAttribute('aria-valuenow'),
        '33',
      )
      assert.equal(
        await page.getByText('另一个用户的专属记录', { exact: true }).count(),
        0,
      )
      await snapshot(page, 'learning-graphite')
      const filtersBefore = await page
        .getByRole('group', { name: '筛选学习记录' })
        .boundingBox()
      await page
        .getByRole('group', { name: '筛选学习记录' })
        .getByRole('button', { name: '已完成', exact: true })
        .click()
      assert.equal(await page.locator('.personal-record').count(), 1)
      assert.equal(await page.locator('.personal-resume').count(), 1)
      const filtersAfter = await page
        .getByRole('group', { name: '筛选学习记录' })
        .boundingBox()
      assert.ok(
        Math.abs(filtersBefore.y - filtersAfter.y) < 1,
        'filter controls remain stationary when filtering records',
      )
      assert.equal(
        await page.locator('.personal-record').getAttribute('href'),
        `/learn/modules/${done.slug}`,
      )
      await page
        .getByRole('group', { name: '筛选学习记录' })
        .getByRole('button', { name: '进行中', exact: true })
        .click()
      assert.equal(await page.locator('.personal-record').count(), 1)
      await navigation
        .getByRole('link', { name: '挑战记录', exact: true })
        .click()
      await page.waitForURL('**/me?tab=challenges')
      await page
        .getByRole('link', {
          name: 'Active Directory 横向移动，进行中',
          exact: true,
        })
        .waitFor()
      assert.equal(
        await page
          .getByRole('link', {
            name: 'Active Directory 横向移动，进行中',
            exact: true,
          })
          .getAttribute('href'),
        `/challenges/${partial}`,
      )
      await page
        .getByRole('group', { name: '筛选挑战记录' })
        .getByRole('button', { name: '已完成', exact: true })
        .click()
      assert.equal(await page.locator('.personal-record').count(), 1)
      await page.reload()
      await page
        .getByRole('link', {
          name: 'Active Directory 横向移动，进行中',
          exact: true,
        })
        .waitFor()
      assert.equal(
        await navigation
          .getByRole('link', { name: '挑战记录', exact: true })
          .getAttribute('aria-current'),
        'page',
      )
      await snapshot(page, 'challenges-graphite')

      await navigation
        .getByRole('link', { name: '账户资料', exact: true })
        .click()
      const name = page.getByLabel('昵称', { exact: true }),
        save = page.getByRole('button', { name: '保存资料', exact: true })
      assert.equal(await save.isDisabled(), true)
      await name.fill('草稿昵称')
      await navigation
        .getByRole('link', { name: '学习记录', exact: true })
        .click()
      await navigation
        .getByRole('link', { name: '账户资料', exact: true })
        .click()
      assert.equal(await name.inputValue(), '草稿昵称')
      await navigation.getByRole('link', {name:'通知',exact:true}).click()
      await page.getByRole('alertdialog').getByRole('button', {name:'继续编辑',exact:true}).click()
      assert.equal(
        new URL(page.url()).pathname,
        '/me',
        'leaving with unsaved input can be cancelled',
      )
      await page.getByRole('button', { name: '取消修改', exact: true }).click()
      assert.equal(await name.inputValue(), 'Lin')
      await name.fill('   ')
      assert.equal(await name.getAttribute('aria-invalid'), 'true')
      assert.equal(await save.isDisabled(), true)
      await name.fill('新的昵称')
      await page.route('**/api/me/profile', (route) =>
        route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: JSON.stringify({ message: '网络暂不可用' }),
        }),
      )
      await save.click()
      await page
        .getByRole('alert')
        .getByText('网络暂不可用', { exact: true })
        .waitFor()
      assert.equal(await name.inputValue(), '新的昵称')
      await page.unroute('**/api/me/profile')
      await name.fill('  River  ')
      await save.click()
      await page.getByText('资料已保存', { exact: true }).waitFor()
      assert.equal(await name.inputValue(), 'River')
      assert.equal(await save.isDisabled(), true)
      assert.equal(
        (await f.request('/api/auth/me', { actor: a })).response.status,
        200,
      )
      assert.equal(
        (await f.sql`select display_name from users where id=${a.id}`)[0]
          .display_name,
        'River',
      )
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await page
        .getByRole('dialog', { name: '用户菜单' })
        .getByText('River', { exact: true })
        .waitFor()
      await page.getByRole('button', { name: '深色外观', exact: true }).click()
      await page.keyboard.press('Escape')
      await snapshot(page, 'profile-midnight')
      await navigation
        .getByRole('link', { name: '学习记录', exact: true })
        .click()
      await page.locator('.personal-resume').waitFor()
      await snapshot(page, 'learning-midnight')
      await page.setViewportSize({ width: 390, height: 844 })
      assert.ok(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      )
      await snapshot(page, 'learning-mobile')
      await navigation
        .getByRole('link', { name: '账户资料', exact: true })
        .click()
      assert.ok(
        await page.evaluate(
          () => document.documentElement.scrollWidth <= innerWidth,
        ),
      )
      await snapshot(page, 'profile-mobile')

      // Keep the Nuxt app alive across a real logout/login to check client cache ownership.
      await page.setViewportSize({ width: 1440, height: 1000 })
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await page.getByRole('button', { name: '退出登录', exact: true }).click()
      await page.waitForURL('**/learn')
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await page.getByRole('link', { name: '登录 / 注册', exact: true }).click()
      await page.locator('#email:enabled').fill(b.email)
      await page.locator('#password').fill(b.password)
      await page.getByRole('button', { name: '登录', exact: true }).click()
      await page.waitForURL('**/learn')
      await page.getByRole('button', { name: /用户菜单/ }).click()
      await page
        .getByRole('dialog', { name: '用户菜单' })
        .getByRole('link', { name: '个人中心', exact: true })
        .click()
      await page
        .getByRole('link', {
          name: '另一个用户的专属记录，进行中',
          exact: true,
        })
        .waitFor()
      assert.equal(
        await page.getByRole('link', { name: /Linux 权限与提权/ }).count(),
        0,
      )

      const emptyContext = await browser.newContext({
        viewport: { width: 1440, height: 1000 },
      })
      await emptyContext.addCookies([
        { name: 'attackbox_session', value: empty.token, url: f.base },
      ])
      const fresh = await emptyContext.newPage()
      fresh.on('pageerror', (error) => errors.push(error.message))
      await fresh.goto(f.base + '/me?tab=profile')
      await fresh.route('**/api/me/learning', (route) =>
        route.fulfill({
          status: 503,
          contentType: 'application/json',
          body: '{}',
        }),
      )
      await fresh
        .getByRole('navigation', { name: '个人中心', exact: true })
        .getByRole('link', { name: '学习记录', exact: true })
        .click()
      await fresh
        .getByRole('heading', { name: '暂时无法读取学习记录', exact: true })
        .waitFor()
      await fresh.unroute('**/api/me/learning')
      await fresh.getByRole('button', { name: '重试', exact: true }).click()
      await fresh
        .getByRole('heading', { name: '从一个感兴趣的模块开始', exact: true })
        .waitFor()
      assert.equal(await fresh.locator('.personal-record').count(), 0)
      await snapshot(fresh, 'learning-empty')
      await fresh
        .getByRole('navigation', { name: '个人中心', exact: true })
        .getByRole('link', { name: '挑战记录', exact: true })
        .click()
      await fresh
        .getByRole('heading', { name: '拿下你的第一个 Flag', exact: true })
        .waitFor()
      assert.equal(await fresh.locator('.personal-record').count(), 0)
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      for (const id of moduleIds)
        await f.sql`delete from modules where id=${id}`
      for (const id of challengeIds)
        await f.sql`delete from challenges where id=${id}`
      await f.cleanup()
    }
  },
)
