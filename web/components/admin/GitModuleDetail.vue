<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { ArrowLeft, GitBranch, ExternalLink, RefreshCw } from 'lucide-vue-next'
import type { AdminModuleDetail } from '~/shared/content'
import type { SyncModuleResult } from '~/shared/content-sync'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import TargetTemplateSelect from '~/components/admin/TargetTemplateSelect.vue'

const props = defineProps<{ detail: AdminModuleDetail }>()
const emit = defineEmits<{ refresh: [] }>()

const tab = ref<'content' | 'runtime'>('content')
const selected = ref(0)
const busy = ref(false)
const notice = ref('')

const form = ref({
  defaultTargetTemplateId: props.detail.module.defaultTargetTemplateId,
  prerequisiteModuleIds: props.detail.prerequisites.map(m => m.id),
  chapters: props.detail.chapters.map(c => ({
    chapterId: c.chapterId,
    targetMode: c.targetMode,
    targetTemplateId: c.targetTemplateId,
  })),
})
const original = ref(JSON.stringify(form.value))
const version = ref(props.detail.module.lockVersion)
const dirty = computed(() => JSON.stringify(form.value) !== original.value)

watch(
  () => props.detail,
  value => {
    if (dirty.value) return
    version.value = value.module.lockVersion
    form.value = {
      defaultTargetTemplateId: value.module.defaultTargetTemplateId,
      prerequisiteModuleIds: value.prerequisites.map(m => m.id),
      chapters: value.chapters.map(c => ({
        chapterId: c.chapterId,
        targetMode: c.targetMode,
        targetTemplateId: c.targetTemplateId,
      })),
    }
    original.value = JSON.stringify(form.value)
  },
)

const { data: targets } = useLazyFetch<{ data: { id: string; name: string; enabled: boolean }[] }>(
  '/api/admin/target-templates/options',
  { key: 'admin:content:targets' },
)
const { data: moduleOptions } = useLazyFetch<{ data: { id: string; title: string }[] }>(
  '/api/admin/modules',
  { key: 'admin:content:modules' },
)
const chapter = computed(() => props.detail.chapters[selected.value])

onBeforeRouteLeave(() => !dirty.value || confirm('放弃未保存的运行配置？'))

async function operate(task: () => Promise<void>) {
  if (busy.value) return
  busy.value = true
  notice.value = ''
  try {
    await task()
  } catch (e: any) {
    notice.value = e?.data?.message || '操作失败，请重试'
  } finally {
    busy.value = false
  }
}

function save() {
  void operate(async () => {
    await $fetch(`/api/admin/modules/${props.detail.module.id}/runtime`, {
      method: 'PUT',
      body: { ...form.value, lockVersion: version.value },
    })
    original.value = JSON.stringify(form.value)
    emit('refresh')
    notice.value = '运行配置已保存'
  })
}

function publish() {
  void operate(async () => {
    await $fetch(
      `/api/admin/modules/${props.detail.module.id}/${props.detail.module.status === 'published' ? 'unpublish' : 'publish'}`,
      { method: 'POST', body: { lockVersion: version.value } },
    )
    emit('refresh')
    notice.value = '发布状态已更新'
  })
}

function syncFromGit() {
  void operate(async () => {
    const response = await $fetch<{ data: SyncModuleResult }>('/api/admin/content/sync', {
      method: 'POST',
      body: { moduleId: props.detail.module.id },
    })
    const res = response.data.synced[0]
    clearNuxtData(k => k.startsWith('admin:') || k.startsWith('learning:'))
    emit('refresh')
    if (res?.action === 'unchanged') {
      notice.value = `已是最新（Commit ${response.data.commit.slice(0, 12)}），无内容变化`
    } else if (res) {
      notice.value = `已从 Git 差异同步更新（下载新图片 ${res.downloadedImages} 张，复用未变图片 ${res.reusedImages} 张）`
    }
  })
}
</script>

