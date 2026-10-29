import test from 'node:test'
import assert from 'node:assert/strict'
import { ref } from 'vue'

test('TERMINAL_SCENES defines complete scripted scenes for nmap, smb, and msf', async () => {
  const { TERMINAL_SCENES, SCENE_KEYS } = await import('../composables/useTerminalPlayer.ts')

  assert.deepEqual(SCENE_KEYS, ['nmap', 'smb', 'msf'])

  for (const key of SCENE_KEYS) {
    const scene = TERMINAL_SCENES[key]
    assert.ok(scene, `Scene ${key} must exist`)
    assert.equal(typeof scene.branch, 'string')
    assert.ok(scene.branch.length > 0, `${key}.branch cannot be empty`)
    assert.equal(typeof scene.command, 'string')
    assert.ok(scene.command.length > 0, `${key}.command cannot be empty`)
    assert.ok(Array.isArray(scene.rows), `${key}.rows must be an array`)
    assert.ok(scene.rows.length >= 6, `${key}.rows must have at least 6 output lines`)

    for (const row of scene.rows) {
      assert.equal(typeof row.type, 'string', `${key} row.type must be string`)
      assert.equal(typeof row.text, 'string', `${key} row.text must be string`)
    }
  }

  // Check specific content
  assert.ok(TERMINAL_SCENES.nmap.branch.includes('recon'))
  assert.ok(TERMINAL_SCENES.nmap.command.startsWith('nmap'))
  assert.ok(TERMINAL_SCENES.nmap.rows.some(r => r.text.includes('22/tcp')))
  assert.ok(TERMINAL_SCENES.nmap.rows.some(r => r.text.includes('Nmap done')))

  assert.ok(TERMINAL_SCENES.smb.branch.includes('smb'))
  assert.ok(TERMINAL_SCENES.smb.command.startsWith('smbclient'))
  assert.ok(TERMINAL_SCENES.smb.rows.some(r => r.text.includes('Training files')))
  assert.ok(TERMINAL_SCENES.smb.rows.some(r => r.text.includes('Session complete')))

  assert.ok(TERMINAL_SCENES.msf.branch.includes('research'))
  assert.ok(TERMINAL_SCENES.msf.command.includes('msfconsole'))
  assert.ok(TERMINAL_SCENES.msf.rows.some(r => r.text.includes('auxiliary/scanner/smb')))
  assert.ok(TERMINAL_SCENES.msf.rows.some(r => r.text.includes('Module search complete')))
})

test('useTerminalPlayer transitions through typing -> execute -> output -> hold -> clear lifecycle', async () => {
  const { useTerminalPlayer, TERMINAL_SCENES } = await import('../composables/useTerminalPlayer.ts')

  const player = useTerminalPlayer({ autoStart: false })

  // 1. Initial State
  assert.equal(player.sceneKey.value, 'nmap')
  assert.equal(player.branch.value, TERMINAL_SCENES.nmap.branch)
  assert.equal(player.phase.value, 'typing')
  assert.equal(player.typedCommand.value, '')
  assert.equal(player.feedLines.value.length, 0)
  assert.equal(player.showCursor.value, true)
  assert.equal(player.isClearing.value, false)

  // 2. Typing phase
  // Step 500ms -> characters should be typed progressively
  player.step(500)
  assert.equal(player.phase.value, 'typing')
  assert.ok(player.typedCommand.value.length > 0)
  assert.equal(player.feedLines.value.length, 0)
  assert.equal(player.showCursor.value, true)
  assert.equal(player.isClearing.value, false)

  // Step enough time to complete command typing (nmap command is 36 chars -> 36 / 0.027 ~ 1334ms)
  player.step(1500)
  assert.equal(player.typedCommand.value, TERMINAL_SCENES.nmap.command)
  assert.equal(player.phase.value, 'execute')
  assert.equal(player.showCursor.value, true)

  // 3. Execute phase (pauses 350ms before showing output)
  player.step(200)
  assert.equal(player.phase.value, 'execute')
  assert.equal(player.showCursor.value, true)

  player.step(200) // Total 400ms in execute -> triggers transition to output
  assert.equal(player.phase.value, 'output')
  assert.equal(player.showCursor.value, false) // Cursor hidden

  // 4. Output phase (emits output rows sequentially)
  player.step(150)
  assert.equal(player.phase.value, 'output')
  assert.ok(player.feedLines.value.length >= 1)
  assert.deepEqual(player.feedLines.value[0], TERMINAL_SCENES.nmap.rows[0])

  // Step enough time to output all 9 rows (9 * 130ms ~ 1170ms)
  player.step(1500)
  assert.equal(player.feedLines.value.length, TERMINAL_SCENES.nmap.rows.length)
  assert.equal(player.phase.value, 'hold')

  // 5. Hold phase (pauses > 1300ms)
  player.step(1000)
  assert.equal(player.phase.value, 'hold')
  assert.equal(player.isClearing.value, false)

  player.step(400) // Total 1400ms > 1300ms -> triggers clear
  assert.equal(player.phase.value, 'clear')
  assert.equal(player.isClearing.value, true)

  // 6. Clear phase (lingers 260ms before switching script)
  player.step(150)
  assert.equal(player.phase.value, 'clear')
  assert.equal(player.isClearing.value, true)

  player.step(150) // Total 300ms >= 260ms -> loops to next scene ('smb')
  assert.equal(player.sceneKey.value, 'smb')
  assert.equal(player.branch.value, TERMINAL_SCENES.smb.branch)
  assert.equal(player.phase.value, 'typing')
  assert.equal(player.isClearing.value, false)
  assert.equal(player.showCursor.value, true)
  assert.equal(player.typedCommand.value, '')
  assert.equal(player.feedLines.value.length, 0)
})

