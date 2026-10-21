import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { api, ensureSandbox } from '../src/api.js'

const credentials = { active: true, worker_url: 'ws://worker.test/ws/cli', ticket: 'pty', gui_worker_url: 'ws://worker.test/ws/gui', gui_ticket: 'vnc' }

function component(name, additions = {}) {
  const source = readFileSync(new URL(`../src/views/${name}.vue`, import.meta.url), 'utf8')
    .split('<script setup>')[1].split('</script>')[0].replace(/^import .*$/mg, '')
  const hooks = { beforeUnmount: [] }
  const context = {
    console, URL, AbortController, DOMException, TextEncoder, Uint8Array,
    ref: value => ({ value }), computed: fn => ({ get value() { return fn() } }),
    watch() {}, onMounted() {}, onBeforeUnmount: fn => hooks.beforeUnmount.push(fn),
    onUnmounted() {}, setTimeout, clearTimeout, setInterval, clearInterval,
    ...additions,
  }
  vm.createContext(context)
  vm.runInContext(source + '\nglobalThis.exercise = ' + ({
    CliView: '{createTerminal, closeAllWindows, onOpenRequest}',
    GuiView: '{connectDesktop, onOpenRequest}',
    WorkspaceView: '{handleOpenCli, handleOpenGui}',
  }[name]), context)
  return { context, hooks }
}

test('concurrent creation waits for the same sandbox', async t => {
  const start = api.startSandbox, active = api.getActiveSandbox
  t.after(() => { api.startSandbox = start; api.getActiveSandbox = active })
  let calls = 0
  api.startSandbox = async () => { calls++; throw Object.assign(new Error('busy'), {status:409}) }
  api.getActiveSandbox = async () => credentials
  assert.equal(await ensureSandbox(), credentials)
  assert.equal(calls, 1)
})

test('closing a viewer cancels waiting for another creation', async t => {
  const start = api.startSandbox, active = api.getActiveSandbox
  t.after(() => { api.startSandbox = start; api.getActiveSandbox = active })
  api.startSandbox = async () => { throw Object.assign(new Error('busy'), {status:409}) }
  api.getActiveSandbox = async () => ({active:false, state:'creating'})
  const controller = new AbortController()
  const pending = ensureSandbox(controller.signal)
  setImmediate(() => controller.abort())
  await assert.rejects(pending, {name:'AbortError'})
})

test('cleanup failure never starts a second sandbox', async t => {
  const start = api.startSandbox, active = api.getActiveSandbox
  t.after(() => { api.startSandbox = start; api.getActiveSandbox = active })
  api.startSandbox = async () => { throw Object.assign(new Error('busy'), {status:409}) }
  api.getActiveSandbox = async () => ({active:false, state:'cleanup_failed'})
  await assert.rejects(ensureSandbox(), /busy/)
})

test('closing a CLI window before credentials arrive never opens a socket', async () => {
  let resolve
  const sockets = []
  class Terminal {
    cols=80; rows=24
    open() {} write() {} onData() {} focus() {} dispose() {}
  }
  class WinBox { constructor(options) { this.options = options } close() { this.options.onclose() } }
  class WebSocket { constructor(url) { sockets.push(url) } }
  const {context} = component('CliView', {
    ensureSandbox: () => new Promise(r => { resolve = r }),
    window: { Terminal, WinBox, innerWidth:1200, innerHeight:800 },
    document: {createElement: () => ({})}, WebSocket,
  })
  const pending = context.exercise.createTerminal()
  context.exercise.closeAllWindows()
  resolve(credentials)
  await pending
  assert.equal(sockets.length, 0)
})

