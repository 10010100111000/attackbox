import path from 'node:path'
import { createHash } from 'node:crypto'
import MarkdownIt from 'markdown-it'
import { LineCounter, parseDocument } from 'yaml'
import { difficulties, normalizations } from '../../shared/content.ts'
import type { Difficulty, Normalization } from '../../shared/content.ts'
import { normalizeAnswer } from '../utils/quiz.ts'
type Token = ReturnType<typeof md.parse>[number]

export interface ContentLocation {
  file: string
  line: number
  field?: string
}
export interface ContentIssue extends ContentLocation {
  message: string
}
export class ContentFormatError extends Error {
  statusCode = 400
  issues: ContentIssue[]
  constructor(issues: ContentIssue[]) {
    super(
      issues
        .map(i => `${i.file}:${i.line}${i.field ? ` [${i.field}]` : ''}: ${i.message}`)
        .join('\n')
    )
    this.name = 'ContentFormatError'
    this.issues = issues
  }
}
export interface ContentImage extends ContentLocation {
  href: string
}
export interface ParsedQuestion {
  matchKey: string
  promptMarkdown: string
  answer: string
  normalization: Normalization
  errorHint: string | null
  source: ContentLocation
}
export interface ParsedChapter {
  title: string
  matchKey: string
  position: number
  access: 'public' | 'authenticated'
  durationMinutes: number | null
  bodyMarkdown: string
  questions: ParsedQuestion[]
  source: ContentLocation
}
export interface ParsedModule {
  sourcePath: string
  sourceHash: string
  title: string
  summary: string
  descriptionMarkdown: string
  learningOutcomes: string[]
  categorySlug: string | null
  difficulty: Difficulty
  chapters: ParsedChapter[]
  cover: string | null
  images: ContentImage[]
}

const md = new MarkdownIt({ html: false })
const maxSourceBytes = 2 * 1024 * 1024
export function contentMatchKey(value: string) {
  return value.normalize('NFKC').trim().replace(/\s+/g, ' ')
}
function fail(location: ContentLocation, message: string): never {
  throw new ContentFormatError([{ ...location, message }])
}
function record(value: unknown, location: ContentLocation): Record<string, unknown> {
  if (!value || typeof value !== 'object' || Array.isArray(value))
    fail(location, '必须是字段配置，不能是数组或标量')
  return value as Record<string, unknown>
}
function fields(value: Record<string, unknown>, allowed: string[], location: ContentLocation) {
  for (const key of Object.keys(value))
    if (!allowed.includes(key)) fail({ ...location, field: key }, '未知配置字段')
}
function text(value: unknown, fallback: string, max: number, location: ContentLocation) {
  if (value === undefined) return fallback
  if (typeof value !== 'string' || value.length > max)
    fail(location, `必须是文本，最多 ${max} 字符`)
  return value
}
function parseYaml(source: string, location: ContentLocation) {
  const counter = new LineCounter()
  const document = parseDocument(source, {
    lineCounter: counter,
    uniqueKeys: true,
    merge: false,
    logLevel: 'silent'
  })
  const error = document.errors[0] || document.warnings[0]
  // YAML diagnostics may contain answer excerpts. Report coordinates, not source snippets.
  if (error)
    fail(
      { ...location, line: location.line + counter.linePos(error.pos[0]).line - 1 },
      'YAML 格式错误或存在重复字段'
    )
  let value: Record<string, unknown>
  try {
    value = record(document.toJS({ maxAliasCount: 0 }), location)
  } catch (error) {
    if (error instanceof ContentFormatError) throw error
    fail(location, '不支持 YAML 别名或循环引用')
  }
  return { value, document, counter }
}
function yamlObject(source: string, location: ContentLocation) {
  return parseYaml(source, location).value
}
function access(value: unknown, fallback: 'public' | 'authenticated', location: ContentLocation) {
  if (value === undefined) return fallback
  if (value !== 'public' && value !== 'authenticated')
    fail(location, '仅支持 public 或 authenticated')
  return value
}
function titleText(token: Token) {
  return (token.children || [])
    .map(t =>
      t.type === 'text' || t.type === 'code_inline' || t.type === 'image'
        ? t.content
        : t.type === 'softbreak'
          ? ' '
          : ''
    )
    .join('')
    .trim()
}
function imageReferences(
  tokens: Token[],
  file: string,
  offset: number,
  fixedLine?: number
): ContentImage[] {
  const result: ContentImage[] = []
  function walk(list: Token[], line: number) {
    for (const token of list) {
      const current = fixedLine ?? (token.map ? token.map[0] + offset : line)
      if (token.type === 'image')
        result.push({ file, line: current, href: String(token.attrGet('src') || '') })
      if (token.children) walk(token.children, current)
    }
  }
  walk(tokens, offset)
  return result
}

