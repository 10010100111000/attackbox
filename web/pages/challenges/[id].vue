<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { ArrowLeft, CheckCircle2, LockKeyhole, Flag, Terminal, Monitor, ChevronUp } from 'lucide-vue-next'
import MarkdownView from '~/components/ui/MarkdownView.vue'
import PracticeDock from '~/components/learning/PracticeDock.vue'
import { useAuth } from '~/composables/useAuth'
import type { Challenge } from '~/shared/challenges'

definePageMeta({ layout: 'learning', key: r => r.fullPath })

const id = String(useRoute().params.id)
const { currentUser } = useAuth()
const learner = computed(() => currentUser.value?.id || '')

const { data, error, refresh } = await useFetch<{ data: Challenge }>(`/api/challenges/${id}`, {
  key: `challenges:detail:${id}`,
  watch: [learner],
})

const ready = ref(false)
const c = computed(() => data.value?.data)
const answers = ref<Record<string, string>>({})
const feedback = ref<Record<string, string>>({})
const busy = ref(false)

// PracticeDock state for CLI / GUI access inside challenge arena
const practiceMode = ref<'cli' | 'gui'>('cli')
const practiceOpened = ref(false)
const practiceVisible = ref(false)
const practiceMinimized = ref(false)
const practiceExpanded = ref(false)
const dockHeight = ref(310)
const viewportHeight = ref(900)
const maxDockHeight = computed(() => Math.max(180, viewportHeight.value - 220))

function updateViewport() {
  viewportHeight.value = window.innerHeight
  dockHeight.value = Math.min(dockHeight.value, maxDockHeight.value)
}

function openPractice(mode: 'cli' | 'gui') {
  practiceOpened.value = true
  practiceMode.value = mode
  practiceVisible.value = true
  practiceMinimized.value = false
}

function closePractice() {
  practiceVisible.value = false
  practiceExpanded.value = false
}

function minimizePractice() {
  practiceMinimized.value = true
  practiceExpanded.value = false
}

onMounted(() => {
  ready.value = true
  updateViewport()
  window.addEventListener('resize', updateViewport)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateViewport)
})

watch(learner, () => {
  answers.value = {}
  feedback.value = {}
})

useHead(() => ({
  title: c.value ? `${c.value.title} — AttackBox` : '挑战 — AttackBox',
  meta: [{ name: 'description', content: c.value?.summary || '安全学习挑战' }],
}))

