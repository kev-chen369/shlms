import { reactive, watch, getCurrentScope, onScopeDispose, type Ref } from 'vue'
import type { Platform } from '../platform'
import { createMaterialsAPI, MaterialsError, type MaterialScope, type MaterialRequest, type MaterialCard, type MaterialDetail, type CatalogCapability } from './materials-api'

type Status = 'idle' | 'position-required' | 'loading' | 'ready' | 'empty' | 'blocked' | 'not-found' | MaterialsError['kind']
// Scope is set explicitly after platform/type selection; no default promotion
// position, demo material, generated link or cached permission is introduced.
export function createMaterialsModel(session: Ref<string | null>, request?: MaterialRequest) {
  const api = createMaterialsAPI(() => session.value, request)
  const state = reactive({ platform: 'JD' as Platform, type: 'PRODUCT' as MaterialScope['type'], scope: null as MaterialScope | null, status: (session.value ? 'position-required' : 'login-required') as Status,
    items: [] as MaterialCard[], capability: null as CatalogCapability | null, reason: '', cursor: '', loadingMore: false, error: '', pageError: '', notice: '',
    detailId: '', detail: null as MaterialDetail | null, detailStatus: 'idle' as Status, detailError: '', detailReason: '' })
  let listVersion = 0, detailVersion = 0, sessionVersion = 0, rejectedSession: string | null = null, disposed = false
  const failure = (error: unknown) => error instanceof MaterialsError ? error : new MaterialsError()
  function closeDetail() { if (disposed) return; clearDetail() }
  function clearDetail() { detailVersion++; state.detailId = ''; state.detail = null; state.detailStatus = 'idle'; state.detailError = ''; state.detailReason = '' }
  function clearData(clearScope = false) {
    listVersion++; clearDetail(); state.items = []; state.cursor = ''; state.loadingMore = false; state.capability = null; state.reason = ''; state.error = ''; state.pageError = ''; state.notice = ''
    if (clearScope) state.scope = null
  }
  function requireLogin() { rejectedSession = session.value; clearData(true); state.status = 'login-required' }
  function supported() { return ['JD', 'TAOBAO', 'MEITUAN'].includes(state.platform) && ['PRODUCT', 'ACTIVITY'].includes(state.type) && (state.platform !== 'MEITUAN' || state.type === 'ACTIVITY') }
  function waitingStatus(): Status { return !session.value || session.value === rejectedSession ? 'login-required' : !supported() ? 'unavailable' : 'position-required' }
  function canRead() {
    if (disposed) return false
    if (!session.value || session.value === rejectedSession) { requireLogin(); return false }
    if (!supported()) { state.status = 'unavailable'; return false }
    if (!state.scope || !state.scope.positionId || !state.scope.scene) { state.status = 'position-required'; return false }
    return true
  }
  function denyCatalog(cap: CatalogCapability) {
    clearData(); state.capability = cap; state.reason = cap.reason; state.status = 'blocked'
  }
  async function load(reset: boolean) {
    if (!canRead()) return
    if (!reset && (state.loadingMore || !state.cursor || state.status !== 'ready')) return
    const scope = { ...state.scope! }, cursor = reset ? '' : state.cursor, auth = sessionVersion
    if (reset) { clearData(); state.status = 'loading' } else state.loadingMore = true
    state.pageError = ''
    const version = ++listVersion
    try {
      const page = await api.list(scope, cursor)
      if (version !== listVersion) return
      if (!page.capability.allowed) { denyCatalog(page.capability); return }
      if (!reset && (page.items.some(item => state.items.some(old => old.id === item.id)) || (page.nextCursor !== '' && page.nextCursor === cursor))) throw new MaterialsError()
      state.items = reset ? page.items : [...state.items, ...page.items]; state.cursor = page.nextCursor; state.capability = page.capability; state.reason = ''; state.status = state.items.length ? 'ready' : 'empty'
    } catch (error) {
      const result = failure(error)
      if (auth === sessionVersion && result.kind === 'login-required') requireLogin()
      else if (version !== listVersion) return
      else if (reset) { state.status = result.kind; state.error = result.message }
      else state.pageError = result.message
    } finally { if (version === listVersion) state.loadingMore = false }
  }
  function selectPlatform(platform: Platform) {
    if (disposed || platform === state.platform) return
    clearData(true); state.platform = platform; state.type = platform === 'MEITUAN' ? 'ACTIVITY' : 'PRODUCT'; state.status = waitingStatus()
  }
  function selectType(type: MaterialScope['type']) {
    if (disposed || type === state.type) return
    clearData(true); state.type = type; state.status = waitingStatus()
  }
  function setScope(input: MaterialScope | null) {
    if (disposed) return
    if (input === null) { clearData(true); state.status = waitingStatus(); return }
    const next = { ...input }
    if (next.platform !== state.platform || next.type !== state.type) { clearData(true); state.status = 'error'; state.error = new MaterialsError().message; return }
    if (state.scope && Object.keys(next).length === Object.keys(state.scope).length && (['platform', 'type', 'terminal', 'positionId', 'scene', 'cityCode', 'business'] as const).every(key => next[key] === state.scope![key])) return
    clearData(true); state.scope = next; state.status = waitingStatus(); void load(true)
  }
  async function openDetail(id: string) {
    if (!canRead() || state.status !== 'ready' || !state.items.some(item => item.id === id)) return
    const version = ++detailVersion, auth = sessionVersion, scope = { ...state.scope! }
    state.detailId = id; state.detail = null; state.detailStatus = 'loading'; state.detailError = ''; state.detailReason = ''
    try {
      const result = await api.detail(id, scope)
      if (version !== detailVersion) return
      if (!result.capability.allowed) { denyCatalog(result.capability); return }
      // Fresh detail invalidates older list pages and continuation positions.
      listVersion++; state.loadingMore = false; state.cursor = ''; state.pageError = ''; state.notice = '物料状态已复核，请刷新目录确认后续物料。'
      state.capability = result.capability
      if (!result.availability.available || !result.item) {
        state.items = state.items.filter(item => item.id !== id); state.detailStatus = 'not-found'; state.detailReason = result.availability.reason; state.detailError = '该物料已失效或不适用，请刷新目录。'
      } else {
        state.items = state.items.map(old => old.id === id ? result.item! : old); state.detail = result; state.detailStatus = 'ready'
      }
      state.status = state.items.length ? 'ready' : 'empty'
    } catch (error) {
      const result = failure(error)
      if (auth === sessionVersion && result.kind === 'login-required') requireLogin()
      else if (version !== detailVersion) return
      else { state.detailStatus = result.kind; state.detailError = result.message }
    }
  }
  const stopWatching = watch(session, () => { sessionVersion++; rejectedSession = null; clearData(true); state.status = waitingStatus() }, { flush: 'sync' })
  function dispose() { if (disposed) return; disposed = true; sessionVersion++; stopWatching(); clearData(true); rejectedSession = null; state.status = 'idle' }
  if (getCurrentScope()) onScopeDispose(dispose)
  return { state, setScope, selectPlatform, selectType, refresh: () => load(true), loadMore: () => load(false), openDetail, closeDetail, dispose }
}
