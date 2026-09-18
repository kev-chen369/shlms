import { afterEach, expect, it, vi } from 'vitest'
import { createPromoterOrdersAPI, type OrderFilter } from '../src/features/promotion/orders-api'

const id = '11111111-1111-1111-1111-111111111111'
const order = { id, channel: 'JD', maskedOrderId: '****1234', orderStatus: 'PAID', positionId: 'p1', attributionMethod: 'SUB_ID', attributedAt: '2026-09-18T12:00:00Z', orderOccurredAt: '2026-09-18T10:00:00Z', statusAt: '2026-09-18T11:00:00Z' }
const history = [{ previousStatus: 'CREATED', status: 'PAID', occurredAt: '2026-09-18T11:00:00Z', projectedAt: '2026-09-18T11:01:00Z' }]
const reply = (data: unknown) => ({ statusCode: 200, data: { code: 0, message: 'success', data } })
afterEach(() => vi.unstubAllGlobals())
it('reads filtered promoter orders without submitting an owner or earning claim', async () => {
  const request = vi.fn(async () => reply({ items: [order], nextCursor: 'bmV4dA' }))
  const page = await createPromoterOrdersAPI(() => 'supplied.token', request).list({ channel: 'JD', positionId: 'p1', orderStatus: 'PAID', from: '2026-09-18T00:00:00+08:00', to: '2026-09-19T00:00:00+08:00' }, 'Y3Vyc29y')
  expect(page).toEqual({ items: [order], nextCursor: 'bmV4dA' })
  expect(request).toHaveBeenCalledExactlyOnceWith('/api/v1/promoter/orders?channel=JD&positionId=p1&orderStatus=PAID&from=2026-09-18T00%3A00%3A00%2B08%3A00&to=2026-09-19T00%3A00%3A00%2B08%3A00&limit=20&cursor=Y3Vyc29y', 'supplied.token')
})
it('reads matching public detail and strips private and unapproved financial fields', async () => {
  const request = vi.fn(async () => reply({ ...order, buyerPhone: 'private', externalOrderId: 'private', expectedIncome: 9999, history: [{ ...history[0], evidenceId: 'private' }], refundEventCount: 2 }))
  expect(await createPromoterOrdersAPI(() => 'token', request).detail(id)).toEqual({ ...order, history, refundEventCount: 2 })
  expect(request).toHaveBeenCalledExactlyOnceWith(`/api/v1/promoter/orders/${id}`, 'token')
})
it('strips extra private fields from order summaries', async () => {
  expect(await createPromoterOrdersAPI(() => 'token', async () => reply({ items: [{ ...order, buyerId: 'private', amount: 123 }], nextCursor: '' })).list()).toEqual({ items: [order], nextCursor: '' })
})
it('binds response checks to the filter snapshot actually sent, not later caller mutations', async () => {
  let resolve!: (value: ReturnType<typeof reply>) => void
  const filter = { channel: 'JD' }, api = createPromoterOrdersAPI(() => 'token', () => new Promise(done => resolve = done))
  const pending = api.list(filter); filter.channel = 'TB'; resolve(reply({ items: [order], nextCursor: '' }))
  expect(await pending).toEqual({ items: [order], nextCursor: '' })
})
it.each([{ from: '2026-09-18T24:00:00Z' }, { positionId: '\ud800' }])('rejects non-RFC3339 dates or unencodable filters without leaking errors', async filter => {
  const request = vi.fn(async () => reply({ items: [], nextCursor: '' }))
  await expect(createPromoterOrdersAPI(() => 'token', request).list(filter)).rejects.toMatchObject({ kind: 'error' })
  expect(request).not.toHaveBeenCalled()
})
it.each([{ channel: 0 }, { positionId: false }, { orderStatus: null }, { from: 0 }])('does not silently turn malformed filters into an unrestricted query', async filter => {
  const request = vi.fn(async () => reply({ items: [], nextCursor: '' }))
  await expect(createPromoterOrdersAPI(() => 'token', request).list(filter as unknown as OrderFilter)).rejects.toMatchObject({ kind: 'error' })
  expect(request).not.toHaveBeenCalled()
})
it('accepts valid backend RFC3339 nanosecond ranges without rounding them into an empty range', async () => {
  const preciseOrder = { ...order, orderOccurredAt: '2026-09-18T10:00:00.000000002Z' }
  const api = createPromoterOrdersAPI(() => 'token', async () => reply({ items: [preciseOrder], nextCursor: '' }))
  expect((await api.list({ from: '2026-09-18T10:00:00.000000001Z', to: '2026-09-18T10:00:00.000000003Z' })).items).toEqual([preciseOrder])
  await expect(api.list({ from: '2026-09-18T10:00:00.000000001Z', to: '2026-09-18T10:00:00.000000002Z' })).rejects.toMatchObject({ kind: 'error' })
})
it.each([null, '', 'token\r\nHeader:value', 'not a token'])('does not make an order request without a usable supplied session', async token => {
  const request = vi.fn(async () => reply({ items: [], nextCursor: '' })), api = createPromoterOrdersAPI(() => token, request)
  await expect(api.list()).rejects.toMatchObject({ kind: 'login-required' })
  await expect(api.detail(id)).rejects.toMatchObject({ kind: 'login-required' })
  expect(request).not.toHaveBeenCalled()
})
it('discards a completed response if the supplied session changed while awaiting it', async () => {
  let token: string | null = 'user-a', resolve!: (value: ReturnType<typeof reply>) => void
  const api = createPromoterOrdersAPI(() => token, () => new Promise(done => resolve = done))
  const pending = api.list().catch(error => error)
  token = 'user-b'; if (resolve) resolve(reply({ items: [order], nextCursor: '' }))
  expect(await pending).toMatchObject({ kind: 'login-required' })
})
it.each([
  { items: [{ ...order, id: 'channel-order-123456789' }], nextCursor: '' },
  { items: [{ ...order, channel: ['JD'] }], nextCursor: '' },
  { items: [{ ...order, maskedOrderId: 'raw-full-order-number' }], nextCursor: '' },
  { items: [{ ...order, orderStatus: 'UNKNOWN' }], nextCursor: '' },
  { items: [{ ...order, attributionMethod: 'GUESS' }], nextCursor: '' },
  { items: [{ ...order, positionId: '' }], nextCursor: '' },
  { items: [{ ...order, attributedAt: 'invalid' }], nextCursor: '' },
  { items: [{ ...order, orderOccurredAt: '2026-02-30T00:00:00Z' }], nextCursor: '' },
  { items: [order, order], nextCursor: '' },
  { items: [], nextCursor: 'next' },
  { items: [order], nextCursor: 'raw/private?cursor' },
  { items: null, nextCursor: '' },
])('rejects malformed or potentially unmasked orders', async data => {
  await expect(createPromoterOrdersAPI(() => 'token', async () => reply(data)).list()).rejects.toMatchObject({ kind: 'error' })
})
it.each([{ channel: 'TB' }, { positionId: 'p2' }, { orderStatus: 'CREATED' }, { to: '2026-09-18T10:00:00Z' }, { from: '2026-09-18T10:00:01Z' }])('rejects an order outside requested filters', async filter => {
  await expect(createPromoterOrdersAPI(() => 'token', async () => reply({ items: [order], nextCursor: '' })).list(filter)).rejects.toMatchObject({ kind: 'error' })
})
it.each([
  { channel: 'PDD' }, { positionId: '\n' }, { from: 'yesterday' }, { from: '2026-09-19T00:00:00Z', to: '2026-09-18T00:00:00Z' }, { orderStatus: 'AVAILABLE' },
])('rejects invalid query context before making a request', async filter => {
  const request = vi.fn(async () => reply({ items: [], nextCursor: '' }))
  await expect(createPromoterOrdersAPI(() => 'token', request).list(filter)).rejects.toMatchObject({ kind: 'error' })
  expect(request).not.toHaveBeenCalled()
})
it.each([
  { ...order, id: '22222222-2222-2222-2222-222222222222', history, refundEventCount: 0 },
  { ...order, history: null, refundEventCount: 0 },
  { ...order, history: [{ ...history[0], status: 'UNKNOWN' }], refundEventCount: 0 },
  { ...order, history: [{ ...history[0], projectedAt: 'invalid' }], refundEventCount: 0 },
  { ...order, history, refundEventCount: -1 },
])('rejects wrong public detail or malformed history', async data => {
  await expect(createPromoterOrdersAPI(() => 'token', async () => reply(data)).detail(id)).rejects.toMatchObject({ kind: 'error' })
})
it.each([[401, 'UNAUTHORIZED', 'login-required'], [404, 'ORDER_NOT_FOUND', 'not-found'], [404, 'MISSING', 'unavailable'], [503, 'INTERNAL', 'error']] as const)('classifies HTTP %s without leaking private errors', async (statusCode, code, kind) => {
  const api = createPromoterOrdersAPI(() => 'token', async () => ({ statusCode, data: { code, message: 'private error' } }))
  await expect(api.detail(id)).rejects.toMatchObject({ kind }); await expect(api.detail(id)).rejects.not.toThrow('private error')
})
it('uses a bounded uni GET and only the supplied bearer token, never URL identity', async () => {
  const request = vi.fn(options => { options.success(reply({ items: [], nextCursor: '' })); return {} }); vi.stubGlobal('uni', { request })
  expect(await createPromoterOrdersAPI(() => 'provided.token').list()).toEqual({ items: [], nextCursor: '' })
  expect(request.mock.calls[0][0]).toMatchObject({ url: '/api/v1/promoter/orders?limit=20', method: 'GET', timeout: 10000, header: { Authorization: 'Bearer provided.token' } })
})
