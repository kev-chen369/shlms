import { ref, effectScope } from 'vue'
import { expect, it, vi } from 'vitest'
import { createMaterialsModel } from '../src/features/promotion/materials-model'
import type { MaterialRequest, MaterialScope } from '../src/features/promotion/materials-api'

const id = '11111111-1111-4111-8111-111111111111', other = '22222222-2222-4222-8222-222222222222'
const scope: MaterialScope = { platform: 'JD', type: 'PRODUCT', terminal: 'H5', positionId: 'p1', scene: 'home', cityCode: '310100', business: 'food' }
const card = { id, platform: 'JD', type: 'PRODUCT', title: '测试物料', endsAt: '2026-09-20T00:00:00Z', sourceUpdatedAt: '2026-09-18T00:00:00Z', ruleVersion: 'v1', region: { mode: 'NATIONWIDE' }, business: '', terminals: ['H5'] }
const ready = { allowed: true, reason: 'READY' }
const page = (items: unknown[] = [card], nextCursor = 'YQ') => ({ items, nextCursor, capability: ready })
const detail = (item = card) => ({ item, capability: ready, availability: { available: true, reason: 'AVAILABLE' } })
function harness(token: string | null = 'user-a') {
 const session = ref(token), pending: { url: string; token: string; resolve: (response: Awaited<ReturnType<MaterialRequest>>) => void }[] = []
 const request = vi.fn((url: string, token: string) => new Promise<Awaited<ReturnType<MaterialRequest>>>(resolve => pending.push({ url, token, resolve })))
 const model = createMaterialsModel(session, request)
 const complete = (index: number, data: unknown, statusCode = 200) => pending[index].resolve({ statusCode, data: statusCode === 200 ? { code: 0, data } : data })
 const settle = async () => { for (let i = 0; i < 12; i++) await Promise.resolve() }
 const loaded = async () => { model.setScope(scope); complete(0, page()); await settle() }
 return { session, model, pending, request, complete, settle, loaded }
}
it('requires a supplied identity and explicit position before requesting anything', async () => {
 const a = harness(null); a.model.setScope(scope); await a.model.refresh(); await a.model.loadMore(); await a.model.openDetail(id)
 expect(a.model.state.status).toBe('login-required'); expect(a.request).not.toHaveBeenCalled()
 const b = harness(); await b.model.refresh(); expect(b.model.state.status).toBe('position-required'); expect(b.request).not.toHaveBeenCalled()
})
it('distinguishes empty, blocked and safe transport errors without fake materials', async () => {
 const h = harness(); await h.loaded(); expect(h.model.state.items).toEqual([card])
 h.model.refresh(); expect(h.model.state.items).toEqual([]); h.complete(1, page([], '')); await h.settle(); expect(h.model.state.status).toBe('empty')
 h.model.refresh(); h.complete(2, { items: [], capability: { allowed: false, reason: 'UNCONFIGURED' } }); await h.settle()
 expect(h.model.state.status).toBe('blocked'); expect(h.model.state.reason).toBe('UNCONFIGURED'); expect(h.model.state.cursor).toBe('')
 h.model.refresh(); h.complete(3, { message: 'private-secret' }, 503); await h.settle(); expect(h.model.state.status).toBe('error'); expect(h.model.state.error).not.toContain('private-secret')
})
it('clears position, detail, cards and old pending pages on platform/type switch', async () => {
 const h = harness(); await h.loaded(); h.model.openDetail(id); h.model.loadMore()
 h.model.selectPlatform('MEITUAN')
 expect(h.model.state.scope).toBeNull(); expect(h.model.state.type).toBe('ACTIVITY'); expect(h.model.state.items).toEqual([]); expect(h.model.state.detailId).toBe(''); expect(h.model.state.status).toBe('position-required')
 h.complete(1, detail()); h.complete(2, page([{ ...card, id: other }], '')); await h.settle(); expect(h.model.state.items).toEqual([])
 h.model.setScope({ ...scope, platform: 'MEITUAN', type: 'ACTIVITY', positionId: 'mt-p' }); h.complete(3, page([{ ...card, platform: 'MT', type: 'ACTIVITY', startsAt: '2026-09-18T00:00:00Z' }], '')); await h.settle()
 expect(h.model.state.status).toBe('ready'); h.model.selectPlatform('JD'); h.model.selectType('ACTIVITY'); expect(h.model.state.scope).toBeNull(); expect(h.model.state.items).toEqual([])
})
it.each([{ positionId: 'p2' }, { scene: 'group' }, { cityCode: '110100' }, { business: 'travel' }, { terminal: 'WX_MINI' }] as Partial<MaterialScope>[])('copies and replaces full scope, ignoring old responses %j', async change => {
 const h = harness(); h.model.setScope(scope)
 const next = { ...scope, ...change }; h.model.setScope(next); next.positionId = 'mutated'
 expect(h.model.state.scope?.positionId).toBe(change.positionId ?? 'p1')
 h.complete(0, page()); await h.settle(); expect(h.model.state.items).toEqual([])
 h.complete(1, page([{ ...card, terminals: [change.terminal ?? 'H5'] }], '')); await h.settle(); expect(h.model.state.status).toBe('ready')
})
it('does not reload identical scope or unsupported selections or mismatched scope', async () => {
 const h = harness(); h.model.setScope(scope); h.model.setScope({ ...scope }); expect(h.pending).toHaveLength(1)
 h.model.selectPlatform('PDD'); await h.model.refresh(); expect(h.model.state.status).toBe('unavailable'); expect(h.pending).toHaveLength(1)
 h.model.selectPlatform('MEITUAN'); h.model.selectType('PRODUCT'); h.model.setScope({ ...scope, platform: 'MEITUAN' }); expect(h.pending).toHaveLength(1)
 h.model.selectPlatform('JD'); h.model.setScope({ ...scope, platform: 'TAOBAO' }); expect(h.model.state.status).toBe('error'); expect(h.pending).toHaveLength(1)
})
it('keeps valid cards/cursor on page error, prevents parallel loads and permits retry', async () => {
 const h = harness(); await h.loaded(); h.model.loadMore(); h.model.loadMore(); expect(h.pending).toHaveLength(2)
 h.complete(1, {}, 503); await h.settle(); expect(h.model.state.items).toEqual([card]); expect(h.model.state.cursor).toBe('YQ'); expect(h.model.state.pageError).not.toBe('')
 h.model.loadMore(); h.complete(2, page([{ ...card, id: other }], '')); await h.settle(); expect(h.model.state.items.map(c => c.id)).toEqual([id, other]); expect(h.model.state.pageError).toBe('')
})
it.each(['duplicate', 'loop'])('rejects %s page and preserves retry state', async kind => {
 const h = harness(); await h.loaded(); h.model.loadMore(); h.complete(1, page([{ ...card, id: kind === 'duplicate' ? id : other }], kind === 'loop' ? 'YQ' : '')); await h.settle()
 expect(h.model.state.items).toEqual([card]); expect(h.model.state.cursor).toBe('YQ'); expect(h.model.state.pageError).not.toBe('')
})
it.each(['more', 'detail'])('clears all catalog data on %s capability denial, preventing stale resurrection', async action => {
 const h = harness(); await h.loaded(); h.model.loadMore(); h.model.openDetail(id)
 const cap = { allowed: false, reason: 'SUSPENDED' }
 h.complete(action === 'more' ? 1 : 2, action === 'more' ? { items: [], capability: cap } : { capability: cap, availability: { available: false, reason: 'CAPABILITY_UNAVAILABLE' } }); await h.settle()
 h.complete(action === 'more' ? 2 : 1, action === 'more' ? detail() : page([{ ...card, id: other }], '')); await h.settle()
 expect(h.model.state.status).toBe('blocked'); expect(h.model.state.reason).toBe('SUSPENDED'); expect(h.model.state.items).toEqual([]); expect(h.model.state.detail).toBeNull(); expect(h.model.state.cursor).toBe('')
})
it('removes expired detail and cancels pages that could reintroduce it', async () => {
 const h = harness(); await h.loaded(); h.model.loadMore(); h.model.openDetail(id)
 h.complete(2, { capability: ready, availability: { available: false, reason: 'EXPIRED' } }); await h.settle(); h.complete(1, page([card], '')); await h.settle()
 expect(h.model.state.items).toEqual([]); expect(h.model.state.detailStatus).toBe('not-found'); expect(h.model.state.detailReason).toBe('EXPIRED'); expect(h.model.state.cursor).toBe(''); expect(h.model.state.notice).not.toBe('')
})
it('fresh detail replaces summary, cancels prior page and clears cursor for refresh', async () => {
 const h = harness(); await h.loaded(); h.model.loadMore(); h.model.openDetail(id)
 h.complete(2, detail({ ...card, title: '新规则名称' })); await h.settle(); h.complete(1, page([card], '')); await h.settle()
 expect(h.model.state.items[0].title).toBe('新规则名称'); expect(h.model.state.detail?.item?.title).toBe('新规则名称'); expect(h.model.state.cursor).toBe(''); expect(h.model.state.detailStatus).toBe('ready')
})
it.each(['list', 'more', 'detail'])('401 from %s clears all cached scope and refuses same token', async action => {
 const h = harness(); await h.loaded(); if (action === 'list') h.model.refresh(); if (action === 'more') h.model.loadMore(); if (action === 'detail') h.model.openDetail(id)
 h.complete(1, {}, 401); await h.settle(); expect(h.model.state.status).toBe('login-required'); expect(h.model.state.scope).toBeNull(); expect(h.model.state.items).toEqual([])
 const count = h.pending.length; h.model.setScope(scope); await h.model.refresh(); expect(h.pending).toHaveLength(count)
 h.session.value = 'user-b'; expect(h.model.state.scope).toBeNull(); expect(h.model.state.status).toBe('position-required'); h.model.setScope(scope); expect(h.pending[count].token).toBe('user-b')
})
it.each([null, 'user-b'])('session change %s immediately clears scope and all pending data', async token => {
 const h = harness(); await h.loaded(); h.model.openDetail(id); h.session.value = token
 expect(h.model.state.items).toEqual([]); expect(h.model.state.scope).toBeNull(); expect(h.model.state.detailId).toBe(''); h.complete(1, detail()); await h.settle(); expect(h.model.state.detail).toBeNull(); expect(h.pending).toHaveLength(2)
})
it('honors late same-identity 401 even after scope changed', async () => {
 const h = harness(); await h.loaded(); h.model.openDetail(id); h.model.setScope({ ...scope, positionId: 'p2' }); h.complete(2, page()); await h.settle(); h.complete(1, {}, 401); await h.settle()
 expect(h.model.state.status).toBe('login-required'); expect(h.model.state.items).toEqual([])
})
it('closing detail ignores old result, reopening refetches, disposal stops everything', async () => {
 const h = harness(); await h.loaded(); h.model.openDetail(id); h.model.closeDetail(); h.complete(1, detail()); await h.settle(); expect(h.model.state.detail).toBeNull()
 h.model.openDetail(id); h.model.dispose(); h.complete(2, detail()); await h.settle(); h.session.value = 'new'; h.model.setScope(scope); await h.model.refresh(); await h.model.openDetail(id); expect(h.pending).toHaveLength(3); expect(h.model.state.items).toEqual([])
})
it('effect scope disposal clears and stops session watchers', async () => {
 const life = effectScope(), h = life.run(() => harness())!; await h.loaded(); life.stop(); h.session.value = 'new'
 expect(h.model.state.scope).toBeNull(); expect(h.model.state.items).toEqual([]); expect(h.pending).toHaveLength(1)
})

