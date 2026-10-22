import {
  ref,
  computed,
  readonly,
  onMounted,
  onBeforeUnmount,
  getCurrentInstance,
  type Ref,
  type ComputedRef,
  type DeepReadonly
} from 'vue'

export type SceneKey = 'nmap' | 'smb' | 'msf'

export interface TerminalLine {
  type: string
  text: string
}

export interface TerminalScene {
  branch: string
  command: string
  rows: TerminalLine[]
}

export type TerminalPhase = 'typing' | 'execute' | 'output' | 'hold' | 'clear'

export const SCENE_KEYS: SceneKey[] = ['nmap', 'smb', 'msf']

export const TERMINAL_SCENES: Record<SceneKey, TerminalScene> = {
  nmap: {
    branch: 'recon / service-discovery',
    command: 'nmap -sV -p 22,80,445 192.0.2.10',
    rows: [
      { type: 'dim', text: 'Starting Nmap 7.95 ( https://nmap.org )' },
      { type: '', text: 'Nmap scan report for lab-target (192.0.2.10)' },
      { type: '', text: 'Host is up (0.0021s latency).' },
      { type: 'dim', text: 'PORT     STATE  SERVICE       VERSION' },
      { type: 'accent', text: '22/tcp   open   ssh           OpenSSH 9.6' },
      { type: '', text: '80/tcp   open   http          nginx 1.24.0' },
      { type: 'accent', text: '445/tcp  open   microsoft-ds  Samba smbd 4.19' },
      { type: 'dim', text: 'Service detection performed.' },
      { type: 'success', text: 'Nmap done: 1 IP address (1 host up) scanned' }
    ]
  },
  smb: {
    branch: 'recon / smb-enumeration',
    command: 'smbclient -L //192.0.2.10 -N',
    rows: [
      { type: 'dim', text: 'Anonymous login successful' },
      { type: '', text: '        Sharename       Type      Comment' },
      { type: 'dim', text: '        ---------       ----      -------' },
      { type: 'accent', text: '        public          Disk      Training files' },
      { type: '', text: '        IPC$            IPC       IPC Service' },
      { type: 'accent', text: '        print$          Disk      Printer Drivers' },
      { type: 'dim', text: 'SMB1 disabled -- no workgroup available' },
      { type: 'success', text: 'Session complete · isolated lab target' }
    ]
  },
  msf: {
    branch: 'research / module-inspection',
    command: 'msfconsole -q -x "search type:auxiliary smb; exit"',
    rows: [
      { type: 'dim', text: 'Matching Modules' },
      { type: '', text: '   #  Name                                 Rank' },
      { type: 'dim', text: '   -  ----                                 ----' },
      { type: 'accent', text: '   0  auxiliary/scanner/smb/smb_version     normal' },
      { type: '', text: '   1  auxiliary/scanner/smb/smb_enumshares  normal' },
      { type: 'accent', text: '   2  auxiliary/scanner/smb/smb_enumusers   normal' },
      { type: 'dim', text: 'Interact with a module by name or index.' },
      { type: 'success', text: 'Module search complete · no modules executed' }
    ]
  }
}

export interface UseTerminalPlayerOptions {
  autoStart?: boolean
  initialScene?: SceneKey
  speedMultiplier?: number
}

export interface TerminalPlayerReturn {
  sceneKey: DeepReadonly<Ref<SceneKey>>
  branch: ComputedRef<string>
  typedCommand: DeepReadonly<Ref<string>>
  showCursor: DeepReadonly<Ref<boolean>>
  feedLines: DeepReadonly<Ref<Array<TerminalLine>>>
  isClearing: DeepReadonly<Ref<boolean>>
  phase: DeepReadonly<Ref<TerminalPhase>>
  isPlaying: DeepReadonly<Ref<boolean>>
  step: (dt: number) => void
  play: () => void
  pause: () => void
  reset: (key?: SceneKey) => void
}

