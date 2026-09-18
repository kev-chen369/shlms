import { expect, it, vi, afterEach } from 'vitest'
import { createMaterialsAPI, type MaterialScope } from '../src/features/promotion/materials-api'

const id = '11111111-1111-4111-8111-111111111111'
const scope: MaterialScope = { platform: 'JD', type: 'PRODUCT', terminal: 'H5', positionId: 'p1', scene: 'home', cityCode: '310100', business: 'food' }
const card = { id, platform: 'JD', type: 'PRODUCT', title: '合成测试物料', endsAt: '2026-09-20T12:00:00Z', sourceUpdatedAt: '2026-09-18T10:00:00Z', ruleVersion: 'v1', region: { mode: 'NATIONWIDE' }, business: '', terminals: ['H5'] }
const ready = { allowed: true, reason: 'READY' }
const reply = (data: unknown) => ({ statusCode: 200, data: { code: 0, data } })
afterEach(() => vi.unstubAllGlobals())

it('sends only complete scope and supplied auth, strips internal and generation fields', async () => {
 const request = vi.fn(async () => reply({ items: [{ ...card, canonicalUrl: 'private', price: 1, canGenerate: true }], capability: ready, nextCursor: 'Y3Vyc29y' }))
 expect(await createMaterialsAPI(() => 'supplied.token', request).list(scope)).toEqual({ items: [card], capability: ready, nextCursor: 'Y3Vyc29y' })
 expect(request).toHaveBeenCalledExactlyOnceWith('/api/v1/promoter/materials?platform=JD&type=PRODUCT&terminal=H5&positionId=p1&scene=home&cityCode=310100&business=food&limit=20', 'supplied.token')
})
it('maps frontend platforms, reads activities without fabricated prices and supports omitted terminal cursor', async () => {
 const activity = { ...card, platform: 'MT', type: 'ACTIVITY', startsAt: '2026-09-18T00:00:00Z' }
 const request = vi.fn(async (_url: string, _token: string) => reply({ items: [activity], capability: ready }))
 expect((await createMaterialsAPI(() => 'token', request).list({ ...scope, platform: 'MEITUAN', type: 'ACTIVITY' }, 'YQ')).items).toEqual([activity])
 expect(request.mock.calls[0][0]).toContain('platform=MT&type=ACTIVITY')
 expect(request.mock.calls[0][0]).toContain('&cursor=YQ')
 const tb = { ...card, platform: 'TB' }
 expect((await createMaterialsAPI(() => 'token', async () => reply({ items: [tb], capability: ready })).list({ ...scope, platform: 'TAOBAO' })).items).toEqual([tb])
})
it('returns safe domain denials and matching detail, not link permission', async () => {
 const api = createMaterialsAPI(() => 'token', async () => reply({ capability: ready, availability: { available: true, reason: 'AVAILABLE' }, item: { ...card, evidenceRef: 'private' } }))
 expect(await api.detail(id, scope)).toEqual({ item: card, capability: ready, availability: { available: true, reason: 'AVAILABLE' } })
 expect(await createMaterialsAPI(() => 'token', async () => reply({ items: [], capability: { allowed: false, reason: 'UNCONFIGURED' } })).list(scope)).toEqual({ items: [], nextCursor: '', capability: { allowed: false, reason: 'UNCONFIGURED' } })
 expect(await createMaterialsAPI(() => 'token', async () => reply({ capability: ready, availability: { available: false, reason: 'EXPIRED' } })).detail(id, scope)).toEqual({ capability: ready, availability: { available: false, reason: 'EXPIRED' } })
})
it.each([null, '', 'bad token', 'token\r\nHeader:value'])('makes no request without usable session %s', async token => {
 const request = vi.fn(async () => reply({ items: [], capability: ready }))
 await expect(createMaterialsAPI(() => token, request).list(scope)).rejects.toMatchObject({ kind: 'login-required' })
 expect(request).not.toHaveBeenCalled()
})
it('discards an old identity response and uses a copied scope', async () => {
 let token = 'a', resolve!: (response: ReturnType<typeof reply>) => void
 const input = { ...scope }, api = createMaterialsAPI(() => token, () => new Promise(done => resolve = done))
 const first = api.list(input); input.platform = 'TAOBAO'; resolve(reply({ items: [card], capability: ready }))
 expect((await first).items).toEqual([card])
 const second = api.list(scope); token = 'b'; resolve(reply({ items: [card], capability: ready }))
 await expect(second).rejects.toMatchObject({ kind: 'login-required' })
})
it.each([{ platform: 'PDD' }, { platform: 'MEITUAN' }, { scene: '' }, { terminal: 'APP' }, { cityCode: null }, { business: false }, { positionId: '\ud800' }, { scene: '活动'.repeat(27) }, { ownerId: 'private' }])('rejects invalid or injected scope without request %j', async change => {
 const request = vi.fn(async () => reply({ items: [], capability: ready }))
 await expect(createMaterialsAPI(() => 'token', request).list({ ...scope, ...change } as MaterialScope)).rejects.toMatchObject({ kind: 'error' })
 expect(request).not.toHaveBeenCalled()
})
it.each([
 { ...card, id: '00000000-0000-0000-0000-000000000000' }, { ...card, platform: 'TB' }, { ...card, type: 'ACTIVITY' },
 { ...card, endsAt: '2026-02-30T00:00:00Z' }, { ...card, sourceUpdatedAt: 'bad' }, { ...card, title: '\n' },
 { ...card, terminals: ['WX_MINI'] }, { ...card, terminals: ['H5', 'H5'] }, { ...card, business: 'travel' },
 { ...card, region: { mode: 'CITIES', cityCodes: ['110100'] } }, { ...card, region: { mode: 'NATIONWIDE', cityCodes: ['310100'] } },
])('rejects malformed or out-of-scope cards %j', async item => {
 await expect(createMaterialsAPI(() => 'token', async () => reply({ items: [item], capability: ready })).list(scope)).rejects.toMatchObject({ kind: 'error' })
})
it.each([
 { items: [card, card], capability: ready }, { items: [], capability: ready, nextCursor: 'YQ' },
 { items: [card], capability: { allowed: false, reason: 'SUSPENDED' } }, { items: [], capability: { allowed: true, reason: 'SUSPENDED' } },
 { items: [], capability: { allowed: false, reason: 'FUTURE' } }, { items: [], capability: ready, nextCursor: null },
 { items: [], capability: ready, nextCursor: 'private/url' },
])('rejects contradictory pages and unsafe cursors %j', async data => {
 await expect(createMaterialsAPI(() => 'token', async () => reply(data)).list(scope)).rejects.toMatchObject({ kind: 'error' })
})
it.each([
 { capability: ready, availability: { available: true, reason: 'AVAILABLE' } },
 { capability: ready, availability: { available: false, reason: 'EXPIRED' }, item: card },
 { capability: { allowed: false, reason: 'SUSPENDED' }, availability: { available: true, reason: 'AVAILABLE' }, item: card },
 { capability: ready, availability: { available: true, reason: 'AVAILABLE' }, item: { ...card, id: '22222222-2222-4222-8222-222222222222' } },
])('rejects conflicting or mismatched detail %j', async data => {
 await expect(createMaterialsAPI(() => 'token', async () => reply(data)).detail(id, scope)).rejects.toMatchObject({ kind: 'error' })
})
it.each([[401, 'login-required'], [404, 'unavailable'], [503, 'error']] as const)('maps HTTP %s without exposing backend error', async (statusCode, kind) => {
 await expect(createMaterialsAPI(() => 'token', async () => ({ statusCode, data: { message: 'private-secret' } })).list(scope)).rejects.toMatchObject({ kind })
})
it('normalizes failed transport and uses uni GET with auth', async () => {
 await expect(createMaterialsAPI(() => 'token', async () => { throw new Error('private-secret') }).list(scope)).rejects.toMatchObject({ message: '推广物料读取失败，请稍后重试' })
 const request = vi.fn((options: any) => options.success(reply({ items: [], capability: ready })))
 vi.stubGlobal('uni', { request })
 await createMaterialsAPI(() => 'token').list(scope)
 expect(request.mock.calls[0][0]).toMatchObject({ method: 'GET', timeout: 10000, header: { Authorization: 'Bearer token' } })
})

