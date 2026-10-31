import { ref, computed } from 'vue'
import { apiErrorMessage } from '../utils/api.ts'

/** One mutation per request. Stopping leaves the active mutation to finish and
 * prevents dispatching subsequent items; no background job or polling. */
export function useSequentialBatch() {
  const running = ref(false),
    stopped = ref(false),
    processed = ref(0),
    total = ref(0),
    active = ref('')
  const results = ref<Record<string, { ok: boolean; message: string }>>({})
  const keys = ref<string[]>([])
  const succeeded = computed(() => keys.value.filter(key => results.value[key]?.ok).length)
  function reset() {
    if (running.value) return
    results.value = {}
    keys.value = []
    processed.value = 0
    total.value = 0
    stopped.value = false
  }
  async function run<T>(
    items: readonly T[],
    key: (item: T) => string,
    action: (item: T) => Promise<void>
  ) {
    if (running.value || !items.length) return
    running.value = true
    stopped.value = false
    processed.value = 0
    total.value = items.length
    keys.value = items.map(key)
    const current = new Set(keys.value)
    results.value = Object.fromEntries(
      Object.entries(results.value).filter(([key]) => !current.has(key))
    )
    try {
      for (const item of items) {
        if (stopped.value) break
        active.value = key(item)
        try {
          await action(item)
          results.value[active.value] = { ok: true, message: '完成' }
        } catch (error) {
          results.value[active.value] = {
            ok: false,
            message: apiErrorMessage(error, '操作失败，请重试')
          }
        }
        processed.value++
      }
    } finally {
      active.value = ''
      running.value = false
    }
  }
  return {
    running,
    processed,
    total,
    active,
    results,
    succeeded,
    stopped,
    run,
    reset,
    stop: () => {
      stopped.value = true
    }
  }
}
