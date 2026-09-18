import type { Platform } from '../platform'
export type Context = { platform: Platform; cityCode?: string; business?: string }
export type Request = (url: string) => Promise<{ statusCode: number; data: unknown }>
export type Coupon = {
  id: string; platform: 'JD' | 'TB' | 'MT'; claimMode: string; actionLabel: string;
  title: string; scope: string; scopeExternalId: string; scopeName: string; currency: 'CNY';
  discountMinor: number; thresholdMinor: number; cityCode: string; cityName: string; business: string;
  ruleVersion: string; updatedAt: string; expiresAt: string;
}
export type CouponPage = { items: Coupon[]; nextCursor: string }
export type City = { code: string; name: string }
export class CouponError extends Error {
  constructor(public readonly kind: 'unopened' | 'unavailable' | 'not-found' | 'error' = 'error') {
    super(kind === 'unopened' ? '该平台券目录尚未开放' : kind === 'unavailable' ? '券目录服务尚未接入' : kind === 'not-found' ? '该券不存在或已失效' : '券目录读取失败，请稍后重试')
  }
}
const platformCodes: Partial<Record<Platform, Coupon['platform']>> = { JD: 'JD', TAOBAO: 'TB', MEITUAN: 'MT' }
const actionLabels: Record<string, string> = {
  IN_SITE_VERIFIED: '立即领取', PLATFORM_CLAIM: '前往平台领券', BUNDLED_OFFER: '领券购买', PLATFORM_ACTIVITY: '去平台领取 / 购买',
}
function code(platform: Platform) {
  const value = platformCodes[platform]
  if (!value) throw new CouponError('unopened')
  return value
}
function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function text(value: unknown, max: number, allowEmpty = false): value is string {
  return typeof value === 'string' && value.length <= max && (allowEmpty || value.length > 0) && value.trim() === value && !/[\x00\r\n]/.test(value)
}
function query(context: Context) {
  if (!text(context.cityCode ?? '', 32, true) || !text(context.business ?? '', 40, true)) throw new CouponError()
  const values: [string, string][] = []
  if (context.cityCode) values.push(['cityCode', context.cityCode])
  if (context.business) values.push(['business', context.business])
  return values
}
function url(path: string, values: [string, string][]) {
  return path + (values.length ? '?' + values.map(([key, value]) => `${key}=${encodeURIComponent(value)}`).join('&') : '')
}
function coupon(value: unknown, context: Context): Coupon {
  const platform = code(context.platform)
  if (!record(value) || value.platform !== platform || !text(value.id, 128) ||
    !text(value.title, 256) || !text(value.ruleVersion, 80) || !text(value.actionLabel, 80) ||
    typeof value.claimMode !== 'string' || !['IN_SITE_VERIFIED', 'PLATFORM_CLAIM', 'BUNDLED_OFFER', 'PLATFORM_ACTIVITY'].includes(value.claimMode) ||
    typeof value.scope !== 'string' || !['PRODUCT', 'CATEGORY', 'SHOP', 'ACTIVITY'].includes(value.scope) || value.currency !== 'CNY' ||
    !Number.isSafeInteger(value.discountMinor) || Number(value.discountMinor) < 0 || !Number.isSafeInteger(value.thresholdMinor) || Number(value.thresholdMinor) < 0 ||
    !text(value.scopeExternalId, 128, true) || !text(value.scopeName, 256, true) ||
    !text(value.cityCode, 32, true) || !text(value.cityName, 80, true) || !text(value.business, 40, true) ||
    !text(value.updatedAt, 80) || !Number.isFinite(Date.parse(value.updatedAt)) || !text(value.expiresAt, 80) || !Number.isFinite(Date.parse(value.expiresAt)) ||
    Date.parse(value.expiresAt) <= Date.now() || value.actionLabel !== actionLabels[value.claimMode] ||
    (value.cityCode !== '' && value.cityCode !== (context.cityCode ?? '')) || (value.business !== '' && value.business !== (context.business ?? '')) ||
    (['SHOP', 'CATEGORY'].includes(value.scope) && !value.scopeExternalId)) throw new CouponError()
  return value as Coupon
}
const uniRequest: Request = url => new Promise((resolve, reject) => {
  uni.request({ url, method: 'GET', timeout: 10000, success: response => resolve({ statusCode: response.statusCode, data: response.data }), fail: () => reject(new CouponError()) })
})
export function createCouponAPI(request: Request = uniRequest) {
  async function read(path: string): Promise<unknown> {
    let response: Awaited<ReturnType<Request>>
    try { response = await request(path) } catch { throw new CouponError() }
    if (response.statusCode === 404) throw new CouponError(record(response.data) && response.data.code === 'COUPON_NOT_FOUND' ? 'not-found' : 'unavailable')
    if (response.statusCode !== 200 || !record(response.data) || response.data.code !== 0 || !('data' in response.data)) throw new CouponError()
    return response.data.data
  }
  return {
    async list(context: Context, cursor = ''): Promise<CouponPage> {
      const values: [string, string][] = [['platform', code(context.platform)], ...query(context), ['limit', '20']]
      if (!text(cursor, 1000, true)) throw new CouponError()
      if (cursor) values.push(['cursor', cursor])
      const data = await read(url('/api/v1/coupons', values))
      if (!record(data) || !Array.isArray(data.items) || data.items.length > 20 || !text(data.nextCursor, 1000, true)) throw new CouponError()
      const items = data.items.map(item => coupon(item, context))
      if (new Set(items.map(item => item.id)).size !== items.length || (items.length === 0 && data.nextCursor !== '')) throw new CouponError()
      return { items, nextCursor: data.nextCursor }
    },
    async cities(platform: Platform): Promise<City[]> {
      const data = await read(url('/api/v1/coupon-cities', [['platform', code(platform)]]))
      if (!record(data) || !Array.isArray(data.items) || !data.items.every(item => record(item) && text(item.code, 32) && text(item.name, 80))) throw new CouponError()
      const items = data.items as City[]
      if (new Set(items.map(item => item.code)).size !== items.length) throw new CouponError()
      return items
    },
    async detail(context: Context, id: string): Promise<Coupon> {
      code(context.platform)
      if (!text(id, 128)) throw new CouponError()
      const values = query(context)
      const item = coupon(await read(url('/api/v1/coupons/' + encodeURIComponent(id), values)), context)
      if (item.id !== id) throw new CouponError()
      return item
    },
  }
}