it('uses an exact scope on detail and accepts a capability denial with no item', async () => {
 const request = vi.fn(async (_url: string, _token: string) => reply({ capability: { allowed: false, reason: 'VERIFICATION_EXPIRED' }, availability: { available: false, reason: 'CAPABILITY_UNAVAILABLE' } }))
 expect(await createMaterialsAPI(() => 'token', request).detail(id, scope)).toEqual({ capability: { allowed: false, reason: 'VERIFICATION_EXPIRED' }, availability: { available: false, reason: 'CAPABILITY_UNAVAILABLE' } })
 expect(request).toHaveBeenCalledExactlyOnceWith('/api/v1/promoter/materials/11111111-1111-4111-8111-111111111111?platform=JD&type=PRODUCT&terminal=H5&positionId=p1&scene=home&cityCode=310100&business=food', 'token')
})
it('copies safe arrays and preserves genuine nanosecond windows without client clock grants', async () => {
 const item = { ...card, type: 'ACTIVITY', startsAt: '2026-09-18T10:00:00.000000001Z', endsAt: '2026-09-18T10:00:00.000000002Z', region: { mode: 'CITIES', cityCodes: ['310100'] }, terminals: ['H5'] }
 const api = createMaterialsAPI(() => 'token', async () => reply({ items: [item], capability: ready }))
 const page = await api.list({ ...scope, type: 'ACTIVITY' })
 item.region.cityCodes[0] = '110100'; item.terminals[0] = 'WX_MINI'
 expect(page.items[0].region.cityCodes).toEqual(['310100']); expect(page.items[0].terminals).toEqual(['H5'])
 await expect(createMaterialsAPI(() => 'token', async () => reply({ items: [{ ...item, region: card.region, terminals: ['H5'], endsAt: item.startsAt }], capability: ready })).list({ ...scope, type: 'ACTIVITY' })).rejects.toMatchObject({ kind: 'error' })
})
it('rejects invalid identifiers, cursors and success envelopes without leaking partial data', async () => {
 const request = vi.fn(async () => reply({ items: [card], capability: ready }))
 const api = createMaterialsAPI(() => 'token', request)
 await expect(api.detail('private-source-id', scope)).rejects.toMatchObject({ kind: 'error' })
 await expect(api.list(scope, 'unsafe/cursor')).rejects.toMatchObject({ kind: 'error' })
 expect(request).not.toHaveBeenCalled()
 for (const data of [{ code: '0', data: { items: [card], capability: ready } }, { code: 0, data: null }, { code: 0, data: { items: Array(21).fill(card), capability: ready } }]) {
  await expect(createMaterialsAPI(() => 'token', async () => ({ statusCode: 200, data })).list(scope)).rejects.toMatchObject({ kind: 'error' })
 }
})

it.each(['toString', 'constructor', '__proto__'])('rejects inherited platform map keys %s without a request', async platform => {
 const request = vi.fn(async () => reply({ items: [], capability: ready }))
 await expect(createMaterialsAPI(() => 'token', request).list({ ...scope, platform } as MaterialScope)).rejects.toMatchObject({ kind: 'error' })
 expect(request).not.toHaveBeenCalled()
})

it.each([{ platform: ['JD'] }, { platform: new String('JD') }])('rejects coercible non-string platforms without a request', async change => {
 const request = vi.fn(async () => reply({ items: [], capability: ready }))
 await expect(createMaterialsAPI(() => 'token', request).list({ ...scope, ...change } as unknown as MaterialScope)).rejects.toMatchObject({ kind: 'error' })
 expect(request).not.toHaveBeenCalled()
})
