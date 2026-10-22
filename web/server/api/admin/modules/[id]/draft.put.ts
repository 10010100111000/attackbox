import { defineEventHandler, getRouterParam, readBody, createError } from 'h3';
import { db } from '../../../../db';
import { modules, moduleRevisions, moduleRevisionChapters, auditLogs } from '../../../../db/schema';
import { eq, and, desc } from 'drizzle-orm';
import { requireAdmin } from '../../../../utils/admin';

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

  const difficulty = body?.difficulty;
  if (difficulty && !['beginner', 'intermediate', 'advanced'].includes(difficulty)) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '难度等级必须为 beginner, intermediate 或 advanced',
    });
  }

  // 查询当前草稿
  const [currentDraft] = await db
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
    if (currentDraft) {
      // 乐观锁检查
      if (body.lockVersion !== undefined && currentDraft.lockVersion !== body.lockVersion) {
        throw createError({
          statusCode: 409,
          statusMessage: 'Conflict',
          message: '内容已被其他管理员修改或版本已更新，请刷新后重试',
        });
      }

      const [updated] = await tx
        .update(moduleRevisions)
        .set({
          title: body.title !== undefined ? body.title : currentDraft.title,
          summary: body.summary !== undefined ? body.summary : currentDraft.summary,
          descriptionMarkdown:
            body.descriptionMarkdown !== undefined
              ? body.descriptionMarkdown
              : currentDraft.descriptionMarkdown,
          difficulty: difficulty || currentDraft.difficulty,
          coverAssetId:
            body.coverAssetId !== undefined ? body.coverAssetId : currentDraft.coverAssetId,
          lockVersion: currentDraft.lockVersion + 1,
          updatedAt: new Date(),
        })
        .where(eq(moduleRevisions.id, currentDraft.id))
        .returning();

      await tx.insert(auditLogs).values({
        actorUserId: admin.id,
        action: 'admin.module.update_draft',
        targetType: 'module_revision',
        targetId: updated.id,
        metadata: {
          moduleId,
          lockVersion: updated.lockVersion,
          title: updated.title,
        },
      });

      return updated;
    } else {
      // 当前没有草稿（上一版本已发布），基于最新版本派生新草稿
      const [lastRev] = await tx
        .select()
        .from(moduleRevisions)
        .where(eq(moduleRevisions.moduleId, moduleId))
        .orderBy(desc(moduleRevisions.revisionNo))
        .limit(1);

      const nextRevNo = (lastRev?.revisionNo || 0) + 1;

      const [newDraft] = await tx
        .insert(moduleRevisions)
        .values({
          moduleId,
          revisionNo: nextRevNo,
          state: 'draft',
          title: body.title || lastRev?.title || '新草稿',
          summary: body.summary !== undefined ? body.summary : lastRev?.summary,
          descriptionMarkdown:
            body.descriptionMarkdown !== undefined
              ? body.descriptionMarkdown
              : lastRev?.descriptionMarkdown,
          difficulty: difficulty || lastRev?.difficulty || 'beginner',
          coverAssetId:
            body.coverAssetId !== undefined ? body.coverAssetId : lastRev?.coverAssetId,
          lockVersion: 1,
          createdBy: admin.id,
        })
        .returning();

      // 如果有上一版本的章节编排，复制到新草稿中
      if (lastRev) {
        const lastChapters = await tx
          .select()
          .from(moduleRevisionChapters)
          .where(eq(moduleRevisionChapters.moduleRevisionId, lastRev.id));

        for (const item of lastChapters) {
          await tx.insert(moduleRevisionChapters).values({
            moduleRevisionId: newDraft.id,
            moduleId,
            chapterId: item.chapterId,
            chapterRevisionId: item.chapterRevisionId,
            position: item.position,
          });
        }
      }

      await tx.insert(auditLogs).values({
        actorUserId: admin.id,
        action: 'admin.module.fork_draft',
        targetType: 'module_revision',
        targetId: newDraft.id,
        metadata: {
          moduleId,
          revisionNo: nextRevNo,
        },
      });

      return newDraft;
    }
  });

  return {
    ok: true,
    data: result,
  };
});
