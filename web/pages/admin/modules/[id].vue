<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useFetch, useHead } from '#imports'
import MarkdownView from '~/components/ui/MarkdownView.vue'

definePageMeta({
  layout: 'admin'
})

interface QuestionDraft {
  id?: string
  position?: number
  promptMarkdown: string
  answer: string
  errorHint: string
  normalization: 'trim' | 'collapse_whitespace' | 'lowercase' | 'number'
}

interface ChapterDraft {
  chapterId: string
  slug: string
  position: number
  title: string
  access: 'public' | 'authenticated'
  bodyMarkdown: string
  isSealed: boolean
  questions: QuestionDraft[]
}

interface ModuleDraftData {
  module: {
    id: string
    slug: string
    publishedRevisionId: string | null
    createdAt: string
    archivedAt: string | null
  }
  draft: {
    id: string
    moduleId: string
    revisionNo: number
    title: string
    summary: string
    descriptionMarkdown: string
    difficulty: 'beginner' | 'intermediate' | 'advanced'
    lockVersion: number
    state: 'draft'
  } | null
  published: {
    id: string
    revisionNo: number
    title: string
    summary: string
    difficulty: 'beginner' | 'intermediate' | 'advanced'
    publishedAt: string
  } | null
  chapters: Array<{
    chapterId: string
    slug: string
    position: number
    draftRevisionId: string
    title: string
    access: 'public' | 'authenticated'
    bodyMarkdown: string
    isSealed: boolean
    questions: Array<{
      id: string
      position: number
      promptMarkdown: string
      errorHint: string | null
      normalization: string | null
      hasAnswer: boolean
    }>
  }>
}

const route = useRoute()
const router = useRouter()
const moduleId = computed(() => String(route.params.id || ''))

const { data: response, pending, error, refresh } = await useFetch<{ ok: boolean; data: ModuleDraftData }>(
  () => `/api/admin/modules/${moduleId.value}`
)

const moduleData = computed<ModuleDraftData | null>(() => {
  return response.value?.data || null
})

// 当前编辑的 Tab：chapters (章节内容) / basic (模块设置)
const activeTab = ref<'chapters' | 'basic'>('chapters')

// 当前选中的章节 ID
const selectedChapterId = ref<string | null>(null)

// 章节编辑器子 Tab：markdown / questions / preview
const chapterEditorTab = ref<'markdown' | 'questions' | 'preview'>('markdown')

// 本地草稿状态
const localChapters = ref<ChapterDraft[]>([])
const localModuleForm = ref({
  title: '',
  summary: '',
  difficulty: 'beginner' as 'beginner' | 'intermediate' | 'advanced',
  lockVersion: 0
})

const isDirty = ref(false)
const isSaving = ref(false)
const isPublishing = ref(false)
const toastMessage = ref('')
let toastTimer: any = null

function showToast(msg: string) {
  toastMessage.value = msg
  clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    toastMessage.value = ''
  }, 3000)
}

// 预览 Modal 状态
const isPreviewModalOpen = ref(false)
const publishErrors = ref<string[]>([])
const isPublishErrorModalOpen = ref(false)

// 同步服务器数据到本地草稿
watch(
  moduleData,
  (val) => {
    if (!val) return
    const d = val.draft || val.published
    localModuleForm.value = {
      title: d?.title || '',
      summary: d?.summary || '',
      difficulty: d?.difficulty || 'beginner',
      lockVersion: val.draft?.lockVersion || 0
    }

    localChapters.value = (val.chapters || []).map((ch) => ({
      chapterId: ch.chapterId,
      slug: ch.slug,
      position: ch.position,
      title: ch.title,
      access: ch.access,
      bodyMarkdown: ch.bodyMarkdown || '',
      isSealed: ch.isSealed,
      questions: (ch.questions || []).map((q) => ({
        id: q.id,
        position: q.position,
        promptMarkdown: q.promptMarkdown,
        answer: '', // 原始答案由服务端散列存储，修改时填入新答案
        errorHint: q.errorHint || '',
        normalization: (q.normalization as any) || 'trim'
      }))
    }))

    if (!selectedChapterId.value && localChapters.value.length > 0) {
      selectedChapterId.value = localChapters.value[0].chapterId
    }
  },
  { immediate: true }
)

