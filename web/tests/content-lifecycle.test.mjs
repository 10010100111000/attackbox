import test from 'node:test'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import postgres from 'postgres'
const databaseUrl=process.env.CONTENT_TEST_DATABASE_URL, baseUrl=process.env.CONTENT_TEST_BASE_URL
// Destructive fixtures require an explicit isolated local database and server.
test('content lifecycle regression: answers, versions, contracts, order, rollback and categories', {skip: !databaseUrl||!baseUrl}, async()=>{
  const target=new URL(databaseUrl)
  assert.ok(['127.0.0.1','localhost','[::1]'].includes(target.hostname))
  assert.ok(target.pathname.endsWith('_test'))
  assert.ok(['127.0.0.1','localhost','[::1]'].includes(new URL(baseUrl).hostname))
  const sql=postgres(databaseUrl,{max:1}), suffix=crypto.randomUUID().slice(0,8)
  let cookie
  async function req(path, method='GET', body, status=200, auth=true){
    const res=await fetch(baseUrl+path,{method,headers:{...(auth&&cookie?{cookie}:{}),...(body?{'content-type':'application/json'}:{})},body:body?JSON.stringify(body):undefined})
    const json=await res.json();assert.equal(res.status,status,`${method} ${path}: ${JSON.stringify(json)}`);return json.data
  }
  try{
    const uid=crypto.randomUUID(),token=crypto.randomBytes(32).toString('hex')
    await sql`INSERT INTO users (id,email,display_name,role,status,email_verified_at) VALUES (${uid},${`content-${suffix}@example.test`},'Content Test','admin','active',now())`
    await sql`INSERT INTO auth_sessions (id,user_id,token_hash,expires_at) VALUES (${crypto.randomUUID()},${uid},${crypto.createHash('sha256').update(token).digest()},now()+interval '1 hour')`
    cookie=`attackbox_session=${token}`
    await req('/api/admin/modules','GET',undefined,401,false)
    await sql`UPDATE users SET role='user' WHERE id=${uid}`
    await req('/api/admin/modules','GET',undefined,403)
    await sql`UPDATE users SET role='admin' WHERE id=${uid}`
    const categories=await req('/api/categories'),categoryId=categories.find(c=>c.slug==='web-security').id
    const custom=await req('/api/admin/categories','POST',{name:`Custom ${suffix}`});assert.ok(custom.id)
    await req('/api/admin/categories','POST',{name:`Custom ${suffix}`},409)
    const created=await req('/api/admin/modules','POST',{title:'Lifecycle module',slug:`lifecycle-${suffix}`,categoryId,chapters:[
      {title:'First',slug:'first',access:'public',bodyMarkdown:'# First',questions:[{promptMarkdown:'A command?',answer:'whoami',normalization:'trim'}]},
      {title:'Second',slug:'second',access:'public',bodyMarkdown:'# Second',questions:[]},
    ]})
    const id=created.module.id,path=`/api/admin/modules/${id}`,detail=()=>req(path)
    let d=await detail(),first=d.chapters[0].chapterId,second=d.chapters[1].chapterId
    assert.equal(d.category.name,'Web 安全');assert.equal(d.chapters[0].slug,'first')
    let list=await req('/api/admin/modules'),item=list.find(m=>m.id===id)
    assert.equal(item.chapterCount,2);assert.equal(item.isPublished,false);assert.equal(item.hasDraft,true)
    const [before]=await sql`SELECT answer_hmac, question_key FROM chapter_questions WHERE chapter_revision_id=${d.chapters[0].chapterRevisionId}`
    const chapterPath=`/api/admin/chapters/${first}/draft`
    const patch={moduleId:id,title:'First edited',access:'public',bodyMarkdown:'# Edited'}
    await req(chapterPath,'PUT',patch);d=await detail()
    const [after]=await sql`SELECT answer_hmac, question_key FROM chapter_questions WHERE chapter_revision_id=${d.chapters[0].chapterRevisionId}`
    assert.deepEqual(after,before,'omitting questions preserves answers and stable keys')
    let q=d.chapters[0].questions[0]
    assert.equal(q.hasAnswer,true);assert.equal('answerHmac' in q,false)
    await req(chapterPath,'PUT',{...patch,questions:[{...q,promptMarkdown:'Updated prompt'}]});d=await detail()
    const [preserved]=await sql`SELECT answer_hmac FROM chapter_questions WHERE chapter_revision_id=${d.chapters[0].chapterRevisionId}`
    assert.deepEqual(preserved.answer_hmac,before.answer_hmac,'blank replacement preserves digest')
    q=d.chapters[0].questions[0]
    await req(chapterPath,'PUT',{...patch,questions:[{...q,normalization:'lowercase'}]},400)
    await req(chapterPath,'PUT',{...patch,questions:[{...q,normalization:'trim_casefold'}]},400)
    await req(chapterPath,'PUT',{...patch,questions:[{...q,normalization:'trim_casefold',answer:'WHOAMI'}]});d=await detail()
    const version={baseRevisionId:d.baseRevisionId,lockVersion:d.lockVersion}
    const races=await Promise.all([1,2].map(async i=>(await fetch(baseUrl+path+'/draft',{method:'PUT',headers:{cookie,'content-type':'application/json'},body:JSON.stringify({...version,summary:`writer ${i}`})})).status))
    assert.deepEqual(races.sort(),[200,409],'concurrent saves must conflict')
    d=await detail()
    const savedTitle=d.chapters[0].title
    await req(path+'/draft','PUT',{title:'Should roll back',baseRevisionId:d.baseRevisionId,lockVersion:d.lockVersion,chapters:[
      {...d.chapters[0],title:'Also roll back',questions:d.chapters[0].questions},
      {...d.chapters[1],questions:[{promptMarkdown:'Bad rule',normalization:'number'}]},
    ]},400)
    d=await detail();assert.equal(d.editable.title,'Lifecycle module');assert.equal(d.chapters[0].title,savedTitle)
    const storedQ=d.chapters[0].questions[0]
    await req(chapterPath,'PUT',{...patch,questions:[{...storedQ,clearAnswer:true}]})
    d=await detail();assert.equal(d.chapters[0].questions[0].hasAnswer,false)
    await req(path+'/publish','POST',{},400)
    await req(chapterPath,'PUT',{...patch,questions:[{...d.chapters[0].questions[0],answer:'WHOAMI'}]})
    d=await detail()
    await req(path+'/publish','POST',{baseRevisionId:d.baseRevisionId,lockVersion:d.lockVersion})
    item=(await req('/api/admin/modules')).find(m=>m.id===id)
    assert.equal(item.isPublished,true);assert.equal(item.hasDraft,false);assert.equal(item.chapterCount,2)
    assert.equal((await req('/api/modules')).find(m=>m.id===id).category.slug,'web-security')
    await req(path+'/chapters/reorder','PUT',{orderedChapterIds:[second,first]});d=await detail()
    assert.deepEqual(d.chapters.map(c=>c.chapterId),[second,first]);assert.equal(d.draft.title,'Lifecycle module');assert.equal(d.draft.categoryId,categoryId)
    await req(path+'/chapters/reorder','PUT',{orderedChapterIds:[first,first]},400)
    await req(path+'/draft','DELETE',{baseRevisionId:d.baseRevisionId,lockVersion:d.lockVersion});d=await detail()
    assert.equal(d.draft,null);assert.deepEqual(d.chapters.map(c=>c.chapterId),[first,second])
    await req(path+'/chapters','POST',{title:'Third',access:'public',bodyMarkdown:'# Third'});d=await detail()
    assert.equal(d.chapters.length,3,'adding after publication preserves historical chapters');assert.equal(d.chapters[0].questions[0].hasAnswer,true)
    assert.equal((await req(`/api/modules/lifecycle-${suffix}`)).chapters.length,2,'draft does not modify public manifest')
    await req(path+'/draft','DELETE',{baseRevisionId:d.baseRevisionId,lockVersion:d.lockVersion})
    await req(path+'/unpublish','POST');d=await detail();assert.ok(d.draft);assert.equal(d.chapters.length,2);assert.equal(d.isPublished,false)
    await req(`/api/modules/lifecycle-${suffix}`,'GET',undefined,404)
    await req(path,'DELETE');await req(path,'GET',undefined,404)
    assert.equal((await req('/api/admin/modules')).some(m=>m.id===id),false)
    const neverPublished=await req('/api/admin/modules','POST',{title:'Draft to delete',slug:`draft-${suffix}`})
    await req(`/api/admin/modules/${neverPublished.module.id}`,'DELETE')
    await req(`/api/admin/modules/${neverPublished.module.id}`,'GET',undefined,404)
  }finally{await sql.end()}
})
