import {rm} from 'node:fs/promises'
import {join} from 'node:path'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import postgres from 'postgres'
import { hash } from '@node-rs/argon2'
export async function learningFixture() {
  const database = process.env.CONTENT_TEST_DATABASE_URL, base = process.env.CONTENT_TEST_BASE_URL
  assert.ok(database && base, 'CONTENT_TEST_DATABASE_URL and CONTENT_TEST_BASE_URL are required')
  const target = new URL(database)
  assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(target.hostname) && target.pathname.endsWith('_test'), 'use an isolated local _test database')
  assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname))
  const sql = postgres(database, { max: 2 }), accounts = [], moduleIds = []
  async function account(role = 'user') {
    const id = crypto.randomUUID(), email = `learning-${id}@example.test`, password = crypto.randomBytes(16).toString('hex'), token = crypto.randomBytes(32).toString('hex')
    await sql`INSERT INTO users (id,email,display_name,password_hash,role,status,email_verified_at) VALUES (${id},${email},'Learning Test',${await hash(password) },${role},'active',now())`
    await sql`INSERT INTO auth_sessions (id,user_id,token_hash,expires_at) VALUES (${crypto.randomUUID()},${id},${crypto.createHash('sha256').update(token).digest()},now()+interval '1 hour')`
    const result = { id, email, password, token }; accounts.push(result); return result
  }
  async function request(path, { actor, method = 'GET', body, status = 200 } = {}) {
    const response = await fetch(base + path, { method, headers: { ...(actor ? { cookie: `attackbox_session=${actor.token}` } : {}), ...(body ? { 'content-type': 'application/json' } : {}) }, body: body ? JSON.stringify(body) : undefined })
    const json = await response.json()
    assert.equal(response.status, status, `${method} ${path}: ${JSON.stringify(json)}`)
    return { data: json.data, response }
  }
  const admin = await account('admin')
  async function createModule(intent = 'publish') {
    const title = 'Linux 学习闭环 ' + crypto.randomUUID().slice(0, 8)
    const { data } = await request('/api/admin/modules', { actor: admin, method: 'POST', body: { title, summary: '从命令行开始，完成章节并继续学习。', intent, chapters: [
      { title: '认识 Shell', slug: 'intro', access: 'public', bodyMarkdown: '认识终端与 Shell。\n\n## 第一条命令\n\n使用 `whoami` 查看当前身份。\n\n```bash\nwhoami\npwd\n```', questions: [{ promptMarkdown: '查看当前用户的命令？', answer: 'whoami', normalization: 'trim_casefold' }, { promptMarkdown: '查看当前目录的命令？', answer: 'pwd', normalization: 'trim' }] },
      { title: '目录导航', slug: 'navigation', access: 'authenticated', bodyMarkdown: '## 目录与路径\n\n使用 cd 切换目录。', questions: [] },
      { title: '公开预览', slug: 'preview', access: 'public', bodyMarkdown: '## 权限预览\n\n此公开章节不受进度锁限制。', questions: [] },
      { title: '权限实战', slug: 'permissions', access: 'authenticated', bodyMarkdown: '## 文件权限\n\n理解 rwx。', questions: [] },
    ] } })
    moduleIds.push(data.module.id)
    return (await request(`/api/admin/modules/${data.module.id}`, { actor: admin })).data
  }
  async function seedTarget(input={},boot='fixture') {
    const uploaded=await fetch(base+'/api/admin/lab-bundles',{method:'POST',headers:{cookie:`attackbox_session=${admin.token}`,'content-type':'application/octet-stream','x-lab-filename':'compose.yml'},body:'services: {web: {image: nginx:1.27}}'})
    assert.equal(uploaded.status,200);const bundle=(await uploaded.json()).data
    const {data:t}=await request('/api/admin/compose-templates',{actor:admin,method:'POST',body:{name:input.name||'Compose fixture',description:input.description||'',accessInstructions:input.accessInstructions||'',bundleId:bundle.id,settings:{file:'compose.yml',entryService:'web',entryNetwork:'default',entryPort:80,prefixBits:28},variables:{},enabled:false}})
    await sql`insert into lab_preparations(template_id,template_version,node_id,boot_id,state,deployment_id) values(${t.id},1,'test-node',${boot},'ready',${'dep_'+'a'.repeat(64)})`
    await sql`update target_templates set enabled=${input.enabled!==false} where id=${t.id}`
    return {...t,enabled:input.enabled!==false,settings:t.composeSettings,variables:{}}
  }
  async function cleanup() {
    for (const id of moduleIds) await sql`DELETE FROM modules WHERE id=${id}`
    const bundles=await sql`select id from lab_bundles where actor_id=${admin.id}`
    await sql`delete from lab_bundles where actor_id=${admin.id}`
    if(process.env.LAB_DATA_DIR)for(const {id}of bundles)await rm(join(process.env.LAB_DATA_DIR,id),{recursive:true,force:true})
    for (const account of accounts) {
      await sql`DELETE FROM audit_logs WHERE actor_user_id=${account.id}`
      await sql`DELETE FROM users WHERE id=${account.id}`
    }
    await sql.end()
  }
  return { sql, base, admin, account, request, createModule, seedTarget, cleanup }
}
