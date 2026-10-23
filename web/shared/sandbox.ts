export interface SandboxNodeStatus {
  id: string
  label: string
  publicDataUrl: string
  state: 'offline' | 'registering' | 'waiting' | 'online'
  bootId: string | null
  lastHeartbeatAt: string | null
  version: string | null
  capabilities: { cpuCores: number; totalMemoryMb: number; os: string; hasKvm: boolean } | null
  telemetry: { loadAverage: number; availableMemoryMb: number; freeDiskGb: number } | null
  inventoryRevision: number | null
  sandboxes: { id: string; state: string; lastActiveAt: string | null; guiRunning: boolean }[]
}
export interface SandboxNodesResponse {
  enabled: boolean
  configurationError: string | null
  schedulingEnabled: boolean
  observedAt: string
  nodes: SandboxNodeStatus[]
}

export interface SandboxPolicy {
  resources: { cpu_weight: number; memory_max_mb: number; memory_swap_max_mb: number; pids_max: number; io_write_bytes_sec: number; io_write_iops: number }
  storage: { tmpfs_tmp_size: string; tmpfs_run_size: string }
  security: { read_only_proc_sys: boolean; mask_sysrq: boolean }
}
export interface SandboxInstance {
  id: string
  nodeId: string
  state: 'creating' | 'running' | 'stopping' | 'cleanup_failed'
  slot: number
  ip: string | null
  startedAt: string
  expiresAt: string | null
  lastActiveAt: string | null
  connected: boolean
  policy: SandboxPolicy
  error: string | null
}
export interface SandboxStatus {
  available: boolean
  instance: SandboxInstance | null
}
export interface SandboxTicket { url: string; ticket: string; expiresAt: string; sandboxId: string }
export interface AdminSandboxInstance extends SandboxInstance { userId: string; email: string | null; displayName: string | null }
