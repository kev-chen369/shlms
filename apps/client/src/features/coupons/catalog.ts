import { reactive, watch, inject, type InjectionKey } from 'vue'
import { selectedPlatform, type Platform } from '../platform'
import { createCouponAPI, CouponError, type Coupon, type City, type Context } from './api'
type Status = 'idle' | 'loading' | 'ready' | 'empty' | CouponError['kind']
export function createCouponCatalog(api: ReturnType<typeof createCouponAPI>) {
  const state = reactive({
    context: { platform: 'JD', cityCode: '', business: '' } as Required<Context>,
    status: 'idle' as Status, error: '', expiredNotice: '', items: [] as Coupon[], nextCursor: '', loadingMore: false, pageError: '',
    cities: [] as City[], citiesStatus: 'idle' as Status,
    detailId: '', detailStatus: 'idle' as Status, detail: null as Coupon | null, detailError: '',
  })
  let initialized = false, listVersion = 0, detailVersion = 0, cityVersion = 0
  const failure = (error: unknown) => error instanceof CouponError ? error : new CouponError()
  function pruneExpired() {
    const now = Date.now(), expiredDetail = state.detail !== null && Date.parse(state.detail.expiresAt) <= now
    const expired = state.items.filter(item => Date.parse(item.expiresAt) <= now || (expiredDetail && item.id === state.detailId))
    if (!expired.length && !expiredDetail) return
    state.items = state.items.filter(item => !expired.some(old => old.id === item.id))
    state.expiredNotice = '部分优惠已到期并隐藏，请刷新目录确认当前规则。'
    if (!state.items.length && state.status === 'ready') state.status = 'empty'
    if (expiredDetail || expired.some(item => item.id === state.detailId)) {
      detailVersion++
      state.detail = null; state.detailStatus = 'not-found'; state.detailError = '该券已到期，请刷新目录'
    }
  }
  function closeDetail() {
    detailVersion++
    state.detailId = ''; state.detail = null; state.detailStatus = 'idle'; state.detailError = ''
  }
  async function loadCities() {
    const version = ++cityVersion, platform = state.context.platform
    state.cities = []; state.citiesStatus = 'loading'
    try {
      const cities = await api.cities(platform)
      if (version !== cityVersion) return
      state.cities = cities; state.citiesStatus = 'ready'
    } catch (error) {
      if (version === cityVersion) state.citiesStatus = failure(error).kind
    }
  }
  async function load(reset: boolean) {
    if (!reset && (state.loadingMore || !state.nextCursor || !['ready', 'empty'].includes(state.status))) return
    const version = ++listVersion, context = { ...state.context }, cursor = reset ? '' : state.nextCursor
    if (reset) {
      closeDetail()
      state.items = []; state.nextCursor = ''; state.status = 'loading'; state.error = ''; state.expiredNotice = ''
      state.loadingMore = false
    } else state.loadingMore = true
    state.pageError = ''
    try {
      const page = await api.list(context, cursor)
      if (version !== listVersion) return
      if (!reset && (page.items.some(item => state.items.some(old => old.id === item.id)) || (page.nextCursor !== '' && page.nextCursor === cursor))) throw new CouponError()
      state.items = reset ? page.items : [...state.items, ...page.items]
      state.nextCursor = page.nextCursor; state.status = state.items.length ? 'ready' : 'empty'
    } catch (error) {
      if (version !== listVersion) return
      const result = failure(error)
      if (reset) { state.status = result.kind; state.error = result.message }
      else state.pageError = result.message
    } finally {
      if (version === listVersion) state.loadingMore = false
    }
  }
  function setContext(context: Context) {
    const next = { platform: context.platform, cityCode: context.cityCode ?? '', business: context.business ?? '' }
    if (initialized && next.platform === state.context.platform && next.cityCode === state.context.cityCode && next.business === state.context.business) return
    const platformChanged = !initialized || next.platform !== state.context.platform
    initialized = true
    state.context = next
    if (platformChanged) void loadCities()
    void load(true)
  }
  function selectPlatform(platform: Platform) {
    if (!initialized || state.context.platform !== platform) setContext({ platform })
  }
  async function openDetail(id: string) {
    pruneExpired()
    if (state.status !== 'ready' || !state.items.some(item => item.id === id)) return
    const version = ++detailVersion, context = { ...state.context }
    state.detailId = id; state.detail = null; state.detailError = ''; state.detailStatus = 'loading'
    try {
      const item = await api.detail(context, id)
      if (version === detailVersion) {
        state.detail = item; state.detailStatus = 'ready'
        state.items = state.items.map(old => old.id === id ? item : old)
      }
    } catch (error) {
      if (version !== detailVersion) return
      const result = failure(error)
      state.detailStatus = result.kind; state.detailError = result.message
      if (result.kind === 'not-found') {
        state.items = state.items.filter(item => item.id !== id)
        state.expiredNotice = '该券已失效并隐藏，请刷新目录。'
        if (!state.items.length) state.status = 'empty'
      }
    }
  }
  return { state, setContext, selectPlatform, closeDetail, openDetail, loadCities, pruneExpired, refresh: () => load(true), loadMore: () => load(false) }
}
export type CouponCatalog = ReturnType<typeof createCouponCatalog>
export const couponCatalogKey: InjectionKey<CouponCatalog> = Symbol('couponCatalog')
const sharedCatalog = createCouponCatalog(createCouponAPI())
export function useCouponCatalog() {
  const catalog = inject(couponCatalogKey, sharedCatalog)
  watch(selectedPlatform, platform => catalog.selectPlatform(platform), { immediate: true, flush: 'sync' })
  return catalog
}