export function useTerminalPlayer(options: UseTerminalPlayerOptions = {}): TerminalPlayerReturn {
  const initialKey = options.initialScene && SCENE_KEYS.includes(options.initialScene)
    ? options.initialScene
    : 'nmap'

  const sceneKey = ref<SceneKey>(initialKey)
  const branch = computed(() => TERMINAL_SCENES[sceneKey.value]?.branch ?? '')
  const typedCommand = ref('')
  const showCursor = ref(true)
  const feedLines = ref<Array<TerminalLine>>([])
  const isClearing = ref(false)
  const phase = ref<TerminalPhase>('typing')
  const isPlaying = ref(false)

  let sceneIndex = SCENE_KEYS.indexOf(initialKey)
  let charIndex = 0
  let rowIndex = 0
  let clock = 0
  let phaseTime = 0
  let lastTime = 0
  let frameId: number | null = null
  let isDisposed = false
  let isReducedMotionCached = false
  let mediaQueryList: MediaQueryList | null = null

  const updateReducedMotion = () => {
    if (typeof window !== 'undefined' && window.matchMedia) {
      if (!mediaQueryList) {
        mediaQueryList = window.matchMedia('(prefers-reduced-motion: reduce)')
      }
      isReducedMotionCached = mediaQueryList.matches
    } else {
      isReducedMotionCached = false
    }
  }

  const getReducedMotion = (): boolean => {
    return isReducedMotionCached
  }

  const syncDocumentScene = (key: SceneKey) => {
    if (typeof document !== 'undefined' && document.documentElement) {
      document.documentElement.dataset.scene = key
    }
  }

  const begin = (key: SceneKey) => {
    sceneKey.value = key
    sceneIndex = SCENE_KEYS.indexOf(key)
    charIndex = 0
    rowIndex = 0
    phase.value = 'typing'
    phaseTime = clock
    typedCommand.value = ''
    feedLines.value = []
    showCursor.value = true
    isClearing.value = false
    syncDocumentScene(key)
  }

  const step = (dt: number) => {
    if (dt <= 0) return
    const reduced = getReducedMotion()
    const speed = (reduced ? 0.7 : 1) * (options.speedMultiplier ?? 1)
    clock += dt * speed
    const elapsed = clock - phaseTime
    const currentScene = TERMINAL_SCENES[sceneKey.value]

    if (phase.value === 'typing') {
      const count = Math.min(currentScene.command.length, Math.floor(elapsed * 0.027))
      if (count !== charIndex) {
        charIndex = count
        typedCommand.value = currentScene.command.slice(0, count)
      }
      if (count === currentScene.command.length) {
        phase.value = 'execute'
        phaseTime = clock
      }
    } else if (phase.value === 'execute') {
      if (elapsed >= 350) {
        phase.value = 'output'
        phaseTime = clock
        showCursor.value = false
      }
    } else if (phase.value === 'output') {
      const count = Math.min(currentScene.rows.length, Math.floor(elapsed / 130) + 1)
      while (rowIndex < count) {
        feedLines.value.push(currentScene.rows[rowIndex])
        rowIndex++
      }
      if (rowIndex === currentScene.rows.length) {
        phase.value = 'hold'
        phaseTime = clock
      }
    } else if (phase.value === 'hold') {
      if (elapsed > 1300) {
        phase.value = 'clear'
        phaseTime = clock
        isClearing.value = true
      }
    } else if (phase.value === 'clear') {
      if (elapsed >= 260) {
        sceneIndex = (sceneIndex + 1) % SCENE_KEYS.length
        begin(SCENE_KEYS[sceneIndex])
      }
    }
  }

  const tick = (now: number) => {
    if (isDisposed || !isPlaying.value) return
    frameId = requestAnimationFrame(tick)

    if (typeof document !== 'undefined' && document.hidden) {
      lastTime = 0
      return
    }

    if (!lastTime) {
      lastTime = now
      return
    }

    const dt = Math.min(now - lastTime, 60)
    lastTime = now
    step(dt)
  }

  const play = () => {
    if (isPlaying.value || isDisposed) return
    isPlaying.value = true
    lastTime = 0

    if (typeof window !== 'undefined' && typeof requestAnimationFrame !== 'undefined') {
      frameId = requestAnimationFrame(tick)
    }
  }

  const pause = () => {
    isPlaying.value = false
    if (frameId !== null && typeof cancelAnimationFrame !== 'undefined') {
      cancelAnimationFrame(frameId)
      frameId = null
    }
    lastTime = 0
  }

  const reset = (key?: SceneKey) => {
    const targetKey = key && SCENE_KEYS.includes(key) ? key : sceneKey.value
    begin(targetKey)
  }

  const onVisibilityChange = () => {
    if (typeof document !== 'undefined' && document.hidden) {
      lastTime = 0
    }
  }

  const onPageHide = (e: PageTransitionEvent) => {
    pause()
    if (!e.persisted) {
      isDisposed = true
    }
  }

  const onPageShow = (e: PageTransitionEvent) => {
    if (e.persisted && !isDisposed) {
      play()
    }
  }

  // Initial sync
  syncDocumentScene(sceneKey.value)

  // Attach lifecycle hooks if running within component setup
  if (getCurrentInstance()) {
    onMounted(() => {
      if (typeof window !== 'undefined') {
        updateReducedMotion()
        if (mediaQueryList?.addEventListener) {
          mediaQueryList.addEventListener('change', updateReducedMotion)
        }
        window.addEventListener('pagehide', onPageHide)
        window.addEventListener('pageshow', onPageShow)
        if (typeof document !== 'undefined') {
          document.addEventListener('visibilitychange', onVisibilityChange)
        }
        if (options.autoStart ?? true) {
          play()
        }
      }
    })

    onBeforeUnmount(() => {
      pause()
      isDisposed = true
      if (mediaQueryList?.removeEventListener) {
        mediaQueryList.removeEventListener('change', updateReducedMotion)
      }
      if (typeof window !== 'undefined') {
        window.removeEventListener('pagehide', onPageHide)
        window.removeEventListener('pageshow', onPageShow)
        if (typeof document !== 'undefined') {
          document.removeEventListener('visibilitychange', onVisibilityChange)
        }
      }
      if (typeof document !== 'undefined' && document.documentElement?.dataset) {
        delete document.documentElement.dataset.scene
      }
    })
  }

  return {
    sceneKey: readonly(sceneKey),
    branch,
    typedCommand: readonly(typedCommand),
    showCursor: readonly(showCursor),
    feedLines: readonly(feedLines),
    isClearing: readonly(isClearing),
    phase: readonly(phase),
    isPlaying: readonly(isPlaying),
    step,
    play,
    pause,
    reset
  }
}
