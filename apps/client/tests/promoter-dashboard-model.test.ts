import { effectScope, ref } from 'vue'
import { afterEach, expect, it } from 'vitest'
import { createPromoterDashboardModel } from '../src/features/promotion/dashboard-model'
import type { DashboardFilter } from '../src/features/promotion/dashboard-api'
const counts = { timeZone: 'Asia/Shanghai', from: '2026-08-19T16:00:00Z', toExclusive: '2026-09-18T16:00:00Z', asOf: '2026-09-18T12:00:00.123456789Z', successfulLinks: 5, copyReports: 8, validOrders: 2 }
const filtered = { ...counts, from: '2026-08-31T16:00:00Z', toExclusive: '2026-09-02T16:00:00Z' }
const cleanup: (() => void)[] = []
afterEach(() => cleanup.splice(0).forEach(fn => fn()))
function harness(token: string | null = 'provided-a') {
  const session = ref(token), pending: { url: string; token: string; complete: (data: unknown, code?: number) => void }[] = []
  const model = createPromoterDashboardModel(session, (url, token) => new Promise(resolve => pending.push({ url, token, complete: (data, code = 200) => resolve({ statusCode: code, data: code === 200 ? { code: 0, data } : data }) })))
  cleanup.push(model.dispose)
  const complete = (index: number, data: unknown, code?: number) => pending[index]?.complete(data, code)
  const settle = async () => { for (let i = 0; i < 12; i++) await Promise.resolve() }
  async function ready() { const result = model.refresh(); complete(0, counts); await result }
  return { session, model, pending, complete, settle, ready }
}
it('no identity means login explanation without outbound reads or fabricated zero counts', async () => {
  const { model, pending } = harness(null); await model.refresh(); model.setFilter({ channel: 'JD' })
  expect(model.state.status).toBe('login-required'); expect(model.state.counts).toBeNull(); expect(pending).toEqual([])
})
it('explicit first refresh loads validated counts and metadata without automatic fake statistics', async () => {
  const { model, ready, pending } = harness(); expect(pending).toEqual([]); expect(model.state.status).toBe('idle')
  await ready(); expect(pending[0]).toMatchObject({ url: '/api/v1/promoter/dashboard', token: 'provided-a' }); expect(model.state.status).toBe('ready'); expect(model.state.counts).toEqual(counts)
})
it('refresh immediately retires old counts; failure cannot present them as current; retry restores validated result', async () => {
  const { model, ready, complete, settle } = harness(); await ready(); model.refresh()
  expect(model.state.counts).toBeNull(); expect(model.state.status).toBe('loading')
  complete(1, { message: 'private secret' }, 500); await settle(); expect(model.state.status).toBe('error'); expect(model.state.error).not.toContain('private secret')
  model.refresh(); expect(model.state.error).toBe(''); complete(2, counts); await settle(); expect(model.state.counts).toEqual(counts)
})
it('retains real zero counts and metadata in empty state, never synthesizes them from missing data', async () => {
  const { model, complete, settle } = harness(); model.refresh(); const zero = { ...counts, successfulLinks: 0, copyReports: 0, validOrders: 0 }
  complete(0, zero); await settle(); expect(model.state.status).toBe('empty'); expect(model.state.counts).toEqual(zero)
})
it('any nonzero approved count is data, not a inferred link-to-order funnel', async () => {
  const { model, complete, settle } = harness(); model.refresh(); const data = { ...counts, successfulLinks: 0, copyReports: 100, validOrders: 20 }
  complete(0, data); await settle(); expect(model.state.status).toBe('ready'); expect(model.state.counts).toEqual(data)
})
it('filter changes clear current counts, bind snapshot and ignore stale filter success', async () => {
  const { model, ready, pending, complete, settle } = harness(); await ready(); model.refresh()
  model.setFilter({ from: '2026-09-01', to: '2026-09-02', channel: 'TB', positionId: 'p1' })
  expect(model.state.counts).toBeNull(); expect(pending[2].url).toBe('/api/v1/promoter/dashboard?from=2026-09-01&to=2026-09-02&channel=TB&positionId=p1')
  complete(2, filtered); complete(1, counts); await settle(); expect(model.state.counts).toEqual(filtered)
})
it('clones input filter and avoids duplicate identical filter reads; explicit refresh still retries', async () => {
  const { model, pending } = harness(), filter = { channel: 'JD' }; model.setFilter(filter); filter.channel = 'TB'
  expect(model.state.filter).toEqual({ channel: 'JD' }); model.setFilter({ channel: 'JD' }); expect(pending).toHaveLength(1)
  model.refresh(); expect(pending).toHaveLength(2)
})
it.each([null, 'provided-b'])('session %s synchronously clears counts, filter and old errors, rejecting old success', async token => {
  const { model, session, complete, ready, settle, pending } = harness(); await ready(); model.setFilter({ channel: 'JD' }); session.value = token
  expect(model.state.counts).toBeNull(); expect(model.state.filter).toEqual({}); expect(model.state.error).toBe(''); expect(model.state.status).toBe(token ? 'loading' : 'login-required')
  complete(1, counts); await settle(); expect(model.state.counts).toBeNull()
  if (token) { expect(pending[2].token).toBe('provided-b'); complete(2, counts); await settle(); expect(model.state.counts).toEqual(counts) }
})
it('latest refresh wins even when old refresh succeeds or fails afterward', async () => {
  const { model, complete, settle } = harness(); model.refresh(); model.refresh(); const latest = { ...counts, validOrders: 15 }
  complete(1, latest); complete(0, { message: 'private' }, 500); await settle(); expect(model.state.counts).toEqual(latest); expect(model.state.status).toBe('ready'); expect(model.state.error).toBe('')
})
it('late 401 within same session clears a newer filter result and locks rejected identity', async () => {
  const { model, complete, settle, pending, session } = harness(); model.refresh(); model.setFilter({ channel: 'TB' }); complete(1, counts); await settle()
  complete(0, { message: 'private' }, 401); await settle(); expect(model.state.status).toBe('login-required'); expect(model.state.counts).toBeNull(); expect(model.state.filter).toEqual({})
  await model.refresh(); model.setFilter({ channel: 'JD' }); expect(pending).toHaveLength(2)
  session.value = 'provided-b'; expect(pending).toHaveLength(3); complete(2, counts); await settle(); expect(model.state.status).toBe('ready')
})
it('old-session unauthorized or success does not clear a new-session accepted snapshot', async () => {
  const { model, session, complete, settle } = harness(); model.refresh(); session.value = 'provided-b'; const latest = { ...counts, copyReports: 30 }
  complete(1, latest); await settle(); complete(0, { code: 'UNAUTHORIZED' }, 401); await settle(); expect(model.state.counts).toEqual(latest); expect(model.state.status).toBe('ready')
})
it('invalid filters fail closed without outbound read or treating old counts as current', async () => {
  const { model, pending, ready, settle } = harness(); await ready(); model.setFilter({ from: '2026-02-30' }); await settle()
  expect(pending).toHaveLength(1); expect(model.state.counts).toBeNull(); expect(model.state.status).toBe('error')
})
it('unavailable service and malformed metadata have separate sanitized states, never zero fallback', async () => {
  const { model, complete, settle } = harness(); model.refresh(); complete(0, { message: 'private' }, 503); await settle()
  expect(model.state.status).toBe('unavailable'); expect(model.state.counts).toBeNull(); expect(model.state.error).not.toContain('private')
  model.refresh(); complete(1, { ...counts, validOrders: -1 }); await settle(); expect(model.state.status).toBe('error'); expect(model.state.counts).toBeNull()
})
it('does not silently ignore unapproved filter keys when known filters are unchanged', async () => {
  const { model, pending, ready, settle } = harness(); await ready(); model.setFilter({ ownerUserId: 'someone' } as DashboardFilter); await settle()
  expect(model.state.status).toBe('error'); expect(model.state.counts).toBeNull(); expect(pending).toHaveLength(1)
})
it.each([null, [], false].map(value => [value]))('does not flatten malformed filter %j into an unrestricted dashboard read', async value => {
  const { model, pending, ready, settle } = harness(); await ready(); model.setFilter(value as DashboardFilter); await settle()
  expect(model.state.status).toBe('error'); expect(model.state.counts).toBeNull(); expect(pending).toHaveLength(1)
})
it('dispose invalidates pending snapshot, clears private state and stops future session reads', async () => {
  const { model, session, complete, settle, pending } = harness(); model.refresh(); model.dispose(); model.dispose(); complete(0, counts); await settle()
  expect(model.state.counts).toBeNull(); expect(model.state.status).toBe('idle'); session.value = 'provided-b'; await model.refresh(); model.setFilter({ channel: 'TB' }); expect(pending).toHaveLength(1)
})
it('Vue owner scope automatically retires the model without future reads or response population', async () => {
  const scope = effectScope(); const h = scope.run(() => harness())!; h.model.refresh(); scope.stop(); h.complete(0, counts); await h.settle()
  h.session.value = 'provided-b'; await h.model.refresh(); expect(h.pending).toHaveLength(1); expect(h.model.state.counts).toBeNull(); expect(h.model.state.status).toBe('idle')
})
