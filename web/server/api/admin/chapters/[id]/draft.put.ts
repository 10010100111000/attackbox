import { defineEventHandler, getRouterParam, readBody, createError } from 'h3';
import { db } from '../../../../db';
import {
  modules,
  moduleRevisions,
  moduleRevisionChapters,
  chapters,
  chapterRevisions,
  chapterQuestions,
  auditLogs,
} from '../../../../db/schema';
import { eq, and, sql, isNotNull } from 'drizzle-orm';
import { requireAdmin } from '../../../../utils/admin';
import { hashQuizAnswer, type NormalizationKind } from '../../../../utils/quiz';

export default defineEventHandler(async (event) => {
  const admin = requireAdmin(event);
  const chapterId = getRouterParam(event, 'id');
  const body = await readBody(event);

  if (!chapterId) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '缺少章节 ID',
    });
  }

  const moduleId = body?.moduleId;
  if (!moduleId) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '缺少归属模块 ID (moduleId)',
    });
  }

  const title = (body?.title || '').trim();
  if (!title) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '章节标题不能为空',
    });
  }

  const access = body?.access || 'public';
  if (!['public', 'authenticated'].includes(access)) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Bad Request',
      message: '访问权限必须为 public 或 authenticated',
    });
  }

  const bodyMarkdown = body?.bodyMarkdown !== undefined ? String(body.bodyMarkdown) : '';
  const incomingQuestions: any[] = Array.isArray(body?.questions) ? body.questions : [];

  // 1. 验证 chapter 存在且归属于 moduleId
  const [ch] = await db
    .select()
    .from(chapters)
    .where(and(eq(chapters.id, chapterId), eq(chapters.moduleId, moduleId)))
    .limit(1);

  if (!ch) {
    throw createError({
      statusCode: 404,
      statusMessage: 'Not Found',
      message: '指定的章节不存在或不属于该模块',
    });
  }

  // 2. 获取当前模块草稿，若无草稿则先建立草稿
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

    // 3. 检查当前草稿编排中是否存在该章节
    const [mrc] = await tx
      .select()
      .from(moduleRevisionChapters)
      .where(
        and(
          eq(moduleRevisionChapters.moduleRevisionId, draft.id),
          eq(moduleRevisionChapters.chapterId, chapterId)
        )
      )
      .limit(1);

    if (!mrc) {
      throw createError({
        statusCode: 400,
        statusMessage: 'Bad Request',
        message: '当前模块草稿中未包含该章节，无法保存正文',
      });
    }

    // 4. 创建新的 chapter_revision 快照
    const [newChRev] = await tx
      .insert(chapterRevisions)
      .values({
        chapterId,
        title,
        access,
        bodyMarkdown,
        isSealed: false,
        createdBy: admin.id,
      })
      .returning();

    // 5. 处理题目列表
    const savedQuestions = [];
    if (incomingQuestions.length > 0) {
      // 预先拉取该章节旧版本的题目答案（用于未传明文答案时的复用）
      const oldQuestions = await tx
        .select()
        .from(chapterQuestions)
        .where(eq(chapterQuestions.chapterRevisionId, mrc.chapterRevisionId));

      const oldHmacMap = new Map<string, Buffer | null>();
      for (const oq of oldQuestions) {
        if (oq.questionKey && oq.answerHmac) {
          oldHmacMap.set(oq.questionKey, oq.answerHmac as Buffer);
        }
      }

      for (let i = 0; i < incomingQuestions.length; i++) {
        const q = incomingQuestions[i];
        const pos = Number(q.position || i + 1);
        const prompt = (q.promptMarkdown || '').trim();
        const norm: NormalizationKind = ['exact', 'trim_casefold'].includes(q.normalization)
          ? q.normalization
          : 'trim';

        if (!prompt) {
          throw createError({
            statusCode: 400,
            statusMessage: 'Bad Request',
            message: `第 ${pos} 题题目内容不能为空`,
          });
        }

        let answerHmac: Buffer | null = null;
        if (typeof q.answer === 'string' && q.answer.trim().length > 0) {
          answerHmac = hashQuizAnswer(q.answer, norm);
        } else if (q.questionKey && oldHmacMap.has(q.questionKey)) {
          answerHmac = oldHmacMap.get(q.questionKey) || null;
        }

        const [savedQ] = await tx
          .insert(chapterQuestions)
          .values({
            chapterRevisionId: newChRev.id,
            questionKey: q.questionKey || undefined,
            position: pos,
            promptMarkdown: prompt,
            errorHint: q.errorHint || null,
            normalization: norm,
            answerHmac,
            answerKeyVersion: 1,
          })
          .returning();

        savedQuestions.push({
          id: savedQ.id,
          questionKey: savedQ.questionKey,
          position: savedQ.position,
          promptMarkdown: savedQ.promptMarkdown,
          errorHint: savedQ.errorHint,
          normalization: savedQ.normalization,
          hasAnswer: savedQ.answerHmac !== null,
        });
      }
    }

    // 6. 更新 module_revision_chapters 指针
    await tx
      .update(moduleRevisionChapters)
      .set({
        chapterRevisionId: newChRev.id,
      })
      .where(
        and(
          eq(moduleRevisionChapters.moduleRevisionId, draft.id),
          eq(moduleRevisionChapters.chapterId, chapterId)
        )
      );

    // 7. 审计日志
    await tx.insert(auditLogs).values({
      actorUserId: admin.id,
      action: 'admin.chapter.update_draft',
      targetType: 'chapter_revision',
      targetId: newChRev.id,
      metadata: {
        moduleId,
        chapterId,
        title,
        questionsCount: savedQuestions.length,
      },
    });

    return {
      revision: newChRev,
      questions: savedQuestions,
    };
  });

  return {
    ok: true,
    data: result,
  };
});
