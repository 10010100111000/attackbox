import { defineEventHandler, getRouterParam, readBody, createError } from 'h3';
import { db } from '../../../../../db';
import { moduleRevisions, moduleRevisionChapters, auditLogs } from '../../../../../db/schema';
import { eq, and } from 'drizzle-orm';
import { requireAdmin } from '../../../../../utils/admin';

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

  const orders: Array<{ chapterId: string; position: number }> = Array.isArray(body?.orders)
    ? body.orders
    : [];

  if (orders.length === 0) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '排序列表不能为空',
    });
  }

  // 1. 获取当前草稿
  const [draft] = await db
    .select()
    .from(moduleRevisions)
    .where(
      and(
        eq(moduleRevisions.moduleId, moduleId),
        eq(moduleRevisions.state, 'draft')
      )
    )
    .limit(1);

  if (!draft) {
    throw createError({
      statusCode: 404,
      statusMessage: 'Not Found',
      message: '模块当前不存在草稿版本，无法调整章节顺序',
    });
  }

  // 2. 获取当前草稿全部章节
  const existingList = await db
    .select()
    .from(moduleRevisionChapters)
    .where(eq(moduleRevisionChapters.moduleRevisionId, draft.id));

  if (existingList.length !== orders.length) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: `排序列表章节数量不匹配，草稿共有 ${existingList.length} 个章节`,
    });
  }

  const existingChapterIds = new Set(existingList.map((c) => c.chapterId));
  const newPositions = new Set<number>();

  for (const item of orders) {
    if (!existingChapterIds.has(item.chapterId)) {
      throw createError({
        statusCode: 400,
        statusMessage: 'Bad Request',
        message: `章节 ${item.chapterId} 不属于当前模块草稿`,
      });
    }
    const pos = Number(item.position);
    if (!Number.isInteger(pos) || pos < 1 || pos > orders.length) {
      throw createError({
        statusCode: 400,
        statusMessage: 'Bad Request',
        message: `位置 position 必须为 1 到 ${orders.length} 的整数`,
      });
    }
    newPositions.add(pos);
  }

  if (newPositions.size !== orders.length) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '排序位置存在重复序号',
    });
  }

  // 3. 事务内重排：利用大偏移量 (10000 + i) 避开 UNIQUE(module_revision_id, position) 瞬态冲突
  await db.transaction(async (tx) => {
    // 阶段 1：先偏移至临时安全区间 (>= 10000 仍满足 position > 0)
    for (let i = 0; i < orders.length; i++) {
      const item = orders[i];
      await tx
        .update(moduleRevisionChapters)
        .set({ position: 10000 + i })
        .where(
          and(
            eq(moduleRevisionChapters.moduleRevisionId, draft.id),
            eq(moduleRevisionChapters.chapterId, item.chapterId)
          )
        );
    }

    // 阶段 2：更新为最终连续位置 (1..N)
    for (const item of orders) {
      await tx
        .update(moduleRevisionChapters)
        .set({ position: Number(item.position) })
        .where(
          and(
            eq(moduleRevisionChapters.moduleRevisionId, draft.id),
            eq(moduleRevisionChapters.chapterId, item.chapterId)
          )
        );
    }

    // 4. 审计日志
    await tx.insert(auditLogs).values({
      actorUserId: admin.id,
      action: 'admin.chapter.reorder',
      targetType: 'module_revision',
      targetId: draft.id,
      metadata: {
        moduleId,
        orders,
      },
    });
  });

  return {
    ok: true,
    message: '章节顺序更新成功',
    data: orders,
  };
});