const activeChapter = computed(() => {
  return localChapters.value.find((c) => c.chapterId === selectedChapterId.value) || null
})

function markDirty() {
  isDirty.value = true
}

// 添加新章节
async function handleAddChapter() {
  try {
    const res = await $fetch<{ ok: boolean; data: { chapter: { id: string } } }>(
      `/api/admin/modules/${moduleId.value}/chapters`,
      {
        method: 'POST',
        body: {
          title: `新建章节 ${localChapters.value.length + 1}`,
          access: 'public',
          bodyMarkdown: '## 章节导语\n\n在此开始撰写本章的学习内容与实操指导。'
        }
      }
    )
    await refresh()
    if (res?.data?.chapter?.id) {
      selectedChapterId.value = res.data.chapter.id
    }
    showToast('章节已添加')
  } catch (err: any) {
    showToast(err?.data?.message || '添加章节失败')
  }
}

// 章节上下重排序
async function handleMoveChapter(index: number, delta: number) {
  const targetIndex = index + delta
  if (targetIndex < 0 || targetIndex >= localChapters.value.length) return

  const copy = [...localChapters.value]
  const [removed] = copy.splice(index, 1)
  copy.splice(targetIndex, 0, removed)
  localChapters.value = copy

  const orderedChapterIds = copy.map((c) => c.chapterId)

  try {
    await $fetch(`/api/admin/modules/${moduleId.value}/chapters/reorder`, {
      method: 'PUT',
      body: { orderedChapterIds }
    })
    await refresh()
    showToast('章节排序已更新')
  } catch (err: any) {
    showToast(err?.data?.message || '更新排序失败')
    await refresh()
  }
}

// 添加练习题
function handleAddQuestion() {
  if (!activeChapter.value) return
  activeChapter.value.questions.push({
    promptMarkdown: '输入命令或答案以验证理解：',
    answer: '',
    errorHint: '请仔细核对语法与拼写',
    normalization: 'trim'
  })
  markDirty()
}

// 移除练习题
function handleRemoveQuestion(index: number) {
  if (!activeChapter.value) return
  activeChapter.value.questions.splice(index, 1)
  markDirty()
}

// 保存草稿
async function handleSaveDraft() {
  isSaving.value = true
  try {
    // 1. 保存模块基本信息
    await $fetch(`/api/admin/modules/${moduleId.value}/draft`, {
      method: 'PUT',
      body: {
        title: localModuleForm.value.title,
        summary: localModuleForm.value.summary,
        difficulty: localModuleForm.value.difficulty,
        lockVersion: localModuleForm.value.lockVersion
      }
    })

    // 2. 如果当前有选中的章节，保存当前章节草稿
    if (activeChapter.value) {
      const ch = activeChapter.value
      await $fetch(`/api/admin/chapters/${ch.chapterId}/draft`, {
        method: 'PUT',
        body: {
          moduleId: moduleId.value,
          title: ch.title,
          access: ch.access,
          bodyMarkdown: ch.bodyMarkdown,
          questions: ch.questions.map((q) => ({
            promptMarkdown: q.promptMarkdown,
            answer: q.answer ? q.answer : undefined,
            errorHint: q.errorHint,
            normalization: q.normalization
          }))
        }
      })
    }

    await refresh()
    isDirty.value = false
    showToast('草稿已保存，线上版本未改变')
  } catch (err: any) {
    if (err?.statusCode === 409) {
      showToast('保存冲突：内容已被其他管理员修改，请刷新后重试')
    } else {
      showToast(err?.data?.message || err?.message || '保存草稿失败')
    }
  } finally {
    isSaving.value = false
  }
}

