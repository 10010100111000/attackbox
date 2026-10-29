<script setup lang="ts">
import {
  DialogRoot,
  DialogPortal,
  DialogOverlay,
  DialogContent,
  DialogTitle,
  DialogDescription
} from 'reka-ui'
import { MessageSquarePlus, X, Check } from 'lucide-vue-next'
import { feedbackCategories, type FeedbackCategory } from '~/shared/feedback'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean]; 'restore-focus': [] }>()
const route = useRoute(),
  { currentUser } = useAuth()
const category = ref<FeedbackCategory>('bug'),
  body = ref(''),
  sourcePath = ref('')
const busy = ref(false),
  submitted = ref(false),
  error = ref('')
const textarea = ref<HTMLTextAreaElement | null>(null),
  doneButton = ref<HTMLButtonElement | null>(null)
const fieldId = useId()

watch(
  () => props.open,
  open => {
    if (!open) return
    sourcePath.value = route.path
    error.value = ''
    if (submitted.value) {
      body.value = ''
      category.value = 'bug'
      submitted.value = false
    }
  }
)
watch(
  () => currentUser.value?.id,
  () => {
    body.value = ''
    submitted.value = false
    error.value = ''
    emit('update:open', false)
  }
)

function close(open: boolean) {
  if (!busy.value) emit('update:open', open)
}
function restoreFocus(event: Event) {
  event.preventDefault()
  emit('restore-focus')
}
function focusInput(event: Event) {
  event.preventDefault()
  void nextTick(() => textarea.value?.focus())
}
async function submit() {
  if (busy.value || !body.value.trim() || !currentUser.value) return
  const author = currentUser.value.id
  busy.value = true
  error.value = ''
  try {
    await $fetch('/api/me/feedback', {
      method: 'POST',
      body: { category: category.value, body: body.value, sourcePath: sourcePath.value },
      retry: false,
      timeout: 15000
    })
    if (currentUser.value?.id === author) {
      submitted.value = true
      await nextTick()
      doneButton.value?.focus()
    }
  } catch (e: any) {
    if (currentUser.value?.id === author)
      error.value = e?.data?.message || '提交未确认，请稍后重试。'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <DialogRoot :open="open" @update:open="close">
    <DialogPortal>
      <DialogOverlay class="feedback-overlay" />
      <DialogContent
        class="feedback-composer"
        @open-auto-focus="focusInput"
        @close-auto-focus="restoreFocus"
        @escape-key-down="busy && $event.preventDefault()"
        @interact-outside="busy && $event.preventDefault()"
      >
        <button
          class="feedback-close"
          type="button"
          aria-label="关闭反馈"
          :disabled="busy"
          @click="close(false)"
        >
          <X :size="18" />
        </button>
        <template v-if="submitted">
          <div class="feedback-symbol success" aria-hidden="true"><Check :size="24" /></div>
          <DialogTitle class="feedback-title">收到，谢谢你。</DialogTitle>
          <DialogDescription class="feedback-description"
            >我们会认真查看这条反馈。</DialogDescription
          >
          <button
            ref="doneButton"
            type="button"
            class="feedback-primary done"
            @click="close(false)"
          >
            继续探索
          </button>
        </template>
        <template v-else>
          <div class="feedback-symbol" aria-hidden="true"><MessageSquarePlus :size="22" /></div>
          <DialogTitle class="feedback-title">问题反馈</DialogTitle>
          <DialogDescription class="feedback-description"
            >哪里不顺手，或者有什么新想法？</DialogDescription
          >
          <form @submit.prevent="submit">
            <fieldset :disabled="busy">
              <legend class="sr-only">反馈内容</legend>
              <div class="feedback-types" role="radiogroup" aria-label="反馈类型">
                <label
                  v-for="option in feedbackCategories"
                  :key="option.value"
                  :class="{ selected: category === option.value }"
                >
                  <input
                    v-model="category"
                    type="radio"
                    :name="fieldId + '-type'"
                    :value="option.value"
                  />
                  <span>{{ option.label }}</span>
                </label>
              </div>
              <label :for="fieldId + '-body'" class="feedback-label">具体说说</label>
              <textarea
                :id="fieldId + '-body'"
                ref="textarea"
                v-model="body"
                rows="5"
                maxlength="4000"
                required
                placeholder="比如：在哪个页面遇到了什么问题，原本希望它怎样工作……"
                :aria-describedby="fieldId + '-context'"
              />
              <div :id="fieldId + '-context'" class="feedback-context">
                <span>附上当前页面</span><code :title="sourcePath">{{ sourcePath }}</code>
              </div>
              <p v-if="error" role="alert" class="feedback-error">{{ error }}</p>
              <footer>
                <span>{{ body.length }} / 4000</span
                ><button type="submit" class="feedback-primary" :disabled="busy || !body.trim()">
                  <UiBusyLabel :busy="busy">提交反馈</UiBusyLabel>
                </button>
              </footer>
            </fieldset>
          </form>
        </template>
      </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>

<style scoped>
.feedback-overlay {
  position: fixed;
  inset: 0;
  z-index: 300;
  background: #0007;
  backdrop-filter: blur(5px);
}
.feedback-composer {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: 301;
  width: min(500px, calc(100vw - 32px));
  max-height: calc(100svh - 32px);
  overflow: auto;
  padding: 30px;
  border: 1px solid var(--line);
  border-radius: 18px;
  background: var(--paper);
  color: var(--ink);
  box-shadow: 0 24px 80px #0004;
}
.feedback-close {
  position: absolute;
  top: 16px;
  right: 16px;
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}
.feedback-close:hover {
  background: var(--soft);
  color: var(--ink);
}
.feedback-symbol {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  margin-bottom: 18px;
  border-radius: 13px;
  background: var(--brand-soft);
  color: var(--brand);
}
.feedback-symbol.success {
  background: var(--success-bg);
  color: var(--success-ink);
}
.feedback-title {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
  letter-spacing: -0.5px;
}
.feedback-description {
  margin: 10px 0 24px;
  font-size: 13px;
  color: var(--muted);
  line-height: 1.8;
}
fieldset {
  padding: 0;
  margin: 0;
  border: 0;
  min-width: 0;
}
.feedback-types {
  display: flex;
  gap: 8px;
  margin-bottom: 24px;
}
.feedback-types label {
  position: relative;
  cursor: pointer;
}
.feedback-types input {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
}
.feedback-types span {
  display: block;
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 12px;
}
.feedback-types .selected span {
  border-color: var(--brand);
  background: var(--brand-soft);
  color: var(--brand);
}
.feedback-types input:focus-visible + span {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}
.feedback-label {
  display: block;
  font-size: 12px;
  font-weight: 550;
  margin-bottom: 10px;
}
textarea {
  display: block;
  box-sizing: border-box;
  width: 100%;
  min-height: 140px;
  max-height: 35vh;
  resize: vertical;
  border: 1px solid var(--line);
  border-radius: 10px;
  padding: 12px;
  background: var(--bg);
  color: var(--ink);
  font: inherit;
  font-size: 13px;
  line-height: 1.8;
}
textarea::placeholder {
  color: var(--muted);
}
textarea:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 2px;
}
.feedback-context {
  display: flex;
  gap: 12px;
  align-items: baseline;
  font-size: 11px;
  color: var(--muted);
  padding-top: 12px;
}
.feedback-context > span {
  flex-shrink: 0;
}
.feedback-context code {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.feedback-error {
  color: var(--error-ink);
  font-size: 12px;
  line-height: 1.8;
  margin: 14px 0 0;
}
footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 24px;
}
footer > span {
  font-size: 11px;
  color: var(--muted);
}
.feedback-primary {
  min-height: 40px;
  border: 0;
  border-radius: 8px;
  padding: 10px 18px;
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
  font-size: 13px;
  font-weight: 550;
  cursor: pointer;
}
.feedback-primary:hover {
  filter: brightness(1.08);
}
button:disabled {
  opacity: 0.5;
  cursor: default;
}
button:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}
.feedback-primary.done {
  margin-top: 6px;
}
@media (prefers-reduced-motion: no-preference) {
  .feedback-overlay {
    animation: feedback-fade 0.15s ease-out;
  }
  .feedback-composer {
    animation: feedback-enter 0.18s ease-out;
  }
}
@keyframes feedback-fade {
  from {
    opacity: 0;
  }
}
@keyframes feedback-enter {
  from {
    opacity: 0;
    transform: translate(-50%, calc(-50% + 6px));
  }
}
</style>