test('useTerminalPlayer supports manual reset and initialScene option', async () => {
  const { useTerminalPlayer, TERMINAL_SCENES } = await import('../composables/useTerminalPlayer.ts')

  const player = useTerminalPlayer({ autoStart: false, initialScene: 'msf' })
  assert.equal(player.sceneKey.value, 'msf')
  assert.equal(player.branch.value, TERMINAL_SCENES.msf.branch)
  assert.equal(player.phase.value, 'typing')

  player.reset('smb')
  assert.equal(player.sceneKey.value, 'smb')
  assert.equal(player.branch.value, TERMINAL_SCENES.smb.branch)
  assert.equal(player.phase.value, 'typing')
  assert.equal(player.typedCommand.value, '')
  assert.equal(player.feedLines.value.length, 0)
})

test('useModuleRail exports required signature and executes safely in SSR / Node environment', async () => {
  const { useModuleRail, handleCardPointerMove } = await import('../composables/useModuleRail.ts')

  assert.equal(typeof useModuleRail, 'function')
  assert.equal(typeof handleCardPointerMove, 'function')

  const mockRef = ref(null)
  const rail = useModuleRail(mockRef)

  // Check interface contracts
  assert.ok('isPaused' in rail)
  assert.equal(typeof rail.pause, 'function')
  assert.equal(typeof rail.resume, 'function')
  assert.equal(typeof rail.measure, 'function')
  assert.equal(typeof rail.init, 'function')
  assert.equal(typeof rail.destroy, 'function')
  assert.equal(typeof rail.handleCardPointerMove, 'function')

  // In Node SSR where loop is not running, isPaused is true
  assert.equal(rail.isPaused.value, true)

  // Methods should not throw when called without DOM
  assert.doesNotThrow(() => rail.pause())
  assert.doesNotThrow(() => rail.resume(100))
  assert.doesNotThrow(() => rail.measure())
  assert.doesNotThrow(() => rail.init())
  assert.doesNotThrow(() => rail.destroy())
})

test('handleCardPointerMove sets --mx and --my CSS properties accurately', async () => {
  const { handleCardPointerMove } = await import('../composables/useModuleRail.ts')

  const styleProperties = new Map()
  const mockCard = {
    getBoundingClientRect: () => ({
      left: 120,
      top: 80,
      width: 300,
      height: 200
    }),
    style: {
      setProperty: (key, val) => styleProperties.set(key, val)
    }
  }

  const mockPointerEvent = {
    clientX: 200,
    clientY: 150
  }

  handleCardPointerMove(mockPointerEvent, mockCard)

  assert.equal(styleProperties.get('--mx'), '80px') // 200 - 120
  assert.equal(styleProperties.get('--my'), '70px') // 150 - 80

  // Gracefully handles null / undefined without errors
  assert.doesNotThrow(() => handleCardPointerMove(mockPointerEvent, null))
  assert.doesNotThrow(() => handleCardPointerMove(mockPointerEvent, {}))
})

