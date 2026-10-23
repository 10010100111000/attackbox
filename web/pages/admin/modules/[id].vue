<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter, onBeforeRouteLeave } from 'vue-router'
import { useFetch, useHead } from '#imports'
import { Plus, ArrowUp, ArrowDown, X, Check, Circle, MoreHorizontal, Save, Send, ArrowLeft, Link2 } from 'lucide-vue-next'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import type { Normalization, ContentCategory, AdminModuleDetail, AdminModuleListItem, Difficulty } from '~/shared/content'
definePageMeta({ layout: 'admin' })
interface QuestionDraft { id?: string; position?: number; promptMarkdown: string; answer: string; hasAnswer?: boolean; clearAnswer?: boolean; errorHint: string; normalization: Normalization }
interface ChapterDraft { chapterId: string; slug: string; position: number; title: string; access: 'public' | 'authenticated'; bodyMarkdown: string; durationMinutes: number | null | ''; questions: QuestionDraft[] }
const route = useRoute(), router = useRouter()
const moduleId = computed(() => String(route.params.id || ''))
const isNew = computed(() => moduleId.value === 'new')
const { data: response, pending, error, refresh } = await useFetch<{ ok: boolean; data: AdminModuleDetail }>(() => `/api/admin/modules/${moduleId.value}`, { immediate: !isNew.value, watch: false })
const { data: categoryResponse, refresh: refreshCategories } = await useFetch<{ data: ContentCategory[] }>('/api/categories')
const categories = computed(() => categoryResponse.value?.data || [])
const { data: moduleOptions, error: moduleOptionsError, refresh: refreshModuleOptions } = await useFetch<{ data: AdminModuleListItem[] }>('/api/admin/modules')
const prerequisiteIds = ref<string[]>([]), prerequisiteSearch = ref(''), prerequisitePickerOpen = ref(false)
const prerequisiteCandidates = computed(() => (moduleOptions.value?.data || []).filter(m => m.id !== moduleId.value && !prerequisiteIds.value.includes(m.id) && `${m.title} ${m.slug}`.toLowerCase().includes(prerequisiteSearch.value.toLowerCase())).slice(0, 6))
function prerequisiteInfo(id: string) { return moduleOptions.value?.data.find(m => m.id === id) || response.value?.data.prerequisites.find(m => m.id === id) }
function addPrerequisite(id: string) { prerequisiteIds.value.push(id); prerequisiteSearch.value = ''; prerequisitePickerOpen.value = false; markDirty() }
function movePrerequisite(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= prerequisiteIds.value.length) return
  const copy = [...prerequisiteIds.value]; [copy[index], copy[target]] = [copy[target], copy[index]]
  prerequisiteIds.value = copy; markDirty()
}
const descriptionPreview = ref(false)
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

