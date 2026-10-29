declare module '@novnc/novnc' {
  export default class RFB extends EventTarget {
    constructor(target: HTMLElement, url: string, options?: { shared?: boolean })
    scaleViewport: boolean
    clipViewport: boolean
    resizeSession: boolean
    viewOnly: boolean
    compressionLevel: number
    qualityLevel: number
    focus(): void
    disconnect(): void
    sendCtrlAltDel(): void
    clipboardPasteFrom(text: string): void
  }
}