test('useModuleRail clones original cards with loop-copy, removes duplicate IDs, and sets a11y attributes via init()', async () => {
  const { useModuleRail } = await import('../composables/useModuleRail.ts')

  // Construct a minimal DOM-like hierarchy to verify cloning logic
  const clonedElements = []
  const childElements = []

  const createCard = (moduleId, idAttr) => {
    const subElem = {
      tagName: 'A',
      attributes: { id: `link-${moduleId}`, href: '#' },
      tabIndex: 0,
      setAttribute: function (a, v) {
        this.attributes[a] = v
      },
      removeAttribute: function (a) {
        delete this.attributes[a]
      }
    }

    return {
      tagName: 'DIV',
      dataset: { module: moduleId },
      attributes: { 'data-module': moduleId, id: idAttr },
      tabIndex: 0,
      offsetLeft: moduleId === 'recon' ? 0 : 400,
      getAttribute: function (attr) {
        return this.attributes[attr] || null
      },
      setAttribute: function (attr, val) {
        this.attributes[attr] = val
      },
      removeAttribute: function (attr) {
        delete this.attributes[attr]
      },
      querySelectorAll: sel => {
        if (sel.includes('[id]')) return [subElem]
        if (sel.includes('a, button')) return [subElem]
        return []
      },
      cloneNode: function () {
        const clonedSubElem = {
          tagName: 'A',
          attributes: { ...subElem.attributes },
          tabIndex: subElem.tabIndex,
          setAttribute: function (a, v) {
            this.attributes[a] = v
          },
          removeAttribute: function (a) {
            delete this.attributes[a]
          }
        }
        const copy = {
          tagName: 'DIV',
          dataset: { ...this.dataset },
          attributes: { ...this.attributes },
          tabIndex: this.tabIndex,
          offsetLeft: this.offsetLeft + 800,
          getAttribute: function (a) {
            return this.attributes[a] || null
          },
          setAttribute: function (a, v) {
            this.attributes[a] = v
          },
          removeAttribute: function (a) {
            delete this.attributes[a]
          },
          remove: function () {
            const idx = childElements.indexOf(this)
            if (idx !== -1) childElements.splice(idx, 1)
          },
          querySelectorAll: sel => {
            if (sel.includes('[id]')) return [clonedSubElem]
            if (sel.includes('a, button')) return [clonedSubElem]
            return []
          }
        }
        return copy
      }
    }
  }

  const card1 = createCard('recon', 'card-recon')
  const card2 = createCard('smb', 'card-smb')
  childElements.push(card1, card2)

  const mockRail = {
    scrollLeft: 0,
    children: childElements,
    querySelectorAll: selector => {
      if (selector.includes(':not([data-loop-copy])')) {
        return childElements.filter(c => !('loopCopy' in c.dataset))
      }
      if (selector.includes('[data-loop-copy]')) {
        return childElements.filter(c => 'loopCopy' in c.dataset)
      }
      return childElements
    },
    appendChild: el => {
      childElements.push(el)
      clonedElements.push(el)
    },
    addEventListener: () => {},
    removeEventListener: () => {}
  }

  const railRef = ref(mockRail)
  const rail = useModuleRail(railRef, { autoStart: false })

  // Setup minimal global environment for init() execution
  const origWindow = globalThis.window
  const origDoc = globalThis.document

  try {
    globalThis.window = {
      addEventListener: () => {},
      removeEventListener: () => {}
    }
    globalThis.document = {
      addEventListener: () => {},
      removeEventListener: () => {}
    }

    // Call actual rail.init() to test cloneCards() and event attachment
    rail.init()

    assert.equal(clonedElements.length, 2, 'Must clone exactly the 2 original cards')
    for (const clone of clonedElements) {
      assert.ok('loopCopy' in clone.dataset, 'Clone must have data-loop-copy')
      assert.equal(clone.attributes['aria-hidden'], 'true', 'Clone must have aria-hidden=true')
      assert.equal(clone.tabIndex, -1, 'Clone root must have tabIndex=-1')
      assert.equal(clone.attributes.id, undefined, 'Clone root must have its id removed')

      // Check sub-elements have stripped id and tabindex=-1
      const sub = clone.querySelectorAll('[id]')[0]
      assert.equal(sub.attributes.id, undefined, 'Clone children must have id removed')
      assert.equal(sub.attributes.tabindex, '-1', 'Clone children must have tabindex="-1"')
    }

    // Test re-init capability after destroy()
    rail.destroy()
    rail.init()
    assert.equal(typeof rail.isPaused.value, 'boolean')
  } finally {
    globalThis.window = origWindow
    globalThis.document = origDoc
  }
})
