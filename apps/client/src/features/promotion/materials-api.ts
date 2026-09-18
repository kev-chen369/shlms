import type { Platform } from '../platform'

export type MaterialScope = { platform: Platform; type: 'PRODUCT' | 'ACTIVITY'; terminal: 'H5' | 'WX_MINI'; positionId: string; scene: string; cityCode?: string; business?: string }
export type MaterialRequest = (url: string, token: string) => Promise<{ statusCode: number; data: unknown }>
export type MaterialCard = { id: string; platform: 'JD' | 'TB' | 'MT'; type: 'PRODUCT' | 'ACTIVITY'; title: string; startsAt?: string; endsAt: string; sourceUpdatedAt: string; ruleVersion: string; region: { mode: 'NATIONWIDE' | 'CITIES'; cityCodes?: string[] }; business: string; terminals: ('H5' | 'WX_MINI')[] }
// Only CATALOG read eligibility, deliberately no generation permission.
export type CatalogCapability = { allowed: boolean; reason: string }
export type MaterialPage = { items: MaterialCard[]; nextCursor: string; capability: CatalogCapability }
export type MaterialDetail = { item?: MaterialCard; capability: CatalogCapability; availability: { available: boolean; reason: string } }
export class MaterialsError extends Error {
  constructor(public readonly kind: 'login-required' | 'unavailable' | 'error' = 'error') {
    super(kind === 'login-required' ? '请登录后查看本人推广物料' : kind === 'unavailable' ? '推广物料服务尚未接入' : '推广物料读取失败，请稍后重试')
  }
}
const platforms: Partial<Record<Platform, MaterialCard['platform']>> = { JD: 'JD', TAOBAO: 'TB', MEITUAN: 'MT' }
const deniedCapabilities = ['UNCONFIGURED', 'PENDING_VERIFICATION', 'SUSPENDED', 'INVALID_DECLARATION', 'SCOPE_MISMATCH', 'INVALID_EVIDENCE', 'VERIFICATION_EXPIRED', 'POSITION_UNAVAILABLE', 'NOT_ENABLED']
const deniedMaterials = ['MATERIAL_UNAVAILABLE', 'INVALID_MATERIAL', 'INVALID_CONTEXT', 'SCOPE_MISMATCH', 'NOT_ACTIVE', 'NOT_STARTED', 'EXPIRED', 'SOURCE_NOT_CURRENT', 'REGION_MISMATCH', 'BUSINESS_MISMATCH', 'TERMINAL_MISMATCH']
function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function invalid(): never { throw new MaterialsError() }
function text(value: unknown, bytes: number, empty = false): value is string {
  if (typeof value !== 'string' || (!empty && !value) || value.trim() !== value || /[\u0000-\u001f\u007f-\u009f]/.test(value)) return false
  try { return encodeURIComponent(value).replace(/%[0-9A-F]{2}/g, 'x').length <= bytes } catch { return false }
}
function idValid(value: unknown): value is string { return typeof value === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value) && value !== '00000000-0000-0000-0000-000000000000' }
function timestamp(value: unknown): value is string {
  if (!text(value, 80) || !/^\d{4}-\d{2}-\d{2}T(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:\.\d{1,9})?(?:Z|[+-](?:[01]\d|2[0-3]):[0-5]\d)$/.test(value) || !Number.isFinite(Date.parse(value))) return false
  const day = value.slice(0, 10)
  return new Date(day + 'T00:00:00Z').toISOString().slice(0, 10) === day
}
function timeOrder(left: string, right: string) {
  const seconds = Math.floor(Date.parse(left) / 1000) - Math.floor(Date.parse(right) / 1000)
  const fraction = (value: string) => Number((value.match(/\.(\d{1,9})/)?.[1] ?? '').padEnd(9, '0'))
  return seconds || fraction(left) - fraction(right)
}
function cursorValid(value: unknown): value is string { return text(value, 1024, true) && /^[A-Za-z0-9_-]*$/.test(value) }
function scopeSnapshot(input: MaterialScope) {
  if (!record(input) || Object.keys(input).some(key => !['platform', 'type', 'terminal', 'positionId', 'scene', 'cityCode', 'business'].includes(key))) invalid()
  const scope = { ...input }
  if (typeof scope.platform !== 'string' || !Object.prototype.hasOwnProperty.call(platforms, scope.platform)) invalid()
  const platform = platforms[scope.platform]
  if (!platform || !['PRODUCT', 'ACTIVITY'].includes(scope.type) || (platform === 'MT' && scope.type !== 'ACTIVITY') || !['H5', 'WX_MINI'].includes(scope.terminal) ||
    !text(scope.positionId, 128) || !text(scope.scene, 80) || !text(scope.cityCode === undefined ? '' : scope.cityCode, 32, true) || !text(scope.business === undefined ? '' : scope.business, 40, true)) invalid()
  return { scope, platform }
}
function capability(value: unknown): CatalogCapability {
  if (!record(value) || typeof value.allowed !== 'boolean' || typeof value.reason !== 'string' || (value.allowed ? value.reason !== 'READY' : !deniedCapabilities.includes(value.reason))) invalid()
  return { allowed: value.allowed, reason: value.reason }
}
function card(value: unknown, scope: MaterialScope, platform: MaterialCard['platform']): MaterialCard {
  if (!record(value) || !idValid(value.id) || value.platform !== platform || value.type !== scope.type || !text(value.title, 256) || !text(value.ruleVersion, 80) || !timestamp(value.endsAt) || !timestamp(value.sourceUpdatedAt) ||
    (value.startsAt !== undefined && !timestamp(value.startsAt)) || (value.type === 'ACTIVITY' && value.startsAt === undefined) || (typeof value.startsAt === 'string' && timeOrder(value.startsAt, value.endsAt) >= 0) ||
    !text(value.business, 40, true) || (value.business !== '' && value.business !== (scope.business ?? '')) || !record(value.region) || !Array.isArray(value.terminals) || value.terminals.length < 1 || value.terminals.length > 2 ||
    !value.terminals.every(t => t === 'H5' || t === 'WX_MINI') || new Set(value.terminals).size !== value.terminals.length || !value.terminals.includes(scope.terminal)) invalid()
  const cities = value.region.cityCodes === undefined ? [] : value.region.cityCodes
  if (!Array.isArray(cities) || !cities.every(code => text(code, 32)) || new Set(cities).size !== cities.length ||
    (value.region.mode === 'NATIONWIDE' ? cities.length !== 0 : value.region.mode !== 'CITIES' || cities.length < 1 || cities.length > 64 || !cities.includes(scope.cityCode ?? ''))) invalid()
  return { id: value.id, platform, type: scope.type, title: value.title, ...(typeof value.startsAt === 'string' ? { startsAt: value.startsAt } : {}), endsAt: value.endsAt, sourceUpdatedAt: value.sourceUpdatedAt, ruleVersion: value.ruleVersion,
    region: { mode: value.region.mode as 'NATIONWIDE' | 'CITIES', ...(cities.length ? { cityCodes: [...cities] as string[] } : {}) }, business: value.business, terminals: [...value.terminals] as MaterialCard['terminals'] }
}
function query(scope: MaterialScope, platform: string) {
  const values: [string, string][] = [['platform', platform], ['type', scope.type], ['terminal', scope.terminal], ['positionId', scope.positionId], ['scene', scope.scene]]
  if (scope.cityCode) values.push(['cityCode', scope.cityCode]); if (scope.business) values.push(['business', scope.business])
  return values
}
function url(path: string, values: [string, string][]) { return path + '?' + values.map(([key, value]) => `${key}=${encodeURIComponent(value)}`).join('&') }
const uniRequest: MaterialRequest = (url, token) => new Promise((resolve, reject) => {
  uni.request({ url, method: 'GET', timeout: 10000, header: { Authorization: `Bearer ${token}` }, success: response => resolve({ statusCode: response.statusCode, data: response.data }), fail: () => reject(new MaterialsError()) })
})
export function createMaterialsAPI(session: () => string | null, request: MaterialRequest = uniRequest) {
  function token() {
    let value: unknown; try { value = session() } catch { throw new MaterialsError('login-required') }
    if (typeof value !== 'string' || !value || value.length > 8192 || !/^[A-Za-z0-9._~+/-]+=*$/.test(value)) throw new MaterialsError('login-required')
    return value
  }
  async function read(path: string) {
    const supplied = token()
    let response: Awaited<ReturnType<MaterialRequest>>
    try { response = await request(path, supplied) } catch { if (token() !== supplied) throw new MaterialsError('login-required'); throw new MaterialsError() }
    if (token() !== supplied) throw new MaterialsError('login-required')
    if (!record(response)) invalid()
    if (response.statusCode === 401) throw new MaterialsError('login-required')
    if (response.statusCode === 404) throw new MaterialsError('unavailable')
    if (response.statusCode !== 200 || !record(response.data) || response.data.code !== 0 || !('data' in response.data)) invalid()
    return response.data.data
  }
  return {
    async list(input: MaterialScope, cursor = ''): Promise<MaterialPage> {
      const { scope, platform } = scopeSnapshot(input)
      if (!cursorValid(cursor)) invalid()
      const values = query(scope, platform); values.push(['limit', '20']); if (cursor) values.push(['cursor', cursor])
      const data = await read(url('/api/v1/promoter/materials', values))
      if (!record(data) || !Array.isArray(data.items) || data.items.length > 20) invalid()
      const cap = capability(data.capability), next = data.nextCursor === undefined ? '' : data.nextCursor
      if (!cursorValid(next) || (!data.items.length && next !== '') || (!cap.allowed && (data.items.length || next))) invalid()
      const items = data.items.map(item => card(item, scope, platform))
      if (new Set(items.map(item => item.id)).size !== items.length) invalid()
      return { items, nextCursor: next, capability: cap }
    },
    async detail(id: string, input: MaterialScope): Promise<MaterialDetail> {
      const { scope, platform } = scopeSnapshot(input)
      if (!idValid(id)) invalid()
      const data = await read(url('/api/v1/promoter/materials/' + id, query(scope, platform)))
      if (!record(data) || !record(data.availability) || typeof data.availability.available !== 'boolean' || typeof data.availability.reason !== 'string') invalid()
      const cap = capability(data.capability), available = data.availability.available, reason = data.availability.reason
      if (available ? !cap.allowed || reason !== 'AVAILABLE' || data.item === undefined : data.item !== undefined || (cap.allowed ? !deniedMaterials.includes(reason) : reason !== 'CAPABILITY_UNAVAILABLE')) invalid()
      const item = available ? card(data.item, scope, platform) : undefined
      if (item && item.id !== id) invalid()
      return { ...(item ? { item } : {}), capability: cap, availability: { available, reason } }
    },
  }
}
