import test from 'node:test'
import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
const script = `import {createJiti} from 'jiti'; try {const {client}=await createJiti(import.meta.url).import('./server/db/index.ts');await client.end()}catch(error){console.error(error.message);process.exit(1)}`
test('database requires explicit configuration even when PostgreSQL defaults exist', () => {
  for (const value of [undefined, '', '   ']) {
    const env = { ...process.env, PGHOST: 'unused.invalid', PGDATABASE: 'unused' }
    if (value === undefined) delete env.DATABASE_URL
    else env.DATABASE_URL = value
    const result = spawnSync(process.execPath, ['--input-type=module', '-e', script], {
      cwd: new URL('../', import.meta.url),
      env,
      encoding: 'utf8',
      timeout: 10000
    })
    assert.equal(result.status, 1)
    assert.match(result.stderr, /DATABASE_URL is required/)
    assert.ok(!result.stderr.includes('postgres://'))
  }
})
test('configured database client initializes without opening a connection', () => {
  const result = spawnSync(process.execPath, ['--input-type=module', '-e', script], {
    cwd: new URL('../', import.meta.url),
    env: { ...process.env, DATABASE_URL: 'postgres://fixture:local-only@127.0.0.1:1/unused_test' },
    encoding: 'utf8',
    timeout: 10000
  })
  assert.equal(result.status, 0, result.stderr)
})