test('opening CLI again focuses its named window without reloading PTYs', () => {
  const calls = [], navigations = []
  const viewer = {location: {origin:'https://app.test', pathname:'/term', replace:p=>navigations.push(p)}, focus() {calls.push('focus')}, postMessage() {} }
  const {context} = component('WorkspaceView', {
    defineProps: () => ({}), defineEmits: () => () => {},
    window: {location:{origin:'https://app.test'}, open:(url,name)=> { calls.push([url,name]); return viewer }},
  })
  context.exercise.handleOpenCli()
  context.exercise.handleOpenCli()
  assert.equal(navigations.length, 0)
  assert.deepEqual(calls.filter(Array.isArray), [['','attackbox_cli'],['','attackbox_cli']])
})

test('GUI reconnect gets a new ticket and reuses the existing desktop', async () => {
  let starts=0, tickets=0
  const connections=[]
  class RFB {
    constructor(target,url) { this.url=url; this.events={}; connections.push(this) }
    addEventListener(type, fn) { this.events[type]=fn }
    disconnect() {} focus() {}
  }
  const {context,hooks} = component('GuiView', {
    ensureSandbox: async()=>{starts++;return credentials},
    api:{getActiveSandbox:async()=>{tickets++;return {...credentials,gui_ticket:'fresh'}}},
    Function: function() { return async()=>({default:RFB}) },
    window:{location:{hostname:'app.test'}, removeEventListener() {}},
  })
  await context.exercise.connectDesktop()
  connections[0].events.disconnect()
  await context.exercise.connectDesktop(true)
  assert.equal(starts,1)
  assert.equal(tickets,1)
  assert.equal(new URL(connections[1].url).searchParams.get('ticket'),'fresh')
  hooks.beforeUnmount.forEach(fn=>fn())
})

test('GUI cannot create a new sandbox while reconnecting after release', async () => {
  let starts=0
  class RFB {}
  const {context} = component('GuiView', {
    ensureSandbox: async()=>{starts++;return credentials},
    api:{getActiveSandbox:async()=>({active:false})},
    Function: function() { return async()=>({default:RFB}) },
  })
  await context.exercise.connectDesktop(true)
  assert.equal(starts,0)
})

test('CLI workspace launch after release starts a terminal, live terminal is preserved', async () => {
  let starts = 0
  const sockets = []
  class Terminal {
    cols=80; rows=24
    open() {} write() {} onData() {} focus() {} dispose() {}
  }
  class WinBox { constructor(options) { this.options=options } close() { this.options.onclose() } }
  class WebSocket {
    static OPEN=1; static CONNECTING=0
    readyState=1
    constructor() { sockets.push(this) }
    send() {}
  }
  const {context} = component('CliView', {
    ensureSandbox:async()=>{starts++;return credentials},
    window:{Terminal,WinBox,innerWidth:1200,innerHeight:800,location:{hostname:'app.test',origin:'https://app.test'}},
    document:{createElement:()=>({})}, WebSocket,
  })
  const event={origin:'https://app.test',data:{type:'attackbox:open-viewer'}}
  await context.exercise.onOpenRequest(event)
  await context.exercise.onOpenRequest(event)
  assert.equal(starts,1)
  sockets[0].readyState=3
  await context.exercise.onOpenRequest(event)
  assert.equal(starts,2)
})

test('GUI workspace launch after release can start again, live desktop is preserved', async () => {
  let starts=0
  const connections=[]
  class RFB {
    constructor(){this.events={};connections.push(this)}
    addEventListener(type,fn){this.events[type]=fn}
    focus() {}
  }
  const {context} = component('GuiView', {
    ensureSandbox:async()=>{starts++;return credentials},
    Function:function(){return async()=>({default:RFB})},
    window:{location:{hostname:'app.test',origin:'https://app.test'}},
  })
  const event={origin:'https://app.test',data:{type:'attackbox:open-viewer'}}
  await context.exercise.onOpenRequest(event)
  await context.exercise.onOpenRequest(event)
  assert.equal(starts,1)
  connections[0].events.disconnect()
  await context.exercise.onOpenRequest(event)
  assert.equal(starts,2)
})
