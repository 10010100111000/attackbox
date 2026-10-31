import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from 'vue'
import { accountEmailCooldownSeconds } from '#shared/account-email'

const storageKey = 'attackbox-email-cooldowns'

/** A local deadline per mailbox; the server remains responsible for rate limits.
 * Session storage survives reloads; expired deadlines are discarded on read. */
export function useEmailCooldown(address: Readonly<Ref<string>>) {
  const deadlines = useState<Record<string, number>>('auth:email-cooldowns', () => ({}))
  const now = ref(Date.now())
  const mailbox = computed(() => address.value.trim().toLowerCase())
  const remaining = computed(() =>
    Math.max(0, Math.ceil(((deadlines.value[mailbox.value] || 0) - now.value) / 1000))
  )
  let timer: ReturnType<typeof setInterval> | undefined
  let active = false
  function live(value: unknown): Record<string, number> {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return {}
    return Object.fromEntries(
      Object.entries(value).filter(
        ([email, until]) =>
          email.length <= 254 &&
          email.includes('@') &&
          typeof until === 'number' &&
          Number.isFinite(until) &&
          until > now.value &&
          until <= now.value + accountEmailCooldownSeconds * 1000
      )
    )
  }
  function persist() {
    try {
      sessionStorage.setItem(storageKey, JSON.stringify(deadlines.value))
    } catch {
      /* Keep memory state if storage is unavailable. */
    }
  }
  function stop() {
    clearInterval(timer)
    timer = undefined
  }
  function tick() {
    now.value = Date.now()
    if (!remaining.value) {
      stop()
      deadlines.value = live(deadlines.value)
      persist()
    }
  }
  function sync() {
    if (!active) return
    tick()
    if (remaining.value && !timer) timer = setInterval(tick, 1000)
  }
  function start(email = address.value) {
    now.value = Date.now()
    deadlines.value = {
      ...live(deadlines.value),
      [email.trim().toLowerCase()]: now.value + accountEmailCooldownSeconds * 1000
    }
    persist()
    sync()
  }
  watch([mailbox, () => deadlines.value[mailbox.value]], sync)
  onMounted(() => {
    active = true
    now.value = Date.now()
    try {
      deadlines.value = live({
        ...JSON.parse(sessionStorage.getItem(storageKey) || '{}'),
        ...deadlines.value
      })
    } catch {
      deadlines.value = live(deadlines.value)
    }
    persist()
    sync()
    document.addEventListener('visibilitychange', sync)
  })
  onBeforeUnmount(() => {
    active = false
    stop()
    document.removeEventListener('visibilitychange', sync)
  })
  return { remaining, start }
}