<template>
  <div class="git-module-detail">
    <NuxtLink to="/admin/modules" class="back"><ArrowLeft :size="14" />学习模块</NuxtLink>
    <header>
      <div>
        <h1>
          {{ detail.module.title }}
          <span>{{ detail.module.status === 'published' ? '已发布' : '未发布' }}</span>
        </h1>
        <p>
          <GitBranch :size="13" />{{ detail.source?.path }}
          <code>{{ detail.source?.commit?.slice(0, 12) || '本地导入' }}</code>
        </p>
      </div>
      <div class="actions">
        <a
          v-if="detail.source?.url"
          :href="detail.source.url"
          target="_blank"
          rel="noopener noreferrer"
          class="button"
        >
          查看源文件
        </a>
        <button class="button" type="button" :disabled="busy" @click="syncFromGit">
          <RefreshCw :size="13" />同步最新内容
        </button>
        <NuxtLink
          v-if="detail.module.status === 'published'"
          :to="`/learn/modules/${detail.module.slug}`"
          target="_blank"
          class="button"
        >
          前台预览<ExternalLink :size="13" />
        </NuxtLink>
        <button class="button primary" :disabled="busy || dirty" @click="publish">
          {{ detail.module.status === 'published' ? '下架' : '发布' }}
        </button>
      </div>
    </header>
    <p v-if="notice" class="notice" role="status">{{ notice }}</p>
    <nav class="tabs">
      <button :class="{ active: tab === 'content' }" @click="tab = 'content'">内容预览</button>
      <button :class="{ active: tab === 'runtime' }" @click="tab = 'runtime'">运行配置</button>
    </nav>
    <div v-if="tab === 'content'" class="content-grid">
      <aside class="panel">
        <p class="hint">内容由 Git 维护，仓库修改后点击右上角“同步最新内容”直接拉取差异更新。</p>
        <button
          v-for="(ch, index) in detail.chapters"
          :key="ch.chapterId"
          class="chapter-choice"
          :class="{ selected: selected === index }"
          @click="selected = index"
        >
          {{ index + 1 }}. {{ ch.title }}
        </button>
        <details class="module-info">
          <summary>模块介绍与目标</summary>
          <img
            v-if="detail.module.coverAssetId"
            :src="`/api/modules/${detail.module.slug}/assets/${detail.module.coverAssetId}`"
            alt="模块封面"
            class="module-cover"
          />
          <p class="hint">封面在 Markdown 配置头填写 cover: ./images/cover.webp，同步后生效。</p>
          <p>{{ detail.module.summary }}</p>
          <MarkdownView :content="detail.module.descriptionMarkdown" :image-urls="detail.imageUrls" />
          <ul>
            <li v-for="goal in detail.module.learningOutcomes" :key="goal">{{ goal }}</li>
          </ul>
        </details>
      </aside>
      <section v-if="chapter" class="panel">
        <div class="chapter-heading">
          <h2>{{ chapter.title }}</h2>
          <span>
            {{ chapter.access === 'public' ? '公开' : '登录可读' }}
            <template v-if="chapter.durationMinutes"> · {{ chapter.durationMinutes }} 分钟</template>
          </span>
        </div>
        <MarkdownView :content="chapter.bodyMarkdown" :image-urls="detail.imageUrls" />
        <div v-if="chapter.questions.length" class="questions">
          <h2>章节练习</h2>
          <div v-for="q in chapter.questions" :key="q.id">
            <MarkdownView :content="q.promptMarkdown" :image-urls="detail.imageUrls" />
            <p class="hint">{{ q.normalization }} · {{ q.hasAnswer ? '已配置答案' : '未配置答案' }}</p>
          </div>
        </div>
      </section>
    </div>
    <form v-else class="panel runtime-settings" @submit.prevent="save">
      <fieldset :disabled="busy">
        <h2>实验环境</h2>
        <label>
          模块默认靶机
          <TargetTemplateSelect v-model="form.defaultTargetTemplateId" :options="targets?.data || []" />
        </label>
        <div v-for="(ch, index) in form.chapters" :key="ch.chapterId" class="chapter-config">
          <label>
            {{ detail.chapters[index]?.title }}
            <select v-model="ch.targetMode" @change="ch.targetTemplateId = null">
              <option value="inherit">继承模块默认</option>
              <option value="none">不使用靶机</option>
              <option value="custom">另选靶机</option>
            </select>
          </label>
          <TargetTemplateSelect
            v-if="ch.targetMode === 'custom'"
            v-model="ch.targetTemplateId"
            :options="targets?.data || []"
          />
        </div>
        <h2>前置学习推荐</h2>
        <p class="hint">不限制访问，仅作为模块详情页的学习建议。</p>
        <select v-model="form.prerequisiteModuleIds" multiple aria-label="前置学习模块">
          <option
            v-for="m in moduleOptions?.data.filter(m => m.id !== detail.module.id)"
            :key="m.id"
            :value="m.id"
          >
            {{ m.title }}
          </option>
        </select>
        <div class="actions">
          <button class="button primary" type="submit" :disabled="!dirty || busy">保存运行配置</button>
          <span v-if="dirty" class="hint">有未保存修改</span>
        </div>
      </fieldset>
    </form>
  </div>
