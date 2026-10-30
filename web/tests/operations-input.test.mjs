import test from 'node:test'
import assert from 'node:assert/strict'
import { randomUUID } from 'node:crypto'
import { emptyAd, parseAd, adState } from '../shared/advertising.ts'
import { analyticsPath, parseVisit, visitSource } from '../shared/analytics.ts'

test('manual advertisements require usable creative, placements and a valid delivery window', () => {
  const ad = {
    ...emptyAd(),
    name: 'Workshop',
    title: 'Public workshop',
    href: 'https://sponsor.example/?utm_source=learn',
    placements: ['learn_catalog', 'reference_footer'],
    enabled: true
  }
  assert.deepEqual(parseAd(ad), ad)
  for (const patch of [
    { name: '' },
    { href: 'javascript:alert(1)' },
    { imageUrl: 'http://example.com/ad.png' },
    { placements: [] },
    { placements: ['unknown'] },
    { placements: ['learn_catalog', 'learn_catalog'] },
    { weight: 0 },
    { weight: 101 },
    { lockVersion: -1 },
    { startsAt: '2027-01-02', endsAt: '2027-01-01' }
  ])
    assert.throws(() => parseAd({ ...ad, ...patch }))
  const now = Date.parse('2026-10-30T00:00:00Z')
  assert.equal(adState(ad, now), '展示中')
  assert.equal(adState({ ...ad, enabled: false }, now), '已停用')
  assert.equal(adState({ ...ad, startsAt: '2026-11-01T00:00:00Z' }, now), '待开始')
  assert.equal(adState({ ...ad, endsAt: '2026-10-30T00:00:00Z' }, now), '已结束')
})

test('analytics records only public paths and explicit campaign fields, without credentials or query strings', () => {
  for (const path of [
    '/',
    '/learn',
    '/learn/modules/module/intro',
    '/references/book',
    '/challenges/lab',
    '/auth'
  ])
    assert.equal(analyticsPath(path), path)
  for (const path of [
    '/admin',
    '/admin/users',
    '/api/auth/login',
    '/term',
    '/me',
    '/auth?token=private',
    '/learn#heading',
    '//evil.test',
    '/learn/../auth',
    '/learn\\evil'
  ])
    assert.throws(() => analyticsPath(path))
  const input = {
    id: randomUUID(),
    path: '/learn',
    referrerHost: 'www.google.com',
    source: '',
    medium: '',
    campaign: '',
    password: 'do-not-store'
  }
  assert.equal('password' in parseVisit(input), false)
  assert.equal(visitSource(parseVisit(input), 'microsb.de').channel, 'search')
  assert.equal(
    visitSource({ ...input, referrerHost: 'google.com.evil.test' }, 'microsb.de').channel,
    'referral'
  )
  assert.equal(
    visitSource({ ...input, referrerHost: 'microsb.de' }, 'microsb.de').channel,
    'direct'
  )
  assert.deepEqual(
    visitSource(
      { ...input, source: 'community', medium: 'post', campaign: 'launch' },
      'microsb.de'
    ),
    { source: 'community', medium: 'post', campaign: 'launch', channel: 'campaign' }
  )
  assert.throws(() =>
    parseVisit({ ...input, referrerHost: 'https://example.com/private?token=secret' })
  )
})
