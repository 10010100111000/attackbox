import { computed, ref, watch, onMounted, onBeforeUnmount } from 'vue'
import type { TargetDisplay } from '~/shared/learning-target'
import { apiErrorMessage, apiFetch } from '~/utils/api'

export interface TargetInstance {
  expiresAt: string
  id: string
  state: string
  instance?: {
    ip?: string
    entry_address?: string
    attacker_address?: string
    interface?: string
    attachment?: string
    services?: import('~/shared/compose-labs').ComposeService[]
    ready: boolean
    readiness: string
  }
  error?: string
}

export function useTargetSession(props: {
  context: Record<string, string>
  target?: TargetDisplay
}) {
  const confirmAction = useConfirm()
  const { currentUser } = useAuth()
  const sandboxSession = useState<import('~/shared/sandbox').SandboxStatus>(
    'sandbox:session',
    () => ({ available: false, instance: null })
  )
  const requestPending = ref(false)
  const current = ref<TargetInstance | null>(null),
    initializing = ref(true),
    action = ref(''),
    message = ref('')
  let alive = true
  let identityGeneration = 0
  const clock = ref(Date.now())
  let clockTimer: ReturnType<typeof setInterval> | undefined
  const remaining = computed(() =>
    current.value?.expiresAt
      ? Math.max(0, Math.ceil((Date.parse(current.value.expiresAt) - clock.value) / 1000))
      : null
  )
  const countdown = computed(() =>
    remaining.value === null
      ? ''
      : `${Math.floor(remaining.value / 60)
          .toString()
          .padStart(2, '0')}:${(remaining.value % 60).toString().padStart(2, '0')}`
  )
  const config = computed(() => (props.target?.status === 'configured' ? props.target : null))
  const unavailable = computed(() => props.target?.status === 'unavailable')
  const state = computed(() => current.value?.state || 'stopped')
  const active = computed(() => !!current.value && state.value !== 'stopped')
  const canRetry = computed(
    () => ['missing', 'created'].includes(state.value) || (active.value && remaining.value === 0)
  )
  const status = computed(() => {
    if (initializing.value) return ''
    if (active.value && remaining.value === 0) return '租期已到期'
    if (action.value === 'start') return ''
    if (action.value === 'stop') return ''
    if (state.value === 'running' && current.value?.instance?.attachment === 'not_connected')
      return '服务运行中 · 尚未接入攻击环境'
    if (current.value?.instance?.ready && state.value === 'running') return '已就绪'
    if (state.value === 'running') return '运行中'
    return (
      (
        {
          starting: '',
          incomplete: '启动未完成，请释放后重试',
          degraded: '部分服务未就绪',
          failed: '服务异常，请释放后重试',
          created: '尚未启动',
          stopping: '',
          cleanup_pending: '清理待确认',
          missing: '实例缺失',
          orphaned: '状态异常',
          unconfirmed: '状态待确认',
          stopped: '未启动'
        } as Record<string, string>
      )[state.value] || '状态待确认'
    )
  })
  async function run(operation: 'inspect' | 'start' | 'stop', confirm = true) {
    if (
      requestPending.value ||
      action.value ||
      !currentUser.value ||
      (unavailable.value && operation === 'start')
    )
      return
    const expectedId = current.value?.id,
      expectedGeneration = identityGeneration,
      expectedContext = JSON.stringify(props.context)
    if (
      operation === 'stop' &&
      confirm &&
      !(await confirmAction({
        title: '停止靶机？',
        description: '当前靶机实例将释放，其中的修改会被清除。',
        confirmLabel: '停止靶机'
      }))
    )
      return
    if (
      !alive ||
      requestPending.value ||
      action.value ||
      !currentUser.value ||
      identityGeneration !== expectedGeneration ||
      current.value?.id !== expectedId ||
      JSON.stringify(props.context) !== expectedContext
    )
      return
    requestPending.value = true
    const generation = identityGeneration
    if (operation !== 'inspect') {
      action.value = operation
      message.value = ''
    }
    try {
      const result =
        operation === 'inspect'
          ? await apiFetch<TargetInstance | null>('/api/targets', {
              query: props.context,
              retry: false
            })
          : await apiFetch<TargetInstance>('/api/targets', {
              method: 'POST',
              body: { ...props.context, action: operation, id: current.value?.id }
            })
      if (alive && generation === identityGeneration) {
        current.value = result
        message.value = result?.error || ''
      }
    } catch (e) {
      if (alive && generation === identityGeneration)
        message.value = apiErrorMessage(e, '操作结果尚未确认')
    } finally {
      if (alive && generation === identityGeneration) {
        requestPending.value = false
        action.value = ''
        initializing.value = false
      }
    }
  }
  function syncClock() {
    const expiresAt = current.value?.expiresAt
    const shouldRun = alive && active.value && !!expiresAt && Date.parse(expiresAt) > Date.now()
    if (shouldRun) {
      clock.value = Date.now()
      if (!clockTimer) {
        clockTimer = setInterval(() => {
          clock.value = Date.now()
          if (
            !current.value?.expiresAt ||
            !active.value ||
            Date.parse(current.value.expiresAt) <= clock.value
          ) {
            clearInterval(clockTimer)
            clockTimer = undefined
            if (alive && active.value) void run('inspect')
          }
        }, 1000)
      }
    } else if (clockTimer) {
      clearInterval(clockTimer)
      clockTimer = undefined
    }
  }
  onMounted(() => {
    syncClock()
    initializing.value = !!currentUser.value && !unavailable.value
    if (props.target) void run('inspect')
    else initializing.value = false
  })
  // When the learner's sandbox starts or changes, refresh target attachment / reverse IP.
  watch(
    () => sandboxSession.value.instance?.id,
    () => {
      if (active.value && current.value?.instance?.attachment) void run('inspect')
    }
  )
  watch([active, () => current.value?.expiresAt], () => {
    if (import.meta.client) syncClock()
  })
  watch(
    () => currentUser.value?.id,
    () => {
      identityGeneration++
      current.value = null
      message.value = ''
      action.value = ''
      requestPending.value = false
      initializing.value = !!currentUser.value
      if (currentUser.value && props.target) void run('inspect')
      else initializing.value = false
    }
  )
  onBeforeUnmount(() => {
    alive = false
    clearInterval(clockTimer)
    clockTimer = undefined
  })

  return {
    current,
    initializing,
    action,
    requestPending,
    message,
    remaining,
    countdown,
    config,
    unavailable,
    state,
    active,
    canRetry,
    status,
    run
  }
}
