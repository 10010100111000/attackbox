import test from 'node:test'
import assert from 'node:assert/strict'
import { setTimeout as delay } from 'node:timers/promises'
import { learningFixture } from './helpers/learning-fixture.mjs'
import { sandboxWorker } from './helpers/sandbox-worker.mjs'

test('preparation status reads bypass the mutation lock; new preparation still waits for it', { skip:!process.env.CONTENT_TEST_DATABASE_URL || !process.env.NODE_TEST_CLUSTER_SECRET, timeout:30000 }, async () => {
  const f = await learningFixture(), pending = []
  let worker, unlock = () => {}, lockTask
  const act = (template,action) => {
    const task = f.request(`/api/admin/target-templates/${template.id}/preparations`, {
      actor:f.admin,method:'POST',body:{nodeId:'test-node',action},
    }).then(result => result.data)
    pending.push(task)
    return task
  }
  const readPromptly = async task => {
    const timeout = new AbortController()
    try {
      return await Promise.race([
        task,
        delay(2000,undefined,{signal:timeout.signal}).then(() => { throw new Error('Status read waited for the global preparation mutation lock') }),
      ])
    } finally { timeout.abort() }
  }
  try {
    worker = await sandboxWorker(f.base,process.env.NODE_TEST_CLUSTER_SECRET,{composeLabs:true,composePrepareHold:true})
    const existing = await f.seedTarget({name:'Existing preparation',enabled:false})
    const empty = await f.seedTarget({name:'No preparation yet',enabled:false})
    await f.sql`delete from lab_preparations where template_id in (${existing.id},${empty.id})`
    assert.equal((await act(existing,'prepare')).state,'preparing')
    let acquired
    const locked = new Promise(resolve => acquired = resolve)
    const release = new Promise(resolve => unlock = resolve)
    lockTask = f.sql.begin(async tx => {
      await tx`select pg_advisory_xact_lock(72486212)`
      acquired()
      await release
    })
    await Promise.race([locked,lockTask.then(() => { throw new Error('Lock transaction ended early') })])
    assert.equal((await readPromptly(act(existing,'status'))).state,'preparing')
    assert.equal(await readPromptly(act(empty,'status')),null)
    const starting = act(empty,'prepare')
    let waiting = false
    for (let attempt=0;attempt<100;attempt++) {
      const [row] = await f.sql`select exists(select 1 from pg_locks where locktype='advisory' and classid=0 and objid=72486212 and objsubid=1 and not granted) as waiting`
      if (row.waiting) { waiting=true;break }
      await delay(20)
    }
    assert.equal(waiting,true,'preparation mutations still use the shared template/preparation lock')
    assert.equal((await readPromptly(act(existing,'status'))).state,'preparing','a waiting writer does not block status reads')
    assert.equal(worker.requests.filter(request=>request.type==='compose_prepare').length,1)
    unlock();await lockTask
    assert.equal((await starting).state,'preparing')
    assert.equal(worker.requests.filter(request=>request.type==='compose_prepare').length,2)
  } finally {
    unlock();await lockTask
    await Promise.allSettled(pending)
    await worker?.close()
    await f.cleanup()
  }
})
