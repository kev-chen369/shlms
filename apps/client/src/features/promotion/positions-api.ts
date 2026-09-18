import type { MaterialRequest } from './materials-api'

export type PositionChoice = { id: string; name: string; scene: string; isDefault: boolean; channels: { channel: 'JD'; readiness: 'WAITING_CONFIGURATION' | 'WAITING_VERIFICATION' | 'UNAVAILABLE' }[] }
export class PositionsError extends Error {
  constructor(public readonly kind: 'login-required' | 'unavailable' | 'error' = 'error') {
    super(kind === 'login-required' ? '请登录后选择本人推广位' : kind === 'unavailable' ? '推广位服务尚未接入' : '推广位读取失败，请稍后重试')
  }
}
function invalid(): never { throw new PositionsError() }
function record(value: unknown): value is Record<string, unknown> { return value !== null && typeof value === 'object' && !Array.isArray(value) }
function text(value: unknown, max: number): value is string {
  if (typeof value !== 'string' || !value.trim() || /[\u0000-\u001f\u007f-\u009f]/.test(value)) return false
  try { encodeURIComponent(value); return [...value].length <= max } catch { return false }
}
function cursor(value: unknown): value is string { return typeof value === 'string' && value.length <= 2048 && /^[A-Za-z0-9_-]*$/.test(value) }
function choice(value: unknown): PositionChoice {
  if (!record(value) || !text(value.id, 256) || !text(value.name, 80) || !text(value.scene, 80) || value.status !== 'ENABLED' || typeof value.isDefault !== 'boolean' || value.canConvert !== false || !Array.isArray(value.channels) || value.channels.length !== 1) invalid()
  // Current public position endpoint exposes JD only. Unknown/new grants require
  // a separately reviewed contract, never inferred from internal position existence.
  const mapping = value.channels[0]
  if (!record(mapping) || mapping.channel !== 'JD' || typeof mapping.readiness !== 'string' || !['WAITING_CONFIGURATION', 'WAITING_VERIFICATION', 'UNAVAILABLE'].includes(mapping.readiness)) invalid()
  return { id: value.id, name: value.name, scene: value.scene, isDefault: value.isDefault, channels: [{ channel: 'JD', readiness: mapping.readiness as PositionChoice['channels'][number]['readiness'] }] }
}
const uniRequest: MaterialRequest = (url, token) => new Promise((resolve, reject) => {
  uni.request({ url, method: 'GET', timeout: 10000, header: { Authorization: `Bearer ${token}` }, success: response => resolve({ statusCode: response.statusCode, data: response.data }), fail: () => reject(new PositionsError()) })
})
export function createPositionsAPI(session: () => string | null, request: MaterialRequest = uniRequest) {
  function token() {
    let value: unknown
    try { value = session() } catch { throw new PositionsError('login-required') }
    if (typeof value !== 'string' || !value || value.length > 8192 || !/^[A-Za-z0-9._~+/-]+=*$/.test(value)) throw new PositionsError('login-required')
    return value
  }
  return {
    async list(after = ''): Promise<{ items: PositionChoice[]; nextCursor: string }> {
      if (!cursor(after)) invalid()
      const supplied = token()
      const url = '/api/v1/promotion-positions?status=ENABLED&limit=20' + (after ? '&cursor=' + encodeURIComponent(after) : '')
      let response: Awaited<ReturnType<MaterialRequest>>
      try { response = await request(url, supplied) } catch { if (token() !== supplied) throw new PositionsError('login-required'); throw new PositionsError() }
      if (token() !== supplied) throw new PositionsError('login-required')
      if (!record(response)) invalid()
      if (response.statusCode === 401) throw new PositionsError('login-required')
      if (response.statusCode === 404) throw new PositionsError('unavailable')
      if (response.statusCode !== 200 || !record(response.data) || response.data.code !== 0 || !record(response.data.data)) invalid()
      const data = response.data.data, next = data.nextCursor === undefined ? '' : data.nextCursor
      if (!Array.isArray(data.items) || data.items.length > 20 || !cursor(next) || (!data.items.length && next)) invalid()
      const items = data.items.map(choice)
      if (new Set(items.map(item => item.id)).size !== items.length) invalid()
      return { items, nextCursor: next }
    },
  }
}
