import { reactive, watch, getCurrentScope, onScopeDispose, type Ref } from 'vue'
import { createPromoterOrdersAPI, compareTime, OrdersError, type OrderFilter, type OrderRequest, type PromoterOrder, type PromoterOrderDetail } from './orders-api'
type Status = 'idle' | 'loading' | 'ready' | 'empty' | OrdersError['kind']
export function createPromoterOrdersModel(session: Ref<string | null>, request?: OrderRequest) {
  const api = createPromoterOrdersAPI(() => session.value, request)
  const state = reactive({ status: (session.value ? 'idle' : 'login-required') as Status, items: [] as PromoterOrder[], filter: {} as OrderFilter, cursor: '', loadingMore: false, error: '', pageError: '', detailId: '', detail: null as PromoterOrderDetail | null, detailStatus: 'idle' as Status, detailError: '' })
  let listVersion = 0, detailVersion = 0, sessionVersion = 0, rejectedSession: string | null = null, disposed = false
  const failure = (error: unknown) => error instanceof OrdersError ? error : new OrdersError()
  function closeDetail() { detailVersion++; state.detailId = ''; state.detail = null; state.detailStatus = 'idle'; state.detailError = '' }
  function clearAll() {
    listVersion++; closeDetail()
    state.items = []; state.filter = {}; state.cursor = ''; state.loadingMore = false; state.error = ''; state.pageError = ''
  }
  function requireLogin() { rejectedSession = session.value; clearAll(); state.status = 'login-required' }
  function canRead() {
    if (disposed) return false
    if (!session.value || session.value === rejectedSession) { requireLogin(); return false }
    return true
  }
  async function load(reset: boolean) {
    if (!canRead()) return
    if (!reset && (state.loadingMore || !state.cursor || !['ready', 'empty'].includes(state.status))) return
    const version = ++listVersion, auth = sessionVersion, filter = { ...state.filter }, cursor = reset ? '' : state.cursor
    if (reset) {
      closeDetail(); state.items = []; state.cursor = ''; state.error = ''; state.status = 'loading'; state.loadingMore = false
    } else state.loadingMore = true
    state.pageError = ''
    try {
      const page = await api.list(filter, cursor)
      if (version !== listVersion) return
      if (!reset && (page.items.some(item => state.items.some(old => old.id === item.id)) || (page.nextCursor !== '' && page.nextCursor === cursor))) throw new OrdersError()
      state.items = reset ? page.items : [...state.items, ...page.items]
      state.cursor = page.nextCursor; state.status = state.items.length ? 'ready' : 'empty'
    } catch (error) {
      const result = failure(error)
      if (auth === sessionVersion && result.kind === 'login-required') requireLogin()
      else if (version !== listVersion) return
      else if (reset) { state.status = result.kind; state.error = result.message }
      else state.pageError = result.message
    } finally { if (version === listVersion) state.loadingMore = false }
  }
  function setFilter(filter: OrderFilter) {
    if (disposed) return
    const next = { ...filter }
    if (state.status !== 'idle' && (['channel', 'positionId', 'orderStatus', 'from', 'to'] as const).every(key => next[key] === state.filter[key])) return
    state.filter = next; void load(true)
  }
  async function openDetail(id: string) {
    if (!canRead() || state.status !== 'ready' || !state.items.some(item => item.id === id)) return
    const version = ++detailVersion, auth = sessionVersion
    state.detailId = id; state.detail = null; state.detailStatus = 'loading'; state.detailError = ''
    try {
      const item = await api.detail(id)
      if (version !== detailVersion) return
      state.detail = item; state.detailStatus = 'ready'
      listVersion++; state.loadingMore = false
      const { history: _history, refundEventCount: _refundCount, ...summary } = item
      const filter = state.filter
      const matches = (!filter.channel || item.channel === filter.channel) && (!filter.positionId || item.positionId === filter.positionId) &&
        (!filter.orderStatus || item.orderStatus === filter.orderStatus) && (!filter.from || compareTime(item.orderOccurredAt, filter.from) >= 0) && (!filter.to || compareTime(item.orderOccurredAt, filter.to) < 0)
      state.items = matches ? state.items.map(old => old.id === id ? summary : old) : state.items.filter(old => old.id !== id)
      if (!state.items.length) state.status = 'empty'
    } catch (error) {
      const result = failure(error)
      if (auth === sessionVersion && result.kind === 'login-required') requireLogin()
      else if (version !== detailVersion) return
      else {
        state.detailStatus = result.kind; state.detailError = result.message
        if (result.kind === 'not-found') {
          listVersion++; state.loadingMore = false
          state.items = state.items.filter(item => item.id !== id)
          if (!state.items.length) state.status = 'empty'
        }
      }
    }
  }
  const stopWatching = watch(session, () => {
    sessionVersion++; rejectedSession = null; clearAll(); state.status = session.value ? 'idle' : 'login-required'
    if (session.value) void load(true)
  }, { flush: 'sync' })
  function dispose() {
    if (disposed) return
    disposed = true; sessionVersion++; stopWatching(); clearAll(); rejectedSession = null; state.status = 'idle'
  }
  if (getCurrentScope()) onScopeDispose(dispose)
  return { state, refresh: () => load(true), loadMore: () => load(false), setFilter, openDetail, closeDetail, dispose }
}
