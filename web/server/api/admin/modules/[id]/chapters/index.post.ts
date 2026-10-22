import { defineEventHandler, getRouterParam, readBody, createError } from 'h3';
import { db } from '../../../../../db';
import {
  modules,
  moduleRevisions,
  moduleRevisionChapters,
  chapters,
  chapterRevisions,
  auditLogs,
} from '../../../../../db/schema';
import { eq, and, sql } from 'drizzle-orm';
import { requireAdmin } from '../../../../../utils/admin';

const SLUG_REGEX = /^[a-z0-9]+(-[a-z0-9]+)*$/;

export default defineEventHandler(async (event) => {
  const admin = requireAdmin(event);
  const moduleId = getRouterParam(event, 'id');
  const body = await readBody(event);

  if (!moduleId) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '缺少模块 ID',
    });
  }

  const slug = (body?.slug || '').trim().toLowerCase();
  const title = (body?.title || '').trim();
  const access = body?.access || 'public';
  const bodyMarkdown = body?.bodyMarkdown || '';

  if (!slug || !SLUG_REGEX.test(slug)) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '章节 Slug 格式无效，必须使用小写字母、数字和中划线（如 intro-to-linux）',
    });
  }

  if (!title) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '章节标题不能为空',
    });
  }

  if (!['public', 'authenticated'].includes(access)) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '访问权限必须为 public 或 authenticated',
    });
  }

  // 检查模块是否存在
  const [mod] = await db
    .select()
    .from(modules)
    .where(eq(modules.id, moduleId))
    .limit(1);

  if (!mod) {
    throw createError({
      statusCode: 404,
      statusMessage: 'Not Found',
      message: '模块不存在',
    });
  }

  // 检查当前模块下 slug 唯一性
  const [existingSlug] = await db
    .select({ id: chapters.id })
    .from(chapters)
    .where(
      and(
        eq(chapters.moduleId, moduleId),
        eq(chapters.slug, slug)
      )
    )
    .limit(1);

  if (existingSlug) {
    throw createError({
      statusCode: 409,
      statusMessage: 'Conflict',
      message: '当前模块下已存在同名 Slug 章节',
    });
  }

  // 获取模块当前草稿，若无草稿则先自动建立草稿
  let [draft] = await db
    .select()
    .from(moduleRevisions)
    .where(
      and(
        eq(moduleRevisions.moduleId, moduleId),
        eq(moduleRevisions.state, 'draft')
      )
    )
    .limit(1);

  const result = await db.transaction(async (tx) => {
    if (!draft) {
      const [lastRev] = await tx
        .select({ revisionNo: moduleRevisions.revisionNo })
        .from(moduleRevisions)
        .where(eq(moduleRevisions.moduleId, moduleId))
        .orderBy(sql`${moduleRevisions.revisionNo} desc`)
        .limit(1);

      const [newDraft] = await tx
        .insert(moduleRevisions)
        .values({
          moduleId,
          revisionNo: (lastRev?.revisionNo || 0) + 1,
          state: 'draft',
          title: '新草稿',
          lockVersion: 1,
          createdBy: admin.id,
        })
        .returning();
      draft = newDraft;
    }

    // 1. 插入章节实体
    const [ch] = await tx
      .insert(chapters)
      .values({
        moduleId,
        slug,
      })
      .returning();

    // 2. 插入初始未封存快照
    const [chRev] = await tx
      .insert(chapterRevisions)
      .values({
        chapterId: ch.id,
        title,
        access,
        bodyMarkdown,
        isSealed: false,
        createdBy: admin.id,
      })
      .returning();

    // 3. 计算当前草稿下的最大 position
    const [maxPosRecord] = await tx
      .select({ maxPos: sql<number>`COALESCE(MAX(${moduleRevisionChapters.position}), 0)` })
      .from(moduleRevisionChapters)
      .where(eq(moduleRevisionChapters.moduleRevisionId, draft.id));

    const nextPosition = Number(maxPosRecord?.maxPos || 0) + 1;

    // 4. 编入草稿清单
    const [mrc] = await tx
      .insert(moduleRevisionChapters)
      .values({
        moduleRevisionId: draft.id,
        moduleId,
        chapterId: ch.id,
        chapterRevisionId: chRev.id,
        position: nextPosition,
      })
      .returning();

    // 5. 审计日志
    await tx.insert(auditLogs).values({
      actorUserId: admin.id,
      action: 'admin.chapter.create',
      targetType: 'chapter',
      targetId: ch.id,
      metadata: {
        moduleId,
        slug,
        title,
        position: nextPosition,
      },
    });

    return {
      chapter: ch,
      revision: chRev,
      position: mrc.position,
    };
  });

  return {
    ok: true,
    data: result,
  };
});