</template>

<style scoped>
.module-cover {
  display: block;
  width: 100%;
  aspect-ratio: 16/9;
  object-fit: cover;
  border-radius: 8px;
  margin: 12px 0;
}
.back {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  text-decoration: none;
  color: var(--muted);
  font-size: 12px;
  margin-bottom: 20px;
}
header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 24px;
}
h1 {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 26px;
  font-weight: 650;
  margin: 0;
}
h1 span {
  font-size: 11px;
  font-weight: 400;
  padding: 4px 8px;
  background: var(--soft);
  border-radius: 4px;
}
header p {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: 12px;
}
code {
  font-size: 11px;
}
h2 {
  font-size: 17px;
  margin: 0 0 18px;
}
.actions {
  display: flex;
  gap: 10px;
  align-items: center;
}
.button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 9px 13px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--paper);
  color: var(--ink);
  text-decoration: none;
  font-size: 12px;
  cursor: pointer;
}
.primary {
  background: var(--green);
  color: var(--on-accent);
  border-color: var(--green);
}
button:disabled {
  opacity: 0.5;
  cursor: default;
}
.tabs {
  display: flex;
  gap: 24px;
  border-bottom: 1px solid var(--line);
  margin-bottom: 24px;
}
.tabs button {
  padding: 12px 0;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--muted);
  font-size: 13px;
  cursor: pointer;
}
.tabs .active {
  border-color: var(--green);
  color: var(--ink);
}
.content-grid {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 24px;
  align-items: start;
}
.panel {
  padding: 24px;
  border: 1px solid var(--line);
  border-radius: 10px;
  background: var(--paper);
}
.hint {
  font-size: 12px;
  color: var(--muted);
  line-height: 1.8;
}
.chapter-choice {
  display: block;
  width: 100%;
  padding: 10px;
  border: 0;
  border-radius: 5px;
  text-align: left;
  background: transparent;
  color: var(--ink);
  font-size: 12px;
  cursor: pointer;
}
.chapter-choice.selected {
  background: var(--soft);
}
.module-info {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid var(--line);
  font-size: 12px;
}
.module-info summary {
  cursor: pointer;
}
.module-info ul {
  padding-left: 20px;
}
.chapter-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}
.chapter-heading span {
  font-size: 11px;
  color: var(--muted);
}
.questions {
  margin-top: 28px;
  padding-top: 24px;
  border-top: 1px solid var(--line);
}
.questions > div + div {
  border-top: 1px solid var(--line);
  padding-top: 14px;
}
.runtime-settings fieldset {
  padding: 0;
  border: 0;
  margin: 0;
}
.runtime-settings label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
  max-width: 420px;
  margin-bottom: 16px;
}
.runtime-settings select {
  padding: 9px;
  border: 1px solid var(--line);
  border-radius: 6px;
  background: var(--bg);
  color: var(--ink);
  font-size: 12px;
}
.runtime-settings select[multiple] {
  width: 420px;
  height: 150px;
  margin-bottom: 20px;
}
.chapter-config {
  display: flex;
  align-items: center;
  gap: 20px;
  margin-bottom: 16px;
}
.chapter-config label {
  width: 300px;
  margin-bottom: 0;
}
.runtime-settings h2:not(:first-child) {
  margin-top: 30px;
}
.notice {
  padding: 12px 16px;
  background: var(--soft);
  border-radius: 6px;
  margin-bottom: 16px;
  font-size: 12px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
:focus-visible {
  outline: 2px solid var(--green);
  outline-offset: 3px;
}
</style>
