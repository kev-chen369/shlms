import { reactive, watch, getCurrentScope, onScopeDispose, type Ref } from 'vue'
import { createPromoterDashboardAPI, DashboardError, type DashboardFilter, type PromoterCounts } from './dashboard-api'
import type { OrderRequest } from './orders-api'
type Status = 'idle' | 'loading' | 'ready' | 'empty' | DashboardError['kind']
export function createPromoterDashboardModel(session: Ref<string | null>, request?: OrderRequest) {
  const api = createPromoterDashboardAPI(() => session.value, request)
  const state = reactive({ status: (session.value ? 'idle' : 'login-required') as Status, counts: null as PromoterCounts | null, filter: {} as DashboardFilter, error: '' })
  let version = 0, sessionVersion = 0, rejectedSession: string | null = null, disposed = false
  function clear() { version++; state.counts = null; state.filter = {}; state.error = '' }
  function requireLogin() { rejectedSession = session.value; clear(); state.status = 'login-required' }
  async function refresh() {
    if (disposed) return
    if (!session.value || session.value === rejectedSession) { requireLogin(); return }
    const current = ++version, auth = sessionVersion, filter = { ...state.filter }
    state.counts = null; state.error = ''; state.status = 'loading'
    try {
      const counts = await api.get(filter)
      if (current !== version) return
      state.counts = counts
      state.status = counts.successfulLinks || counts.copyReports || counts.validOrders ? 'ready' : 'empty'
    } catch (error) {
      const failure = error instanceof DashboardError ? error : new DashboardError()
      if (auth === sessionVersion && failure.kind === 'login-required') requireLogin()
      else if (current === version) { state.status = failure.kind; state.error = failure.message }
    }
  }
  function setFilter(filter: DashboardFilter) {
    if (disposed) return
    if (!session.value || session.value === rejectedSession) { requireLogin(); return }
    if (typeof filter !== 'object' || filter === null || Array.isArray(filter)) {
      clear(); state.status = 'error'; state.error = new DashboardError().message; return
    }
    const next = { ...filter }, keys = Object.keys(next) as (keyof DashboardFilter)[]
    if (state.status !== 'idle' && keys.length === Object.keys(state.filter).length && keys.every(key => next[key] === state.filter[key])) return
    state.filter = next; void refresh()
  }
  const stopWatching = watch(session, () => {
    sessionVersion++; rejectedSession = null; clear(); state.status = session.value ? 'idle' : 'login-required'
    if (session.value) void refresh()
  }, { flush: 'sync' })
  function dispose() {
    if (disposed) return
    disposed = true; sessionVersion++; stopWatching(); clear(); rejectedSession = null; state.status = 'idle'
  }
  if (getCurrentScope()) onScopeDispose(dispose)
  return { state, refresh, setFilter, dispose }
}