const moduleData = computed<AdminModuleDetail | null>(() => response.value?.data || null)
const isPublished = computed(() => !isNew.value && moduleData.value?.module.status === 'published')
const moreOpen = ref(false)
const activeTab = ref<'chapters' | 'basic'>(route.query.tab === 'chapters' ? 'chapters' : 'basic')
const selectedChapterId = ref<string | null>(null)
const chapterEditorTab = ref<'markdown' | 'questions' | 'preview'>('markdown')
const localChapters = ref<ChapterDraft[]>([])
const localModuleForm = ref({ title: '', slug: '', summary: '', descriptionMarkdown: '', difficulty: 'beginner' as Difficulty, categoryId: null as string | null, learningOutcomes: '', lockVersion: 0 })
const isDirty = ref(false), isSaving = ref(false), saveIntent = ref<'draft' | 'publish' | null>(null), isMutating = ref(false), toastMessage = ref('')
const busy = computed(() => isSaving.value || isMutating.value)
let toastTimer: ReturnType<typeof setTimeout> | undefined
function showToast(msg: string) { toastMessage.value = msg; clearTimeout(toastTimer); toastTimer = setTimeout(() => toastMessage.value = '', 4000) }
function message(err: any) { return err?.data?.message || err?.data?.statusMessage || err?.message || '操作失败' }
watch(response, val => {
  if (!val?.data || isNew.value) return
  const data = val.data, d = data.module
  prerequisiteIds.value = data.prerequisites.map(m => m.id)
  const previousSlug = localChapters.value.find(c => c.chapterId === selectedChapterId.value)?.slug
  localModuleForm.value = { title: d?.title || '', slug: data.module.slug, summary: d?.summary || '', descriptionMarkdown: d?.descriptionMarkdown || '', difficulty: d?.difficulty || 'beginner', categoryId: d?.categoryId || null, learningOutcomes: d.learningOutcomes.join('\n'), lockVersion: d.lockVersion }
  localChapters.value = (data.chapters || []).map((ch) => ({ ...ch, questions: (ch.questions || []).map((q) => ({ ...q, answer: '', clearAnswer: false, errorHint: q.errorHint || '' })) }))
  if (!localChapters.value.some(c => c.chapterId === selectedChapterId.value)) selectedChapterId.value = localChapters.value.find(c => c.slug === previousSlug)?.chapterId || localChapters.value[0]?.chapterId || null
}, { immediate: true })
const activeChapter = computed(() => localChapters.value.find(c => c.chapterId === selectedChapterId.value) || null)
function markDirty() { isDirty.value = true }
function handleAddChapter() {
  const id = `local-${localId()}`
  localChapters.value.push({ chapterId: id, slug: `chapter-${localId().slice(0,8)}`, position: localChapters.value.length + 1, title: `新建章节 ${localChapters.value.length + 1}`, access: 'public', bodyMarkdown: '', durationMinutes: null, questions: [] })
  selectedChapterId.value = id; activeTab.value = 'chapters'; markDirty()
}
function handleRemoveChapter(index: number) {
  const target = localChapters.value[index]
  if (!target) return
  if (!confirm(`删除章节「${target.title}」？保存后生效。`)) return
  localChapters.value.splice(index, 1)
  localChapters.value.forEach((ch, i) => { ch.position = i + 1 })
  if (selectedChapterId.value === target.chapterId) {
    selectedChapterId.value = localChapters.value[Math.min(index, localChapters.value.length - 1)]?.chapterId || null
  }
  markDirty()
}
function handleMoveChapter(index: number, delta: number) {
  const target = index + delta
  if (target < 0 || target >= localChapters.value.length) return
  const copy = [...localChapters.value], [chapter] = copy.splice(index, 1)
  copy.splice(target, 0, chapter); copy.forEach((ch,i) => ch.position = i+1); localChapters.value = copy; markDirty()
}
function handleAddQuestion() { activeChapter.value?.questions.push({ promptMarkdown: '', answer: '', errorHint: '', normalization: 'trim', hasAnswer: false }); markDirty() }
function handleRemoveQuestion(index: number) { activeChapter.value?.questions.splice(index, 1); markDirty() }
async function handleSave(intent: 'draft' | 'publish'): Promise<boolean> {
  if (busy.value) return false
  if (intent === 'draft' && isPublished.value && !confirm('保存为草稿会同时下架模块，学习者将暂时无法访问。确认继续？')) return false
  if (!localModuleForm.value.title.trim()) { activeTab.value = 'basic'; showToast('请填写模块名称'); return false }
  isSaving.value = true; saveIntent.value = intent
  try {
    const payload = { ...localModuleForm.value, intent, prerequisiteModuleIds: prerequisiteIds.value,
      learningOutcomes: localModuleForm.value.learningOutcomes.split(/\r?\n/).map(line => line.trim()).filter(Boolean),
      lockVersion: isNew.value ? undefined : localModuleForm.value.lockVersion,
      chapters: localChapters.value.map(ch => ({
        chapterId: ch.chapterId.startsWith('local-') ? undefined : ch.chapterId,
        slug: ch.slug, title: ch.title, access: ch.access, bodyMarkdown: ch.bodyMarkdown, durationMinutes: ch.durationMinutes === '' ? null : ch.durationMinutes,
        questions: ch.questions.map(q => ({ id: q.id, promptMarkdown: q.promptMarkdown,
          answer: q.answer.length ? q.answer : undefined, clearAnswer: q.clearAnswer || false, errorHint: q.errorHint, normalization: q.normalization }))
      })) }
    if (isNew.value) {
      const res = await $fetch<any>('/api/admin/modules', { method: 'POST', body: payload })
      isDirty.value = false
      await router.replace({ path: `/admin/modules/${res.data.module.id}`, query: { tab: activeTab.value } })
    } else await $fetch(`/api/admin/modules/${moduleId.value}`, { method: 'PUT', body: payload })
    await refresh(); isDirty.value = false; showToast(intent === 'publish' ? '已保存并发布' : '已保存为草稿'); return true
  } catch (err) { showToast(message(err)); return false }
  finally { isSaving.value = false; saveIntent.value = null }
}
async function handleDeleteModule() {
  if (busy.value || !confirm(isNew.value ? '放弃未保存内容并取消新建？' : '永久删除该模块及其全部章节和题目？此操作无法撤销。')) return
  isMutating.value = true
  try { if (!isNew.value) await $fetch(`/api/admin/modules/${moduleId.value}`, { method: 'DELETE', body: { lockVersion: localModuleForm.value.lockVersion } }); isDirty.value = false; await router.push('/admin/modules') }
  catch (err) { showToast(message(err)) } finally { isMutating.value = false }
}
function beforeUnload(event: BeforeUnloadEvent) { if (isDirty.value) { event.preventDefault(); event.returnValue = '' } }
onMounted(() => window.addEventListener('beforeunload', beforeUnload))
onBeforeUnmount(() => { clearTimeout(toastTimer); window.removeEventListener('beforeunload', beforeUnload) })
onBeforeRouteLeave(() => !isDirty.value || confirm('有未保存修改，确认离开？'))
useHead({ title: computed(() => `${isNew.value ? '新建' : '编辑'}模块：${localModuleForm.value.title || '未命名模块'} — AttackBox`) })
</script>
<template>
  <div class="module-editor-page">
    <!-- 全局 Toast 提示 -->
    <div v-if="toastMessage" class="editor-toast">
      {{ toastMessage }}
    </div>

    <!-- 加载中 -->
    <div v-if="pending && !isNew" class="state-panel">
      正在加载模块详情…
    </div>

    <!-- 异常状态 -->
    <div v-else-if="!isNew && (error || !moduleData)" class="state-panel">
      <h2>模块不存在或无权访问</h2>
      <NuxtLink to="/admin/modules" class="back-btn"><ArrowLeft :size="15" aria-hidden="true" />返回学习模块</NuxtLink>
    </div>

    <!-- 正常编辑器主体 -->
    <div v-else>
      <!-- 页面顶栏操作行 -->
      <div class="editor-head">
        <div>
          <div class="title-status-row">
            <h1 class="editor-title">{{ localModuleForm.title || '未命名模块' }}</h1>
            <span v-if="isPublished" class="badge published">已发布</span>
            <span v-else class="badge unpublished">草稿</span>
          </div>
          <p class="module-sub-meta">
            {{ localChapters.length }} 章节<span v-if="!isNew"> · /learn/modules/{{ localModuleForm.slug }}</span><span v-if="isPublished"> · 保存并发布后更新线上内容</span>
          </p>
        </div>

        <div class="action-buttons">
          <span class="save-status-text" :class="{ dirty: isDirty }">
            <Circle v-if="isDirty" :size="12" aria-hidden="true" /><Check v-else-if="!isNew" :size="14" aria-hidden="true" />{{ isNew ? '尚未创建' : isDirty ? '有未保存修改' : '已保存' }}
          </span>

          <button type="button" class="btn secondary" :disabled="busy || (!isNew && !isDirty && !isPublished)" @click="handleSave('draft')">
            <Save :size="15" aria-hidden="true" />{{ saveIntent === 'draft' ? '保存中…' : '保存为草稿' }}
          </button>
          <button type="button" class="btn primary" :disabled="busy || (!isNew && !isDirty && isPublished)" @click="handleSave('publish')">
            <Send :size="15" aria-hidden="true" />{{ saveIntent === 'publish' ? '发布中…' : '保存并发布' }}
          </button>

          <div class="more-actions">
            <button type="button" class="btn secondary" :disabled="busy" :aria-expanded="moreOpen" @click="moreOpen = !moreOpen"><MoreHorizontal :size="18" aria-hidden="true" /><span class="sr-only">更多</span></button>
            <div v-if="moreOpen" class="more-menu">
              <button type="button" class="danger" :disabled="busy" @click="handleDeleteModule">{{ isNew ? '取消新建' : '删除模块' }}</button>
            </div>
          </div>
        </div>
      </div>

      <fieldset :disabled="busy" class="editor-content">
      <!-- 选项卡切换 -->
      <div class="tabs-bar">
        <button
          type="button"
          class="tab-link"
          :class="{ active: activeTab === 'basic' }"
          @click="activeTab = 'basic'"
        >
          模块设置
        </button>
        <button
          type="button"
          class="tab-link"
          :class="{ active: activeTab === 'chapters' }"
          @click="activeTab = 'chapters'"
        >
          章节内容 ({{ localChapters.length }})
        </button>
      </div>

      <!-- 1. 章节内容双栏编辑 -->
      <div v-if="activeTab === 'chapters'" class="editor-double-layout">
        <!-- 左侧章节列表 -->
        <aside class="chapter-sidebar">
          <div class="chapter-sidebar-head">
            <span>章节内容 · {{ localChapters.length }}</span>
            <button
              type="button"
              class="add-chapter-icon-btn"
              title="添加新章节"
              @click="handleAddChapter"
            >
              <Plus :size="17" aria-hidden="true" />
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
                <span class="ch-meta">{{ ch.questions.length }} 道练习<span v-if="ch.durationMinutes"> · {{ ch.durationMinutes }} 分钟</span> · {{ ch.access === 'authenticated' ? '需登录' : '公开' }}</span>
              </button>

              <div class="reorder-btns">
                <button
                  type="button"
                  :disabled="busy || idx === 0"
                  aria-label="上移"
                  @click="handleMoveChapter(idx, -1)"
                >
                  <ArrowUp :size="14" aria-hidden="true" />
                </button>
                <button
                  type="button"
                  :disabled="busy || idx === localChapters.length - 1"
                  aria-label="下移"
                  @click="handleMoveChapter(idx, 1)"
                >
                  <ArrowDown :size="14" aria-hidden="true" />
                </button>
              </div>
              <button
                type="button"
                class="chapter-delete-btn"
                :disabled="busy"
                aria-label="删除章节"
                title="删除章节"
                @click="handleRemoveChapter(idx)"
              >
                <X :size="14" aria-hidden="true" />
              </button>
            </div>

            <div v-if="localChapters.length === 0" class="empty-chapters">
              添加第一个章节
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
            <div class="field-grid chapter-fields">
              <div class="form-field">
                <label for="chapter-title">章节名称</label>
                <input
                  id="chapter-title"
                  v-model="activeChapter.title"
                  type="text"
                  class="form-input"
                  placeholder="例如：认识终端与 Shell"
                  @input="markDirty"
                />
              </div>

              <div class="form-field">
                <label for="chapter-access">访问权限</label>
                <select
                  id="chapter-access"
                  v-model="activeChapter.access"
                  class="form-input"
                  @change="markDirty"
                >
                  <option value="public">公开</option>
                  <option value="authenticated">登录后阅读 (需注册激活)</option>
                </select>
              </div>
              <div class="form-field">
                <label for="chapter-duration">预计时长</label>
                <div class="duration-input"><input id="chapter-duration" v-model.number="activeChapter.durationMinutes" type="number" min="1" max="1440" step="1" class="form-input" placeholder="选填" @input="markDirty" /><span>分钟</span></div>
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
                章节实操练习 ({{ activeChapter.questions.length }})
              </button>
              <button
                type="button"
                :class="{ active: chapterEditorTab === 'preview' }"
                @click="chapterEditorTab = 'preview'"
              >
                预览
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
                支持标准 Markdown 与代码高亮块（```bash），修改后点击保存。
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
                <Plus :size="15" aria-hidden="true" /> 添加练习题
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
      <div v-else class="basic-settings-panel settings-grid">
        <section class="settings-card basics-card" aria-labelledby="basic-heading">
          <header class="settings-heading"><h2 id="basic-heading">基本信息</h2><p>展示在模块列表与详情页。</p></header>
          <div class="form-field">
            <label for="module-title">模块名称 <span class="required-label" aria-hidden="true">必填</span></label>
            <input id="module-title" v-model="localModuleForm.title" type="text" class="form-input" maxlength="160" placeholder="例如 Linux 权限与访问控制" @input="markDirty" />
          </div>
          <div class="settings-field-row">
            <div class="form-field">
              <div class="field-label-row"><label for="module-category">内容分类</label><button v-if="!isAddingCategory" class="text-action" type="button" @click="isAddingCategory = true"><Plus :size="13" aria-hidden="true" />新建</button></div>
              <select id="module-category" v-model="localModuleForm.categoryId" class="form-input" @change="markDirty"><option :value="null">未分类</option><option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option></select>
              <div v-if="isAddingCategory" class="category-create"><input v-model="newCategoryName" class="form-input" aria-label="新分类名称" placeholder="分类名称" @keydown.enter.prevent="addCategory" /><button type="button" class="text-action" :disabled="categorySaving" @click="addCategory">添加</button><button type="button" class="icon-action" aria-label="取消新建分类" @click="isAddingCategory = false"><X :size="14" /></button></div>
            </div>
            <div class="form-field"><label for="module-difficulty">学习难度</label><select id="module-difficulty" v-model="localModuleForm.difficulty" class="form-input" @change="markDirty"><option value="beginner">基础入门</option><option value="intermediate">中级进阶</option><option value="advanced">高阶实战</option></select></div>
          </div>
          <div class="form-field">
            <label for="module-summary">模块简介</label>
            <textarea id="module-summary" v-model="localModuleForm.summary" class="form-input" rows="3" maxlength="500" placeholder="简要说明模块主题，出现在模块卡片与详情页顶部。" @input="markDirty"></textarea>
          </div>
          <div class="form-field">
            <div class="field-label-row"><label for="module-description">模块介绍（Markdown）</label><button class="text-action" type="button" :aria-pressed="descriptionPreview" @click="descriptionPreview = !descriptionPreview">{{ descriptionPreview ? '编辑' : '预览' }}</button></div>
            <div v-if="descriptionPreview" class="description-preview"><MarkdownView :content="localModuleForm.descriptionMarkdown" /><p v-if="!localModuleForm.descriptionMarkdown" class="field-hint">尚未填写模块介绍。</p></div>
            <textarea v-else id="module-description" v-model="localModuleForm.descriptionMarkdown" class="form-input description-editor" rows="8" placeholder="补充模块范围、练习方式或适用对象，留空则不展示。" @input="markDirty"></textarea>
          </div>
          <div class="slug-setting"><Link2 :size="15" aria-hidden="true" /><div class="slug-setting-body"><label for="module-slug">访问标识</label><input id="module-slug" v-model="localModuleForm.slug" :disabled="!isNew" class="form-input" placeholder="留空自动生成，例如 linux-permissions" @input="markDirty" /><span class="field-hint">{{ isNew ? '创建后固定，用于模块链接。' : `/learn/modules/${localModuleForm.slug}` }}</span></div></div>
        </section>
        <div class="settings-side">
        <section class="settings-card goals-card" aria-labelledby="goals-heading">
          <header class="settings-heading"><h2 id="goals-heading">学习目标</h2><p>对应模块详情页的“学完后，你将能够”，每行填写一项。</p></header>
          <label class="sr-only" for="module-outcomes">学完后，你将能够</label>
          <textarea id="module-outcomes" v-model="localModuleForm.learningOutcomes" class="form-input" rows="5" placeholder="每行一个目标，例如：&#10;识别文件权限与用户身份&#10;排查常见访问权限问题" @input="markDirty"></textarea>
          <span class="field-hint">每行一项，最多 12 项。留空不展示。</span>
        </section>
        <section class="settings-card prerequisites-card" aria-labelledby="prerequisites-heading">
          <header class="settings-heading"><h2 id="prerequisites-heading">前置学习模块</h2><p>按建议顺序展示，可直接进入对应模块。</p></header>
          <ol v-if="prerequisiteIds.length" class="prerequisite-list">
            <li v-for="(id, index) in prerequisiteIds" :key="id" class="prerequisite-item">
              <span class="prerequisite-index">{{ index + 1 }}</span>
              <div class="prerequisite-name"><strong>{{ prerequisiteInfo(id)?.title || '模块暂不可用' }}</strong><span v-if="prerequisiteInfo(id)?.status === 'unpublished'">草稿 · 发布后才会展示</span></div>
              <div class="prerequisite-actions"><button type="button" class="icon-action" :disabled="index === 0" aria-label="前置模块上移" @click="movePrerequisite(index, -1)"><ArrowUp :size="13" /></button><button type="button" class="icon-action" :disabled="index === prerequisiteIds.length - 1" aria-label="前置模块下移" @click="movePrerequisite(index, 1)"><ArrowDown :size="13" /></button><button type="button" class="icon-action" :aria-label="`移除前置模块 ${prerequisiteInfo(id)?.title || ''}`" @click="prerequisiteIds.splice(index, 1); markDirty()"><X :size="13" /></button></div>
            </li>
          </ol>
          <button v-if="!prerequisitePickerOpen && prerequisiteIds.length < 12" class="add-prerequisite" type="button" @click="prerequisitePickerOpen = true"><Plus :size="15" aria-hidden="true" />添加模块</button>
          <div v-if="prerequisitePickerOpen" class="prerequisite-picker">
            <div class="picker-search"><input v-model="prerequisiteSearch" class="form-input" aria-label="搜索前置模块" placeholder="搜索名称或访问标识" /><button type="button" class="icon-action" aria-label="关闭模块选择" @click="prerequisitePickerOpen = false"><X :size="15" /></button></div>
            <div v-if="moduleOptionsError" class="field-hint">加载失败。<button type="button" class="text-action" @click="refreshModuleOptions()">重试</button></div>
            <template v-else><button v-for="candidate in prerequisiteCandidates" :key="candidate.id" type="button" class="prerequisite-option" @click="addPrerequisite(candidate.id)"><span>{{ candidate.title }}</span><span v-if="candidate.status === 'unpublished'" class="field-hint">草稿</span><Plus :size="14" aria-hidden="true" /></button><p v-if="!prerequisiteCandidates.length" class="field-hint">没有可添加的模块。</p></template>
          </div>
          <span class="field-hint prerequisite-note">选填，按推荐顺序展示，不限制进入本模块。</span>
        </section>
        </div>
      </div>
      </fieldset>
    </div>

  </div>
</template>

<style scoped>
.settings-grid { display: grid; grid-template-columns: minmax(0, 1fr) 350px; gap: 22px; align-items: start; }
.settings-card { min-width: 0; border: 1px solid var(--line); background: var(--paper); border-radius: 12px; padding: 26px; }
.settings-side { display: grid; gap: 22px; min-width: 0; }
.settings-heading { margin-bottom: 24px; }
.settings-heading h2 { font-size: 16px; font-weight: 650; color: var(--ink); margin: 0 0 6px; }
.settings-heading p { color: var(--muted); font-size: 12px; line-height: 1.7; margin: 0; }
.settings-field-row { display: grid; grid-template-columns: 1fr 1fr; gap: 18px; }
.field-label-row { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; min-height: 25px; }
.required-label { color: var(--muted); font-size: 10px; font-weight: 400; margin-left: 5px; }
.text-action { display: inline-flex; align-items: center; gap: 3px; font-size: 12px; color: var(--green); background: transparent; border: 0; cursor: pointer; padding: 0; }
.category-create { display: flex; align-items: center; gap: 8px; margin-top: 8px; }
.category-create .form-input { min-width: 0; }
.description-editor { font-family: monospace; line-height: 1.8; }
.description-preview { min-height: 180px; border: 1px solid var(--line); border-radius: 6px; padding: 12px; }
.slug-setting { display: flex; align-items: flex-start; gap: 12px; padding-top: 22px; border-top: 1px solid var(--line); color: var(--muted); }
.slug-setting > svg { margin-top: 4px; }
.slug-setting-body { min-width: 0; flex: 1; }
.slug-setting label { display: block; margin-bottom: 8px; font-size: 12px; }
.slug-setting .field-hint { overflow-wrap: anywhere; }
.prerequisite-list { list-style: none; margin: 0 0 14px; padding: 0; display: grid; gap: 10px; }
.prerequisite-item { display: flex; align-items: center; gap: 8px; padding-bottom: 12px; border-bottom: 1px solid var(--line); }
.prerequisite-index { font-family: monospace; color: var(--muted); font-size: 11px; }
.prerequisite-name { flex: 1; min-width: 0; }
.prerequisite-name strong { display: block; overflow-wrap: anywhere; font-size: 12px; font-weight: 550; color: var(--ink); }
.prerequisite-name > span { display: block; color: var(--muted); font-size: 10px; margin-top: 4px; }
.prerequisite-actions { display: flex; align-items: center; }
.icon-action { display: inline-flex; align-items: center; justify-content: center; padding: 5px; background: transparent; color: var(--muted); border: 0; cursor: pointer; border-radius: 4px; }
.icon-action:hover:not(:disabled) { background: var(--soft); color: var(--ink); }
.icon-action:disabled { opacity: .3; cursor: default; }
.add-prerequisite { display: flex; align-items: center; justify-content: center; gap: 7px; width: 100%; border: 1px dashed var(--line); border-radius: 6px; color: var(--muted); background: transparent; padding: 10px; font-size: 12px; cursor: pointer; }
.prerequisite-picker { border: 1px solid var(--line); padding: 8px; border-radius: 7px; }
.picker-search { display: flex; align-items: center; gap: 4px; }
.prerequisite-option { display: flex; align-items: center; gap: 8px; text-align: left; width: 100%; background: transparent; border: 0; padding: 10px 5px; color: var(--ink); font-size: 12px; cursor: pointer; }
.prerequisite-option > span:first-child { flex: 1; }
.prerequisite-option:hover { background: var(--soft); }
.prerequisite-note { margin-top: 12px; line-height: 1.7; }
.chapter-fields { grid-template-columns: minmax(0, 1fr) 160px 130px; }
.duration-input { position: relative; }
.duration-input input { padding-right: 42px; }
.duration-input > span { position: absolute; top: 10px; right: 12px; color: var(--muted); font-size: 12px; pointer-events: none; }
@media (max-width: 1050px) { .settings-grid { grid-template-columns: minmax(0, 1fr) 310px; } .settings-card { padding: 20px; } .chapter-fields { grid-template-columns: 1fr 1fr; } }
@media (max-width: 900px) { .settings-grid { grid-template-columns: 1fr; } .settings-field-row { grid-template-columns: 1fr; gap: 0; } }
.btn, .save-status-text, .back-btn { display: inline-flex; align-items: center; justify-content: center; gap: 7px; }

.more-actions { position: relative; }
.more-menu { position: absolute; top: calc(100% + 8px); right: 0; z-index: 20; min-width: 130px; padding: 5px; border: 1px solid var(--line); border-radius: 8px; background: var(--paper); box-shadow: 0 8px 24px #0002; }
.more-menu button { display: block; width: 100%; text-align: left; padding: 9px 12px; background: transparent; border: 0; border-radius: 5px; color: var(--ink); cursor: pointer; }
.more-menu button:hover { background: var(--soft); }
.more-menu .danger { color: var(--error-ink, #bf4545); }

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
  flex-wrap: wrap;
  gap: 18px;
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

.badge.unpublished {
  background: var(--soft);
  color: var(--muted);
}

.action-buttons {
  display: flex;
  flex-wrap: wrap;
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

.chapter-delete-btn {
  background: transparent;
  border: none;
  color: var(--color-error);
  font-size: 13px;
  line-height: 1;
  padding: 2px 4px;
  cursor: pointer;
}

.chapter-delete-btn:disabled {
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
.chapter-main-panel {
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

@media (max-width: 900px) {
  .editor-double-layout {
    grid-template-columns: 1fr;
  }
}
</style>
