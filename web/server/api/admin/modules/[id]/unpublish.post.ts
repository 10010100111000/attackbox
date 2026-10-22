import { defineEventHandler, getRouterParam, createError } from 'h3';
import { db } from '../../../../db';
import { modules, auditLogs } from '../../../../db/schema';
import { eq } from 'drizzle-orm';
import { requireAdmin } from '../../../../utils/admin';

export default defineEventHandler(async (event) => {
  const admin = requireAdmin(event);
  const moduleId = getRouterParam(event, 'id');

  if (!moduleId) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '缺少模块 ID',
    });
  }

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

  if (!mod.publishedRevisionId) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '该模块当前未处于发布状态，无需下架',
    });
  }

  const previousRevisionId = mod.publishedRevisionId;

  const result = await db.transaction(async (tx) => {
    const [updatedMod] = await tx
      .update(modules)
      .set({
        publishedRevisionId: null,
      })
      .where(eq(modules.id, moduleId))
      .returning();

    await tx.insert(auditLogs).values({
      actorUserId: admin.id,
      action: 'admin.module.unpublish',
      targetType: 'module',
      targetId: moduleId,
      metadata: {
        previousRevisionId,
      },
    });

    return updatedMod;
  });

  return {
    ok: true,
    message: '模块已成功下架，历史版本与学习记录完整保留',
    data: result,
  };
});
