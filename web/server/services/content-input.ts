import { createError } from 'h3'
import { randomUUID } from 'node:crypto'
import { normalizations, difficulties, type QuestionInput, type ModuleInput } from '../../shared/content'
import { hashQuizAnswer } from '../utils/quiz'
import type { chapterQuestions } from '../db/schema/content'
export function fail(message: string, statusCode = 400): never { throw createError({ statusCode, message }) }
export function slugFor(value: unknown, prefix: string) {
  const valueString = typeof value === 'string' ? value.trim().toLowerCase() : ''
  const result = valueString || `${prefix}-${randomUUID().slice(0, 8)}`
  if (result.length > 100 || !/^[a-z0-9]+(-[a-z0-9]+)*$/.test(result)) fail('访问标识仅支持小写字母、数字和短横线')
  return result
}
export function validateModule(input: ModuleInput) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) fail('模块格式无效')
  if (input.lockVersion !== undefined && (!Number.isInteger(input.lockVersion) || input.lockVersion < 1)) fail('无效的修改标识')
  if (input.descriptionMarkdown !== undefined && typeof input.descriptionMarkdown !== 'string') fail('模块介绍必须是文本')
  if (input.prerequisiteModuleIds !== undefined && (!Array.isArray(input.prerequisiteModuleIds) || input.prerequisiteModuleIds.length > 12 || new Set(input.prerequisiteModuleIds).size !== input.prerequisiteModuleIds.length || input.prerequisiteModuleIds.some(id => typeof id !== 'string' || !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(id)))) fail('前置模块最多 12 个，不能重复或使用无效标识')
  if (input.intent !== undefined && !['draft', 'publish'].includes(input.intent)) fail('无效的保存操作')
  if (input.learningOutcomes !== undefined && (!Array.isArray(input.learningOutcomes) || input.learningOutcomes.length > 12 || input.learningOutcomes.some(item => typeof item !== 'string' || !item.trim() || item.length > 200))) fail('学习目标最多 12 项，每项为 1–200 字文本')
  if (input.difficulty !== undefined && !difficulties.includes(input.difficulty)) fail('无效难度')
  if (input.title !== undefined && (typeof input.title !== 'string' || !input.title.trim() || input.title.length > 160)) fail('模块名称不能为空，且不能超过 160 字')
  if (input.summary !== undefined && (typeof input.summary !== 'string' || input.summary.length > 500)) fail('模块简介不能超过 500 字')
}
export function questionValues(input: QuestionInput, old?: typeof chapterQuestions.$inferSelect) {
  if (!normalizations.includes(input.normalization)) fail('无效答案规则，仅支持 exact、trim、trim_casefold')
  if (typeof input.promptMarkdown !== 'string') fail('题目必须是文本')
  const replacing = typeof input.answer === 'string' && input.answer.length > 0
  if (old?.answerHmac && old.normalization !== input.normalization && !replacing && !input.clearAnswer) fail('修改答案规则时必须重新填写标准答案')
  if (input.clearAnswer && replacing) fail('不能同时清除和替换答案')
  return {
    promptMarkdown: input.promptMarkdown,
    errorHint: input.errorHint || null,
    normalization: input.normalization,
    answerHmac: input.clearAnswer ? null : replacing ? hashQuizAnswer(input.answer!, input.normalization) : old?.answerHmac ?? null,
    answerKeyVersion: replacing ? 1 : old?.answerKeyVersion ?? 1,
  }
}
