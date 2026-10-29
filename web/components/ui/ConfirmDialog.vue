<script setup lang="ts">
import {
  AlertDialogRoot,
  AlertDialogPortal,
  AlertDialogOverlay,
  AlertDialogContent,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogCancel
} from 'reka-ui'
import { TriangleAlert, CircleHelp } from 'lucide-vue-next'
const { $confirmation } = useNuxtApp()
const request = $confirmation.request
const route = useRoute()
const { currentUser } = useAuth()
function respond(accepted: boolean) {
  if (request.value) $confirmation.controller.finish(request.value.id, accepted)
}
watch(
  () => route.fullPath,
  () => $confirmation.controller.cancel()
)
watch(
  () => currentUser.value?.id,
  () => $confirmation.controller.cancel(),
  { flush: 'sync' }
)
onBeforeUnmount(() => $confirmation.controller.cancel())
</script>

<template>
  <AlertDialogRoot
    :open="!!request"
    @update:open="
      value => {
        if (!value) respond(false)
      }
    "
  >
    <AlertDialogPortal>
      <AlertDialogOverlay class="confirm-overlay" />
      <AlertDialogContent
        v-if="request"
        class="confirm-dialog"
        :data-tone="request.tone || 'danger'"
        @close-auto-focus.prevent
      >
        <div class="confirm-icon" aria-hidden="true">
          <CircleHelp v-if="request.tone === 'neutral'" :size="22" /><TriangleAlert
            v-else
            :size="22"
          />
        </div>
        <AlertDialogTitle class="confirm-title">{{ request.title }}</AlertDialogTitle>
        <AlertDialogDescription class="confirm-description">{{
          request.description
        }}</AlertDialogDescription>
        <div class="confirm-actions">
          <AlertDialogCancel class="confirm-cancel">{{
            request.cancelLabel || '取消'
          }}</AlertDialogCancel>
          <button type="button" class="confirm-accept" @click="respond(true)">
            {{ request.confirmLabel || '确认' }}
          </button>
        </div>
      </AlertDialogContent>
    </AlertDialogPortal>
  </AlertDialogRoot>
</template>

<style scoped>
.confirm-overlay {
  position: fixed;
  inset: 0;
  z-index: 300;
  background: #0007;
  backdrop-filter: blur(5px);
}
.confirm-dialog {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  z-index: 301;
  width: min(420px, calc(100vw - 32px));
  max-height: calc(100svh - 32px);
  overflow: auto;
  padding: 28px;
  background: var(--paper);
  color: var(--ink);
  border: 1px solid var(--line);
  border-radius: 16px;
  box-shadow: 0 24px 80px #0004;
}
.confirm-icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 13px;
  margin-bottom: 20px;
  background: var(--error-bg);
  color: var(--error-ink);
}
.confirm-title {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  letter-spacing: -0.4px;
}
.confirm-description {
  color: var(--body);
  font-size: 13px;
  line-height: 1.85;
  margin: 12px 0 26px;
  white-space: pre-line;
  overflow-wrap: anywhere;
}
.confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}
.confirm-actions button {
  min-height: 42px;
  border-radius: 8px;
  padding: 10px 16px;
  font-size: 13px;
  font-weight: 550;
  cursor: pointer;
}
.confirm-cancel {
  background: transparent;
  color: var(--ink);
  border: 1px solid var(--line);
}
.confirm-cancel:hover {
  background: var(--soft);
}
.confirm-accept {
  background: var(--error-ink);
  color: var(--paper);
  border: 1px solid transparent;
}
.confirm-accept:hover {
  filter: brightness(1.08);
}
[data-tone='neutral'] .confirm-icon {
  background: var(--brand-soft);
  color: var(--brand);
}
[data-tone='neutral'] .confirm-accept {
  background: var(--btn-primary-bg);
  color: var(--btn-primary-fg);
}
.confirm-actions button:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}
@media (prefers-reduced-motion: no-preference) {
  .confirm-overlay {
    animation: confirm-fade 0.15s ease-out;
  }
  .confirm-dialog {
    animation: confirm-enter 0.18s ease-out;
  }
}
@keyframes confirm-fade {
  from {
    opacity: 0;
  }
}
@keyframes confirm-enter {
  from {
    opacity: 0;
    transform: translate(-50%, calc(-50% + 6px));
  }
}
</style>
