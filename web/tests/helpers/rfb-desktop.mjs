// Minimal local RFB fixture for exercising the real noVNC client, not a real desktop.
export function testDesktop(socket, inputs) {
  let buffer = Buffer.alloc(0), phase = 'version', drawn = false
  socket.send(Buffer.from('RFB 003.008\n'))
  function frame() {
    const width = 640, height = 360, header = Buffer.alloc(16)
    header[0] = 0; header.writeUInt16BE(1, 2); header.writeUInt16BE(width, 8); header.writeUInt16BE(height, 10)
    const pixels = Buffer.alloc(width * height * 4)
    for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) {
      const offset = (y * width + x) * 4, stripe = x > 40 && x < 600 && y > 55 && y < 115
      pixels[offset] = stripe ? 168 : 35; pixels[offset + 1] = stripe ? 190 : 30; pixels[offset + 2] = stripe ? 110 : 27
    }
    socket.send(Buffer.concat([header, pixels])); drawn = true
  }
  socket.on('message', data => {
    buffer = Buffer.concat([buffer, data])
    while (buffer.length) {
      if (phase === 'version') {
        if (buffer.length < 12) return
        buffer = buffer.subarray(12); phase = 'security'; socket.send(Buffer.from([1, 1]))
      } else if (phase === 'security') {
        buffer = buffer.subarray(1); phase = 'init'; socket.send(Buffer.alloc(4))
      } else if (phase === 'init') {
        buffer = buffer.subarray(1); phase = 'normal'
        const name = Buffer.from('AttackBox protocol test desktop'), init = Buffer.alloc(24)
        init.writeUInt16BE(640, 0); init.writeUInt16BE(360, 2)
        init[4] = 32; init[5] = 24; init[7] = 1
        for (const offset of [8, 10, 12]) init.writeUInt16BE(255, offset)
        init[14] = 16; init[15] = 8; init[16] = 0; init.writeUInt32BE(name.length, 20)
        socket.send(Buffer.concat([init, name]))
      } else {
        const type = buffer[0]
        let size
        if (type === 0) size = 20
        else if (type === 2) { if (buffer.length < 4) return; size = 4 + buffer.readUInt16BE(2) * 4 }
        else if (type === 3) size = 10
        else if (type === 4) size = 8
        else if (type === 5) size = 6
        else if (type === 6) { if (buffer.length < 8) return; size = 8 + buffer.readUInt32BE(4) }
        else { socket.close(1008, 'Unsupported test RFB message'); return }
        if (buffer.length < size) return
        if (type === 3 && (!drawn || buffer[1] === 0)) frame()
        if (type === 4) inputs.push({ type: 'key', down: !!buffer[1], key: buffer.readUInt32BE(4) })
        if (type === 5) inputs.push({ type: 'pointer', buttons: buffer[1], x: buffer.readUInt16BE(2), y: buffer.readUInt16BE(4) })
        buffer = buffer.subarray(size)
      }
    }
  })
}
