import test from 'node:test'
import assert from 'node:assert/strict'
import { createJiti } from 'jiti'
import { learningFixture } from './helpers/learning-fixture.mjs'
test('chapters, questions and challenge stages swap positions directly; invalid final order rolls back', {skip:!process.env.CONTENT_TEST_DATABASE_URL},async()=>{
 process.env.DATABASE_URL=process.env.CONTENT_TEST_DATABASE_URL
 const f=await learningFixture(),jiti=createJiti(import.meta.url)
 const database=await jiti.import('../server/db/index.ts'),content=await jiti.import('../server/services/content.ts')
 let challengeId
 try{
  const constraints=await f.sql`select conname,condeferrable,condeferred from pg_constraint where conname in ('chapters_module_position_unique','chapter_questions_position_unique','challenge_stages_challenge_id_position_unique')`
  assert.equal(constraints.length,3);assert.ok(constraints.every(c=>c.condeferrable&&c.condeferred))
  assert.equal((await f.sql`select to_regclass('public.content_previews') as name`)[0].name,null)
  const d=await f.createModule('draft')
  const reordered=d.chapters.toReversed().map(ch=>({...ch,questions:ch.questions.toReversed()}))
  await database.db.transaction(tx=>content.saveModuleInTransaction(tx,d.module.id,f.admin.id,{lockVersion:d.module.lockVersion,chapters:reordered}))
  const saved=(await f.request(`/api/admin/modules/${d.module.id}`,{actor:f.admin})).data
  assert.deepEqual(saved.chapters.map(ch=>ch.chapterId),reordered.map(ch=>ch.chapterId))
  const first=saved.chapters.at(-1)
  assert.deepEqual(first.questions.map(q=>q.id),d.chapters[0].questions.toReversed().map(q=>q.id))
  await assert.rejects(f.sql.begin(tx=>tx`update chapter_questions set position=1 where chapter_id=${first.chapterId}`),e=>e.code==='23505')
  assert.deepEqual((await f.sql`select id from chapter_questions where chapter_id=${first.chapterId} order by position`).map(q=>q.id),first.questions.map(q=>q.id))
  const body={title:'Order fixture',summary:'',bodyMarkdown:'Practice',category:'',difficulty:'beginner',intent:'draft',stages:[{title:'A',promptMarkdown:'A',hint:'',flag:'a'},{title:'B',promptMarkdown:'B',hint:'',flag:'b'}]}
  challengeId=(await f.request('/api/admin/challenges',{actor:f.admin,method:'POST',body})).data.id
  const path=`/api/admin/challenges/${challengeId}`
  const before=(await f.request(path,{actor:f.admin})).data
  await f.request(path,{actor:f.admin,method:'PUT',body:{...body,lockVersion:before.lockVersion,stages:before.stages.toReversed()}})
  assert.deepEqual((await f.request(path,{actor:f.admin})).data.stages.map(s=>s.id),before.stages.toReversed().map(s=>s.id))
 }finally{if(challengeId)await f.sql`delete from challenges where id=${challengeId}`;await f.cleanup();await database.client.end()}
})
