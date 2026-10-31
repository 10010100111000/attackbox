import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import { chromium } from 'playwright'
import { chromiumOptions } from './helpers/browser.mjs'
import { learningFixture } from './helpers/learning-fixture.mjs'

const enabled = !!process.env.CONTENT_TEST_DATABASE_URL && !!process.env.CONTENT_TEST_BASE_URL

test(
  'account deletion enforces lifecycle and ownership, clears credentials/progress, and works through confirmation UI',
  { skip: !enabled, timeout: 60000 },
  async () => {
    const f = await learningFixture()
    let browser, deletedActor
    try {
      const target = await f.account(),
        pending = await f.account(),
        owner = await f.account()
      deletedActor = target.id
      const path = `/api/admin/users/${target.id}`
      await f.request(path, { method: 'DELETE', status: 401 })
      await f.request(path, { actor: pending, method: 'DELETE', status: 403 })
      await f.request(`/api/admin/users/${f.admin.id}`, {
        actor: f.admin,
        method: 'DELETE',
        status: 409
      })
      await f.request(path, { actor: f.admin, method: 'DELETE', status: 409 })
      const mod = await f.createModule()
      await f.sql`insert into chapter_progress(user_id,chapter_id) values(${target.id},${mod.chapters[0].chapterId})`
      await f.sql`insert into audit_logs(actor_user_id,action,target_type,target_id) values(${target.id},'test.account.delete','user',${target.id})`
      await f.sql`insert into email_tokens(target_email,purpose,token_hash,expires_at) values(${target.email},'verify_email',${crypto.randomBytes(32)},now()+interval '1 hour')`
      await f.sql`update users set status='suspended' where id=${target.id}`
      const targetId = crypto.randomUUID()
      await f.sql`insert into target_instances(id,user_id,scope,node_id,boot_id,state,deployment_id,expires_at) values(${targetId},${target.id},'deletion-test','test-node','test-boot','running',${'dep_' + 'a'.repeat(64)},now()+interval '1 hour')`
      const blocked = (await f.request(path, { actor: f.admin })).data
      assert.equal(blocked.deletion.allowed, false)
      assert.match(blocked.deletion.reason, /靶场/)
      await f.request(path, { actor: f.admin, method: 'DELETE', status: 409 })
      await f.sql`update target_instances set state='stopped' where id=${targetId}`
      await f.request(path, { actor: f.admin, method: 'DELETE' })
      await f.request(path, { actor: f.admin, status: 404 })
      for (const rows of [
        await f.sql`select id from auth_sessions where user_id=${target.id}`,
        await f.sql`select id from email_tokens where target_email=${target.email}`,
        await f.sql`select chapter_id from chapter_progress where user_id=${target.id}`,
        await f.sql`select id from target_instances where user_id=${target.id}`
      ])
        assert.equal(rows.length, 0)
      assert.equal(
        (
          await f.sql`select actor_user_id from audit_logs where action='test.account.delete' and target_id=${target.id}`
        )[0].actor_user_id,
        null
      )
      assert.equal(
        (
          await f.sql`select id from audit_logs where action='user.delete' and target_id=${target.id}`
        )[0] !== undefined,
        true
      )
      await f.sql`update users set status='suspended' where id=${owner.id}`
      await f.sql`insert into assets(storage_key,original_name,media_type,byte_size,sha256,visibility,uploaded_by) values(${crypto.randomUUID()},'owned.png','image/png',1,${crypto.randomBytes(32)},'private',${owner.id})`
      await f.request(`/api/admin/users/${owner.id}`, {
        actor: f.admin,
        method: 'DELETE',
        status: 409
      })
      assert.equal(
        (await f.request(`/api/admin/users/${owner.id}`, { actor: f.admin })).data.deletion.allowed,
        false
      )
      await f.sql`update users set status='pending',email_verified_at=null where id=${pending.id}`
      browser = await chromium.launch(chromiumOptions())
      const context = await browser.newContext()
      await context.addCookies([{ name: 'attackbox_session', value: f.admin.token, url: f.base }])
      const page = await context.newPage()
      const errors = []
      page.on('pageerror', error => errors.push(error.message))
      await page.goto(f.base + `/admin/users/${pending.id}`)
      await page.waitForFunction(() => !!document.getElementById('__nuxt')?.__vue_app__)
      await page.getByRole('button', { name: '删除账号', exact: true }).click()
      const dialog = page.getByRole('alertdialog')
      await dialog.getByRole('button', { name: '取消', exact: true }).click()
      assert.equal((await f.sql`select id from users where id=${pending.id}`).length, 1)
      await page.getByRole('button', { name: '删除账号', exact: true }).click()
      await dialog.getByRole('button', { name: '删除账号', exact: true }).click()
      await page.waitForURL('**/admin/users')
      assert.equal((await f.sql`select id from users where id=${pending.id}`).length, 0)
      assert.deepEqual(errors, [])
    } finally {
      await browser?.close()
      if (deletedActor)
        await f.sql`delete from audit_logs where action='test.account.delete' and target_id=${deletedActor}`
      await f.cleanup()
    }
  }
)
