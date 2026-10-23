import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import postgres from 'postgres'
const databaseUrl = process.env.CONTENT_TEST_DATABASE_URL, baseUrl = process.env.CONTENT_TEST_BASE_URL

test('direct content lifecycle: stable IDs, answers, atomic saves, immediate updates, permissions and deletion', { skip: !databaseUrl || !baseUrl }, async () => {
  const target = new URL(databaseUrl)
  assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(target.hostname))
  assert.ok(target.pathname.endsWith('_test'))
  assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(new URL(baseUrl).hostname))
  const sql = postgres(databaseUrl, { max: 1 }), suffix = crypto.randomUUID().slice(0, 8)
  let cookie
  async function req(path, method = 'GET', body, status = 200, auth = true) {
    const res = await fetch(baseUrl + path, { method, headers: { ...(auth && cookie ? { cookie } : {}), ...(body ? { 'content-type': 'application/json' } : {}) }, body: body ? JSON.stringify(body) : undefined })
    const json = await res.json(); assert.equal(res.status, status, `${method} ${path}: ${JSON.stringify(json)}`); return json.data
  }
  try {
    const uid = crypto.randomUUID(), token = crypto.randomBytes(32).toString('hex')
    await sql`INSERT INTO users (id,email,display_name,role,status,email_verified_at) VALUES (${uid},${`content-${suffix}@example.test`},'Content Test','admin','active',now())`
    await sql`INSERT INTO auth_sessions (id,user_id,token_hash,expires_at) VALUES (${crypto.randomUUID()},${uid},${crypto.createHash('sha256').update(token).digest()},now()+interval '1 hour')`
    cookie = `attackbox_session=${token}`
    await req('/api/admin/modules', 'GET', undefined, 401, false)
    await sql`UPDATE users SET role='user' WHERE id=${uid}`
    await req('/api/admin/modules', 'GET', undefined, 403)
    await sql`UPDATE users SET role='admin' WHERE id=${uid}`
    const categories = await req('/api/categories'), categoryId = categories.find(c => c.slug === 'web-security').id
    const custom = await req('/api/admin/categories', 'POST', { name: `Custom ${suffix}` }); assert.ok(custom.id)
    await req('/api/admin/categories', 'POST', { name: `Custom ${suffix}` }, 409)
    const slug = `lifecycle-${suffix}`
    const created = await req('/api/admin/modules', 'POST', { title: 'Lifecycle module', slug, categoryId, chapters: [
      { title: 'First', slug: 'first', access: 'public', bodyMarkdown: '# First', questions: [{ promptMarkdown: 'A command?', answer: 'whoami', normalization: 'trim' }] },
      { title: 'Second', slug: 'second', access: 'authenticated', bodyMarkdown: '# Second', questions: [] },
    ] })
    const id = created.module.id, path = `/api/admin/modules/${id}`, publicPath = `/api/modules/${slug}`
    const detail = () => req(path)
    let d = await detail()
    const first = d.chapters[0].chapterId, second = d.chapters[1].chapterId, qid = d.chapters[0].questions[0].id
    async function save(patch, status = 200) { return req(path, 'PUT', { lockVersion: d.module.lockVersion, ...patch }, status) }
    async function action(name, status = 200) { return req(path + '/' + name, 'POST', { lockVersion: d.module.lockVersion }, status) }
    const hash = async () => (await sql`SELECT answer_hmac FROM chapter_questions WHERE id=${qid}`)[0]?.answer_hmac
    const before = await hash()
    assert.equal(d.category.name, 'Web 安全'); assert.equal(d.module.status, 'unpublished')
    assert.equal('draft' in d, false); assert.equal('publishedRevisionId' in d.module, false)
    let item = (await req('/api/admin/modules')).find(m => m.id === id)
    assert.equal(item.chapterCount, 2); assert.equal(item.title, 'Lifecycle module')
    await req(publicPath, 'GET', undefined, 404, false)
    await save({ chapters: d.chapters.map(c => ({ ...c, questions: undefined })) }); d = await detail()
    assert.deepEqual(await hash(), before, 'omitting questions preserves answers')
    await save({ chapters: d.chapters.map(c => ({ ...c, questions: c.questions.map(q => ({ ...q, promptMarkdown: 'Updated prompt' })) })) }); d = await detail()
    assert.equal(d.chapters[0].questions[0].id, qid); assert.deepEqual(await hash(), before)
    assert.equal('answerHmac' in d.chapters[0].questions[0], false)
    let question = d.chapters[0].questions[0]
    await save({ chapters: [{ ...d.chapters[0], questions: [{ ...question, normalization: 'lowercase' }] }, d.chapters[1]] }, 400)
    await save({ chapters: [{ ...d.chapters[0], questions: [{ ...question, normalization: 'trim_casefold' }] }, d.chapters[1]] }, 400)
    await save({ chapters: [{ ...d.chapters[0], questions: [{ ...question, normalization: 'trim_casefold', answer: 'WHOAMI' }] }, d.chapters[1]] }); d = await detail()
    const races = await Promise.all([1, 2].map(async i => (await fetch(baseUrl + path, { method: 'PUT', headers: { cookie, 'content-type': 'application/json' }, body: JSON.stringify({ lockVersion: d.module.lockVersion, summary: `writer ${i}` }) })).status))
    assert.deepEqual(races.sort(), [200, 409]); d = await detail()
    await req(path, 'PUT', { title: 'Missing version' }, 409)
    await save({ title: 'Should roll back', chapters: [{ ...d.chapters[0], title: 'Also rollback' }, { ...d.chapters[1], questions: [{ promptMarkdown: 'Bad', normalization: 'number' }] }] }, 400)
    d = await detail(); assert.equal(d.module.title, 'Lifecycle module'); assert.equal(d.chapters[0].title, 'First')
    await save({ chapters: [{ ...d.chapters[0], questions: [{ ...d.chapters[0].questions[0], clearAnswer: true }] }, d.chapters[1]] }); d = await detail()
    assert.equal(d.chapters[0].questions[0].hasAnswer, false)
    await action('publish', 400)
    await save({ chapters: [{ ...d.chapters[0], questions: [{ ...d.chapters[0].questions[0], answer: 'WHOAMI' }] }, d.chapters[1]] }); d = await detail()
    await action('publish'); d = await detail(); assert.equal(d.module.status, 'published')
    assert.equal((await req('/api/modules', 'GET', undefined, 200, false)).find(m => m.id === id).category.slug, 'web-security')
    const publicChapter = await req(publicPath + '/chapters/first', 'GET', undefined, 200, false)
    assert.equal(publicChapter.questions[0].id, qid); assert.equal('answerHmac' in publicChapter.questions[0], false)
    await req(publicPath + '/chapters/second', 'GET', undefined, 401, false)
    assert.equal((await req(publicPath, 'GET', undefined, 200, false)).chapters[1].isLocked, true)
    assert.equal((await req(publicPath + '/chapters/first/attempt', 'POST', { questionId: qid, answer: ' whoami ' }, 200, false)).correct, true)
    assert.equal((await req(publicPath + '/chapters/first/attempt', 'POST', { questionId: qid, answer: 'wrong' }, 200, false)).correct, false)
    await save({ chapters: [d.chapters[1], { ...d.chapters[0], bodyMarkdown: '# Immediately updated' }], title: 'Live title' }); d = await detail()
    assert.deepEqual(d.chapters.map(c => c.chapterId), [second, first])
    assert.equal((await req(publicPath, 'GET', undefined, 200, false)).module.title, 'Live title')
    assert.equal((await req(publicPath + '/chapters/first', 'GET', undefined, 200, false)).chapter.bodyMarkdown, '# Immediately updated')
    assert.equal(d.chapters[1].questions[0].id, qid, 'saving and publishing retain question identity')
    await save({ title: 'Invalid live update', chapters: [{ ...d.chapters[0], bodyMarkdown: '' }, d.chapters[1]] }, 400)
    await save({ chapters: [] }, 400)
    await save({ chapters: [d.chapters[0], { ...d.chapters[1], questions: [{ ...d.chapters[1].questions[0], clearAnswer: true }] }] }, 400)
    await save({ chapters: [{ ...d.chapters[0], chapterId: crypto.randomUUID() }, d.chapters[1]] }, 409)
    await save({ chapters: [d.chapters[0], { ...d.chapters[1], questions: [{ ...d.chapters[1].questions[0], id: crypto.randomUUID() }] }] }, 409)
    d = await detail(); assert.equal(d.module.title, 'Live title'); assert.equal(d.chapters.length, 2)
    await save({ chapters: [...d.chapters, { title: 'Third', slug: 'third', access: 'public', bodyMarkdown: '# Third', questions: [] }] }); d = await detail()
    assert.equal((await req(publicPath)).chapters.length, 3, 'new chapter appears immediately and existing ones remain')
    await save({ chapters: [d.chapters[0], d.chapters[0]] }, 400)
    await action('unpublish'); d = await detail()
    assert.equal(d.module.status, 'unpublished'); assert.equal(d.chapters.length, 3)
    await req(publicPath, 'GET', undefined, 404, false)
    await req(publicPath + '/chapters/first/attempt', 'POST', { questionId: qid, answer: 'whoami' }, 404, false)
    await save({ chapters: [d.chapters[1]] }); d = await detail()
    assert.equal(d.chapters.length, 1); assert.equal(d.chapters[0].chapterId, first)
    assert.equal((await sql`SELECT id FROM chapters WHERE id=${second}`).length, 0)
    await action('publish'); d = await detail()
    await req(path, 'DELETE', { lockVersion: d.module.lockVersion })
    await req(path, 'GET', undefined, 404)
    assert.equal((await sql`SELECT id FROM modules WHERE id=${id}`).length, 0)
    assert.equal((await sql`SELECT id FROM chapters WHERE module_id=${id}`).length, 0)
    assert.equal((await sql`SELECT id FROM chapter_questions WHERE id=${qid}`).length, 0)
    assert.equal((await req('/api/admin/modules')).some(m => m.id === id), false)
    const neverPublished = await req('/api/admin/modules', 'POST', { title: 'Unsaved content', slug: `delete-${suffix}` })
    await req(`/api/admin/modules/${neverPublished.module.id}`, 'DELETE', { lockVersion: 1 })
    await req(`/api/admin/modules/${neverPublished.module.id}`, 'GET', undefined, 404)
  } finally { await sql.end() }
})