/** Server-only source representation. Never serialize this object to a learner. */
export function parseModuleMarkdown(sourcePath: string, raw: string): ParsedModule {
  const origin = { file: sourcePath, line: 1 }
  if (
    path.posix.isAbsolute(sourcePath) ||
    sourcePath.includes('\\') ||
    sourcePath.split('/').some(p => !p || p === '.' || p === '..') ||
    !sourcePath.endsWith('.md')
  )
    fail(origin, '需要仓库内的相对 .md 文件路径')
  if (Buffer.byteLength(raw) > maxSourceBytes) fail(origin, '单个教程不能超过 2 MiB')
  const title = path.posix.basename(sourcePath, '.md').trim()
  if (!title || title.length > 160) fail(origin, '文件名作为模块名，长度必须为 1–160 字符')
  const lines = raw
    .replace(/^\uFEFF/, '')
    .replace(/\r\n?/g, '\n')
    .split('\n')
  if (lines[0] !== '---') fail(origin, '文档必须以 --- 配置头开始')
  const closing = lines.indexOf('---', 1)
  if (closing < 0) fail(origin, '配置头缺少结束的 ---')
  const headerLocation = { file: sourcePath, line: 2 }
  const headerSource = lines.slice(1, closing).join('\n')
  const headerParsed = headerSource.trim() ? parseYaml(headerSource, headerLocation) : null
  const meta = headerParsed ? headerParsed.value : {}
  fields(
    meta,
    [
      'cover',
      'summary',
      'description',
      'category',
      'difficulty',
      'access',
      'learningOutcomes',
      'chapters'
    ],
    headerLocation
  )
  // Field nodes give authors an actual line, including for multiline values.
  function fieldLocation(field: string): ContentLocation {
    const node = headerParsed?.document.get(field, true)
    return {
      file: sourcePath,
      line:
        headerParsed &&
        node &&
        typeof node === 'object' &&
        'range' in node &&
        Array.isArray(node.range)
          ? headerParsed.counter.linePos(node.range[0]).line + 1
          : 2,
      field
    }
  }
  const cover =
    meta.cover === undefined ? null : text(meta.cover, '', 500, fieldLocation('cover')).trim()
  if (cover === '') fail(fieldLocation('cover'), '封面路径不能为空，未配置请省略 cover')
  const summary = text(meta.summary, '', 500, fieldLocation('summary'))
  const descriptionMarkdown = text(
    meta.description,
    '',
    maxSourceBytes,
    fieldLocation('description')
  )
  const difficulty = meta.difficulty ?? 'beginner'
  if (!difficulties.includes(difficulty as Difficulty))
    fail(fieldLocation('difficulty'), '仅支持 beginner、intermediate 或 advanced')
  const defaultAccess = access(meta.access, 'authenticated', fieldLocation('access'))
  const categorySlug =
    meta.category === undefined || meta.category === null
      ? null
      : text(meta.category, '', 80, fieldLocation('category')).trim()
  if (categorySlug !== null && !categorySlug)
    fail(fieldLocation('category'), '分类名称不能为空，未分类请省略该字段')
  const outcomes = meta.learningOutcomes ?? []
  if (
    !Array.isArray(outcomes) ||
    outcomes.length > 12 ||
    outcomes.some(x => typeof x !== 'string' || !x.trim() || x.length > 200)
  )
    fail(fieldLocation('learningOutcomes'), '目标最多 12 项，每项为 1–200 字文本')
  const chapterConfig =
    meta.chapters === undefined ? {} : record(meta.chapters, fieldLocation('chapters'))
  const bodyOffset = closing + 2 // Markdown token maps are zero-based; diagnostics are one-based.
  const bodyLines = lines.slice(closing + 1)
  const body = bodyLines.join('\n')
  const tokens = md.parse(body, {})
  const headings = tokens.filter(t => t.type === 'heading_open' && t.tag === 'h1' && t.level === 0)
  if (!headings.length) fail({ ...origin, line: bodyOffset }, '至少需要一个一级标题作为章节')
  if (bodyLines.slice(0, headings[0]!.map![0]).join('\n').trim())
    fail({ ...origin, line: bodyOffset }, '正文必须从一级标题开始，模块介绍请写入配置头')
  for (const token of tokens) {
    if (token.type === 'code_block' && /^\s*(?:`{3,}|~{3,})quiz\b/im.test(token.content))
      fail(
        { file: sourcePath, line: token.map![0] + bodyOffset, field: 'quiz' },
        'quiz 不能放在缩进代码块中'
      )
  }
  const quizzes = tokens.filter(t => t.type === 'fence' && /^quiz(?:\s|$)/i.test(t.info.trim()))
  for (const token of quizzes) {
    const location = { file: sourcePath, line: token.map![0] + bodyOffset, field: 'quiz' }
    if (token.level !== 0 || token.info.trim() !== 'quiz')
      fail(location, '题目必须使用顶层、独立的 quiz 围栏代码块')
    const last = bodyLines[token.map![1] - 1]!.trim()
    if (!new RegExp(`^${token.markup[0]}{${token.markup.length},}$`).test(last))
      fail(location, 'quiz 代码块未闭合')
  }
  const usedTitles = new Set<string>()
  const usedConfig = new Set<string>()
  const descriptionTokens = md.parse(descriptionMarkdown, {})
  if (descriptionTokens.some(t => t.type === 'fence' && /^quiz(?:\s|$)/i.test(t.info.trim())))
    fail(fieldLocation('description'), '模块介绍不能包含 quiz，请将题目放在对应章节中')
  const images: ContentImage[] = imageReferences(
    descriptionTokens,
    sourcePath,
    fieldLocation('description').line
  )
  if (cover) images.push({ ...fieldLocation('cover'), href: cover })
  const chapters = headings.map((heading, index): ParsedChapter => {
    const start = heading.map![0],
      end = headings[index + 1]?.map![0] ?? bodyLines.length
    const chapterTitle = titleText(tokens[tokens.indexOf(heading) + 1]!)
    const location = { file: sourcePath, line: start + bodyOffset }
    const matchKey = contentMatchKey(chapterTitle)
    if (!matchKey || chapterTitle.length > 160) fail(location, '章节标题必须为 1–160 字符')
    if (usedTitles.has(matchKey)) fail(location, '章节标题重复，无法确定原章节关联')
    usedTitles.add(matchKey)
    const settingsValue = Object.hasOwn(chapterConfig, chapterTitle)
      ? chapterConfig[chapterTitle]
      : undefined
    const settings =
      settingsValue === undefined ? {} : record(settingsValue, { ...location, field: 'chapters' })
    if (settingsValue !== undefined) usedConfig.add(chapterTitle)
    fields(settings, ['access', 'durationMinutes'], { ...location, field: 'chapters' })
    const duration = settings.durationMinutes ?? null
    if (
      duration !== null &&
      (!Number.isInteger(duration) || Number(duration) < 1 || Number(duration) > 1440)
    )
      fail({ ...location, field: 'durationMinutes' }, '预计时长必须为 1–1440 分钟的整数')
    const chapterQuizzes = quizzes.filter(t => t.map![0] >= start && t.map![0] < end)
    const usedQuestions = new Set<string>()
    const questions = chapterQuizzes.map((token): ParsedQuestion => {
      const source = { file: sourcePath, line: token.map![0] + bodyOffset }
      const q = yamlObject(token.content, { ...source, line: source.line + 1 })
      fields(q, ['prompt', 'answer', 'normalization', 'errorHint'], source)
      const promptMarkdown = text(q.prompt, '', 20000, { ...source, field: 'prompt' }).trim()
      if (!promptMarkdown) fail({ ...source, field: 'prompt' }, '题目不能为空')
      const questionKey = contentMatchKey(promptMarkdown)
      if (usedQuestions.has(questionKey)) fail(source, '本章节题干重复，无法确定原题目关联')
      usedQuestions.add(questionKey)
      const normalization = q.normalization ?? 'trim'
      if (!normalizations.includes(normalization as Normalization))
        fail({ ...source, field: 'normalization' }, '仅支持 exact、trim 或 trim_casefold')
      const answer = text(q.answer, '', 2000, { ...source, field: 'answer' })
      if (!normalizeAnswer(answer, normalization as Normalization).length)
        fail({ ...source, field: 'answer' }, '标准答案不能为空；数字或布尔值答案请用引号包成文本')
      const errorHint =
        q.errorHint == null ? null : text(q.errorHint, '', 4000, { ...source, field: 'errorHint' })
      const promptTokens = md.parse(promptMarkdown, {})
      if (promptTokens.some(t => t.type === 'fence' && /^quiz(?:\s|$)/i.test(t.info.trim())))
        fail(source, '题目内不能嵌套 quiz')
      images.push(...imageReferences(promptTokens, sourcePath, source.line, source.line))
      return {
        matchKey: questionKey,
        promptMarkdown,
        answer,
        normalization: normalization as Normalization,
        errorHint,
        source
      }
    })
    const prose = bodyLines
      .slice(heading.map![1], end)
      .map((line, offset) =>
        chapterQuizzes.some(
          t => offset + heading.map![1] >= t.map![0] && offset + heading.map![1] < t.map![1]
        )
          ? ''
          : line
      )
      .join('\n')
      .trim()
    images.push(
      ...imageReferences(
        tokens.filter(
          t => t.map && t.map[0] >= heading.map![1] && t.map[0] < end && !chapterQuizzes.includes(t)
        ),
        sourcePath,
        bodyOffset
      )
    )
    return {
      title: chapterTitle,
      matchKey,
      position: index + 1,
      access: access(settings.access, defaultAccess, { ...location, field: 'access' }),
      durationMinutes: duration as number | null,
      bodyMarkdown: prose,
      questions,
      source: location
    }
  })
  for (const key of Object.keys(chapterConfig))
    if (!usedConfig.has(key))
      fail({ ...fieldLocation('chapters'), field: `chapters.${key}` }, '没有对应的一级标题')
  return {
    sourcePath,
    sourceHash: createHash('sha256').update(raw).digest('hex'),
    title,
    summary,
    descriptionMarkdown,
    learningOutcomes: outcomes.map(x => (x as string).trim()),
    categorySlug,
    difficulty: difficulty as Difficulty,
    chapters,
    cover,
    images
  }
}

/** Explicit allow-list. Private source, answer, hints and matching data stay on the server. */
export function learnerContentPreview(module: ParsedModule) {
  return {
    cover: module.cover,
    title: module.title,
    summary: module.summary,
    descriptionMarkdown: module.descriptionMarkdown,
    learningOutcomes: module.learningOutcomes,
    difficulty: module.difficulty,
    chapters: module.chapters.map(c => ({
      title: c.title,
      position: c.position,
      access: c.access,
      durationMinutes: c.durationMinutes,
      bodyMarkdown: c.bodyMarkdown,
      questions: c.questions.map(q => ({ promptMarkdown: q.promptMarkdown }))
    }))
  }
}
