import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtemp, rm, readFile, stat } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { gunzipSync } from 'node:zlib'
import {
  buildReference,
  referencePublication,
  readReference,
  readReferenceFile,
  pruneReferenceBuilds
} from '../server/content/reference-files.ts'

test('private compilation, atomic publication, rollback, gzip, replacement and withdrawal work under restrictive umask', async () => {
  const root = await mkdtemp(path.join(tmpdir(), 'reference-files-'))
  const oldContent = process.env.CONTENT_DATA_DIR,
    oldPublic = process.env.REFERENCE_PUBLIC_DIR,
    mask = process.umask(0o077)
  process.env.CONTENT_DATA_DIR = path.join(root, 'private')
  process.env.REFERENCE_PUBLIC_DIR = path.join(root, 'public')
  try {
    const input = {
      id: 'fixture',
      slug: 'fixture',
      title: 'Document',
      summary: '',
      bodyMarkdown: '# One\n\nfirst\n\n# Two\n\nsecond',
      imageUrls: {}
    }
    const meta = await buildReference(input)
    await assert.rejects(readReference('fixture'), e => e.statusCode === 404)
    await referencePublication(async files => files.publish(meta))
    assert.equal((await readReference('fixture')).meta.revision, meta.revision)
    const file = path.join(
      process.env.REFERENCE_PUBLIC_DIR,
      'fixture',
      meta.revision,
      'chunk-1.html'
    )
    assert.equal((await stat(file)).mode & 0o777, 0o644)
    assert.equal((await stat(path.dirname(file))).mode & 0o777, 0o755)
    assert.equal(gunzipSync(await readFile(file + '.gz')).toString(), await readFile(file, 'utf8'))
    await assert.rejects(
      referencePublication(async files => {
        await files.withdraw('fixture')
        throw Error('database rollback')
      })
    )
    assert.equal((await readReference('fixture')).meta.revision, meta.revision)
    const changed = await buildReference({ ...input, bodyMarkdown: '# Updated\n\nnew text' })
    await assert.rejects(
      referencePublication(async files => {
        await files.publish(changed)
        throw Error('database rollback')
      })
    )
    assert.equal((await readReference('fixture')).meta.revision, meta.revision)
    await referencePublication(async files => files.publish(changed))
    assert.ok((await readReference('fixture')).html.includes('new text'))
    await assert.rejects(
      readReferenceFile(`fixture/${meta.revision}/chunk-1.html`),
      e => e.statusCode === 404
    )
    await assert.rejects(readReferenceFile('../private/a'), e => e.statusCode === 404)
    await referencePublication(async files => files.withdraw('fixture'))
    await assert.rejects(readReference('fixture'), e => e.statusCode === 404)
    await pruneReferenceBuilds(input.id)
  } finally {
    process.umask(mask)
    if (oldContent === undefined) delete process.env.CONTENT_DATA_DIR
    else process.env.CONTENT_DATA_DIR = oldContent
    if (oldPublic === undefined) delete process.env.REFERENCE_PUBLIC_DIR
    else process.env.REFERENCE_PUBLIC_DIR = oldPublic
    await rm(root, { recursive: true, force: true })
  }
})
