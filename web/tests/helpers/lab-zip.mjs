import { deflateRawSync } from 'node:zlib'
function crc32(data) {
  let crc = 0xffffffff
  for (const byte of data) {
    crc ^= byte
    for (let i = 0; i < 8; i++) crc = (crc >>> 1) ^ (crc & 1 ? 0xedb88320 : 0)
  }
  return (crc ^ 0xffffffff) >>> 0
}
export function labZip(entries) {
  const locals = [],
    centrals = []
  let offset = 0
  for (const item of entries) {
    const name = Buffer.from(item.name),
      data = Buffer.from(item.data || ''),
      compressed = item.compress ? deflateRawSync(data) : data,
      crc = crc32(data),
      method = item.compress ? 8 : 0
    const local = Buffer.alloc(30 + name.length)
    local.writeUInt32LE(0x04034b50)
    local.writeUInt16LE(20, 4)
    local.writeUInt16LE(method, 8)
    local.writeUInt32LE(crc, 14)
    local.writeUInt32LE(compressed.length, 18)
    local.writeUInt32LE(data.length, 22)
    local.writeUInt16LE(name.length, 26)
    name.copy(local, 30)
    locals.push(local, compressed)
    const central = Buffer.alloc(46 + name.length)
    central.writeUInt32LE(0x02014b50)
    central.writeUInt16LE(0x0314, 4)
    central.writeUInt16LE(20, 6)
    central.writeUInt16LE(method, 10)
    central.writeUInt32LE(crc, 16)
    central.writeUInt32LE(compressed.length, 20)
    central.writeUInt32LE(data.length, 24)
    central.writeUInt16LE(name.length, 28)
    central.writeUInt32LE(((item.mode || 0x81a4) << 16) >>> 0, 38)
    central.writeUInt32LE(offset, 42)
    name.copy(central, 46)
    centrals.push(central)
    offset += local.length + compressed.length
  }
  const directory = Buffer.concat(centrals),
    end = Buffer.alloc(22)
  end.writeUInt32LE(0x06054b50)
  end.writeUInt16LE(entries.length, 8)
  end.writeUInt16LE(entries.length, 10)
  end.writeUInt32LE(directory.length, 12)
  end.writeUInt32LE(offset, 16)
  return Buffer.concat([...locals, directory, end])
}
