export interface ComposeSettings {
  file: string
  entryService: string
  entryNetwork: string
  entryPort: number
  prefixBits: number
}
export interface LabGitSource {
  repository: string
  branch: string
  directory: string
  commit: string
  hasToken: boolean
}
export interface ComposePreview {
  source?: LabGitSource | null
  id: string
  hash: string
  files: number
  composeFiles: string[]
  services: {
    name: string
    image: string
    build?: { context: string; dockerfile: string; target?: string }
    networks: string[]
    ports?: { port: number; protocol: 'tcp' | 'udp' }[]
  }[]
  networks: string[]
  variables: string[]
  missingFiles: string[]
  issues: string[]
  notes?: string[]
}
export interface ComposePreparation {
  attemptId: string
  mode: string
  version: number
  bootId: string
  id: string
  nodeId: string
  state: string
  deploymentId: string | null
  error: string | null
  templateVersion: number
  logs?: string[]
}
export interface ComposeSummary {
  services: { name: string; image: string }[]
  networks: string[]
  entry_service: string
  entry_network: string
  entry_port: number
  report: string[]
}
export interface ComposeService {
  name: string
  state: string
  health: string
  ips: Record<string, string>
}
export interface ComposeInstance {
  target_id: string
  owner_id: string
  state: string
  ready: boolean
  readiness: string
  entry_address?: string
  attacker_address?: string
  interface?: string
  attachment: string
  services: ComposeService[]
  expires_at?: string
}
