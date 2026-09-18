import { afterEach, expect, it, vi } from 'vitest'
import { createPromoterDashboardAPI, type DashboardFilter } from '../src/features/promotion/dashboard-api'
const counts = { timeZone: 'Asia/Shanghai', from: '2026-09-01T00:00:00+08:00', toExclusive: '2026-09-03T00:00:00+08:00', asOf: '2026-09-18T12:00:00.123456789Z', successfulLinks: 5, copyReports: 8, validOrders: 2 }
const filter = { from: '2026-09-01', to: '2026-09-02', channel: 'JD', positionId: 'p1' }
const reply = (data: unknown) => ({ statusCode: 200, data: { code: 0, message: 'success', data } })
afterEach(() => vi.unstubAllGlobals())
it('reads natural-day dashboard with only approved filter query and explicitly supplied identity', async () => {
  const request = vi.fn(async () => reply(counts))
  expect(await createPromoterDashboardAPI(() => 'provided.identity', request).get(filter)).toEqual(counts)
  expect(request).toHaveBeenCalledExactlyOnceWith('/api/v1/promoter/dashboard?from=2026-09-01&to=2026-09-02&channel=JD&positionId=p1', 'provided.identity')
})
it('whitelists counts and metadata without private or financial derived fields', async () => {
  const api = createPromoterDashboardAPI(() => 'provided', async () => reply({ ...counts, ownerUserId: 'private', conversionRate: 0.5, income: 9999, raw: 'secret' }))
  expect(await api.get(filter)).toEqual(counts)
})
it('accepts valid zero counts, not a fabricated failure or earnings total', async () => {
  const zero = { ...counts, successfulLinks: 0, copyReports: 0, validOrders: 0 }
  expect(await createPromoterDashboardAPI(() => 'provided', async () => reply(zero)).get(filter)).toEqual(zero)
})
it('reads default thirty-day range without computing dates from client clock', async () => {
  const data = { ...counts, from: '2026-08-20T00:00:00+08:00', toExclusive: '2026-09-19T00:00:00+08:00' }
  const request = vi.fn(async () => reply(data)); expect(await createPromoterDashboardAPI(() => 'provided', request).get()).toEqual(data)
  expect(request).toHaveBeenCalledExactlyOnceWith('/api/v1/promoter/dashboard', 'provided')
})
it('binds returned boundaries to the request filter snapshot, not caller mutations', async () => {
  let resolve!: (value: ReturnType<typeof reply>) => void
  const draft = { ...filter }, pending = createPromoterDashboardAPI(() => 'provided', () => new Promise(done => resolve = done)).get(draft)
  draft.from = '2026-09-02'; resolve(reply(counts)); expect(await pending).toEqual(counts)
})
it('encodes position and never places provided token or owner in URL', async () => {
  const request = vi.fn(async () => reply(counts)); await createPromoterDashboardAPI(() => 'provided.identity', request).get({ ...filter, positionId: '渠道 & 位/1' })
  expect(request).toHaveBeenCalledExactlyOnceWith('/api/v1/promoter/dashboard?from=2026-09-01&to=2026-09-02&channel=JD&positionId=%E6%B8%A0%E9%81%93%20%26%20%E4%BD%8D%2F1','provided.identity')
})
it.each([null, '', 'bad token', 'token\r\nHeader:val'])('no usable provided session %s means no outbound request', async token => {
  const request = vi.fn(async () => reply(counts)); await expect(createPromoterDashboardAPI(() => token, request).get(filter)).rejects.toMatchObject({ kind: 'login-required' }); expect(request).not.toHaveBeenCalled()
})
it('sanitizes a throwing identity owner without making a request', async () => {
  const request = vi.fn(async () => reply(counts)); await expect(createPromoterDashboardAPI(() => { throw Error('private identity') }, request).get(filter)).rejects.toMatchObject({ kind: 'login-required' }); expect(request).not.toHaveBeenCalled()
})
it.each([null, [], { from: false }, { channel: 0 }, { positionId: null }, { from: '2026-02-30' }, { from: '2026-9-1' }, { from: '2026-09-01T00:00:00Z' }, { from: '2026-09-02', to: '2026-09-01' }, { from: '2025-09-01', to: '2026-09-02' }, { channel: 'PDD' }, { positionId: '\n' }, { positionId: '\ud800' }, { positionId: 'p'.repeat(129) }, { ownerUserId: 'someone-else' }].map(value => [value]))('rejects malformed or unapproved filters %j before reading', async value => {
  const request = vi.fn(async () => reply(counts)); await expect(createPromoterDashboardAPI(() => 'provided', request).get(value as DashboardFilter)).rejects.toMatchObject({ kind: 'error' }); expect(request).not.toHaveBeenCalled()
})
it.each([
  { ...counts, timeZone: 'UTC' }, { ...counts, from: '2026-02-30T00:00:00+08:00' }, { ...counts, from: '2026-09-01T01:00:00+08:00' }, { ...counts, from: '2026-09-01T00:00:00.000000001+08:00' },
  { ...counts, toExclusive: counts.from }, { ...counts, toExclusive: '2027-10-01T00:00:00+08:00' }, { ...counts, from: '2026-09-02T00:00:00+08:00' }, { ...counts, toExclusive: '2026-09-02T00:00:00+08:00' },
  { ...counts, asOf: 'invalid' }, { ...counts, asOf: '2026-09-18T24:00:00Z' }, { ...counts, successfulLinks: -1 }, { ...counts, copyReports: 0.5 }, { ...counts, validOrders: Number.MAX_SAFE_INTEGER + 1 }, { ...counts, validOrders: '2' }, { ...counts, copyReports: null }, null,
])('rejects invalid boundaries, snapshot metadata or untrustworthy counts %j', async data => {
  await expect(createPromoterDashboardAPI(() => 'provided', async () => reply(data)).get(filter)).rejects.toMatchObject({ kind: 'error' })
})
it('rejects a non-thirty-day response to default range rather than presenting a mislabeled default', async () => {
  await expect(createPromoterDashboardAPI(() => 'provided', async () => reply(counts)).get()).rejects.toMatchObject({ kind: 'error' })
})
it('accepts equivalent UTC representations of Shanghai midnight', async () => {
  const data = { ...counts, from: '2026-08-31T16:00:00Z', toExclusive: '2026-09-02T16:00:00Z' }
  expect(await createPromoterDashboardAPI(() => 'provided', async () => reply(data)).get(filter)).toEqual(data)
})
it.each([null, 'other-identity'])('rejects a late snapshot after exit or identity change %s', async next => {
  let session: string | null = 'original', resolve!: (value: ReturnType<typeof reply>) => void
  const pending = createPromoterDashboardAPI(() => session, () => new Promise(done => resolve = done)).get(filter).catch(error => error)
  session = next; resolve(reply(counts)); expect(await pending).toMatchObject({ kind: 'login-required' })
})
it.each([[401, 'UNAUTHORIZED', 'login-required'], [404, 'MISSING', 'unavailable'], [503, 'DASHBOARD_UNAVAILABLE', 'unavailable'], [500, 'INTERNAL', 'error']] as const)('sanitizes HTTP %s into %s/%s', async (statusCode, code, kind) => {
  const result = await createPromoterDashboardAPI(() => 'provided', async () => ({ statusCode, data: { code, message: 'private secret' } })).get(filter).catch(error => error)
  expect(result).toMatchObject({ kind }); expect(result.message).not.toContain('private secret')
})
it('sanitizes malformed success envelope and transport failures', async () => {
  await expect(createPromoterDashboardAPI(() => 'provided', async () => ({ statusCode: 200, data: { code: 1, data: counts, message: 'private secret' } })).get(filter)).rejects.toMatchObject({ kind: 'error' })
  await expect(createPromoterDashboardAPI(() => 'provided', async () => { throw Error('private network') }).get(filter)).rejects.not.toThrow('private network')
})
it('default transport uses a bounded uni GET with provided Bearer header only', async () => {
  const request = vi.fn(options => { options.success(reply(counts)); return {} }); vi.stubGlobal('uni', { request })
  expect(await createPromoterDashboardAPI(() => 'provided.identity').get(filter)).toEqual(counts)
  expect(request.mock.calls[0][0]).toMatchObject({ url: '/api/v1/promoter/dashboard?from=2026-09-01&to=2026-09-02&channel=JD&positionId=p1', method: 'GET', timeout: 10000, header: { Authorization: 'Bearer provided.identity' } })
})
it('accepts exactly 366 inclusive natural days but rejects 367 before requesting', async () => {
  const data = { ...counts, from: '2023-01-01T00:00:00+08:00', toExclusive: '2024-01-02T00:00:00+08:00' }
  const request = vi.fn(async () => reply(data)), api = createPromoterDashboardAPI(() => 'provided', request)
  expect(await api.get({ from: '2023-01-01', to: '2024-01-01' })).toEqual(data)
  await expect(api.get({ from: '2023-01-01', to: '2024-01-02' })).rejects.toMatchObject({ kind: 'error' }); expect(request).toHaveBeenCalledTimes(1)
})
it('allows copy report count greater than links or orders without inventing funnel ratios', async () => {
  const data = { ...counts, successfulLinks: 0, copyReports: 100, validOrders: 20 }
  expect(await createPromoterDashboardAPI(() => 'provided', async () => reply(data)).get(filter)).toEqual(data)
})
it.each([{ from: '2026-09-01' }, { to: '2026-09-02' }])('accepts valid one-sided filter %j and validates only its requested boundary', async value => {
  const request = vi.fn(async (_url: string) => reply(counts)); expect(await createPromoterDashboardAPI(() => 'provided', request).get(value)).toEqual(counts)
  expect(request.mock.calls[0][0]).toBe(value.from ? '/api/v1/promoter/dashboard?from=2026-09-01' : '/api/v1/promoter/dashboard?to=2026-09-02')
})
it('default transport sanitizes uni failure and exit during transport failure', async () => {
  let provided: string | null = 'provided'
  const request = vi.fn(options => { provided = null; options.fail({ errMsg: 'private network' }); return {} }); vi.stubGlobal('uni', { request })
  await expect(createPromoterDashboardAPI(() => provided).get(filter)).rejects.toMatchObject({ kind: 'login-required' })
})
it('rejects a structurally malformed transport response without leaking runtime errors', async () => {
  const api = createPromoterDashboardAPI(() => 'provided', async () => null as never)
  await expect(api.get(filter)).rejects.toMatchObject({ kind: 'error' })
})
it.each([
  ['1991-07-01', '1991-07-01T00:00:00+09:00', '1991-07-02T00:00:00+09:00'],
  ['1991-04-14', '1991-04-14T00:00:00+08:00', '1991-04-15T00:00:00+09:00'],
  ['1991-09-15', '1991-09-15T00:00:00+09:00', '1991-09-16T00:00:00+08:00'],
])('accepts backend Shanghai natural day %s including historical DST duration', async (date, from, toExclusive) => {
  const data = { ...counts, from, toExclusive }
  expect(await createPromoterDashboardAPI(() => 'provided', async () => reply(data)).get({ from: date, to: date })).toEqual(data)
})
it('rejects fixed +08 midnight during actual Shanghai +09 summer time', async () => {
  const data = { ...counts, from: '1991-07-01T00:00:00+08:00', toExclusive: '1991-07-02T00:00:00+08:00' }
  await expect(createPromoterDashboardAPI(() => 'provided', async () => reply(data)).get({ from: '1991-07-01', to: '1991-07-01' })).rejects.toMatchObject({ kind: 'error' })
})
it('accepts historical Shanghai midnight serialized in UTC without losing second-level LMT offset', async () => {
  const data = { ...counts, from: '1899-12-31T15:54:17Z', toExclusive: '1900-01-01T15:54:17Z' }
  expect(await createPromoterDashboardAPI(() => 'provided', async () => reply(data)).get({ from: '1900-01-01', to: '1900-01-01' })).toEqual(data)
})
it('fails closed if runtime has no Shanghai timezone support instead of assuming fixed UTC+8', async () => {
  vi.stubGlobal('Intl', undefined)
  await expect(createPromoterDashboardAPI(() => 'provided', async () => reply(counts)).get(filter)).rejects.toMatchObject({ kind: 'unavailable' })
})
it('accepts thirty default calendar days spanning DST rather than requiring thirty 24-hour durations', async () => {
  const data = { ...counts, from: '1991-04-14T00:00:00+08:00', toExclusive: '1991-05-14T00:00:00+09:00', asOf: '1991-05-13T12:00:00+09:00' }
  expect(await createPromoterDashboardAPI(() => 'provided', async () => reply(data)).get()).toEqual(data)
})
