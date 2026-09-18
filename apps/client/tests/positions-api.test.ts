import { afterEach, describe, expect, it, vi } from 'vitest'
import { createPositionsAPI } from '../src/features/promotion/positions-api'
const position = () => ({ id: 'position-1', name: '本人社群', scene: 'group', status: 'ENABLED', isDefault: true, version: 1, createdAt: '2026-09-19T00:00:00Z', canConvert: false, channels: [{ channel: 'JD', readiness: 'WAITING_CONFIGURATION' }] })
const response = (items: unknown[] = [position()], nextCursor = '') => ({ statusCode: 200, data: { code: 0, data: { items, nextCursor } } })
describe('authenticated position choices', () => {
  afterEach(() => vi.unstubAllGlobals())
  it('uses authenticated GET with bounded timeout at the uni boundary', async () => {
    const request = vi.fn((options: { success: (data: unknown) => void }) => options.success(response()))
    vi.stubGlobal('uni', { request })
    expect((await createPositionsAPI(() => 'token').list()).items).toHaveLength(1)
    expect(request.mock.calls[0][0]).toMatchObject({ url: '/api/v1/promotion-positions?status=ENABLED&limit=20', method: 'GET', timeout: 10000, header: { Authorization: 'Bearer token' } })
  })
  it('does not expose upstream transport errors', async () => {
    await expect(createPositionsAPI(() => 'token', async () => { throw Error('secret-value') }).list()).rejects.toMatchObject({ kind: 'error', message: '推广位读取失败，请稍后重试' })
  })
  it('reads enabled own positions without owner or channel injection', async () => {
    const request = vi.fn(async () => response())
    const page = await createPositionsAPI(() => 'real-token', request).list('Y3Vyc29y')
    expect(request).toHaveBeenCalledWith('/api/v1/promotion-positions?status=ENABLED&limit=20&cursor=Y3Vyc29y', 'real-token')
    expect(page.items[0]).toEqual({ id: 'position-1', name: '本人社群', scene: 'group', isDefault: true, channels: [{ channel: 'JD', readiness: 'WAITING_CONFIGURATION' }] })
  })
  it.each([null, '', 'bad token', 'x'.repeat(8193)])('does not request without valid session %s', async token => {
    const request = vi.fn(async () => response())
    await expect(createPositionsAPI(() => token, request).list()).rejects.toMatchObject({ kind: 'login-required' })
    expect(request).not.toHaveBeenCalled()
  })
  it('rejects stale identity response', async () => {
    let token = 'first'; let resolve!: (value: ReturnType<typeof response>) => void
    const result = createPositionsAPI(() => token, () => new Promise(done => { resolve = done })).list()
    token = 'second'; resolve(response())
    await expect(result).rejects.toMatchObject({ kind: 'login-required' })
  })
  it.each([401, 403, 404, 503])('sanitizes HTTP %s', async statusCode => {
    await expect(createPositionsAPI(() => 'token', async () => ({ statusCode, data: { message: 'secret-url' } })).list()).rejects.toMatchObject({ kind: statusCode === 401 ? 'login-required' : statusCode === 404 ? 'unavailable' : 'error' })
  })
  it('strips private properties and copies channel data', async () => {
    const item = { ...position(), ownerUserId: 'other', externalPositionId: 'private' }
    const page = await createPositionsAPI(() => 'token', async () => response([item])).list()
    item.channels[0].readiness = 'UNAVAILABLE'
    expect(page.items[0]).not.toHaveProperty('ownerUserId')
    expect(page.items[0]).not.toHaveProperty('externalPositionId')
    expect(page.items[0].channels[0].readiness).toBe('WAITING_CONFIGURATION')
  })
  it.each([
    { status: 'DISABLED' }, { canConvert: true }, { id: '' }, { scene: '' }, { name: 'x'.repeat(81) },
    { id: 'x'.repeat(257) }, { name: '中'.repeat(81) }, { scene: 'bad\nscene' }, { isDefault: 'true' }, { channels: [{ channel: 'TB', readiness: 'READY' }] },
    { channels: [{ channel: 'JD', readiness: 'READY' }] }, { channels: [] },
    { channels: [{ channel: 'JD', readiness: 'WAITING_CONFIGURATION' }, { channel: 'JD', readiness: 'UNAVAILABLE' }] },
  ])('rejects malformed or unsupported choices %j', async patch => {
    await expect(createPositionsAPI(() => 'token', async () => response([{ ...position(), ...patch }])).list()).rejects.toMatchObject({ kind: 'error' })
  })
  it.each(['WAITING_CONFIGURATION', 'WAITING_VERIFICATION', 'UNAVAILABLE'])('preserves safe readiness %s without generation grant', async readiness => {
    const item = position(); item.channels[0].readiness = readiness
    expect((await createPositionsAPI(() => 'token', async () => response([item])).list()).items[0].channels[0].readiness).toBe(readiness)
  })
  it('rejects duplicate choices and empty cursor pages', async () => {
    for (const data of [response([position(), position()]), response([], 'YQ')]) {
      await expect(createPositionsAPI(() => 'token', async () => data).list()).rejects.toMatchObject({ kind: 'error' })
    }
  })
  it('rejects cursor injection before network', async () => {
    const request = vi.fn(async () => response())
    await expect(createPositionsAPI(() => 'token', request).list('x&owner=other')).rejects.toMatchObject({ kind: 'error' })
    expect(request).not.toHaveBeenCalled()
  })
  it('keeps valid empty directory empty', async () => {
    expect(await createPositionsAPI(() => 'token', async () => response([])).list()).toEqual({ items: [], nextCursor: '' })
  })
  it('accepts backend character limits without confusing material request byte limits', async () => {
    const item = { ...position(), id: 'x'.repeat(256), name: '中'.repeat(80), scene: '中'.repeat(80) }
    const page = await createPositionsAPI(() => 'token', async () => response([item])).list()
    expect(page.items[0].name).toBe('中'.repeat(80))
    expect(page.items[0].id).toHaveLength(256)
  })
  it.each([
    { code: 1, data: { items: [] } }, { code: 0, data: null },
    { code: 0, data: { items: 'bad' } }, { code: 0, data: { items: [position()], nextCursor: null } },
    { code: 0, data: { items: [position()], nextCursor: 'x&owner=other' } },
    { code: 0, data: { items: Array.from({ length: 21 }, (_, i) => ({ ...position(), id: 'p-' + i })) } },
  ])('rejects malformed directory envelopes %j', async data => {
    await expect(createPositionsAPI(() => 'token', async () => ({ statusCode: 200, data })).list()).rejects.toMatchObject({ kind: 'error' })
  })
})
