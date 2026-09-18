export type OrderRequest = (url: string, token: string) => Promise<{ statusCode: number; data: unknown }>
export type OrderFilter = { channel?: string; positionId?: string; orderStatus?: string; from?: string; to?: string }
export type PromoterOrder = { id: string; channel: string; maskedOrderId: string; orderStatus: string; positionId: string; attributionMethod: string; attributedAt: string; orderOccurredAt: string; statusAt: string }
export type OrderHistory = { previousStatus: string; status: string; occurredAt: string; projectedAt: string }
export type PromoterOrderDetail = PromoterOrder & { history: OrderHistory[]; refundEventCount: number }
export type PromoterOrderPage = { items: PromoterOrder[]; nextCursor: string }
export class OrdersError extends Error {
  constructor(public readonly kind: 'login-required' | 'not-found' | 'unavailable' | 'error' = 'error') {
    super(kind === 'login-required' ? '请登录后查看本人推广订单' : kind === 'not-found' ? '该订单不存在或不可查看' : kind === 'unavailable' ? '推广订单服务尚未接入' : '推广订单读取失败，请稍后重试')
  }
}
const statuses = ['CREATED', 'PAID', 'CONFIRMED', 'COMMISSION_CONFIRMED', 'SETTLEMENT_PENDING', 'SETTLED', 'CANCELLED', 'INVALID', 'REFUNDED']
const publicID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/
function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function text(value: unknown, max: number, empty = false): value is string {
  if (typeof value !== 'string' || value.length > max || (!empty && !value.length) || value.trim() !== value || /[\x00\r\n]/.test(value)) return false
  try { encodeURIComponent(value); return true } catch { return false }
}
function timestamp(value: unknown): value is string {
  if (!text(value, 80) || !/^\d{4}-\d{2}-\d{2}T(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(value) || !Number.isFinite(Date.parse(value))) return false
  const day = value.slice(0, 10), parsed = new Date(day + 'T00:00:00Z')
  return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 10) === day
}
function cursorValid(value: unknown): value is string { return text(value, 134, true) && /^[A-Za-z0-9_-]*$/.test(value) }
export function compareTime(left: string, right: string) {
  const seconds = Math.floor(Date.parse(left) / 1000) - Math.floor(Date.parse(right) / 1000)
  const fraction = (value: string) => Number((value.match(/\.(\d{1,9})/)?.[1] ?? '').padEnd(9, '0'))
  return seconds || fraction(left) - fraction(right)
}
function status(value: unknown): value is string { return typeof value === 'string' && statuses.includes(value) }
function parseOrder(value: unknown, filter: OrderFilter = {}): PromoterOrder {
  if (!record(value) || typeof value.id !== 'string' || !publicID.test(value.id) || typeof value.channel !== 'string' || !['JD', 'TB', 'MT'].includes(value.channel) ||
    !text(value.maskedOrderId, 16) || !value.maskedOrderId.startsWith('****') || ![4, 8].includes(Array.from(value.maskedOrderId).length) ||
    !status(value.orderStatus) || !text(value.positionId, 128) || typeof value.attributionMethod !== 'string' || !['SUB_ID', 'LINK_REQUEST', 'CHANNEL_POSITION'].includes(value.attributionMethod) ||
    !timestamp(value.attributedAt) || !timestamp(value.orderOccurredAt) || !timestamp(value.statusAt) ||
    (filter.channel && value.channel !== filter.channel) || (filter.positionId && value.positionId !== filter.positionId) || (filter.orderStatus && value.orderStatus !== filter.orderStatus) ||
    (filter.from && compareTime(value.orderOccurredAt, filter.from) < 0) || (filter.to && compareTime(value.orderOccurredAt, filter.to) >= 0)) throw new OrdersError()
  return { id: value.id, channel: value.channel, maskedOrderId: value.maskedOrderId, orderStatus: value.orderStatus, positionId: value.positionId, attributionMethod: value.attributionMethod, attributedAt: value.attributedAt, orderOccurredAt: value.orderOccurredAt, statusAt: value.statusAt }
}
const uniRequest: OrderRequest = (url, token) => new Promise((resolve, reject) => {
  uni.request({ url, method: 'GET', timeout: 10000, header: { Authorization: `Bearer ${token}` }, success: response => resolve({ statusCode: response.statusCode, data: response.data }), fail: () => reject(new OrdersError()) })
})
export function createPromoterOrdersAPI(session: () => string | null, request: OrderRequest = uniRequest) {
  function token() {
    let value: unknown
    try { value = session() } catch { throw new OrdersError('login-required') }
    if (typeof value !== 'string' || !value.length || value.length > 8192 || !/^[A-Za-z0-9._~+/-]+=*$/.test(value)) throw new OrdersError('login-required')
    return value
  }
  async function read(url: string) {
    const supplied = token()
    let response: Awaited<ReturnType<OrderRequest>>
    try { response = await request(url, supplied) } catch { if (token() !== supplied) throw new OrdersError('login-required'); throw new OrdersError() }
    if (token() !== supplied) throw new OrdersError('login-required')
    if (response.statusCode === 401) throw new OrdersError('login-required')
    if (response.statusCode === 404) throw new OrdersError(record(response.data) && response.data.code === 'ORDER_NOT_FOUND' ? 'not-found' : 'unavailable')
    if (response.statusCode !== 200 || !record(response.data) || response.data.code !== 0 || !('data' in response.data)) throw new OrdersError()
    return response.data.data
  }
  return {
    async list(filter: OrderFilter = {}, cursor = ''): Promise<PromoterOrderPage> {
      if (!record(filter)) throw new OrdersError()
      filter = { ...filter }
      for (const key of ['channel', 'positionId', 'orderStatus', 'from', 'to'] as const) if (filter[key] !== undefined && !text(filter[key], 128, true)) throw new OrdersError()
      if ((filter.channel && !['JD', 'TB', 'MT'].includes(filter.channel)) || (filter.positionId && !text(filter.positionId, 128)) ||
        (filter.orderStatus && !status(filter.orderStatus)) || (filter.from && !timestamp(filter.from)) || (filter.to && !timestamp(filter.to)) ||
        (filter.from && filter.to && compareTime(filter.from, filter.to) >= 0) || !cursorValid(cursor)) throw new OrdersError()
      const values: [string, string][] = []
      for (const key of ['channel', 'positionId', 'orderStatus', 'from', 'to'] as const) if (filter[key]) values.push([key, filter[key]])
      values.push(['limit', '20']); if (cursor) values.push(['cursor', cursor])
      const data = await read('/api/v1/promoter/orders?' + values.map(([key, value]) => `${key}=${encodeURIComponent(value)}`).join('&'))
      if (!record(data) || !Array.isArray(data.items) || data.items.length > 20 || !cursorValid(data.nextCursor)) throw new OrdersError()
      const items = data.items.map(item => parseOrder(item, filter))
      if (new Set(items.map(item => item.id)).size !== items.length || (!items.length && data.nextCursor !== '')) throw new OrdersError()
      return { items, nextCursor: data.nextCursor }
    },
    async detail(id: string): Promise<PromoterOrderDetail> {
      if (typeof id !== 'string' || !publicID.test(id)) throw new OrdersError()
      const data = await read('/api/v1/promoter/orders/' + id), item = parseOrder(data)
      if (item.id !== id || !record(data) || !Array.isArray(data.history) || !Number.isSafeInteger(data.refundEventCount) || Number(data.refundEventCount) < 0) throw new OrdersError()
      const history = data.history.map(change => {
        if (!record(change) || (change.previousStatus !== '' && !status(change.previousStatus)) || !status(change.status) || !timestamp(change.occurredAt) || !timestamp(change.projectedAt)) throw new OrdersError()
        return { previousStatus: change.previousStatus as string, status: change.status, occurredAt: change.occurredAt, projectedAt: change.projectedAt }
      })
      return { ...item, history, refundEventCount: data.refundEventCount as number }
    },
  }
}
