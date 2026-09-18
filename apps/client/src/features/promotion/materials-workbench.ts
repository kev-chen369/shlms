import { reactive, watch, getCurrentScope, onScopeDispose, type Ref } from 'vue'
import { createMaterialsModel } from './materials-model'
import { createPositionsAPI, PositionsError, type PositionChoice } from './positions-api'
import type { MaterialRequest } from './materials-api'

export function createMaterialsWorkbench(session: Ref<string | null>, terminal: 'H5' | 'WX_MINI', request?: MaterialRequest) {
  const materials = createMaterialsModel(session, request)
  const api = createPositionsAPI(() => session.value, request)
  const state = reactive({ status: (session.value ? 'idle' : 'login-required') as 'idle' | 'loading' | 'ready' | 'empty' | PositionsError['kind'], items: [] as PositionChoice[], cursor: '', loadingMore: false, selectedId: '', scene: '', error: '', pageError: '' })
  let version = 0, auth = 0, rejected: string | null = null, disposed = false
  const waiting = () => !session.value || session.value === rejected ? 'login-required' : materials.state.platform !== 'JD' ? 'unavailable' : 'idle'
  function clearSelection() { state.selectedId = ''; state.scene = ''; state.error = ''; materials.setScope(null) }
  function clear() { version++; state.items = []; state.cursor = ''; state.loadingMore = false; state.pageError = ''; clearSelection(); state.status = waiting() }
  function login() { rejected = session.value; clear(); state.status = 'login-required' }
  function bytes(value: string, max: number, empty = false) {
    if ((!empty && !value) || value.trim() !== value || /[\u0000-\u001f\u007f-\u009f]/.test(value)) return false
    try { return encodeURIComponent(value).replace(/%[0-9a-f]{2}/gi, 'x').length <= max } catch { return false }
  }
  function applicable(item: PositionChoice) { return bytes(item.id, 128) && bytes(item.scene, 80) && item.channels.some(mapping => mapping.channel === 'JD' && mapping.readiness !== 'UNAVAILABLE') }
  async function load(reset: boolean) {
    if (disposed) return
    if (waiting() === 'login-required') { login(); return }
    if (materials.state.platform !== 'JD') { clear(); state.status = 'unavailable'; return }
    if (!reset && (!state.cursor || state.loadingMore || state.status !== 'ready')) return
    const after = reset ? '' : state.cursor, identity = auth
    if (reset) { clear(); state.status = 'loading' } else state.loadingMore = true
    state.pageError = ''; const current = ++version
    try {
      const page = await api.list(after)
      if (current !== version || disposed) return
      if (!reset && (page.items.some(item => state.items.some(old => old.id === item.id)) || (page.nextCursor && page.nextCursor === after))) throw new PositionsError()
      state.items = reset ? page.items : [...state.items, ...page.items]; state.cursor = page.nextCursor; state.status = state.items.length ? 'ready' : 'empty'
    } catch (error) {
      const failure = error instanceof PositionsError ? error : new PositionsError()
      if (disposed) return
      if (identity === auth && failure.kind === 'login-required') login()
      else if (current !== version) return
      else if (reset) { state.status = failure.kind; state.error = failure.message }
      else state.pageError = failure.message
    } finally { if (current === version) state.loadingMore = false }
  }
  function choose(id: string) {
    if (disposed || waiting() === 'login-required' || materials.state.platform !== 'JD') return
    clearSelection()
    const item = state.items.find(item => item.id === id)
    if (!item) { state.error = '请选择接口返回的本人推广位。'; return }
    if (!applicable(item)) { state.error = '该推广位或场景不适用当前物料读取范围，请选择其他位。'; return }
    state.selectedId = item.id; state.scene = item.scene
  }
  function apply(cityCode: string, business: string) {
    if (disposed || waiting() === 'login-required' || materials.state.platform !== 'JD') return
    const item = state.items.find(item => item.id === state.selectedId)
    if (!item || !applicable(item)) { state.error = '请选择适用当前物料读取范围的本人推广位。'; return }
    if (!bytes(cityCode, 32, true) || !bytes(business, 40, true)) { state.error = '城市 / 业务范围无效，请使用无控制字符的城市编码（最多32字节）和业务（最多40字节）。'; return }
    state.error = ''
    materials.setScope({ platform: 'JD', type: materials.state.type, terminal, positionId: item.id, scene: item.scene, ...(cityCode ? { cityCode } : {}), ...(business ? { business } : {}) })
  }
  const stopSession = watch(session, () => { auth++; rejected = null; clear() }, { flush: 'sync' })
  const stopContext = watch(() => [materials.state.platform, materials.state.type], clear, { flush: 'sync' })
  const stopAuth = watch(() => materials.state.status, status => { if (status === 'login-required' && session.value && session.value !== rejected) login() }, { flush: 'sync' })
  function dispose() { if (disposed) return; disposed = true; auth++; version++; stopSession(); stopContext(); stopAuth(); clear(); materials.dispose() }
  if (getCurrentScope()) onScopeDispose(dispose)
  return { state, materials, applicable, choose, apply, refreshPositions: () => load(true), morePositions: () => load(false), dispose }
}
