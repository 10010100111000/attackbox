import test from 'node:test'
import assert from 'node:assert/strict'
import { applyChapterAccess, summarizeProgress } from '../shared/learning.ts'
const chapter = (slug, access = 'authenticated', completedAt = null, lastVisitedAt = null) => ({ slug, access, completedAt, lastVisitedAt })
test('public previews remain open; private chapters need login and all preceding chapters', () => {
  const rows = [chapter('a', 'public'), chapter('b'), chapter('c', 'public'), chapter('d')]
  assert.deepEqual(applyChapterAccess(rows, false).map(c => c.lockReason), [null, 'login', null, 'login'])
  assert.deepEqual(applyChapterAccess(rows, true).map(c => c.lockReason), [null, 'prerequisite', null, 'prerequisite'])
  rows[0].completedAt = '2026-10-23T10:00:00Z'
  assert.deepEqual(applyChapterAccess(rows, true).map(c => c.lockReason), [null, null, null, 'prerequisite'])
  rows[3].completedAt = '2026-10-23T10:00:00Z'
  assert.equal(applyChapterAccess(rows, true)[3].isLocked, false, 'completed chapters stay reviewable after content reordering')
  assert.equal(applyChapterAccess([chapter('first')], true)[0].isLocked, false)
})
test('resume prefers last visited accessible unfinished chapter, skips locked content, and supports review', () => {
  const rows = [chapter('a', 'public', null, '2026-10-23T10:00:00Z'), chapter('b', 'public', null, '2026-10-23T11:00:00Z'), chapter('c', 'authenticated', null, '2026-10-23T12:00:00Z')]
  assert.equal(summarizeProgress(applyChapterAccess(rows, true)).resumeChapterSlug, 'b')
  rows[0].completedAt = rows[1].completedAt = rows[2].completedAt = '2026-10-23T13:00:00Z'
  const done = summarizeProgress(applyChapterAccess(rows, true))
  assert.equal(done.percent, 100); assert.equal(done.isCompleted, true); assert.equal(done.resumeChapterSlug, 'a')
  assert.deepEqual(summarizeProgress([]), { completedChapters: 0, totalChapters: 0, percent: 0, isCompleted: false, resumeChapterSlug: null })
})
