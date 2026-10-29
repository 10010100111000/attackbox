import { rm } from 'node:fs/promises'
import { join } from 'node:path'
import assert from 'node:assert/strict'
import crypto from 'node:crypto'
import postgres from 'postgres'
import { hash } from '@node-rs/argon2'

function matchKey(value) {
  return String(value).normalize('NFKC').trim().replace(/\s+/g, ' ')
}

export async function learningFixture() {
  const database = process.env.CONTENT_TEST_DATABASE_URL,
    base = process.env.CONTENT_TEST_BASE_URL
  assert.ok(database && base, 'CONTENT_TEST_DATABASE_URL and CONTENT_TEST_BASE_URL are required')
  const target = new URL(database)
  assert.ok(
    ['127.0.0.1', 'localhost', '[::1]'].includes(target.hostname) &&
      target.pathname.endsWith('_test'),
    'use an isolated local _test database'
  )
  assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(new URL(base).hostname))
  const sql = postgres(database, { max: 2 }),
    accounts = [],
    moduleIds = [],
    seededTargets = []
  async function account(role = 'user') {
    const id = crypto.randomUUID(),
      email = `learning-${id}@example.test`,
      password = crypto.randomBytes(16).toString('hex'),
      token = crypto.randomBytes(32).toString('hex')
    await sql`INSERT INTO users (id,email,display_name,password_hash,role,status,email_verified_at) VALUES (${id},${email},'Learning Test',${await hash(password)},${role},'active',now())`
    await sql`INSERT INTO auth_sessions (id,user_id,token_hash,expires_at) VALUES (${crypto.randomUUID()},${id},${crypto.createHash('sha256').update(token).digest()},now()+interval '1 hour')`
    const result = { id, email, password, token }
    accounts.push(result)
    return result
  }
  async function request(path, { actor, method = 'GET', body, status = 200 } = {}) {
    const response = await fetch(base + path, {
      method,
      headers: {
        ...(actor ? { cookie: `attackbox_session=${actor.token}` } : {}),
        ...(body ? { 'content-type': 'application/json' } : {})
      },
      body: body ? JSON.stringify(body) : undefined
    })
    const json = await response.json()
    assert.equal(response.status, status, `${method} ${path}: ${JSON.stringify(json)}`)
    return { data: json.data, response }
  }
  const admin = await account('admin')

  async function writeChapters(tx, moduleId, chapterInputs) {
    const existing = await tx`SELECT * FROM chapters WHERE module_id = ${moduleId}`
    const supplied = chapterInputs.map(c => c.chapterId).filter(Boolean)
    for (const old of existing) {
      if (!supplied.includes(old.id)) await tx`DELETE FROM chapters WHERE id = ${old.id}`
    }
    await tx`DELETE FROM chapter_sources WHERE module_id = ${moduleId}`
    for (const [index, ch] of chapterInputs.entries()) {
      const old = existing.find(c => c.id === ch.chapterId)
      const chapterId = old ? old.id : crypto.randomUUID()
      const slug = old ? old.slug : ch.slug || `chapter-${crypto.randomUUID().slice(0, 8)}`
      const mode = ch.targetMode || old?.target_mode || 'inherit'
      const templateId =
        mode === 'custom' ? (ch.targetTemplateId ?? old?.target_template_id ?? null) : null
      const duration =
        ch.durationMinutes === undefined ? (old?.duration_minutes ?? null) : ch.durationMinutes
      if (old) {
        await tx`UPDATE chapters SET title = ${ch.title}, access = ${ch.access || 'public'}, body_markdown = ${ch.bodyMarkdown}, position = ${index + 1}, duration_minutes = ${duration}, target_mode = ${mode}, target_template_id = ${templateId} WHERE id = ${chapterId}`
      } else {
        await tx`INSERT INTO chapters (id, module_id, slug, position, title, access, body_markdown, duration_minutes, target_mode, target_template_id) VALUES (${chapterId}, ${moduleId}, ${slug}, ${index + 1}, ${ch.title}, ${ch.access || 'public'}, ${ch.bodyMarkdown}, ${duration}, ${mode}, ${templateId})`
      }
      await tx`INSERT INTO chapter_sources (chapter_id, module_id, match_key) VALUES (${chapterId}, ${moduleId}, ${matchKey(ch.title)})`
      if (ch.questions !== undefined) {
        const oldQs = await tx`SELECT * FROM chapter_questions WHERE chapter_id = ${chapterId}`
        await tx`DELETE FROM question_sources WHERE chapter_id = ${chapterId}`
        const retained = new Set()
        for (const [qIndex, q] of ch.questions.entries()) {
          const prev = oldQs.find(o => o.id === q.id)
          const qId = prev ? prev.id : crypto.randomUUID()
          const norm = q.normalization || prev?.normalization || 'trim'
          const ans = typeof q.answer === 'string' ? q.answer : (prev?.answer ?? 'answer')
          if (prev) {
            await tx`UPDATE chapter_questions SET position = ${qIndex + 1}, prompt_markdown = ${q.promptMarkdown}, error_hint = ${q.errorHint || null}, normalization = ${norm}, answer = ${ans} WHERE id = ${qId}`
          } else {
            await tx`INSERT INTO chapter_questions (id, chapter_id, position, prompt_markdown, error_hint, normalization, answer) VALUES (${qId}, ${chapterId}, ${qIndex + 1}, ${q.promptMarkdown}, ${q.errorHint || null}, ${norm}, ${ans})`
          }
          await tx`INSERT INTO question_sources (question_id, chapter_id, match_key) VALUES (${qId}, ${chapterId}, ${matchKey(q.promptMarkdown)})`
          retained.add(qId)
        }
        for (const oldQ of oldQs) {
          if (!retained.has(oldQ.id)) await tx`DELETE FROM chapter_questions WHERE id = ${oldQ.id}`
        }
      }
    }
  }

  async function createModule(intent = 'publish', custom = {}) {
    const id = crypto.randomUUID()
    const suffix = id.slice(0, 8)
    const title = custom.title || 'Linux 学习闭环 ' + suffix
    const slug = custom.slug || `module-${suffix}`
    const summary = custom.summary ?? '从命令行开始，完成章节并继续学习。'
    const chapters = custom.chapters || [
      {
        title: '认识 Shell',
        slug: 'intro',
        access: 'public',
        bodyMarkdown:
          '认识终端与 Shell。\n\n## 第一条命令\n\n使用 `whoami` 查看当前身份。\n\n```bash\nwhoami\npwd\n```',
        questions: [
          {
            promptMarkdown: '查看当前用户的命令？',
            answer: 'whoami',
            normalization: 'trim_casefold'
          },
          { promptMarkdown: '查看当前目录的命令？', answer: 'pwd', normalization: 'trim' }
        ]
      },
      {
        title: '目录导航',
        slug: 'navigation',
        access: 'authenticated',
        bodyMarkdown: '## 目录与路径\n\n使用 cd 切换目录。',
        questions: []
      },
      {
        title: '公开预览',
        slug: 'preview',
        access: 'public',
        bodyMarkdown: '## 权限预览\n\n此公开章节不受进度锁限制。',
        questions: []
      },
      {
        title: '权限实战',
        slug: 'permissions',
        access: 'authenticated',
        bodyMarkdown: '## 文件权限\n\n理解 rwx。',
        questions: []
      }
    ]
    const sourcePath = custom.sourcePath || `${title}.md`
    const sourceHash = crypto
      .createHash('sha256')
      .update(JSON.stringify({ title, chapters }))
      .digest('hex')
    await sql.begin(async tx => {
      await tx`INSERT INTO modules (id, slug, title, summary, description_markdown, learning_outcomes, difficulty, category_id, status, lock_version, created_by) VALUES (${id}, ${slug}, ${title}, ${summary}, ${custom.descriptionMarkdown || ''}, ${custom.learningOutcomes || []}, ${custom.difficulty || 'beginner'}, ${custom.categoryId || null}, 'unpublished', 1, ${admin.id})`
      await writeChapters(tx, id, chapters)
      await tx`INSERT INTO module_sources (module_id, origin, source_path, source_hash, commit) VALUES (${id}, 'local', ${sourcePath}, ${sourceHash}, null)`
    })
    moduleIds.push(id)
    if (intent === 'publish') {
      await request(`/api/admin/modules/${id}/publish`, {
        actor: admin,
        method: 'POST',
        body: { lockVersion: 1 }
      })
    }
    return (await request(`/api/admin/modules/${id}`, { actor: admin })).data
  }

  async function syncModule(id, patch = {}) {
    await sql.begin(async tx => {
      if (patch.title !== undefined)
        await tx`UPDATE modules SET title = ${patch.title} WHERE id = ${id}`
      if (patch.summary !== undefined)
        await tx`UPDATE modules SET summary = ${patch.summary} WHERE id = ${id}`
      if (patch.chapters !== undefined) await writeChapters(tx, id, patch.chapters)
      const sourceHash = crypto.createHash('sha256').update(crypto.randomUUID()).digest('hex')
      await tx`UPDATE module_sources SET source_hash = ${sourceHash}, synced_at = now() WHERE module_id = ${id}`
      await tx`UPDATE modules SET lock_version = lock_version + 1, updated_at = now() WHERE id = ${id}`
    })
    return (await request(`/api/admin/modules/${id}`, { actor: admin })).data
  }

  async function seedTarget(input = {}, boot = 'fixture') {
    const uploaded = await fetch(base + '/api/admin/lab-bundles', {
      method: 'POST',
      headers: {
        cookie: `attackbox_session=${admin.token}`,
        'content-type': 'application/octet-stream',
        'x-lab-filename': 'compose.yml'
      },
      body: 'services: {web: {image: nginx:1.27}}'
    })
    assert.equal(uploaded.status, 200)
    const bundle = (await uploaded.json()).data
    const { data: t } = await request('/api/admin/target-templates', {
      actor: admin,
      method: 'POST',
      body: {
        name: input.name || 'Compose fixture',
        description: input.description || '',
        accessInstructions: input.accessInstructions || '',
        bundleId: bundle.id,
        settings: {
          file: 'compose.yml',
          entryService: 'web',
          entryNetwork: 'default',
          entryPort: 80,
          prefixBits: 28
        },
        variables: {},
        enabled: false
      }
    })
    await sql`insert into lab_preparations(template_id,template_version,node_id,boot_id,state,deployment_id) values(${t.id},1,'test-node',${boot},'ready',${'dep_' + 'a'.repeat(64)})`
    await sql`update target_templates set enabled=${input.enabled !== false} where id=${t.id}`
    seededTargets.push(t.id)
    return { ...t, enabled: input.enabled !== false, settings: t.settings, variables: {} }
  }
  async function cleanup() {
    for (const id of moduleIds) await sql`DELETE FROM modules WHERE id=${id}`
    for (const id of seededTargets) await sql`DELETE FROM target_templates WHERE id=${id}`
    const bundles = await sql`select id from lab_bundles where actor_id=${admin.id}`
    await sql`delete from lab_bundles where actor_id=${admin.id}`
    if (process.env.LAB_DATA_DIR)
      for (const { id } of bundles)
        await rm(join(process.env.LAB_DATA_DIR, id), { recursive: true, force: true })
    for (const account of accounts) {
      await sql`DELETE FROM audit_logs WHERE actor_user_id=${account.id}`
      await sql`DELETE FROM users WHERE id=${account.id}`
    }
    await sql.end()
  }
  return { sql, base, admin, account, request, createModule, syncModule, seedTarget, cleanup }
}
