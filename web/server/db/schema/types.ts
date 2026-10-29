import { customType } from 'drizzle-orm/pg-core'

export const bytea = customType<{ data: Buffer; driverData: Buffer }>({
  dataType() {
    return 'bytea'
  },
  toDriver(value: Buffer): Buffer {
    return value
  },
  fromDriver(value: unknown): Buffer {
    if (Buffer.isBuffer(value)) {
      return value
    }
    if (typeof value === 'string') {
      if (value.startsWith('\\x')) {
        return Buffer.from(value.slice(2), 'hex')
      }
      return Buffer.from(value)
    }
    return Buffer.from(value as any)
  }
})

export const inet = customType<{ data: string; driverData: string }>({
  dataType() {
    return 'inet'
  },
  toDriver(value: string): string {
    return value
  },
  fromDriver(value: unknown): string {
    return String(value)
  }
})
