import { expect, it, vi } from 'vitest'
import { effectScope, ref } from 'vue'
import { createPromoterOrdersModel } from '../src/features/promotion/orders-model'
import type { OrderRequest } from '../src/features/promotion/orders-api'
const id = '11111111-1111-1111-1111-111111111111', other = '22222222-2222-2222-2222-222222222222'
const order = { id, channel: 'JD', maskedOrderId: '****1234', orderStatus: 'PAID', positionId: 'p1', attributionMethod: 'SUB_ID', attributedAt: '2026-09-18T12:00:00Z', orderOccurredAt: '2026-09-18T10:00:00Z', statusAt: '2026-09-18T11:00:00Z' }
const detail = { ...order, history: [], refundEventCount: 0 }
function harness(token: string | null = 'provided-user-a') {
  const session = ref(token), pending: { url: string; token: string; resolve: (response: Awaited<ReturnType<OrderRequest>>) => void }[] = []
  const request = vi.fn((url: string, token: string) => new Promise<Awaited<ReturnType<OrderRequest>>>(resolve => pending.push({ url, token, resolve })))
  const model = createPromoterOrdersModel(session, request)
  const complete = (index: number, data: unknown, statusCode = 200) => pending[index].resolve({ statusCode, data: statusCode === 200 ? { code: 0, data } : data })
  const settle = async () => { for (let i = 0; i < 12; i++) await Promise.resolve() }
  const ready = async () => { model.refresh(); if (pending[0]) complete(0, { items: [order], nextCursor: 'bmV4dA' }); await settle() }
  return { model, session, pending, request, complete, settle, ready }
}
it('does not request any order service while signed out, even through detail or pagination actions', async () => {
  const { model, request } = harness(null)
  await model.refresh(); await model.loadMore(); await model.openDetail(id)
  expect(model.state.status).toBe('login-required'); expect(model.state.items).toEqual([]); expect(request).not.toHaveBeenCalled()
})
it('loads only real validated summaries and distinguishes empty from a read error', async () => {
  const { model, ready, complete, settle } = harness(); await ready()
  expect(model.state.items).toEqual([order]); expect(model.state.status).toBe('ready')
  model.refresh(); expect(model.state.items).toEqual([]); complete(1, { message: 'private' }, 503); await settle()
  expect(model.state.status).toBe('error'); expect(model.state.error).not.toContain('private')
  model.refresh(); complete(2, { items: [], nextCursor: '' }); await settle(); expect(model.state.status).toBe('empty')
})
it.each([null, 'provided-user-b'])('immediately invalidates summaries, filters, cursor and pending detail after session becomes %s', async token => {
  const { model, session, ready, complete, settle } = harness(); await ready()
  model.setFilter({ positionId: 'p1' }); complete(1, { items: [order], nextCursor: 'bmV4dA' }); await settle()
  model.openDetail(id); session.value = token
  expect(model.state.items).toEqual([]); expect(model.state.cursor).toBe(''); expect(model.state.filter).toEqual({}); expect(model.state.detailId).toBe('')
  complete(2, detail); await settle(); expect(model.state.detail).toBeNull()
  expect(model.state.status).toBe(token ? 'loading' : 'login-required')
})
it('accepts only the newest session list and supplied bearer token', async () => {
  const { model, session, pending, complete, settle } = harness(); model.refresh(); session.value = 'provided-user-b'
  expect(pending[1].token).toBe('provided-user-b')
  complete(1, { items: [{ ...order, id: other }], nextCursor: '' }); complete(0, { items: [order], nextCursor: 'bmV4dA' }); await settle()
  expect(model.state.items.map(o => o.id)).toEqual([other]); expect(model.state.cursor).toBe('')
})
it('switching filters clears old list, detail and pagination before ignoring stale results', async () => {
  const { model, ready, complete, settle, pending } = harness(); await ready(); model.openDetail(id)
  model.setFilter({ channel: 'TB' }); expect(model.state.items).toEqual([]); expect(model.state.detailId).toBe(''); expect(model.state.cursor).toBe('')
  expect(pending[2].url).toBe('/api/v1/promoter/orders?channel=TB&limit=20')
  complete(1, detail); complete(2, { items: [{ ...order, channel: 'TB' }], nextCursor: '' }); await settle()
  expect(model.state.detail).toBeNull(); expect(model.state.items[0].channel).toBe('TB')
})
it('does not reload identical filters or mutate them when caller changes its input', async () => {
  const { model, pending } = harness(), filter = { channel: 'JD' }; model.setFilter(filter); filter.channel = 'TB'
  expect(model.state.filter).toEqual({ channel: 'JD' }); model.setFilter({ channel: 'JD' }); expect(pending).toHaveLength(1)
})
it('keeps cursor and good orders for pagination retry and prevents parallel loads', async () => {
  const { model, ready, complete, settle, pending } = harness(); await ready(); model.loadMore(); model.loadMore(); expect(pending).toHaveLength(2)
  complete(1, {}, 503); await settle(); expect(model.state.items).toEqual([order]); expect(model.state.cursor).toBe('bmV4dA'); expect(model.state.loadingMore).toBe(false)
  model.loadMore(); expect(pending[2].url).toContain('cursor=bmV4dA'); complete(2, { items: [{ ...order, id: other }], nextCursor: '' }); await settle()
  expect(model.state.items.map(o => o.id)).toEqual([id, other]); expect(model.state.pageError).toBe('')
})
it.each(['duplicate', 'loop'])('rejects a %s page while preserving the prior cursor', async kind => {
  const { model, ready, complete, settle } = harness(); await ready(); model.loadMore()
  complete(1, { items: [{ ...order, id: kind === 'duplicate' ? id : other }], nextCursor: kind === 'loop' ? 'bmV4dA' : '' }); await settle()
  expect(model.state.items).toEqual([order]); expect(model.state.cursor).toBe('bmV4dA'); expect(model.state.pageError).not.toBe('')
})
it.each(['list', 'more', 'detail'])('clears all order cache on %s 401 and will not reuse the rejected session', async action => {
  const { model, ready, complete, settle, request, session } = harness(); await ready()
  if (action === 'list') model.refresh(); if (action === 'more') model.loadMore(); if (action === 'detail') model.openDetail(id)
  complete(1, { message: 'private' }, 401); await settle()
  expect(model.state.status).toBe('login-required'); expect(model.state.items).toEqual([]); expect(model.state.detailId).toBe(''); expect(model.state.cursor).toBe('')
  const count = request.mock.calls.length; await model.refresh(); await model.loadMore(); await model.openDetail(id); expect(request).toHaveBeenCalledTimes(count)
  session.value = 'provided-user-b'; expect(model.state.status).toBe('loading'); expect(request).toHaveBeenCalledTimes(count + 1)
})
it('never restores an inaccessible detail from a prior pending order page', async () => {
  const { model, ready, complete, settle } = harness(); await ready(); model.loadMore(); model.openDetail(id)
  complete(2, { code: 'ORDER_NOT_FOUND' }, 404); await settle(); complete(1, { items: [order], nextCursor: '' }); await settle()
  expect(model.state.items).toEqual([]); expect(model.state.detailStatus).toBe('not-found'); expect(model.state.loadingMore).toBe(false); expect(model.state.cursor).toBe('bmV4dA')
})
it('closing detail ignores its response without clearing valid orders', async () => {
  const { model, ready, complete, settle } = harness(); await ready(); model.openDetail(id); model.closeDetail(); complete(1, detail); await settle()
  expect(model.state.detail).toBeNull(); expect(model.state.detailId).toBe(''); expect(model.state.items).toEqual([order])
})
it('updates a stale summary from fresh detail without adding private history fields to the list', async () => {
  const { model, ready, complete, settle } = harness(); await ready(); model.openDetail(id)
  complete(1, { ...detail, orderStatus: 'CONFIRMED' }); await settle()
  expect(model.state.items).toEqual([{ ...order, orderStatus: 'CONFIRMED' }]); expect(model.state.detail?.orderStatus).toBe('CONFIRMED')
})
it('keeps fresh detail readable but hides its summary if status no longer matches the selected filter', async () => {
  const { model, complete, settle } = harness(); model.setFilter({ orderStatus: 'PAID' })
  complete(0, { items: [order], nextCursor: 'bmV4dA' }); await settle(); model.loadMore(); model.openDetail(id)
  complete(2, { ...detail, orderStatus: 'CONFIRMED' }); await settle(); complete(1, { items: [order], nextCursor: '' }); await settle()
  expect(model.state.items).toEqual([]); expect(model.state.status).toBe('empty'); expect(model.state.detail?.orderStatus).toBe('CONFIRMED')
  expect(model.state.cursor).toBe('bmV4dA'); expect(model.state.loadingMore).toBe(false)
})
it('hides an order whose earlier backfilled event no longer matches the nanosecond date filter', async () => {
  const { model, complete, settle } = harness()
  model.setFilter({ from: '2026-09-18T10:00:00.000000001Z', to: '2026-09-18T10:00:00.000000003Z' })
  complete(0, { items: [{ ...order, orderOccurredAt: '2026-09-18T10:00:00.000000002Z' }], nextCursor: '' }); await settle()
  model.openDetail(id); complete(1, detail); await settle()
  expect(model.state.items).toEqual([]); expect(model.state.detail?.id).toBe(id)
})
it.each(['list', 'detail'])('honors a late %s 401 from the same session even after its view request was invalidated', async action => {
  const { model, ready, complete, settle } = harness(); await ready()
  if (action === 'list') model.refresh(); else model.openDetail(id)
  model.setFilter({ channel: 'JD' }); complete(2, { items: [order], nextCursor: '' }); await settle()
  complete(1, {}, 401); await settle()
  expect(model.state.status).toBe('login-required'); expect(model.state.items).toEqual([])
})
it.each(['list', 'detail'])('disposal clears private data and rejects pending %s results and future session actions', async action => {
  const { model, session, ready, complete, settle, request } = harness(); await ready()
  if (action === 'list') model.refresh(); else model.openDetail(id)
  model.dispose(); model.dispose(); const count = request.mock.calls.length
  expect(model.state.items).toEqual([]); expect(model.state.cursor).toBe(''); expect(model.state.detail).toBeNull()
  complete(1, action === 'list' ? { items: [order], nextCursor: 'bmV4dA' } : detail); await settle()
  expect(model.state.items).toEqual([]); expect(model.state.detail).toBeNull(); expect(model.state.status).toBe('idle')
  session.value = 'provided-user-b'; model.setFilter({ channel: 'JD' }); await model.refresh(); await model.loadMore(); await model.openDetail(id)
  expect(request).toHaveBeenCalledTimes(count); expect(model.state.items).toEqual([]); expect(model.state.detail).toBeNull(); expect(model.state.filter).toEqual({})
})
it('automatically disposes the model when its owning Vue scope ends', async () => {
  const scope = effectScope(), h = scope.run(() => harness())!; await h.ready(); scope.stop()
  h.session.value = 'provided-user-b'; h.model.refresh(); await h.settle()
  expect(h.model.state.items).toEqual([]); expect(h.request).toHaveBeenCalledTimes(1)
})
