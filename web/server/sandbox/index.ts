import { getRuntimePolicy } from '../services/runtime-policy'
import { sandboxConfig } from './config'
import { SandboxHub } from './hub'

import { SandboxController } from './controller'

let hub: SandboxHub | undefined
export function sandboxHub() {
  return (hub ||= new SandboxHub(sandboxConfig()))
}
let controller: SandboxController | undefined
export function sandboxController() {
  return (controller ||= new SandboxController(sandboxHub(), Date.now, 30000, getRuntimePolicy))
}