it.each([[503, 'error'], [404, 'unavailable']] as const)('detail %s preserves directory and supports an explicit fresh retry', async (status, kind) => {
 const h = harness(); await h.loaded(); h.model.openDetail(id); h.complete(1, { message: 'private-secret' }, status); await h.settle()
 expect(h.model.state.items).toEqual([card]); expect(h.model.state.status).toBe('ready'); expect(h.model.state.detailStatus).toBe(kind); expect(h.model.state.detail).toBeNull(); expect(h.model.state.detailError).not.toContain('private-secret')
 h.model.openDetail(id); h.complete(2, detail()); await h.settle(); expect(h.model.state.detailStatus).toBe('ready'); expect(h.model.state.detail?.item?.id).toBe(id)
})
it('ignores old identity 401 without clearing a freshly loaded new identity', async () => {
 const h = harness(); h.model.setScope(scope); h.session.value = 'user-b'; h.model.setScope({ ...scope, positionId: 'p2' })
 h.complete(1, page([{ ...card, id: other }], '')); await h.settle(); h.complete(0, {}, 401); await h.settle()
 expect(h.model.state.status).toBe('ready'); expect(h.model.state.scope?.positionId).toBe('p2'); expect(h.model.state.items.map(item => item.id)).toEqual([other])
})
it('session ABA cannot make an old successful response current again', async () => {
 const h = harness(); h.model.setScope(scope); h.session.value = 'user-b'; h.session.value = 'user-a'; h.model.setScope(scope)
 h.complete(1, page([{ ...card, id: other }], '')); await h.settle(); h.complete(0, page()); await h.settle()
 expect(h.model.state.items.map(item => item.id)).toEqual([other]); expect(h.model.state.cursor).toBe('')
})
