import test from 'node:test'
import assert from 'node:assert/strict'
import { learningFixture } from './helpers/learning-fixture.mjs'
test(
  'learning progress: authorization, persistence, concurrent completion, updates and deletion',
  { skip: !process.env.CONTENT_TEST_DATABASE_URL || !process.env.CONTENT_TEST_BASE_URL },
  async () => {
    const f = await learningFixture()
    try {
      const user = await f.account(),
        other = await f.account(),
        concurrent = await f.account()
      let d = await f.createModule()
      const modulePath = `/api/modules/${d.module.slug}`,
        first = `${modulePath}/chapters/intro`,
        second = `${modulePath}/chapters/navigation`,
        preview = `${modulePath}/chapters/preview`,
        last = `${modulePath}/chapters/permissions`
      const q = d.chapters[0].questions,
        req = async (path, options = {}) => (await f.request(path, options)).data
      const attempt = (question, answer, actor = user) =>
        req(first + '/attempt', {
          actor,
          method: 'POST',
          body: { questionId: question.id, answer }
        })
      const finish = (path, actor = user, status = 200) =>
        req(path + '/complete', {
          actor,
          method: 'POST',
          status,
          body: { completed: true, userId: user.id }
        })
      await req('/api/me/learning', { status: 401 })
      assert.deepEqual(await req('/api/me/learning', { actor: user }), [])
      const guest = await req(modulePath)
      assert.equal(guest.progress, null)
      assert.equal(guest.chapters[1].lockReason, 'login')
      assert.equal((await req(modulePath, { actor: user })).chapters[1].lockReason, 'prerequisite')
      await req(second, { status: 401 })
      await req(second, { actor: user, status: 403 })
      for (const suffix of ['/visit', '/complete', '/attempt'])
        await req(last + suffix, {
          actor: user,
          method: 'POST',
          body: { questionId: q[0].id, answer: 'whoami' },
          status: 403
        })
      await req(preview)
      await req(preview, { actor: user })
      await finish(first, user, 409)
      assert.equal((await attempt(q[0], 'WHOAMI', null)).correct, true)
      assert.equal(
        (await req(first, { actor: user })).progress.passedQuestionIds.length,
        0,
        'guest attempts never save account progress'
      )
      await attempt(q[0], 'wrong')
      await Promise.all([attempt(q[0], 'WHOAMI'), attempt(q[0], ' whoami ')])
      assert.equal((await req(first, { actor: user })).progress.passedQuestionIds.length, 1)
      await finish(first, user, 409)
      const completed = await attempt(q[1], 'pwd')
      assert.ok(completed.progress.completedAt)
      assert.equal((await req(modulePath, { actor: user })).chapters[1].isLocked, false)
      assert.equal((await req(modulePath, { actor: other })).chapters[1].isLocked, true)
      assert.equal((await req(first, { actor: user })).progress.passedQuestionIds.length, 2)
      const publicResponse = await f.request(first)
      assert.ok(!('answer' in publicResponse.data.questions[0]))
      assert.ok(!('answerHmac' in publicResponse.data.questions[0]))
      assert.match(publicResponse.response.headers.get('cache-control'), /no-store/)
      assert.match(publicResponse.response.headers.get('vary'), /Cookie/i)
      await Promise.all([attempt(q[0], 'whoami', concurrent), attempt(q[1], 'pwd', concurrent)])
      assert.ok(
        (await req(first, { actor: concurrent })).progress.completedAt,
        'parallel final answers finish the chapter atomically'
      )
      const stamp = (await finish(second)).progress.completedAt
      await Promise.all([finish(second), finish(second)])
      assert.equal((await req(second, { actor: user })).progress.completedAt, stamp)
      const [before] =
        await f.sql`SELECT started_at FROM chapter_progress WHERE user_id=${user.id} AND chapter_id=${d.chapters[1].chapterId}`
      await req(second + '/visit', { actor: user, method: 'POST' })
      const [after] =
        await f.sql`SELECT started_at FROM chapter_progress WHERE user_id=${user.id} AND chapter_id=${d.chapters[1].chapterId}`
      assert.deepEqual(after.started_at, before.started_at)
      const history = await req('/api/me/learning', { actor: user })
      assert.equal(history[0].progress.completedChapters, 2)
      assert.equal(history[0].progress.resumeChapterSlug, 'preview')
      await req(last, { actor: user, status: 403 })
      await finish(preview)
      await finish(last)
      assert.equal((await req(modulePath, { actor: user })).progress.isCompleted, true)
      // Syncing updated answers and new questions preserves completed chapter records.
      d = await f.syncModule(d.module.id, {
        chapters: [
          {
            ...d.chapters[0],
            questions: [
              { ...q[0], answer: 'id' },
              { ...q[1], answer: 'pwd' },
              { promptMarkdown: 'New question', answer: 'new', normalization: 'trim' }
            ]
          },
          ...d.chapters.slice(1)
        ]
      })
      assert.equal(
        (await req(first, { actor: user })).progress.completedAt,
        completed.progress.completedAt
      )
      assert.equal((await attempt(q[0], 'whoami')).correct, false)
      assert.equal((await attempt(q[0], 'id')).correct, true)
      // Inserting an earlier private chapter re-locks unfinished later chapters, not completed ones.
      d = await f.syncModule(d.module.id, {
        chapters: [
          {
            title: '新增基础',
            slug: 'new-foundation',
            access: 'authenticated',
            bodyMarkdown: '# Foundation',
            questions: []
          },
          ...d.chapters
        ]
      })
      assert.equal(
        (await req(modulePath, { actor: user })).chapters.find(c => c.slug === 'permissions')
          .isLocked,
        false
      )
      assert.equal(
        (await req(modulePath, { actor: concurrent })).chapters.find(c => c.slug === 'navigation')
          .lockReason,
        'prerequisite'
      )
      await req(second, { actor: user })
      d = (await f.request(`/api/admin/modules/${d.module.id}`, { actor: f.admin })).data
      await req(`/api/admin/modules/${d.module.id}/unpublish`, {
        actor: f.admin,
        method: 'POST',
        body: { lockVersion: d.module.lockVersion }
      })
      await req(first, { actor: user, status: 404 })
      await finish(second, user, 404)
      await req(first + '/visit', { actor: user, method: 'POST', status: 404 })
      assert.deepEqual(await req('/api/me/learning', { actor: user }), [])
      // Deleting content cascades through both progress tables.
      await f.sql`DELETE FROM modules WHERE id=${d.module.id}`
      assert.equal((await f.sql`SELECT * FROM chapter_progress WHERE user_id=${user.id}`).length, 0)
      assert.equal(
        (await f.sql`SELECT * FROM question_completions WHERE user_id=${user.id}`).length,
        0
      )
    } finally {
      await f.cleanup()
    }
  }
)
