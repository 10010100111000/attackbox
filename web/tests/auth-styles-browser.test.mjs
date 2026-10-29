import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer } from 'node:http'
import { readFile, writeFile } from 'node:fs/promises'
import { resolve, extname, sep } from 'node:path'
import { chromium } from 'playwright'
import { chromiumOptions } from './helpers/browser.mjs'

// Serve the production prerender and its actual Nuxt chunks; no application
// server or database is needed. Auth APIs are isolated fixtures.
test(
  'production auth forms retain typography, alignment and theme isolation across all modes',
  { timeout: 60000 },
  async () => {
    const root = resolve(process.env.AUTH_STYLE_PUBLIC_DIR || '.output/public')
    const types = {
      '.js': 'text/javascript',
      '.mjs': 'text/javascript',
      '.css': 'text/css',
      '.json': 'application/json',
      '.svg': 'image/svg+xml',
      '.png': 'image/png',
      '.webp': 'image/webp'
    }
    const server = createServer(async (req, res) => {
      try {
        const url = new URL(req.url, 'http://localhost')
        if (url.pathname.startsWith('/api/')) {
          res.setHeader('Content-Type', 'application/json')
          if (url.pathname === '/api/auth/challenge')
            return res.end(JSON.stringify({ enabled: false, siteKey: '' }))
          if (url.pathname === '/api/auth/registration-policy')
            return res.end(JSON.stringify({ domains: ['gmail.com'] }))
          res.statusCode = 401
          return res.end(JSON.stringify({ message: '登录验证未通过' }))
        }
        const path =
          url.pathname === '/auth' || url.pathname === '/auth/'
            ? '/auth/index.html'
            : decodeURIComponent(url.pathname)
        const file = resolve(root, '.' + path)
        if (!file.startsWith(root + sep)) {
          res.statusCode = 404
          return res.end()
        }
        res.setHeader('Content-Type', types[extname(file)] || 'text/html')
        res.end(await readFile(file))
      } catch {
        res.statusCode = 404
        res.end()
      }
    })
    await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
    const base = `http://127.0.0.1:${server.address().port}`
    let browser
    const snapshots = {}
    try {
      browser = await chromium.launch(chromiumOptions())
      for (const theme of ['graphite', 'midnight']) {
        const context = await browser.newContext({
          viewport: { width: 1440, height: 1000 },
          reducedMotion: 'reduce'
        })
        await context.addInitScript(
          theme => localStorage.setItem('attackbox-appearance', theme),
          theme
        )
        const page = await context.newPage(),
          errors = []
        page.setDefaultTimeout(7000)
        page.on('pageerror', error => errors.push(error.message))
        for (const mode of ['login', 'register', 'recover', 'reset', 'verify']) {
          await page.goto(`${base}/auth?mode=${mode}`)
          await page.waitForFunction(
            mode =>
              document.title.startsWith(
                {
                  login: '登录',
                  register: '注册',
                  recover: '找回密码',
                  reset: '重设密码',
                  verify: '验证并激活'
                }[mode]
              ),
            mode
          )
          await page.locator('.form-area h1').waitFor()
          const snapshot = await page.locator('.form-area').evaluate(area => {
            const values = selector => {
              const el = area.querySelector(selector)
              if (!el) return null
              const s = getComputedStyle(el)
              return {
                fontSize: s.fontSize,
                color: s.color,
                marginTop: s.marginTop,
                marginBottom: s.marginBottom,
                padding: s.padding,
                lineHeight: s.lineHeight
              }
            }
            return {
              title: values('h1'),
              intro: values('.intro'),
              field: values('.field'),
              label: values('.field label'),
              fieldset: values('fieldset')
            }
          })
          snapshots[theme + ':' + mode] = snapshot
          assert.equal(snapshot.title.fontSize, mode === 'verify' ? '29px' : '32px')
          assert.equal(snapshot.intro.fontSize, mode === 'verify' ? '13px' : '12px')
          if (snapshot.field) {
            assert.equal(snapshot.field.marginBottom, '20px')
            assert.equal(snapshot.label.fontSize, '11px')
            assert.equal(snapshot.fieldset.padding, '0px')
          }
          if (mode === 'login') {
            await page.getByLabel('邮箱', { exact: true }).fill('fixture@gmail.com')
            await page.getByLabel('密码', { exact: true }).fill('fixture-password')
            await page.getByRole('button', { name: '登录', exact: true }).click()
            const status = page.getByRole('status').filter({ hasText: '登录验证未通过' })
            await status.waitFor()
            assert.equal(await status.evaluate(el => getComputedStyle(el).padding), '12px 14px')
          }
          if (mode === 'verify')
            assert.equal(
              await page
                .locator('.verify-state')
                .evaluate(el => getComputedStyle(el).animationName),
              'none'
            )
          await page.evaluate(() => {
            const probe = document.createElement('h1')
            probe.id = 'style-scope-probe'
            document.body.append(probe)
          })
          assert.equal(
            await page.locator('#style-scope-probe').evaluate(el => getComputedStyle(el).fontSize),
            '16px'
          )
        }
        assert.deepEqual(errors, [])
        await context.close()
      }
      if (process.env.AUTH_STYLE_CAPTURE)
        await writeFile(process.env.AUTH_STYLE_CAPTURE, JSON.stringify(snapshots, null, 2) + '\n')
    } finally {
      await browser?.close()
      await new Promise(resolve => server.close(resolve))
    }
  }
)