async function submit(stageId: string) {
  if (busy.value) return
  busy.value = true
  try {
    const r = await $fetch<{ data: { correct: boolean } }>(`/api/challenges/${id}/attempt`, {
      method: 'POST',
      body: { stageId, flag: answers.value[stageId] },
    })
    feedback.value[stageId] = r.data.correct
      ? '阶段已通过'
      : c.value?.stages.find(s => s.id === stageId)?.hint
        ? 'Flag 不正确，可以查看提示后再试'
        : 'Flag 不正确，可以继续尝试'
    if (r.data.correct) {
      delete answers.value[stageId]
      await refresh()
      clearNuxtData('challenges:list')
    }
  } catch (e: any) {
    feedback.value[stageId] = e?.data?.message || '提交失败，请重试'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="challenge-shell">
    <div class="challenge-top-bar">
      <NuxtLink to="/challenges" class="back">
        <ArrowLeft :size="14" aria-hidden="true" />
        <span>挑战列表</span>
      </NuxtLink>

      <div class="toolbar-tools" role="group" aria-label="练习环境">
        <button
          type="button"
          :disabled="!ready"
          :aria-pressed="practiceVisible && !practiceMinimized && practiceMode === 'cli'"
          @click="openPractice('cli')"
        >
          <Terminal :size="15" aria-hidden="true" />
          <span>CLI</span>
          <span class="tool-word">终端</span>
        </button>
        <button
          type="button"
          :disabled="!ready"
          :aria-pressed="practiceVisible && !practiceMinimized && practiceMode === 'gui'"
          @click="openPractice('gui')"
        >
          <Monitor :size="15" aria-hidden="true" />
          <span>GUI</span>
          <span class="tool-word">桌面</span>
        </button>
      </div>
    </div>

    <div class="challenge-scroll" :inert="practiceVisible && !practiceMinimized && practiceExpanded">
      <main class="challenge-detail">
        <p v-if="error" role="alert">挑战不存在或尚未发布。</p>

        <template v-else-if="c">
          <header class="challenge-header">
            <div class="eyebrow">{{ c.category || '综合挑战' }}</div>
            <h1>{{ c.title }}</h1>
            <p>{{ c.summary }}</p>
            <div class="progress">
              <Flag :size="16" aria-hidden="true" />
              <span>
                {{
                  currentUser
                    ? `${c.completedStages} / ${c.totalStages} 阶段已完成`
                    : `${c.totalStages} 个阶段`
                }}
              </span>
              <span v-if="currentUser && c.isCompleted" class="success">
                <CheckCircle2 :size="16" aria-hidden="true" />
                <span>挑战已完成</span>
              </span>
            </div>
          </header>

          <MarkdownView :content="c.bodyMarkdown" />
          <TargetEnvironment v-if="c.target" :key="String($route.params.id)" :target="c.target" :context="{kind: 'challenge', challengeId: String($route.params.id)}" />

          <section aria-label="阶段任务" class="stage-list">
            <article v-for="(s, index) in c.stages" :key="s.id" class="stage">
              <header class="stage-heading">
                <span class="number">{{ String(index + 1).padStart(2, '0') }}</span>
                <h2>{{ s.title }}</h2>
                <CheckCircle2 v-if="s.passedAt" class="success" :size="20" aria-hidden="true" />
                <LockKeyhole v-else-if="s.locked" :size="18" aria-hidden="true" />
              </header>

              <MarkdownView :content="s.promptMarkdown" />

              <details v-if="s.hint" class="hint">
                <summary>查看提示</summary>
                <p>{{ s.hint }}</p>
              </details>

              <p v-if="s.passedAt" class="success" role="status">阶段已通过</p>
              <NuxtLink
                v-else-if="!currentUser"
                :to="`/auth?returnTo=${encodeURIComponent('/challenges/' + id)}`"
                class="login"
              >
                登录后提交 Flag
              </NuxtLink>
              <p v-else-if="s.locked" class="locked">完成前面的阶段后即可提交。</p>

              <form v-else @submit.prevent="submit(s.id)">
                <label class="sr-only" :for="`flag-${s.id}`">阶段 {{ index + 1 }} Flag</label>
                <input
                  :id="`flag-${s.id}`"
                  v-model="answers[s.id]"
                  placeholder="flag{...}"
                  maxlength="2000"
                  autocomplete="off"
                  spellcheck="false"
                  :disabled="busy || !ready"
                />
                <button :disabled="busy || !ready || !answers[s.id]?.trim()">
                  {{ busy ? '校验中…' : '提交 Flag' }}
                </button>
              </form>

              <p v-if="feedback[s.id] && !s.passedAt" role="status">
                {{ feedback[s.id] }}
              </p>
            </article>
          </section>
        </template>
      </main>
    </div>

    <PracticeDock
      v-if="practiceOpened"
      v-show="practiceVisible && !practiceMinimized"
      :key="learner"
      :active="practiceVisible && !practiceMinimized"
      v-model:height="dockHeight"
      v-model:expanded="practiceExpanded"
      :mode="practiceMode"
      :max-height="maxDockHeight"
      @minimize="minimizePractice"
      @close="closePractice"
    />

    <button
      v-if="practiceVisible && practiceMinimized"
      type="button"
      class="dock-restore"
      aria-label="恢复练习面板"
      @click="practiceMinimized = false"
    >
      <Terminal v-if="practiceMode === 'cli'" :size="14" aria-hidden="true" />
      <Monitor v-else :size="14" aria-hidden="true" />
      <span>{{ practiceMode === 'cli' ? '终端' : '桌面' }}</span>
      <ChevronUp :size="15" aria-hidden="true" />
    </button>
  </div>
</template>

<style scoped>
.challenge-shell {
  display: flex;
  flex-direction: column;
  height: calc(100dvh - 68px);
  min-height: 360px;
  position: relative;
  background: var(--reader-bg);
  color: var(--ink);
}

.challenge-top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  height: 45px;
  flex-shrink: 0;
  padding: 0 28px;
  border-bottom: 1px solid var(--line);
  background: var(--reader-bg);
}

.back {
  display: inline-flex;
  gap: 7px;
  align-items: center;
  color: var(--muted);
  text-decoration: none;
  font-size: 12px;
}

.back:hover {
  color: var(--ink);
}

.toolbar-tools {
  display: flex;
  align-items: center;
  gap: 5px;
}

.toolbar-tools button {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 9px;
  border: 1px solid transparent;
  border-radius: 5px;
  background: transparent;
  font-size: 11px;
  color: var(--muted);
  cursor: pointer;
}

.toolbar-tools button:hover:not(:disabled) {
  background: var(--soft);
  color: var(--ink);
}

.toolbar-tools button[aria-pressed='true'] {
  border-color: var(--line);
  background: var(--paper);
  color: var(--ink);
}

.challenge-scroll {
  flex: 1;
  overflow: auto;
  min-height: 0;
  overscroll-behavior: contain;
}

.challenge-detail {
  width: 100%;
  max-width: 1000px;
  margin: 0 auto;
  padding: 36px 28px 80px;
  color: var(--ink);
  box-sizing: border-box;
}

.challenge-header {
  padding: 8px 0 28px;
}

h1 {
  font-size: clamp(28px, 3.2vw, 36px);
  margin: 12px 0 18px;
}

.challenge-header > p {
  color: var(--muted);
  font-size: 15px;
  line-height: 1.8;
}

.eyebrow {
  font-size: 12px;
  color: var(--green);
}

.progress {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  color: var(--muted);
}

.success {
  color: var(--green);
  display: inline-flex;
  align-items: center;
  gap: 7px;
}

.progress .success {
  margin-left: auto;
}

.stage-list {
  margin-top: 38px;
}

.stage {
  border-top: 1px solid var(--line);
  padding: 28px 0;
}

.stage-heading {
  display: flex;
  align-items: center;
  gap: 15px;
  padding: 0;
  margin-bottom: 20px;
}

.stage-heading h2 {
  flex: 1;
  font-size: 20px;
  margin: 0;
}

.number {
  color: var(--muted);
  font: 20px ui-monospace, monospace;
}

.hint {
  font-size: 13px;
  color: var(--muted);
  margin: 20px 0;
}

.hint summary {
  cursor: pointer;
}

.hint p {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  line-height: 1.8;
}

form {
  display: flex;
  gap: 10px;
  margin: 22px 0;
}

input {
  flex: 1;
  min-width: 0;
  padding: 11px 13px;
  background: var(--paper);
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 6px;
  font: 13px ui-monospace, monospace;
}

form button {
  background: var(--ink);
  color: var(--paper);
  border: 0;
  border-radius: 6px;
  padding: 10px 18px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 550;
}

form button:disabled {
  opacity: 0.5;
  cursor: default;
}

.locked,
article > p,
.login {
  font-size: 13px;
}

.locked {
  color: var(--muted);
}

.login {
  color: var(--green);
}

.dock-restore {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 35px;
  flex-shrink: 0;
  background: var(--paper);
  color: var(--muted);
  border: 0;
  border-top: 1px solid var(--line);
  cursor: pointer;
  font-size: 11px;
}

@media (max-width: 600px) {
  .challenge-top-bar {
    padding: 0 16px;
  }
  .tool-word {
    display: none;
  }
  .challenge-detail {
    padding: 26px 18px 56px;
  }
}
</style>