// 发布模块
async function handlePublish() {
  isPublishing.value = true
  publishErrors.value = []

  try {
    // 先保存最新修改
    if (isDirty.value) {
      await handleSaveDraft()
    }

    await $fetch(`/api/admin/modules/${moduleId.value}/publish`, {
      method: 'POST'
    })

    await refresh()
    showToast('模块已成功公开发布！')
  } catch (err: any) {
    const errorData = err?.data?.data || []
    if (Array.isArray(errorData) && errorData.length > 0) {
      publishErrors.value = errorData
      isPublishErrorModalOpen.value = true
    } else {
      showToast(err?.data?.message || '发布校验失败，请检查章节与答案设置')
    }
  } finally {
    isPublishing.value = false
  }
}

// 下架模块
async function handleUnpublish() {
  if (!confirm('确认下架该模块？下架后前台学员将无法继续访问，已有历史进度与章节数据将完整保留。')) {
    return
  }

  try {
    await $fetch(`/api/admin/modules/${moduleId.value}/unpublish`, {
      method: 'POST'
    })
    await refresh()
    showToast('模块已下架')
  } catch (err: any) {
    showToast(err?.data?.message || '下架失败')
  }
}

useHead({
  title: computed(() => `编辑模块：${localModuleForm.value.title || '草稿'} — AttackBox`)
})
</script>

