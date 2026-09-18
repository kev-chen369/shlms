import type { OrderRequest } from './orders-api'
export type DashboardFilter = { from?: string; to?: string; channel?: string; positionId?: string }
export type PromoterCounts = { timeZone: 'Asia/Shanghai'; from: string; toExclusive: string; asOf: string; successfulLinks: number; copyReports: number; validOrders: number }
export class DashboardError extends Error {
  constructor(public readonly kind: 'login-required' | 'unavailable' | 'error' = 'error') {
    super(kind === 'login-required' ? '请登录后查看本人推广统计' : kind === 'unavailable' ? '推广统计服务暂不可用' : '推广统计读取失败，请稍后重试')
  }
}
const DAY = 86400000
function record(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null && !Array.isArray(value) }
function day(value: unknown): value is string {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const parsed = new Date(value + 'T00:00:00Z')
  return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value
}
const calendarStart = (value: string) => Date.parse(value + 'T00:00:00Z')
function timestamp(value: unknown): value is string {
  return typeof value === 'string' && /^\d{4}-\d{2}-\d{2}T(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/.test(value) && day(value.slice(0, 10)) && Number.isFinite(Date.parse(value))
}
function localMidnight(value: string, formatter: Intl.DateTimeFormat): string | null {
  if (/[1-9]/.test(value.match(/\.(\d+)/)?.[1] || '')) return null
  const parts = formatter.formatToParts(new Date(value)), part = (name: string) => parts.find(item => item.type === name)?.value || ''
  if (part('hour') !== '00' || part('minute') !== '00' || part('second') !== '00') return null
  const year = part('era') === 'BC' ? 1 - Number(part('year')) : Number(part('year'))
  return `${String(year).padStart(4, '0')}-${part('month')}-${part('day')}`
}
function position(value: unknown): value is string {
  if (typeof value !== 'string' || value.length > 128 || value.trim() !== value || /[\x00\r\n]/.test(value)) return false
  try { encodeURIComponent(value); return true } catch { return false }
}
function parse(value: unknown, filter: DashboardFilter): PromoterCounts {
  if (!record(value) || value.timeZone !== 'Asia/Shanghai' || !timestamp(value.from) || !timestamp(value.toExclusive) || !timestamp(value.asOf) ||
    !['successfulLinks', 'copyReports', 'validOrders'].every(key => Number.isSafeInteger(value[key]) && Number(value[key]) >= 0)) throw new DashboardError()
  let fromDay: string | null, toDay: string | null
  try {
    const formatter = new Intl.DateTimeFormat('en', { timeZone: 'Asia/Shanghai', calendar: 'gregory', numberingSystem: 'latn', hourCycle: 'h23', era: 'short', year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' })
    if (formatter.resolvedOptions().timeZone !== 'Asia/Shanghai') throw new Error()
    fromDay = localMidnight(value.from, formatter); toDay = localMidnight(value.toExclusive, formatter)
  } catch { throw new DashboardError('unavailable') }
  const duration = Date.parse(value.toExclusive) - Date.parse(value.from)
  if (!fromDay || !toDay || duration <= 0 || duration > 366 * DAY || (filter.from && fromDay !== filter.from) ||
    (filter.to && calendarStart(toDay) !== calendarStart(filter.to) + DAY) || (!filter.from && !filter.to && (calendarStart(toDay) - calendarStart(fromDay)) / DAY !== 30)) throw new DashboardError()
  return { timeZone: 'Asia/Shanghai', from: value.from, toExclusive: value.toExclusive, asOf: value.asOf, successfulLinks: value.successfulLinks as number, copyReports: value.copyReports as number, validOrders: value.validOrders as number }
}
const uniRequest: OrderRequest = (url, token) => new Promise((resolve, reject) => {
  uni.request({ url, method: 'GET', timeout: 10000, header: { Authorization: `Bearer ${token}` }, success: response => resolve({ statusCode: response.statusCode, data: response.data }), fail: () => reject(new DashboardError()) })
})
export function createPromoterDashboardAPI(session: () => string | null, request: OrderRequest = uniRequest) {
  function token() {
    let value: unknown
    try { value = session() } catch { throw new DashboardError('login-required') }
    if (typeof value !== 'string' || !value.length || value.length > 8192 || !/^[A-Za-z0-9._~+/-]+=*$/.test(value)) throw new DashboardError('login-required')
    return value
  }
  return {
    async get(filter: DashboardFilter = {}): Promise<PromoterCounts> {
      if (!record(filter) || Object.keys(filter).some(key => !['from', 'to', 'channel', 'positionId'].includes(key))) throw new DashboardError()
      filter = { ...filter }
      if ((filter.from !== undefined && !day(filter.from)) || (filter.to !== undefined && !day(filter.to)) ||
        (filter.from && filter.to && (filter.from > filter.to || (calendarStart(filter.to) + DAY - calendarStart(filter.from)) / DAY > 366)) ||
        (filter.channel !== undefined && (typeof filter.channel !== 'string' || !['', 'JD', 'TB', 'MT'].includes(filter.channel))) ||
        (filter.positionId !== undefined && !position(filter.positionId))) throw new DashboardError()
      const values: string[] = []
      for (const key of ['from', 'to', 'channel', 'positionId'] as const) if (filter[key]) values.push(`${key}=${encodeURIComponent(filter[key])}`)
      const url = '/api/v1/promoter/dashboard' + (values.length ? '?' + values.join('&') : ''), supplied = token()
      let response: Awaited<ReturnType<OrderRequest>>
      try { response = await request(url, supplied) } catch { if (token() !== supplied) throw new DashboardError('login-required'); throw new DashboardError() }
      if (token() !== supplied) throw new DashboardError('login-required')
      if (!record(response)) throw new DashboardError()
      if (response.statusCode === 401) throw new DashboardError('login-required')
      if (response.statusCode === 404 || response.statusCode === 503) throw new DashboardError('unavailable')
      if (response.statusCode !== 200 || !record(response.data) || response.data.code !== 0 || !('data' in response.data)) throw new DashboardError()
      return parse(response.data.data, filter)
    },
  }
}
