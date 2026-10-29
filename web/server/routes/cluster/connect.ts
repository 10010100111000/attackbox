import { defineWebSocketHandler } from 'h3'
import { authorizeWorker } from '../../sandbox/config'
import { sandboxHub } from '../../sandbox'
export default defineWebSocketHandler({
  upgrade(request) {
    const status = authorizeWorker(sandboxHub().config, request.headers)
    if (status) return new Response('Worker connection unavailable', { status })
  },
  open(peer) {
    sandboxHub().open(peer, peer.request.headers.get('x-node-id') || '')
  },
  message(peer, message) {
    sandboxHub().message(peer, message.text())
  },
  close(peer) {
    sandboxHub().close(peer)
  },
  error(peer) {
    sandboxHub().close(peer)
  }
})
