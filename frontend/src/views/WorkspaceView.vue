<template>
  <section class="space-y-6">
    <!-- Offensive Security HUD Control Bar -->
    <div class="shadcn-card px-4 py-3 sm:px-5 sm:py-3.5">
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-3">
        <!-- Left: Instance Status & Lease Indicator -->
        <div class="flex items-center gap-3 flex-wrap">
          <!-- Standby State -->
          <div v-if="!isRunning" class="flex items-center gap-2.5">
            <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono font-medium bg-zinc-100 text-zinc-600 border border-zinc-200">
              <span class="w-2 h-2 rounded-full bg-zinc-400"></span>
              <span>STANDBY</span>
            </span>
            <span class="text-xs text-zinc-400 font-mono hidden sm:inline-block">On-demand instance · Launch terminal or desktop</span>
          </div>

          <!-- Active State -->
          <div v-else class="flex items-center gap-2.5 flex-wrap">
            <span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono font-medium bg-emerald-50 text-emerald-700 border border-emerald-200 shadow-2xs">
              <span class="w-2 h-2 rounded-full bg-emerald-500 animate-pulse"></span>
              <span>ACTIVE</span>
            </span>

            <div class="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-zinc-50 border border-zinc-200 text-xs font-mono text-zinc-700">
              <svg class="w-3.5 h-3.5 text-zinc-400" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
                <circle cx="12" cy="12" r="10"></circle>
                <polyline points="12 6 12 12 16 14"></polyline>
              </svg>
              <span class="text-zinc-500 text-[11px]">LEASE:</span>
              <span class="font-semibold text-zinc-900 tracking-wider">{{ leaseDisplay }}</span>
              <span class="text-[10px] text-zinc-400 hidden sm:inline" title="Terminates automatically after 10m idle">(IDLE: 10m)</span>
            </div>
          </div>
        </div>

        <!-- Right: Action Buttons -->
        <div class="flex items-center gap-2 flex-wrap">
          <!-- CLI Terminal Button -->
          <button
            @click="handleOpenCli"
            :disabled="startingCli || startingGui"
            class="px-3.5 py-1.5 rounded-md text-xs font-mono font-semibold bg-zinc-900 text-white hover:bg-zinc-800 shadow-sm flex items-center gap-1.5 transition-all cursor-pointer active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
            title="Open CLI terminal in a new window"
          >
            <svg v-if="startingCli" class="animate-spin h-3.5 w-3.5 text-zinc-300" viewBox="0 0 24 24" fill="none">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
            </svg>
            <svg v-else class="w-3.5 h-3.5 text-zinc-300" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <polyline points="4 17 10 11 4 5"></polyline>
              <line x1="12" y1="19" x2="20" y2="19"></line>
            </svg>
            <span>{{ startingCli ? 'SPAWNING...' : 'TERMINAL' }}</span>
          </button>

          <!-- GUI Desktop Button -->
          <button
            @click="handleOpenGui"
            :disabled="startingCli || startingGui"
            class="px-3.5 py-1.5 rounded-md text-xs font-mono font-semibold bg-zinc-900 text-white hover:bg-zinc-800 shadow-sm flex items-center gap-1.5 transition-all cursor-pointer active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
            title="Open GUI desktop in a new window"
          >
            <svg v-if="startingGui" class="animate-spin h-3.5 w-3.5 text-zinc-300" viewBox="0 0 24 24" fill="none">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
            </svg>
            <svg v-else class="w-3.5 h-3.5 text-zinc-300" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <rect x="2" y="3" width="20" height="14" rx="2"></rect>
              <line x1="8" y1="21" x2="16" y2="21"></line>
              <line x1="12" y1="17" x2="12" y2="21"></line>
            </svg>
            <span>{{ startingGui ? 'LAUNCHING...' : 'DESKTOP' }}</span>
          </button>

          <!-- Release Sandbox Button (Active only) -->
          <button
            v-if="isRunning"
            @click="handleDestroy"
            :disabled="destroying"
            class="px-2.5 py-1.5 rounded-md text-xs font-mono font-medium text-zinc-600 hover:text-rose-600 bg-zinc-100 hover:bg-rose-50 border border-zinc-200 hover:border-rose-200 flex items-center gap-1.5 transition-all disabled:opacity-50 cursor-pointer active:scale-95"
            title="Terminate current sandbox environment"
          >
            <svg v-if="destroying" class="animate-spin h-3.5 w-3.5 text-zinc-500" viewBox="0 0 24 24" fill="none">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
            </svg>
            <svg v-else class="w-3.5 h-3.5" fill="none" stroke="currentColor" stroke-width="2" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"></path>
            </svg>
            <span>{{ destroying ? 'RELEASING...' : 'RELEASE' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- TARGETS Section -->
    <div class="space-y-3">
      <div class="flex items-center justify-between pb-1 border-b border-zinc-200/60">
        <h3 class="text-xs font-mono font-bold tracking-wider text-zinc-900">TARGETS</h3>
        <span class="text-[11px] font-mono text-zinc-400">ISOLATED SUBNET · DIRECT ROUTING</span>
      </div>

      <!-- Target Cards Grid -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div class="shadcn-card p-4 flex flex-col justify-between hover:border-zinc-300 transition-colors">
          <div>
            <div class="flex items-start justify-between gap-2">
              <div>
                <h4 class="text-xs font-semibold text-zinc-900">DVWA Vulnerable Web App</h4>
                <div class="flex items-center gap-1.5 mt-1.5 flex-wrap">
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">LINUX</span>
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">WEB</span>
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">DOCKER</span>
                </div>
              </div>
              <span class="px-2 py-0.5 rounded text-[10px] font-mono font-medium bg-zinc-100 text-zinc-700 border border-zinc-200">
                ACTIVE
              </span>
            </div>
            <p class="text-xs text-zinc-500 mt-2.5 leading-relaxed font-sans">OWASP Top 10 training environment: SQLi, XSS, File Upload, and Command Execution.</p>
          </div>
          <div class="mt-4 pt-3 border-t border-zinc-100 flex items-center justify-between gap-2">
            <div class="text-xs font-mono flex items-center gap-1.5">
              <span class="text-zinc-400">IP:</span>
              <span class="font-semibold text-zinc-900">172.17.0.2</span>
            </div>
            <button
              @click="copyIp('172.17.0.2')"
              class="px-2 py-0.5 text-[11px] font-mono rounded bg-zinc-100 hover:bg-zinc-200 text-zinc-700 transition-colors cursor-pointer"
            >
              COPY
            </button>
          </div>
        </div>

        <div class="shadcn-card p-4 flex flex-col justify-between hover:border-zinc-300 transition-colors">
          <div>
            <div class="flex items-start justify-between gap-2">
              <div>
                <h4 class="text-xs font-semibold text-zinc-900">Struts2 Target Lab</h4>
                <div class="flex items-center gap-1.5 mt-1.5 flex-wrap">
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">LINUX</span>
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">RCE</span>
                  <span class="px-1.5 py-0.5 bg-zinc-100 text-zinc-600 rounded text-[10px] font-mono">LAB-NET</span>
                </div>
              </div>
              <span class="px-2 py-0.5 rounded text-[10px] font-mono font-medium bg-zinc-100 text-zinc-500 border border-zinc-200">
                REACHABLE
              </span>
            </div>
            <p class="text-xs text-zinc-500 mt-2.5 leading-relaxed font-sans">Remote code execution baseline targets (S2-045, S2-057).</p>
          </div>
          <div class="mt-4 pt-3 border-t border-zinc-100 flex items-center justify-between gap-2">
            <div class="text-xs font-mono flex items-center gap-1.5">
              <span class="text-zinc-400">IP:</span>
              <span class="font-semibold text-zinc-900">10.10.10.25</span>
            </div>
            <button
              @click="copyIp('10.10.10.25')"
              class="px-2 py-0.5 text-[11px] font-mono rounded bg-zinc-100 hover:bg-zinc-200 text-zinc-700 transition-colors cursor-pointer"
            >
              COPY
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { api } from '../api.js'

const props = defineProps({
  isRunning: Boolean,
  expiresIn: {
    type: Number,
    default: 0
  }
})

const emit = defineEmits(['show-toast', 'env-updated'])

const destroying = ref(false)
const startingCli = ref(false)
const startingGui = ref(false)
const remainingSeconds = ref(props.expiresIn || 0)
let timer = null

watch(() => props.expiresIn, (newVal) => {
  remainingSeconds.value = typeof newVal === 'number' ? newVal : 0
})

onMounted(() => {
  timer = setInterval(() => {
    if (props.isRunning && remainingSeconds.value > 0) {
      remainingSeconds.value--
      if (remainingSeconds.value === 0) {
        emit('env-updated')
      }
    }
  }, 1000)
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
})

const leaseDisplay = computed(() => {
  if (!props.isRunning || remainingSeconds.value <= 0) {
    return '00:00:00'
  }
  const total = remainingSeconds.value
  const h = String(Math.floor(total / 3600)).padStart(2, '0')
  const m = String(Math.floor((total % 3600) / 60)).padStart(2, '0')
  const s = String(total % 60).padStart(2, '0')
  return `${h}:${m}:${s}`
})

async function handleOpenCli() {
  if (!props.isRunning) {
    startingCli.value = true
    try {
      await api.toggleCli()
      emit('env-updated')
    } catch (e) {
      emit('show-toast', 'Failed to start sandbox: ' + (e.message || 'System busy'))
      startingCli.value = false
      return
    } finally {
      startingCli.value = false
    }
  }
  window.open('/term', '_blank')
  emit('show-toast', 'Terminal launched in new window')
}

async function handleOpenGui() {
  if (!props.isRunning) {
    startingGui.value = true
    try {
      await api.toggleGui()
      emit('env-updated')
    } catch (e) {
      emit('show-toast', 'Failed to start sandbox: ' + (e.message || 'System busy'))
      startingGui.value = false
      return
    } finally {
      startingGui.value = false
    }
  }
  window.open('/gui', '_blank')
  emit('show-toast', 'Remote desktop launched in new window')
}

async function handleDestroy() {
  if (!confirm('Terminate sandbox session? Unsaved state in memory will be discarded.')) {
    return
  }
  destroying.value = true
  try {
    const data = await api.destroyEnv()
    if (!data.success) {
      emit('show-toast', data.error || 'Failed to release')
      return
    }
    emit('show-toast', 'Sandbox session terminated')
    emit('env-updated')
  } catch (e) {
    emit('show-toast', 'Release exception: ' + e.message)
  } finally {
    destroying.value = false
  }
}

function copyIp(ip) {
  navigator.clipboard.writeText(ip).then(
    () => emit('show-toast', 'Copied: ' + ip),
    () => emit('show-toast', 'Failed to copy')
  )
}
</script>
