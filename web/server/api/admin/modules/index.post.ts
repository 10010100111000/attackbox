import { defineEventHandler, readBody, createError } from 'h3';
import { db } from '../../../db';
import { modules, moduleRevisions, auditLogs } from '../../../db/schema';
import { eq } from 'drizzle-orm';
import { requireAdmin } from '../../../utils/admin';

const SLUG_REGEX = /^[a-z0-9]+(-[a-z0-9]+)*$/;

export default defineEventHandler(async (event) => {
  const admin = requireAdmin(event);
  const body = await readBody(event);

  const slug = (body?.slug || '').trim().toLowerCase();
  const title = (body?.title || '').trim();
  const summary = (body?.summary || '').trim();
  const difficulty = body?.difficulty || 'beginner';

  if (!slug || !SLUG_REGEX.test(slug)) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '模块 Slug 格式无效，必须使用小写字母、数字和中划线（如 web-basics）',
    });
  }

  if (!['beginner', 'intermediate', 'advanced'].includes(difficulty)) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '难度等级必须为 beginner, intermediate 或 advanced',
    });
  }

  // 检查 slug 唯一性
  const [existing] = await db
    .select({ id: modules.id })
    .from(modules)
    .where(eq(modules.slug, slug))
    .limit(1);

  if (existing) {
    throw createError({
      statusCode: 409,
      statusMessage: 'Conflict',
      message: '该模块 Slug 已存在',
    });
  }

  // 事务内创建模块实体与首个 Revision 1 草稿
  const result = await db.transaction(async (tx) => {
    const [mod] = await tx
      .insert(modules)
      .values({
        slug,
        createdBy: admin.id,
      })
      .returning();

    const [draftRev] = await tx
      .insert(moduleRevisions)
      .values({
        moduleId: mod.id,
        revisionNo: 1,
        state: 'draft',
        title: title || '未命名模块',
        summary: summary || null,
        difficulty,
        lockVersion: 1,
        createdBy: admin.id,
      })
      .returning();

    await tx.insert(auditLogs).values({
      actorUserId: admin.id,
      action: 'admin.module.create',
      targetType: 'module',
      targetId: mod.id,
      metadata: {
        slug: mod.slug,
        revisionNo: draftRev.revisionNo,
        title: draftRev.title,
      },
    });

    return {
      module: mod,
      draft: draftRev,
    };
  });

  return {
    ok: true,
    data: result,
  };
});
