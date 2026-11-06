import { writeBrowserClipboard } from '~/utils/clipboard'

export function useMarkdownCopy(copyable: () => boolean = () => true) {
  return function handleCopyClick(event: MouseEvent) {
    if (!copyable()) return
    const target = event.target as HTMLElement
    const btn = target.closest('.copy-code-btn') as HTMLButtonElement | null
    if (!btn) return

    const codeEl = btn.closest('.code-block')?.querySelector('pre code')
    const codeToCopy = btn.dataset.code
      ? decodeURIComponent(btn.dataset.code)
      : (codeEl?.textContent ?? '')
    if (!codeToCopy) return

    void writeBrowserClipboard(codeToCopy).then(ok => {
      if (!ok) {
        btn.textContent = '复制失败'
        return
      }
      const originalText = btn.textContent
      btn.textContent = '已复制 ✓'
      btn.classList.add('copied')
      setTimeout(() => {
        btn.textContent = originalText
        btn.classList.remove('copied')
      }, 2000)
    })
  }
}
