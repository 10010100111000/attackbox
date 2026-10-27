import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import { learningFixture } from './helpers/learning-fixture.mjs'

test('Git-managed content lifecycle: retired legacy write endpoints, quiz normalization, prerequisite DAG and unpublished deletion', { skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.CONTENT_TEST_BASE_URL }, async () => {
  const f = await learningFixture(), suffix = crypto.randomUUID().slice(0, 8)
  const req = async (path, method = 'GET', body, status = 200, actor = f.admin) => (await f.request(path, { actor, method, body, status })).data
  try {
    await f.request('/api/admin/modules', { status: 401 })
    const learner = await f.account('user')
    await f.request('/api/admin/modules', { actor: learner, status: 403 })
    const retiredPost = await fetch(f.base + '/api/admin/modules', { method: 'POST', headers: { cookie: `attackbox_session=${f.admin.token}`, 'content-type': 'application/json' }, body: JSON.stringify({ title: 'Retired' }) })
    assert.ok([404, 405].includes(retiredPost.status), 'POST /api/admin/modules is retired')

    const [category] = await f.sql`insert into content_categories (slug,name) values (${`lifecycle-web-${suffix}`},'Lifecycle Web') returning id`
    const categoryId=category.id
    const slug = `lifecycle-${suffix}`
    let d = await f.createModule('draft', {
      title: 'Lifecycle module',
      slug,
      categoryId,
      learningOutcomes: ['使用 Shell 查看系统信息', '定位常见权限问题'],
      chapters: [
        { title: 'First', slug: 'first', access: 'public', bodyMarkdown: '# First', questions: [{ promptMarkdown: 'A command?', answer: 'whoami', normalization: 'trim' }] },
        { title: 'Second', slug: 'second', access: 'authenticated', bodyMarkdown: '# Second', questions: [] },
      ],
    })
    const id = d.module.id, path = `/api/admin/modules/${id}`, publicPath = `/api/modules/${slug}`
    const first = d.chapters[0].chapterId, second = d.chapters[1].chapterId, qid = d.chapters[0].questions[0].id
    assert.deepEqual(d.module.learningOutcomes, ['使用 Shell 查看系统信息', '定位常见权限问题'])
    assert.equal(d.category.name, 'Lifecycle Web'); assert.equal(d.module.status, 'unpublished')
    assert.equal(d.chapters[0].questions[0].answer, null, 'admin detail never returns plain answers')
    assert.equal(d.chapters[0].questions[0].hasAnswer, true)
    await f.request(publicPath, { status: 404 })

    await req(path + '/publish', 'POST', { lockVersion: d.module.lockVersion })
    d = await req(path); assert.equal(d.module.status, 'published')
    assert.equal((await req('/api/modules', 'GET', undefined, 200, null)).find(m => m.id === id).category.slug, `lifecycle-web-${suffix}`)
    assert.deepEqual((await req(publicPath, 'GET', undefined, 200, null)).module.learningOutcomes, d.module.learningOutcomes)

    const publicChapter = await req(publicPath + '/chapters/first', 'GET', undefined, 200, null)
    assert.equal(publicChapter.questions[0].id, qid)
    assert.equal('answerHmac' in publicChapter.questions[0], false)
    assert.equal('answer' in publicChapter.questions[0], false, 'public chapter never exposes the standard answer')
    await f.request(publicPath + '/chapters/second', { status: 401 })
    assert.equal((await req(publicPath, 'GET', undefined, 200, null)).chapters[1].isLocked, true)
    assert.equal((await req(publicPath + '/chapters/first/attempt', 'POST', { questionId: qid, answer: ' whoami ' }, 200, null)).correct, true)
    assert.equal((await req(publicPath + '/chapters/first/attempt', 'POST', { questionId: qid, answer: 'wrong' }, 200, null)).correct, false)

    // Syncing updated answer and normalization takes effect immediately while retaining question identity.
    d = await f.syncModule(id, { chapters: [{ ...d.chapters[0], questions: [{ id: qid, promptMarkdown: 'A command?', answer: 'id', normalization: 'exact' }] }, d.chapters[1]] })
    assert.equal((await req(publicPath + '/chapters/first/attempt', 'POST', { questionId: qid, answer: 'WHOAMI' }, 200, null)).correct, false)
    assert.equal((await req(publicPath + '/chapters/first/attempt', 'POST', { questionId: qid, answer: 'id' }, 200, null)).correct, true)
    assert.equal((await req(publicPath + '/chapters/first/attempt', 'POST', { questionId: qid, answer: ' id ' }, 200, null)).correct, false)

    // Reordering chapters and updating prose preserves chapter/question IDs.
    d = await f.syncModule(id, { title: 'Live title', chapters: [d.chapters[1], { ...d.chapters[0], bodyMarkdown: '# Immediately updated', questions: [{ id: qid, promptMarkdown: 'A command?', answer: 'WHOAMI', normalization: 'trim_casefold' }] }] })
    assert.deepEqual(d.chapters.map(c => c.chapterId), [second, first])
    assert.equal((await req(publicPath, 'GET', undefined, 200, null)).module.title, 'Live title')
    assert.equal((await req(publicPath + '/chapters/first', 'GET', undefined, 200, null)).chapter.bodyMarkdown, '# Immediately updated')
    assert.equal(d.chapters[1].questions[0].id, qid)

    // Published modules cannot be deleted directly until unpublished.
    await req(path, 'DELETE', { lockVersion: d.module.lockVersion }, 409)
    await req(path + '/unpublish', 'POST', { lockVersion: d.module.lockVersion })
    d = await req(path); assert.equal(d.module.status, 'unpublished')
    await f.request(publicPath, { status: 404 })
    await req(path, 'DELETE', { lockVersion: d.module.lockVersion })
    await f.request(path, { actor: f.admin, status: 404 })
    assert.equal((await f.sql`SELECT id FROM modules WHERE id=${id}`).length, 0)

    // Prerequisite graph and concurrent cycle prevention via /runtime.
    const make = (name, intent = 'publish') => f.createModule(intent, {
      title: name,
      slug: `${name}-${suffix}`,
      chapters: [{ title: name, slug: 'intro', access: 'public', bodyMarkdown: '# Intro', durationMinutes: 15, questions: [] }],
    })
    const pa = await make('prereq-a'), pb = await make('prereq-b'), pc = await make('prereq-c', 'draft')
    const graphPath = `/api/admin/modules/${pa.module.id}`, graphRuntime = `${graphPath}/runtime`
    let graph = await req(graphPath)
    await req(graphRuntime, 'PUT', { lockVersion: graph.module.lockVersion, prerequisiteModuleIds: [pb.module.id, pc.module.id] })
    graph = await req(graphPath)
    assert.deepEqual(graph.prerequisites.map(p => p.id), [pb.module.id, pc.module.id])
    const publicGraph = await req(`/api/modules/prereq-a-${suffix}`, 'GET', undefined, 200, null)
    assert.deepEqual(publicGraph.prerequisites.map(p => p.id), [pb.module.id], 'draft prerequisites must never leak')
    assert.equal(publicGraph.chapters[0].durationMinutes, 15)
    await req(graphRuntime, 'PUT', { lockVersion: graph.module.lockVersion, prerequisiteModuleIds: [pa.module.id] }, 400)
    await req(graphRuntime, 'PUT', { lockVersion: graph.module.lockVersion, prerequisiteModuleIds: [pb.module.id, pb.module.id] }, 400)
    await req(graphRuntime, 'PUT', { lockVersion: graph.module.lockVersion, prerequisiteModuleIds: [crypto.randomUUID()] }, 400)

    const bp = `/api/admin/modules/${pb.module.id}`
    let bdetail = await req(bp)
    await req(`${bp}/runtime`, 'PUT', { lockVersion: bdetail.module.lockVersion, prerequisiteModuleIds: [pa.module.id] }, 400)
    assert.deepEqual((await req(bp)).prerequisites, [], 'cycle failure rolls back')

    let cdetail = await req(`/api/admin/modules/${pc.module.id}`)
    await req(`/api/admin/modules/${pc.module.id}/publish`, 'POST', { lockVersion: cdetail.module.lockVersion })
    assert.deepEqual((await req(`/api/modules/prereq-a-${suffix}`, 'GET', undefined, 200, null)).prerequisites.map(p => p.id), [pb.module.id, pc.module.id])

    bdetail = await req(bp)
    await req(`${bp}/unpublish`, 'POST', { lockVersion: bdetail.module.lockVersion })
    bdetail = await req(bp)
    await req(bp, 'DELETE', { lockVersion: bdetail.module.lockVersion })
    assert.deepEqual((await req(graphPath)).prerequisites.map(p => p.id), [pc.module.id], 'deleting a target removes the recommendation')

    graph = await req(graphPath)
    await req(graphRuntime, 'PUT', { lockVersion: graph.module.lockVersion, prerequisiteModuleIds: [] })
    graph = await req(graphPath); cdetail = await req(`/api/admin/modules/${pc.module.id}`)
    const opposing = await Promise.all([
      [graphRuntime, graph, pc.module.id],
      [`/api/admin/modules/${pc.module.id}/runtime`, cdetail, pa.module.id],
    ].map(async ([rPath, current, targetId]) =>
      (await fetch(f.base + rPath, { method: 'PUT', headers: { cookie: `attackbox_session=${f.admin.token}`, 'content-type': 'application/json' }, body: JSON.stringify({ lockVersion: current.module.lockVersion, prerequisiteModuleIds: [targetId] }) })).status))
    assert.deepEqual(opposing.sort(), [200, 400], 'concurrent graph edits remain acyclic')
  } finally { await f.sql`delete from modules where category_id in (select id from content_categories where slug=${`lifecycle-web-${suffix}`})`;await f.sql`delete from content_categories where slug=${`lifecycle-web-${suffix}`}`;await f.cleanup() }
})
