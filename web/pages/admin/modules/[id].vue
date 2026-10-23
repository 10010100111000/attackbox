<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter, onBeforeRouteLeave } from 'vue-router'
import { useFetch, useHead } from '#imports'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import type { Normalization, ContentCategory, AdminModuleDetail, Difficulty } from '~/shared/content'
definePageMeta({ layout: 'admin' })
interface QuestionDraft { id?: string; questionKey?: string; position?: number; promptMarkdown: string; answer: string; hasAnswer?: boolean; clearAnswer?: boolean; errorHint: string; normalization: Normalization }
interface ChapterDraft { chapterId: string; slug: string; position: number; title: string; access: 'public' | 'authenticated'; bodyMarkdown: string; isSealed: boolean; questions: QuestionDraft[] }
const route = useRoute(), router = useRouter()
const moduleId = computed(() => String(route.params.id || ''))
const isNew = computed(() => moduleId.value === 'new')
const { data: response, pending, error, refresh } = await useFetch<{ ok: boolean; data: AdminModuleDetail }>(() => `/api/admin/modules/${moduleId.value}`, { immediate: !isNew.value, watch: false })
const { data: categoryResponse, refresh: refreshCategories } = await useFetch<{ data: ContentCategory[] }>('/api/categories')
const categories = computed(() => categoryResponse.value?.data || [])
const isAddingCategory = ref(false), newCategoryName = ref(''), categorySaving = ref(false)
async function addCategory() {
  if (categorySaving.value || !newCategoryName.value.trim()) return
  categorySaving.value = true
  try {
    const response = await $fetch<{ data: ContentCategory }>('/api/admin/categories', { method: 'POST', body: { name: newCategoryName.value } })
    await refreshCategories(); localModuleForm.value.categoryId = response.data.id
    isAddingCategory.value = false; newCategoryName.value = ''; markDirty()
  } catch (err) { showToast(message(err)) } finally { categorySaving.value = false }
}
function localId() { return globalThis.crypto?.randomUUID?.() || `${Date.now().toString(36)}-${Math.random().toString(36).slice(2,10)}` }

