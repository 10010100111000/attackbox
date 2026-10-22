import { defineEventHandler, getRouterParam, createError } from 'h3';
import { db } from '../../../../db';
import {
  modules,
  moduleRevisions,
  moduleRevisionChapters,
  chapterRevisions,
  chapterQuestions,
  auditLogs,
} from '../../../../db/schema';
import { eq, and, inArray, isNull } from 'drizzle-orm';
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

  // 1. 获取模块实体
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

  // 2. 获取当前待发布的草稿
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
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '当前模块没有待发布的草稿版本',
    });
  }

  // 3. 发布完整性校验
  // 3.1 标题非空检查
  if (!draft.title || draft.title.trim().length === 0) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '模块标题不能为空，无法发布',
    });
  }

  // 3.2 章节编排检查：至少包含一个章节
  const mrcList = await db
    .select({
      chapterId: moduleRevisionChapters.chapterId,
      chapterRevisionId: moduleRevisionChapters.chapterRevisionId,
      position: moduleRevisionChapters.position,
    })
    .from(moduleRevisionChapters)
    .where(eq(moduleRevisionChapters.moduleRevisionId, draft.id));

  if (mrcList.length === 0) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '模块草稿未包含任何章节，至少需要 1 个章节才能发布',
    });
  }

  const chapterRevisionIds = mrcList.map((m) => m.chapterRevisionId);

  // 3.3 检查所有章节修订正文与标题
  const chRevs = await db
    .select()
    .from(chapterRevisions)
    .where(inArray(chapterRevisions.id, chapterRevisionIds));

  for (const chRev of chRevs) {
    if (!chRev.title || chRev.title.trim().length === 0) {
      throw createError({
        statusCode: 400,
        statusMessage: 'Bad Request',
        message: '存在未设置标题的章节快照，无法发布',
      });
    }
  }

  // 3.4 检查题目与答案完整性：题目绝不允许答案缺失
  const missingAnswerQuestions = await db
    .select({
      id: chapterQuestions.id,
      position: chapterQuestions.position,
      promptMarkdown: chapterQuestions.promptMarkdown,
      chapterRevisionId: chapterQuestions.chapterRevisionId,
    })
    .from(chapterQuestions)
    .where(
      and(
        inArray(chapterQuestions.chapterRevisionId, chapterRevisionIds),
        isNull(chapterQuestions.answerHmac)
      )
    );

  if (missingAnswerQuestions.length > 0) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: `发布失败：存在 ${missingAnswerQuestions.length} 道题目尚未设置标准答案，请完善答案后再发布`,
    });
  }

  // 4. 原子事务：封存快照 + 切换发布状态与发布指针
  const now = new Date();
  const publishResult = await db.transaction(async (tx) => {
    // 4.1 封存所有引用的章节修订 (is_sealed = true)
    await tx
      .update(chapterRevisions)
      .set({ isSealed: true })
      .where(inArray(chapterRevisions.id, chapterRevisionIds));

    // 4.2 将草稿转为 published 状态
    const [publishedRev] = await tx
      .update(moduleRevisions)
      .set({
        state: 'published',
        publishedAt: now,
        updatedAt: now,
      })
      .where(eq(moduleRevisions.id, draft.id))
      .returning();

    // 4.3 原子更新模块的发布指针 published_revision_id
    const [updatedMod] = await tx
      .update(modules)
      .set({
        publishedRevisionId: draft.id,
      })
      .where(eq(modules.id, moduleId))
      .returning();

    // 4.4 记录审计日志
    await tx.insert(auditLogs).values({
      actorUserId: admin.id,
      action: 'admin.module.publish',
      targetType: 'module',
      targetId: moduleId,
      metadata: {
        revisionNo: publishedRev.revisionNo,
        revisionId: publishedRev.id,
        chaptersCount: mrcList.length,
      },
    });

    return {
      module: updatedMod,
      publishedRevision: publishedRev,
      chaptersCount: mrcList.length,
    };
  });

  return {
    ok: true,
    message: '模块发布成功',
    data: publishResult,
  };
});
