import test from 'node:test'
import assert from 'node:assert/strict'
import { createServer, request as httpRequest } from 'node:http'
import { once } from 'node:events'
import { createApp, defineEventHandler, setResponseHeader, setResponseStatus, createError, toNodeListener } from 'h3'
import { createJiti } from 'jiti'
const { guardApiRequest, applyResponseCachePolicy, markRenderedResponse, cookieVary, isApiPath } = await createJiti(import.meta.url).import('../server/utils/request-policy.ts')

async function fixture(run) {
  const mutations = []
  const app = createApp({ onBeforeResponse: applyResponseCachePolicy })
  app.use(defineEventHandler(guardApiRequest))
  app.use(defineEventHandler(event => {
    if (!['GET','HEAD','OPTIONS'].includes(event.method)) mutations.push(event.path)
    if (event.path === '/api/denied') throw createError({ statusCode:401, message:'No session' })
    if (event.path.startsWith('/asset.js')) {
      setResponseHeader(event,'Cache-Control','public, max-age=31536000, immutable')
      setResponseHeader(event,'Vary','Accept-Encoding')
      if (event.path.includes('?notModified')) { setResponseStatus(event,304); return '' }
      setResponseHeader(event,'Content-Type','application/javascript')
      return 'export default 1'
    }
    if (event.path === '/document') {
      setResponseHeader(event,'Content-Type','text/html; charset=utf-8')
      setResponseHeader(event,'Vary','Accept-Encoding')
      return '<main>Private document</main>'
    }
    if (event.path === '/rendered-payload') {
      markRenderedResponse(event)
      setResponseHeader(event,'Content-Type','application/json')
      return { account:'private' }
    }
    if (event.path === '/api/web-response' || event.path === '/web-document') {
      return new Response(event.path === '/web-document' ? '<main>Document</main>' : '{"ok":true}', {
        headers:{ 'Content-Type':event.path === '/web-document' ? 'text/html' : 'application/json', 'Cache-Control':'public, max-age=60', Vary:'Accept-Encoding' },
      })
    }
    if (event.path === '/api/override') {
      setResponseHeader(event,'Cache-Control','public, max-age=60')
      setResponseHeader(event,'Vary','Accept-Encoding, cookie, COOKIE')
    }
    return { reachedHandler:true }
  }))
  const server = createServer(toNodeListener(app))
  server.listen(0,'127.0.0.1'); await once(server,'listening')
  const base = 'http://127.0.0.1:' + server.address().port
  try { await run({ base,mutations }) } finally { await new Promise(resolve => server.close(resolve)) }
}

test('API protection covers every write namespace, decoded paths and future endpoints', async () => {
  await fixture(async ({ base,mutations }) => {
    for (const path of ['/api','/api/auth/logout','/api/admin/modules/id/publish','/api/admin/content/sync','/api/modules/m/chapters/c/complete','/api/new-feature/action','/%61pi/admin/modules/id/publish']) {
      for (const method of ['POST','PUT','PATCH','DELETE']) {
        const response = await fetch(base + path, { method,headers:{ origin:'https://elsewhere.invalid' } })
        assert.equal(response.status,403,method + ' ' + path)
        assert.equal(response.headers.get('cache-control'),'private, no-store')
      }
    }
    for (const headers of [{ origin:'null' }, { origin:base, 'sec-fetch-site':'cross-site' }, { origin:'https://elsewhere.invalid','sec-fetch-site':'same-site' }]) {
      assert.equal((await fetch(base+'/api/auth/login',{method:'POST',headers})).status,403)
    }
    assert.equal(mutations.length,0,'rejected operations never reached the handler')
    for (const headers of [{}, { origin:base }]) {
      assert.equal((await fetch(base+'/api/action',{method:'POST',headers})).status,200)
    }
    const proxyStatus = headers => new Promise((resolve,reject) => {
      const request = httpRequest(base+'/api/action',{method:'POST',headers},response=>{response.resume();resolve(response.statusCode)})
      request.on('error',reject);request.end()
    })
    assert.equal(await proxyStatus({host:'microsb.de','x-forwarded-proto':'https',origin:'https://microsb.de'}),200)
    assert.equal(await proxyStatus({host:'microsb.de','x-forwarded-host':'evil.test',origin:'http://evil.test'}),403)
    for (const method of ['GET','HEAD','OPTIONS']) assert.equal((await fetch(base+'/api/action',{method,headers:{ origin:'https://elsewhere.invalid' }})).status,200)
    assert.equal((await fetch(base+'/apiary',{method:'POST',headers:{origin:'https://elsewhere.invalid'}})).status,200)
  })
})

test('API and rendered documents are private; static asset caching stays intact', async () => {
  await fixture(async ({ base }) => {
    for (const path of ['/api/auth/me','/api/denied','/api/override','/api/web-response','/document','/web-document','/rendered-payload']) {
      const response = await fetch(base + path)
      assert.equal(response.headers.get('cache-control'),'private, no-store',path)
      const vary = response.headers.get('vary').split(',').map(value=>value.trim().toLowerCase())
      assert.equal(vary.filter(value=>value==='cookie').length,1,path)
      if (['/api/override','/api/web-response','/document','/web-document'].includes(path)) assert.ok(vary.includes('accept-encoding'))
    }
    for (const path of ['/asset.js','/asset.js?notModified=true']) {
      const response = await fetch(base + path)
      assert.match(response.headers.get('cache-control'),/public.*immutable/)
      assert.equal(response.headers.get('vary'),'Accept-Encoding')
    }
  })
})

test('cache variation preserves existing fields without duplicate Cookie tokens or prefix collisions', () => {
  assert.equal(cookieVary('Accept-Encoding','Cookie, cookie'),'Accept-Encoding, Cookie')
  assert.equal(cookieVary(['Accept-Encoding','Accept-Language'], 'COOKIE'),'Accept-Encoding, Accept-Language, COOKIE')
  assert.equal(cookieVary('Accept-Encoding, *'),'\u002a')
  for (const path of ['/api','/api/auth/me','/api?query=1']) assert.equal(isApiPath(path),true)
  for (const path of ['/apiary','/apiculture','/not-api?path=/api/me','/_nuxt/app.js']) assert.equal(isApiPath(path),false)
})