const moduleData = computed<AdminModuleDetail | null>(() => isNew.value ? { module: { id: 'new', slug: '', publishedRevisionId: null, createdAt: '', archivedAt: null }, draft: null, published: null, editable: null, category: null, isPublished: false, hasDraft: false, chapterCount: 0, baseRevisionId: null, lockVersion: 0, chapters: [] } : response.value?.data || null)
const activeTab = ref<'chapters' | 'basic'>(isNew.value ? 'basic' : 'chapters')
const selectedChapterId = ref<string | null>(null)
const chapterEditorTab = ref<'markdown' | 'questions' | 'preview'>('markdown')
const localChapters = ref<ChapterDraft[]>([])
const localModuleForm = ref({ title: '', slug: '', summary: '', descriptionMarkdown: '', difficulty: 'beginner' as Difficulty, categoryId: null as string | null, lockVersion: 0 })
const isDirty = ref(false), isSaving = ref(false), isPublishing = ref(false), isMutating = ref(false), toastMessage = ref('')
const busy = computed(() => isSaving.value || isPublishing.value || isMutating.value)
let toastTimer: ReturnType<typeof setTimeout> | undefined
const isPreviewModalOpen = ref(false), publishErrors = ref<string[]>([]), isPublishErrorModalOpen = ref(false)
function showToast(msg: string) { toastMessage.value = msg; clearTimeout(toastTimer); toastTimer = setTimeout(() => toastMessage.value = '', 4000) }
function message(err: any) { return err?.data?.message || err?.data?.statusMessage || err?.message || '操作失败' }
watch(response, val => {
  if (!val?.data || isNew.value) return
  const data = val.data, d = data.editable || data.draft || data.published
  const previousSlug = localChapters.value.find(c => c.chapterId === selectedChapterId.value)?.slug
  localModuleForm.value = { title: d?.title || '', slug: data.module.slug, summary: d?.summary || '', descriptionMarkdown: d?.descriptionMarkdown || '', difficulty: d?.difficulty || 'beginner', categoryId: d?.categoryId || null, lockVersion: data.lockVersion ?? data.draft?.lockVersion ?? 0 }
  localChapters.value = (data.chapters || []).map((ch) => ({ ...ch, questions: (ch.questions || []).map((q) => ({ ...q, answer: '', clearAnswer: false, errorHint: q.errorHint || '' })) }))
  if (!localChapters.value.some(c => c.chapterId === selectedChapterId.value)) selectedChapterId.value = localChapters.value.find(c => c.slug === previousSlug)?.chapterId || localChapters.value[0]?.chapterId || null
}, { immediate: true })
const activeChapter = computed(() => localChapters.value.find(c => c.chapterId === selectedChapterId.value) || null)
function markDirty() { isDirty.value = true }
function handleAddChapter() {
  const id = `local-${localId()}`
  localChapters.value.push({ chapterId: id, slug: `chapter-${localId().slice(0,8)}`, position: localChapters.value.length + 1, title: `新建章节 ${localChapters.value.length + 1}`, access: 'public', bodyMarkdown: '', isSealed: false, questions: [] })
  selectedChapterId.value = id; activeTab.value = 'chapters'; markDirty()
}
function handleMoveChapter(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= localChapters.value.length) return
  const copy = [...localChapters.value], [chapter] = copy.splice(index, 1)
  copy.splice(target, 0, chapter); copy.forEach((ch,i) => ch.position = i+1); localChapters.value = copy; markDirty()
}
function handleAddQuestion() { activeChapter.value?.questions.push({ promptMarkdown: '', answer: '', errorHint: '', normalization: 'trim', hasAnswer: false }); markDirty() }
function handleRemoveQuestion(index: number) { activeChapter.value?.questions.splice(index, 1); markDirty() }
async function handleSaveDraft(): Promise<boolean> {
  if (isSaving.value) return false
  if (!localModuleForm.value.title.trim()) { activeTab.value = 'basic'; showToast('请填写模块名称'); return false }
  isSaving.value = true
  try {
    const payload = { ...localModuleForm.value,
      baseRevisionId: isNew.value ? undefined : response.value?.data?.baseRevisionId,
      lockVersion: isNew.value ? undefined : localModuleForm.value.lockVersion,
      chapters: localChapters.value.map(ch => ({
        chapterId: ch.chapterId.startsWith('local-') ? undefined : ch.chapterId,
        slug: ch.slug, title: ch.title, access: ch.access, bodyMarkdown: ch.bodyMarkdown,
        questions: ch.questions.map(q => ({ id: q.id, questionKey: q.questionKey, promptMarkdown: q.promptMarkdown,
          answer: q.answer.length ? q.answer : undefined, clearAnswer: q.clearAnswer || false, errorHint: q.errorHint, normalization: q.normalization }))
      })) }
    if (isNew.value) {
      const res = await $fetch<any>('/api/admin/modules', { method: 'POST', body: payload })
      isDirty.value = false
      await router.replace(`/admin/modules/${res.data.module.id}`)
    } else await $fetch(`/api/admin/modules/${moduleId.value}/draft`, { method: 'PUT', body: payload })
    await refresh(); isDirty.value = false; showToast('全部草稿已保存，线上版本未改变'); return true
  } catch (err) { showToast(message(err)); return false }
  finally { isSaving.value = false }
}
async function handlePublish() {
  if (busy.value) return
  isPublishing.value = true; publishErrors.value = []
  try {
    if ((isNew.value || isDirty.value || !moduleData.value?.draft) && !(await handleSaveDraft())) return
    await $fetch(`/api/admin/modules/${moduleId.value}/publish`, { method: 'POST', body: { lockVersion: localModuleForm.value.lockVersion, baseRevisionId: response.value?.data?.baseRevisionId } })
    await refresh(); isDirty.value = false; showToast('模块已发布')
  } catch (err) { showToast(message(err)) }
  finally { isPublishing.value = false }
}
async function handleUnpublish() {
  if (busy.value || !confirm('确认下架？章节与历史版本会完整保留。')) return
  if (isDirty.value && !confirm('继续将放弃未保存的本地修改，是否继续？')) return
  isMutating.value = true
  try { await $fetch(`/api/admin/modules/${moduleId.value}/unpublish`, { method: 'POST' }); await refresh(); isDirty.value = false; showToast('已下架，内容已保留为草稿') }
  catch (err) { showToast(message(err)) } finally { isMutating.value = false }
}
async function handleDiscardDraft() {
  if (busy.value || !confirm('移除保存的草稿并放弃本地修改？历史发布版本不会删除。')) return
  isMutating.value = true
  try { await $fetch(`/api/admin/modules/${moduleId.value}/draft`, { method: 'DELETE', body: { baseRevisionId: response.value?.data?.baseRevisionId, lockVersion: localModuleForm.value.lockVersion } }); await refresh(); isDirty.value = false; showToast('草稿已移除，已回到历史发布内容') }
  catch (err) { showToast(message(err)) } finally { isMutating.value = false }
}
async function handleDeleteModule() {
  if (busy.value || !confirm(isNew.value ? '放弃未保存内容并取消新建？' : '删除该模块？它将从学习页和管理列表移除，历史内容保留。')) return
  isMutating.value = true
  try { if (!isNew.value) await $fetch(`/api/admin/modules/${moduleId.value}`, { method: 'DELETE' }); isDirty.value = false; await router.push('/admin/modules') }
  catch (err) { showToast(message(err)) } finally { isMutating.value = false }
}
function beforeUnload(event: BeforeUnloadEvent) { if (isDirty.value) { event.preventDefault(); event.returnValue = '' } }
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
onBeforeUnmount(() => { clearTimeout(toastTimer); window.removeEventListener('beforeunload', beforeUnload) })
onBeforeRouteLeave(() => !isDirty.value || confirm('有未保存修改，确认离开？'))
useHead({ title: computed(() => `${isNew.value ? '新建' : '编辑'}模块：${localModuleForm.value.title || '草稿'} — AttackBox`) })
</script>
<template>
  <div class="module-editor-page">
    <!-- 全局 Toast 提示 -->
    <div v-if="toastMessage" class="editor-toast">
      {{ toastMessage }}
    </div>

    <!-- 加载中 -->
    <div v-if="pending && !isNew" class="state-panel">
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
          <button v-if="!isNew && moduleData.draft && (moduleData.published || moduleData.editable?.revisionNo > 1)" type="button" class="btn secondary" :disabled="busy" @click="handleDiscardDraft">移除草稿</button>
          <button type="button" class="btn danger-outline" :disabled="busy" @click="handleDeleteModule">{{ isNew ? '取消新建' : '删除模块' }}</button>
          <span class="save-status-text" :class="{ dirty: isDirty }">
            {{ isNew ? '首次保存后创建模块' : isDirty ? '● 有未保存修改' : '✓ 已保存' }}
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
            :disabled="busy"
            @click="handleSaveDraft"
          >
            {{ isSaving ? '保存中…' : '保存草稿' }}
          </button>

          <button
            type="button"
            class="btn primary"
            :disabled="busy"
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

      <fieldset :disabled="busy" class="editor-content">
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
                  :disabled="busy || idx === 0"
                  aria-label="上移"
                  @click="handleMoveChapter(idx, -1)"
                >
                  ↑
                </button>
                <button
                  type="button"
                  :disabled="busy || idx === localChapters.length - 1"
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
                    <label>标准答案 * (用于服务端判题比对)</label><small v-if="q.hasAnswer">已设置；留空保留。更换规则时需重填。</small><label v-if="q.hasAnswer"><input v-model="q.clearAnswer" type="checkbox" @change="q.answer = ''; markDirty()" /> 清除已保存答案</label>
                    <input
                      v-model="q.answer"
                      :disabled="q.clearAnswer"
                      type="text"
                      class="form-input mono"
                      :placeholder="q.hasAnswer ? '已设置，留空保留原答案' : '填写标准答案'"
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
                      <option value="exact">精确匹配 (exact)</option>
                      <option value="trim_casefold">去首尾空格并忽略大小写 (trim_casefold)</option>
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
        <div class="form-field"><label for="module-category">内容分类（与难度独立）</label><select id="module-category" v-model="localModuleForm.categoryId" class="form-input" @change="markDirty"><option :value="null">未分类</option><option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option></select>
            <button v-if="!isAddingCategory" type="button" class="btn secondary" @click="isAddingCategory = true">＋ 新建分类</button>
            <div v-else class="category-create"><input v-model="newCategoryName" class="form-input" placeholder="分类名称，例如 Web 安全" @keydown.enter.prevent="addCategory" /><button type="button" class="btn secondary" :disabled="categorySaving" @click="addCategory">添加分类</button><button type="button" class="btn secondary" @click="isAddingCategory = false">取消</button></div>
          </div>
          <div v-if="isNew" class="form-field"><label>访问标识（留空自动生成）</label><input v-model="localModuleForm.slug" class="form-input" placeholder="例如 web-security" @input="markDirty" /></div>
          <div class="form-field">
          <label for="module-title">模块名称</label>
          <input
            id="module-title"
            v-model="localModuleForm.title"
            type="text"
            class="form-input"
            @input="markDirty"
          />
        </div>

        <div v-if="!isNew" class="form-field">
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
          <label for="module-summary">模块简介</label>
          <textarea
            id="module-summary"
            v-model="localModuleForm.summary"
            class="form-input"
            rows="4"
            @input="markDirty"
          ></textarea>
        </div>

        <div class="form-field">
          <label for="module-difficulty">难度等级</label>
          <select
            id="module-difficulty"
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
            :disabled="busy"
            @click="handleSaveDraft"
          >
            {{ isSaving ? '保存中…' : '保存模块设置' }}
          </button>
        </div>
      </div>
      </fieldset>
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
.editor-content { border: 0; padding: 0; margin: 0; min-width: 0; }
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
