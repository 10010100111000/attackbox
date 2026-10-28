import test from 'node:test'
import assert from 'node:assert/strict'
import { randomUUID, createHash } from 'node:crypto'
import { createJiti } from 'jiti'
import { learningFixture } from './helpers/learning-fixture.mjs'

test('Git sync stores one plain answer, applies edits immediately and keeps progress and learner responses private', { skip:!process.env.CONTENT_TEST_DATABASE_URL, timeout:60000 }, async () => {
  process.env.DATABASE_URL = process.env.CONTENT_TEST_DATABASE_URL
  const f = await learningFixture(), jiti = createJiti(import.meta.url)
  const { syncRepositoryModules } = await jiti.import('../server/services/content-sync.ts')
  const database = await jiti.import('../server/db/index.ts')
  const oldFetch = globalThis.$fetch
  const sourcePath = `Answers-${randomUUID()}.md`
  const config = { repository:'fixture/plain-answers', branch:'main', directory:'' }
  const commit = 'a'.repeat(40), tree = 'b'.repeat(40)
  let moduleId, answer = 'whoami', normalization = 'trim_casefold'
  const sync = async () => {
    const markdown = `---\nsummary: Answer storage fixture\naccess: public\n---\n\n# Introduction\n\nRead and practice.\n\n\`\`\`quiz\nprompt: Which command?\nanswer: ${answer}\nnormalization: ${normalization}\n\`\`\`\n`
    const sha = createHash('sha1').update(markdown).digest('hex')
    globalThis.$fetch = async url => {
      const suffix = String(url).split('/repos/fixture/plain-answers/')[1]
      if (suffix === 'commits/main') return { sha:commit }
      if (suffix === `git/commits/${commit}`) return { tree:{sha:tree} }
      if (suffix === `git/trees/${tree}?recursive=1`) return { tree:[{path:sourcePath,type:'blob',mode:'100644',sha,size:Buffer.byteLength(markdown)}] }
      if (suffix === `git/blobs/${sha}`) return { encoding:'base64',content:Buffer.from(markdown).toString('base64') }
      throw new Error('Unexpected GitHub fixture request: '+url)
    }
    return syncRepositoryModules(config, 'fixture-token', f.admin.id, { sourcePath })
  }
  try {
    moduleId = (await sync()).synced[0].moduleId
    const adminPath = `/api/admin/modules/${moduleId}`
    const detail = (await f.request(adminPath,{actor:f.admin})).data
    const question = detail.chapters[0].questions[0]
    assert.equal(question.hasAnswer,true)
    assert.equal((await f.sql`select answer from chapter_questions where id=${question.id}`)[0].answer,'whoami')
    await f.request(adminPath+'/publish',{actor:f.admin,method:'POST',body:{lockVersion:detail.module.lockVersion}})
    const chapterPath = `/api/modules/${detail.module.slug}/chapters/${detail.chapters[0].slug}`
    const learner = await f.account()
    const attempt = async value => (await f.request(chapterPath+'/attempt',{actor:learner,method:'POST',body:{questionId:question.id,answer:value}})).data
    assert.equal((await attempt('wrong')).correct,false)
    const firstPass = await attempt(' WHOAMI ')
    assert.equal(firstPass.correct,true)
    assert.ok(firstPass.progress.completedAt)
    answer = 'id'; normalization = 'exact'
    assert.equal((await sync()).synced[0].moduleId,moduleId)
    assert.equal((await attempt('whoami')).correct,false)
    assert.equal((await attempt(' id ')).correct,false)
    const changedPass = await attempt('id')
    assert.equal(changedPass.correct,true)
    assert.equal(changedPass.progress.completedAt,firstPass.progress.completedAt)
    assert.equal((await f.sql`select answer from chapter_questions where id=${question.id}`)[0].answer,'id')
    for (const actor of [undefined,learner]) {
      const chapter = (await f.request(chapterPath,{actor})).data
      assert.ok(!('answer' in chapter.questions[0]))
    }
    const html = await (await fetch(f.base+chapterPath)).text()
    assert.ok(!html.includes('"answer":'))
  } finally {
    globalThis.$fetch = oldFetch
    if (moduleId) await f.sql`delete from modules where id=${moduleId}`
    await f.cleanup()
    await database.client.end()
  }
})
