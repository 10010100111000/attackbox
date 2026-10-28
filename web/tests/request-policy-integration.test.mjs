import test from 'node:test'
import assert from 'node:assert/strict'
import { readdir } from 'node:fs/promises'
import { learningFixture } from './helpers/learning-fixture.mjs'

test('request policy blocks uncovered mutations, preserves authorization and protects dynamic responses', { skip:!process.env.CONTENT_TEST_DATABASE_URL, timeout:60000 }, async () => {
  const f = await learningFixture()
  const request = (path, actor, method = 'GET', body, headers = {}) => fetch(f.base + path, {
    method, headers:{ ...(actor ? { cookie:`attackbox_session=${actor.token}` } : {}), ...(body ? { 'content-type':'application/json' } : {}), ...headers },
    body:body ? JSON.stringify(body) : undefined,
  })
  const assertPrivate = (response, path) => {
    assert.equal(response.headers.get('cache-control'),'private, no-store',path)
    const fields = response.headers.get('vary').split(',').map(value=>value.trim().toLowerCase())
    assert.equal(fields.filter(value=>value==='cookie').length,1,path)
  }
  try {
    const learner = await f.account(), other = await f.account()
    const module = await f.createModule('draft', { chapters:[{ title:'Policy chapter',slug:'intro',access:'public',bodyMarkdown:'## Read and finish',questions:[] }] })
    const adminPath = '/api/admin/modules/' + module.module.id
    const chapterPath = `/api/modules/${module.module.slug}/chapters/intro`
    const foreign = { origin:'https://elsewhere.invalid' }

    for (const [path,method,actor,body] of [
      [adminPath+'/publish','POST',f.admin,{lockVersion:module.module.lockVersion}],
      [adminPath,'DELETE',f.admin,{lockVersion:module.module.lockVersion}],
      ['/api/admin/content/sync','POST',f.admin,{}],
      ['/api/admin/content/repository','PUT',f.admin,{}],
      [chapterPath+'/complete','POST',learner,{}],
      ['/api/auth/logout','POST',learner,{}],
      ['/api/sandbox/start','POST',learner,{}],
      ['/api/targets/any-instance/stop','POST',learner,{}],
      ['/api/auth/login','POST',null,{}],
      ['/api/feature-not-yet-registered/action','PATCH',f.admin,{}],
      ['/%61pi/auth/logout','POST',learner,{}],
    ]) {
      const response = await request(path,actor,method,body,foreign)
      assert.equal(response.status,403,method+' '+path)
      assertPrivate(response,path)
    }
    assert.equal((await f.sql`select status from modules where id=${module.module.id}`)[0].status,'unpublished')
    assert.equal((await f.sql`select * from chapter_progress where user_id=${learner.id}`).length,0)
    assert.equal((await request('/api/auth/me',learner)).status,200,'cross-origin logout did not revoke the session')
    assert.equal((await request('/api/admin/modules',learner)).status,403)
    assert.equal((await request('/api/me/profile',null)).status,401)

    const sameSite = await request(adminPath+'/publish',f.admin,'POST',{lockVersion:module.module.lockVersion},{origin:f.base})
    assert.equal(sameSite.status,200)
    const complete = await request(chapterPath+'/complete',learner,'POST',{}, {origin:f.base})
    assert.equal(complete.status,200)
    assert.ok((await f.sql`select completed_at from chapter_progress where user_id=${learner.id}`)[0].completed_at)

    for (const [path,actor] of [
      ['/api/auth/me',learner], ['/api/auth/me',other], ['/api/auth/me',null], ['/api/admin/modules',f.admin],
      ['/api/admin/content/template',f.admin], ['/api/modules',learner], ['/api/me/learning',learner],
      ['/api/landing',null], ['/api/auth/challenge',null],
    ]) assertPrivate(await request(path,actor),path)
    assert.equal((await (await request('/api/auth/me',learner)).json()).user.id,learner.id)
    assert.equal((await (await request('/api/auth/me',other)).json()).user.id,other.id)

    for (const path of ['/','/learn','/references','/me','/admin/modules','/admin/feedback','/auth','/term','/gui','/challenges',`/learn/modules/${module.module.slug}`]) {
      const response = await request(path,f.admin)
      assertPrivate(response,path)
      await response.arrayBuffer()
    }
    // Runtime pages use inline SSR payloads, not a /me/_payload.json route.
    const missingPayload = await request('/me/_payload.json',f.admin)
    assert.equal(missingPayload.status,404)
    const asset = (await readdir(new URL('../.output/public/_nuxt/',import.meta.url))).find(file=>file.endsWith('.js'))
    assert.ok(asset,'built static JavaScript asset exists')
    const staticResponse = await request('/_nuxt/'+asset,learner)
    assert.equal(staticResponse.status,200)
    assert.match(staticResponse.headers.get('cache-control'),/public.*immutable/)
    assert.ok(!staticResponse.headers.get('vary')?.toLowerCase().includes('cookie'))
    await staticResponse.arrayBuffer()
    const etag = staticResponse.headers.get('etag')
    if (etag) {
      const cached = await request('/_nuxt/'+asset,learner,'GET',undefined,{'if-none-match':etag})
      assert.equal(cached.status,304)
      assert.match(cached.headers.get('cache-control'),/public.*immutable/)
      assert.ok(!cached.headers.get('vary')?.toLowerCase().includes('cookie'))
    }
    const logout = await request('/api/auth/logout',learner,'POST',{}, {origin:f.base})
    assert.equal(logout.status,200)
    assert.equal((await request('/api/auth/me',learner)).status,401)
  } finally { await f.cleanup() }
})