<template>
  <div class="module-editor-page">
    <!-- 全局 Toast 提示 -->
    <div v-if="toastMessage" class="editor-toast">
      {{ toastMessage }}
    </div>

    <!-- 加载中 -->
    <div v-if="pending" class="state-panel">
      正在加载模块详情与修订历史…
    </div>

    <!-- 异常状态 -->
    <div v-else-if="error || !moduleData" class="state-panel">
      <h2>模块不存在或无权访问</h2>
      <NuxtLink to="/admin/modules" class="back-btn">← 返回模块列表</NuxtLink>
    </div>

    <!-- 正常编辑器主体 -->
    <div v-else>
      <!-- 页面顶栏操作行 -->
      <div class="editor-head">
        <div>
          <div class="title-status-row">
            <h1 class="editor-title">{{ localModuleForm.title || '未命名模块' }}</h1>
            <span v-if="moduleData.module.publishedRevisionId" class="badge published">已发布</span>
            <span v-else class="badge draft">草稿</span>
          </div>
          <p class="module-sub-meta">
            {{ localChapters.length }} 章节 · /learn/{{ moduleData.module.slug }}
          </p>
        </div>

        <div class="action-buttons">
          <span class="save-status-text" :class="{ dirty: isDirty }">
            {{ isDirty ? '● 有未保存修改' : '✓ 草稿已保存' }}
          </span>

          <button
            type="button"
            class="btn secondary"
            @click="isPreviewModalOpen = true"
          >
            预览大纲
          </button>

          <button
            type="button"
            class="btn secondary"
            :disabled="isSaving"
            @click="handleSaveDraft"
          >
            {{ isSaving ? '保存中…' : '保存草稿' }}
          </button>

          <button
            type="button"
            class="btn primary"
            :disabled="isPublishing"
            @click="handlePublish"
          >
            {{ isPublishing ? '发布中…' : (moduleData.module.publishedRevisionId ? '发布更新' : '发布模块') }}
          </button>

          <button
            v-if="moduleData.module.publishedRevisionId"
            type="button"
            class="btn danger-outline"
            @click="handleUnpublish"
          >
            下架
          </button>
        </div>
      </div>

      <!-- 选项卡切换 -->
      <div class="tabs-bar">
        <button
          type="button"
          class="tab-link"
          :class="{ active: activeTab === 'chapters' }"
          @click="activeTab = 'chapters'"
        >
          章节内容 ({{ localChapters.length }})
        </button>
        <button
          type="button"
          class="tab-link"
          :class="{ active: activeTab === 'basic' }"
          @click="activeTab = 'basic'"
        >
          模块设置
        </button>
      </div>

      <!-- 1. 章节内容双栏编辑 -->
      <div v-if="activeTab === 'chapters'" class="editor-double-layout">
        <!-- 左侧章节列表 -->
        <aside class="chapter-sidebar">
          <div class="chapter-sidebar-head">
            <span>章节清单 · {{ localChapters.length }}</span>
            <button
              type="button"
              class="add-chapter-icon-btn"
              title="添加新章节"
              @click="handleAddChapter"
            >
              ＋
            </button>
          </div>

          <div class="chapter-items-wrap">
            <div
              v-for="(ch, idx) in localChapters"
              :key="ch.chapterId"
              class="chapter-item-row"
              :class="{ selected: ch.chapterId === selectedChapterId }"
            >
              <button
                type="button"
                class="chapter-select-btn"
                @click="selectedChapterId = ch.chapterId"
              >
                <strong class="ch-title">{{ idx + 1 }}. {{ ch.title }}</strong>
                <span class="ch-meta">{{ ch.questions.length }} 道练习 · {{ ch.access === 'authenticated' ? '需登录' : '公开' }}</span>
              </button>

              <div class="reorder-btns">
                <button
                  type="button"
                  :disabled="idx === 0"
                  aria-label="上移"
                  @click="handleMoveChapter(idx, -1)"
                >
                  ↑
                </button>
                <button
                  type="button"
                  :disabled="idx === localChapters.length - 1"
                  aria-label="下移"
                  @click="handleMoveChapter(idx, 1)"
                >
                  ↓
                </button>
              </div>
            </div>

            <div v-if="localChapters.length === 0" class="empty-chapters">
              点击上方 ＋ 添加第一个章节
            </div>
          </div>
        </aside>

        <!-- 右侧章节编辑面板 -->
        <section class="chapter-main-panel">
          <div v-if="activeChapter">
            <div class="panel-head">
              <h2>编辑章节：{{ activeChapter.title }}</h2>
            </div>

            <!-- 章节基础配置 -->
            <div class="field-grid">
              <div class="form-field">
                <label>章节名称</label>
                <input
                  v-model="activeChapter.title"
                  type="text"
                  class="form-input"
                  placeholder="例如：认识终端与 Shell"
                  @input="markDirty"
                />
              </div>

              <div class="form-field">
                <label>访问权限</label>
                <select
                  v-model="activeChapter.access"
                  class="form-input"
                  @change="markDirty"
                >
                  <option value="public">公开阅读 (无需登录)</option>
                  <option value="authenticated">登录后阅读 (需注册激活)</option>
                </select>
              </div>
            </div>

            <!-- 子 Tab：Markdown / 练习题 / 预览 -->
            <div class="editor-sub-tabs">
              <button
                type="button"
                :class="{ active: chapterEditorTab === 'markdown' }"
                @click="chapterEditorTab = 'markdown'"
              >
                Markdown 正文
              </button>
              <button
                type="button"
                :class="{ active: chapterEditorTab === 'questions' }"
                @click="chapterEditorTab = 'questions'"
              >
                章节练习 ({{ activeChapter.questions.length }})
              </button>
              <button
                type="button"
                :class="{ active: chapterEditorTab === 'preview' }"
                @click="chapterEditorTab = 'preview'"
              >
                阅读预览
              </button>
            </div>

            <!-- Markdown 编辑区 -->
            <div v-if="chapterEditorTab === 'markdown'" class="markdown-edit-area">
              <textarea
                v-model="activeChapter.bodyMarkdown"
                class="code-textarea"
                rows="18"
                placeholder="撰写 Markdown 内容，支持 # 标题、代码块与 callout 格式…"
                @input="markDirty"
              ></textarea>
              <div class="editor-tip">
                支持标准 Markdown 与代码高亮块（```bash），修改后记得保存草稿。
              </div>
            </div>

            <!-- 练习题配置 -->
            <div v-else-if="chapterEditorTab === 'questions'" class="questions-edit-area">
              <div
                v-for="(q, qIndex) in activeChapter.questions"
                :key="qIndex"
                class="question-editor-card"
              >
                <div class="q-header">
                  <strong>练习题 {{ qIndex + 1 }}</strong>
                  <button
                    type="button"
                    class="btn-delete-q"
                    @click="handleRemoveQuestion(qIndex)"
                  >
                    删除题目
                  </button>
                </div>

                <div class="form-field">
                  <label>题目要求与描述 (支持 Markdown)</label>
                  <input
                    v-model="q.promptMarkdown"
                    type="text"
                    class="form-input"
                    placeholder="例如：查看当前工作目录，应该输入哪条命令？"
                    @input="markDirty"
                  />
                </div>

                <div class="field-grid">
                  <div class="form-field">
                    <label>标准答案 * (用于服务端判题比对)</label>
                    <input
                      v-model="q.answer"
                      type="text"
                      class="form-input mono"
                      placeholder="例如：pwd"
                      @input="markDirty"
                    />
                  </div>

                  <div class="form-field">
                    <label>答案规范化规则</label>
                    <select
                      v-model="q.normalization"
                      class="form-input"
                      @change="markDirty"
                    >
                      <option value="trim">去除首尾空格 (trim)</option>
                      <option value="collapse_whitespace">合并多余空白 (collapse_whitespace)</option>
                      <option value="lowercase">忽略大小写 (lowercase)</option>
                      <option value="number">纯数字比对 (number)</option>
                    </select>
                  </div>
                </div>

                <div class="form-field">
                  <label>错误提示 (答错时向学员展示)</label>
                  <input
                    v-model="q.errorHint"
                    type="text"
                    class="form-input"
                    placeholder="例如：它是 print working directory 的缩写。"
                    @input="markDirty"
                  />
                </div>
              </div>

              <button
                type="button"
                class="btn-add-q"
                @click="handleAddQuestion"
              >
                ＋ 添加练习题
              </button>
            </div>

            <!-- 阅读预览 -->
            <div v-else-if="chapterEditorTab === 'preview'" class="preview-area">
              <div class="preview-badge">学员阅读效果 · 实时预览</div>
              <MarkdownView :content="activeChapter.bodyMarkdown" />
            </div>
          </div>

          <div v-else class="empty-selection">
            请在左侧选择一个章节进行编辑
          </div>
        </section>
      </div>

      <!-- 2. 模块设置 Tab -->
      <div v-else class="basic-settings-panel">
        <div class="form-field">
          <label>模块名称</label>
          <input
            v-model="localModuleForm.title"
            type="text"
            class="form-input"
            @input="markDirty"
          />
        </div>

        <div class="form-field">
          <label>模块访问标识 (Slug)</label>
          <input
            :value="moduleData.module.slug"
            type="text"
            class="form-input"
            disabled
          />
          <span class="field-hint">模块访问标识创建后不可修改，用于外部链接访问。</span>
        </div>

        <div class="form-field">
          <label>模块简介</label>
          <textarea
            v-model="localModuleForm.summary"
            class="form-input"
            rows="4"
            @input="markDirty"
          ></textarea>
        </div>

        <div class="form-field">
          <label>难度等级</label>
          <select
            v-model="localModuleForm.difficulty"
            class="form-input"
            @change="markDirty"
          >
            <option value="beginner">基础入门 (Beginner)</option>
            <option value="intermediate">中级进阶 (Intermediate)</option>
            <option value="advanced">高阶实战 (Advanced)</option>
          </select>
        </div>

        <div class="settings-actions">
          <button
            type="button"
            class="btn primary"
            :disabled="isSaving"
            @click="handleSaveDraft"
          >
            {{ isSaving ? '保存中…' : '保存模块设置' }}
          </button>
        </div>
      </div>
    </div>

    <!-- 预览 Modal -->
    <div v-if="isPreviewModalOpen" class="modal-backdrop" @click.self="isPreviewModalOpen = false">
      <div class="modal-card">
        <div class="modal-header">
          <h2>{{ localModuleForm.title }} · 路线大纲预览</h2>
          <button type="button" class="close-btn" @click="isPreviewModalOpen = false">✕</button>
        </div>
        <p class="preview-summary">{{ localModuleForm.summary || '暂无简介' }}</p>
        <div class="outline-preview-list">
          <div
            v-for="(ch, idx) in localChapters"
            :key="ch.chapterId"
            class="outline-item"
          >
            <strong>{{ idx + 1 }}. {{ ch.title }}</strong>
            <span class="outline-item-meta">{{ ch.questions.length }} 道练习题 · {{ ch.access === 'authenticated' ? '需登录' : '公开' }}</span>
          </div>
        </div>
        <div class="modal-footer">
          <button type="button" class="btn secondary" @click="isPreviewModalOpen = false">
            关闭预览
          </button>
        </div>
      </div>
    </div>

    <!-- 发布前阻断校验 Modal -->
    <div v-if="isPublishErrorModalOpen" class="modal-backdrop" @click.self="isPublishErrorModalOpen = false">
      <div class="modal-card">
        <div class="modal-header">
          <h2>发布前还需完善</h2>
          <button type="button" class="close-btn" @click="isPublishErrorModalOpen = false">✕</button>
        </div>
        <p class="publish-error-desc">
          发布模块要求所有章节具备正文且所有题目配置了标准答案，以下检查未通过：
        </p>
        <ul class="publish-error-list">
          <li v-for="(err, eIdx) in publishErrors" :key="eIdx">
            {{ err }}
          </li>
        </ul>
        <div class="modal-footer">
          <button type="button" class="btn primary" @click="isPublishErrorModalOpen = false">
            继续编辑并完善
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.module-editor-page {
  max-width: 1200px;
}

.editor-toast {
  position: fixed;
  top: 75px;
  right: 32px;
  background: var(--ink);
  color: var(--paper);
  padding: 10px 18px;
  border-radius: 8px;
  font-size: 13px;
  z-index: 1000;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
}

.editor-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.title-status-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.editor-title {
  font-size: 24px;
  font-weight: 750;
  margin: 0;
  color: var(--ink);
}

.module-sub-meta {
  font-size: 12px;
  color: var(--muted);
  margin: 4px 0 0;
}

.badge {
  font-size: 11px;
  padding: 2px 7px;
  border-radius: 4px;
  font-weight: 500;
}

.badge.published {
  background: var(--success-bg);
  color: var(--success-ink);
}

.badge.draft {
  background: var(--soft);
  color: var(--muted);
}

.action-buttons {
  display: flex;
  align-items: center;
  gap: 10px;
}

.save-status-text {
  font-size: 12px;
  color: var(--muted);
  margin-right: 6px;
}

.save-status-text.dirty {
  color: var(--accent);
  font-weight: 600;
}

.btn {
  padding: 7px 14px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
  border: 1px solid transparent;
}

.btn.primary {
  background: var(--green);
  color: var(--on-accent);
}

.btn.secondary {
  background: var(--paper);
  border-color: var(--line);
  color: var(--ink);
}

.btn.danger-outline {
  background: transparent;
  border-color: var(--color-error);
  color: var(--color-error);
}

/* 选项卡 */
.tabs-bar {
  display: flex;
  gap: 20px;
  border-bottom: 1px solid var(--line);
  margin-bottom: 24px;
}

.tab-link {
  background: transparent;
  border: none;
  padding: 0 0 10px;
  font-size: 14px;
  color: var(--muted);
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tab-link.active {
  color: var(--ink);
  font-weight: 600;
  border-bottom-color: var(--green);
}

/* 双栏编辑器 */
.editor-double-layout {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 24px;
  align-items: start;
}

/* 左侧章节列表 */
.chapter-sidebar {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 8px;
  overflow: hidden;
}

.chapter-sidebar-head {
  padding: 12px 14px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid var(--line);
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
}

.add-chapter-icon-btn {
  background: transparent;
  border: none;
  font-size: 16px;
  color: var(--green);
  cursor: pointer;
}

.chapter-items-wrap {
  display: flex;
  flex-direction: column;
}

.chapter-item-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-bottom: 1px solid var(--line);
  gap: 6px;
}

.chapter-item-row:last-child {
  border-bottom: none;
}

.chapter-item-row.selected {
  background: var(--mint);
}

.chapter-select-btn {
  flex: 1;
  text-align: left;
  background: transparent;
  border: none;
  cursor: pointer;
  padding: 0;
  display: flex;
  flex-direction: column;
}

.ch-title {
  font-size: 13px;
  color: var(--ink);
  margin-bottom: 2px;
}

.ch-meta {
  font-size: 11px;
  color: var(--muted);
}

.reorder-btns {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.reorder-btns button {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 3px;
  padding: 0 4px;
  font-size: 10px;
  cursor: pointer;
  color: var(--muted);
}

.reorder-btns button:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.empty-chapters {
  padding: 24px;
  text-align: center;
  font-size: 12px;
  color: var(--muted);
}

/* 右侧面板 */
.chapter-main-panel,
.basic-settings-panel {
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 28px;
}

.panel-head h2 {
  font-size: 18px;
  font-weight: 700;
  margin: 0 0 20px;
  color: var(--ink);
}

.field-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-bottom: 18px;
}

.form-field {
  margin-bottom: 16px;
}

.form-field label {
  display: block;
  font-size: 12px;
  font-weight: 600;
  color: var(--ink);
  margin-bottom: 6px;
}

.form-input {
  width: 100%;
  padding: 8px 12px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--bg);
  color: var(--ink);
  font-size: 13px;
  box-sizing: border-box;
}

.form-input.mono {
  font-family: monospace;
}

.form-input:focus {
  outline: 2px solid var(--green);
}

.field-hint {
  font-size: 11px;
  color: var(--muted);
  margin-top: 4px;
  display: block;
}

/* 编辑器子 Tab */
.editor-sub-tabs {
  display: flex;
  gap: 20px;
  border-bottom: 1px solid var(--line);
  margin: 24px 0 16px;
}

.editor-sub-tabs button {
  background: transparent;
  border: none;
  padding: 0 0 8px;
  font-size: 13px;
  color: var(--muted);
  border-bottom: 2px solid transparent;
  cursor: pointer;
}

.editor-sub-tabs button.active {
  color: var(--ink);
  font-weight: 600;
  border-bottom-color: var(--green);
}

.code-textarea {
  width: 100%;
  background: var(--bg);
  border: 1px solid var(--line);
  border-radius: 6px;
  padding: 12px;
  font-family: monospace;
  font-size: 13px;
  line-height: 1.8;
  color: var(--ink);
  resize: vertical;
  box-sizing: border-box;
  outline: none;
}

.code-textarea:focus {
  border-color: var(--green);
}

.editor-tip {
  font-size: 12px;
  color: var(--muted);
  margin-top: 8px;
}

/* 题目编辑器 */
.question-editor-card {
  padding: 18px 0;
  border-bottom: 1px solid var(--line);
}

.q-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  font-size: 13px;
}

.btn-delete-q {
  background: transparent;
  border: none;
  color: var(--color-error);
  font-size: 12px;
  cursor: pointer;
}

.btn-add-q {
  margin-top: 16px;
  padding: 8px 16px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 6px;
  font-size: 13px;
  color: var(--ink);
  cursor: pointer;
}

.preview-area {
  padding-top: 8px;
}

.preview-badge {
  font-size: 11px;
  color: var(--muted);
  margin-bottom: 16px;
}

.empty-selection {
  padding: 64px;
  text-align: center;
  color: var(--muted);
}

.settings-actions {
  margin-top: 24px;
}

/* Modal */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
  padding: 16px;
}

.modal-card {
  max-width: 560px;
  width: 100%;
  background: var(--paper);
  border: 1px solid var(--line);
  border-radius: 12px;
  padding: 24px;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.modal-header h2 {
  font-size: 18px;
  margin: 0;
  color: var(--ink);
}

.close-btn {
  background: transparent;
  border: none;
  font-size: 16px;
  cursor: pointer;
  color: var(--muted);
}

.preview-summary {
  font-size: 13px;
  color: var(--muted);
  margin-bottom: 20px;
}

.outline-preview-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 360px;
  overflow-y: auto;
}

.outline-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: var(--soft);
  border-radius: 6px;
  font-size: 13px;
}

.outline-item-meta {
  font-size: 11px;
  color: var(--muted);
}

.publish-error-desc {
  font-size: 13px;
  color: var(--color-error);
  margin-bottom: 16px;
}

.publish-error-list {
  padding-left: 20px;
  font-size: 13px;
  color: var(--ink);
  line-height: 1.6;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 24px;
}

@media (max-width: 900px) {
  .editor-double-layout {
    grid-template-columns: 1fr;
  }
}
</style>
