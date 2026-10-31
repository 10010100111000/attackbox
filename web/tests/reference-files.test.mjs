import test from 'node:test'
import assert from 'node:assert/strict'
import { mkdtemp, rm, readFile, stat } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { gunzipSync } from 'node:zlib'
import {
  buildReference,
  prepareReferenceBuild,
  retireReferenceBuilds,
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
    await referencePublication(async files => files.publish(), meta)
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
        await files.publish()
        throw Error('database rollback')
      }, changed)
    )
    assert.equal((await readReference('fixture')).meta.revision, meta.revision)
    await referencePublication(async files => files.publish(), changed)
    assert.ok((await readReference('fixture')).html.includes('new text'))
    const newest = await buildReference({ ...input, bodyMarkdown: '# Latest\n\nlatest text' })
    await assert.rejects(
      referencePublication(async files => {
        await files.publish()
        // A newer writer committed after the first writer released its DB lock,
        // before its failed commit reached the filesystem rollback handler.
        await referencePublication(async next => next.publish(), newest)
        throw Error('late database rollback')
      }, changed)
    )
    assert.ok((await readReference('fixture')).html.includes('latest text'))
    await assert.rejects(
      readReferenceFile(`fixture/${meta.revision}/chunk-1.html`),
      e => e.statusCode === 404
    )
    await assert.rejects(readReferenceFile('../private/a'), e => e.statusCode === 404)
    await referencePublication(async files => files.withdraw('fixture'))
    await assert.rejects(readReference('fixture'), e => e.statusCode === 404)
    const staged = await prepareReferenceBuild({ ...input, bodyMarkdown: '# Staged\n\nprivate' })
    const cleanup = await retireReferenceBuilds(input.id)
    // Retirement does not touch in-progress builds; deferred deletion cannot
    // remove a new build installed after the cleanup transaction releases locks.
    await staged.install()
    await cleanup()
    await staged.discard()
    await referencePublication(async files => {
      // Copies completed before the transaction callback starts.
      await pruneReferenceBuilds(input.id)
      await files.publish()
    }, staged.meta)
    assert.ok((await readReference('fixture')).html.includes('private'))
    await referencePublication(async files => files.withdraw('fixture'))
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
