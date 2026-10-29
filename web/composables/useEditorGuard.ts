import { onMounted, onBeforeUnmount, toValue, type MaybeRefOrGetter } from 'vue'
import { onBeforeRouteLeave, onBeforeRouteUpdate } from 'vue-router'
import { useConfirm } from './useConfirm'

interface EditorGuardOptions {
  dirty: MaybeRefOrGetter<boolean>
  busy: MaybeRefOrGetter<boolean>
  title: string
  description: string
}

/** Protect both route changes and document exits with the same editor state. */
export function useEditorGuard(options: EditorGuardOptions) {
  const confirm = useConfirm()
  async function canLeave() {
    if (toValue(options.busy)) return false
    if (!toValue(options.dirty)) return true
    const accepted = await confirm({
      title: options.title,
      description: options.description,
      confirmLabel: '放弃修改',
      cancelLabel: '继续编辑',
    })
    return accepted && !toValue(options.busy)
  }
  function beforeUnload(event: BeforeUnloadEvent) {
    if (toValue(options.dirty) || toValue(options.busy)) {
      event.preventDefault()
      event.returnValue = ''
    }
  }
  onBeforeRouteLeave(canLeave)
  onBeforeRouteUpdate(canLeave)
  onMounted(() => window.addEventListener('beforeunload', beforeUnload))
  onBeforeUnmount(() => window.removeEventListener('beforeunload', beforeUnload))
  return { canLeave }
}
