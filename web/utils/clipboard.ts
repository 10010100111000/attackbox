const MAX_CLIPBOARD_LENGTH = 65536
const CHANNEL_NAME = 'attackbox:clipboard'

let cachedClipboardText = ''
let listenersInstalled = false
let channel: BroadcastChannel | undefined
const syncListeners = new Set<(text: string) => void>()

function clipText(text: string): string {
  return text.slice(0, MAX_CLIPBOARD_LENGTH)
}

function notifyListeners(text: string, broadcast = true) {
  const clipped = clipText(text)
  if (!clipped) return
  cachedClipboardText = clipped
  for (const listener of syncListeners) {
    try {
      listener(clipped)
    } catch {
      /* Ignore listener errors so clipboard sync stays resilient. */
    }
  }
  if (broadcast) {
    try {
      channel?.postMessage({ type: 'clipboard', text: clipped })
    } catch {
      /* BroadcastChannel is optional across browser contexts. */
    }
  }
}

function ensureGlobalClipboardListeners() {
  if (listenersInstalled || typeof window === 'undefined' || typeof document === 'undefined') return
  listenersInstalled = true

  if (typeof BroadcastChannel !== 'undefined') {
    try {
      channel = new BroadcastChannel(CHANNEL_NAME)
      channel.onmessage = event => {
        const data = event.data
        if (data?.type === 'clipboard' && typeof data.text === 'string' && data.text) {
          notifyListeners(data.text, false)
        }
      }
    } catch {
      channel = undefined
    }
  }

  const captureSelectionCopy = (event: ClipboardEvent) => {
    const clipboardText = event.clipboardData?.getData('text/plain')
    if (clipboardText) {
      notifyListeners(clipboardText, true)
      return
    }
    const active = document.activeElement
    if (
      active instanceof HTMLInputElement ||
      active instanceof HTMLTextAreaElement
    ) {
      const start = active.selectionStart ?? 0
      const end = active.selectionEnd ?? 0
      if (end > start) {
        notifyListeners(active.value.slice(start, end), true)
        return
      }
    }
    const selected = window.getSelection?.()?.toString() || ''
    if (selected) notifyListeners(selected, true)
  }

  document.addEventListener('copy', captureSelectionCopy, true)
  document.addEventListener('cut', captureSelectionCopy, true)
}

function fallbackExecCopy(text: string): boolean {
  if (typeof document === 'undefined' || !document.body) return false
  const previousActive = document.activeElement as HTMLElement | null
  const selection = window.getSelection?.()
  const savedRanges: Range[] = []
  if (selection) {
    for (let i = 0; i < selection.rangeCount; i++) {
      savedRanges.push(selection.getRangeAt(i).cloneRange())
    }
  }

  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.setAttribute('aria-hidden', 'true')
  textarea.style.position = 'fixed'
  textarea.style.top = '-9999px'
  textarea.style.left = '-9999px'
  textarea.style.opacity = '0'
  textarea.style.pointerEvents = 'none'
  document.body.appendChild(textarea)

  let copied = false
  try {
    textarea.focus({ preventScroll: true })
    textarea.select()
    textarea.setSelectionRange(0, textarea.value.length)
    copied = document.execCommand('copy')
  } catch {
    copied = false
  } finally {
    document.body.removeChild(textarea)
    if (selection) {
      selection.removeAllRanges()
      for (const range of savedRanges) {
        try {
          selection.addRange(range)
        } catch {
          /* Ignore detached range restoration. */
        }
      }
    }
    try {
      previousActive?.focus?.({ preventScroll: true })
    } catch {
      /* Ignore focus restoration failures. */
    }
  }
  return copied
}

export async function writeBrowserClipboard(
  text: string,
  options: { broadcast?: boolean } = {}
): Promise<boolean> {
  ensureGlobalClipboardListeners()
  const clipped = clipText(text)
  if (!clipped) return false
  notifyListeners(clipped, options.broadcast !== false)

  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(clipped)
      return true
    } catch {
      /* Fall through to execCommand fallback when Clipboard API is restricted. */
    }
  }
  return fallbackExecCopy(clipped)
}

export async function readBrowserClipboard(): Promise<string | null> {
  ensureGlobalClipboardListeners()
  if (typeof navigator !== 'undefined' && navigator.clipboard?.readText) {
    try {
      const text = clipText(await navigator.clipboard.readText())
      if (text) cachedClipboardText = text
      return text
    } catch {
      /* Fall back to in-session synced clipboard cache when readText is blocked. */
    }
  }
  return cachedClipboardText || null
}

export function onClipboardSync(listener: (text: string) => void): () => void {
  ensureGlobalClipboardListeners()
  syncListeners.add(listener)
  return () => {
    syncListeners.delete(listener)
  }
}

export function decodeOsc52Clipboard(data: string): string | null {
  const separator = data.indexOf(';')
  if (separator < 0) return null
  const base64 = data.slice(separator + 1).trim()
  if (!base64 || base64 === '?') return null
  try {
    const binary = atob(base64)
    const bytes = new Uint8Array(binary.length)
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i)
    }
    return clipText(new TextDecoder().decode(bytes))
  } catch {
    return null
  }
}
