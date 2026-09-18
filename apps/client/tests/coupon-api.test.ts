import { afterEach, expect, it, vi } from 'vitest'
import { createCouponAPI } from '../src/features/coupons/api'

const item = {
  id: 'MT:coupon-1', platform: 'MT', claimMode: 'PLATFORM_ACTIVITY', actionLabel: '去平台领取 / 购买',
  title: '测试接口券', scope: 'ACTIVITY', scopeExternalId: '', scopeName: '', currency: 'CNY',
  discountMinor: 500, thresholdMinor: 2000, cityCode: '110100', cityName: '北京', business: '',
  ruleVersion: 'v1', updatedAt: '2026-09-18T00:00:00Z', expiresAt: '2030-01-01T00:00:00Z',
}
const reply = (data: unknown) => ({ statusCode: 200, data: { code: 0, message: 'success', data } })
afterEach(() => vi.unstubAllGlobals())

it('maps UI platforms to the existing service contract and encodes context and cursor', async () => {
  const request = vi.fn(async () => reply({ items: [], nextCursor: '' }))
  const api = createCouponAPI(request)
  expect(await api.list({ platform: 'MEITUAN', cityCode: '110100', business: '外卖' }, 'a+b/=')).toEqual({ items: [], nextCursor: '' })
  expect(request).toHaveBeenCalledExactlyOnceWith('/api/v1/coupons?platform=MT&cityCode=110100&business=%E5%A4%96%E5%8D%96&limit=20&cursor=a%2Bb%2F%3D')
})
it.each([['JD', 'JD'], ['TAOBAO', 'TB']] as const)('reads cities for %s using backend platform %s', async (platform, code) => {
  const request = vi.fn(async () => reply({ items: [{ code: '110100', name: '北京' }] }))
  expect(await createCouponAPI(request).cities(platform)).toEqual([{ code: '110100', name: '北京' }])
  expect(request).toHaveBeenCalledExactlyOnceWith(`/api/v1/coupon-cities?platform=${code}`)
})
it('reads a detail in the current city/business context without unsupported platform query', async () => {
  const request = vi.fn(async () => reply(item))
  expect(await createCouponAPI(request).detail({ platform: 'MEITUAN', cityCode: '110100' }, 'MT:coupon-1')).toEqual(item)
  expect(request).toHaveBeenCalledExactlyOnceWith('/api/v1/coupons/MT%3Acoupon-1?cityCode=110100')
})
it.each(['PDD', 'ELEME'] as const)('does not call any service for unopened %s', async platform => {
  const request = vi.fn(async () => reply({ items: [], nextCursor: '' }))
  const api = createCouponAPI(request)
  for (const call of [() => api.list({ platform }), () => api.cities(platform), () => api.detail({ platform }, 'MT:coupon-1')]) {
    await expect(call()).rejects.toMatchObject({ kind: 'unopened' })
  }
  expect(request).not.toHaveBeenCalled()
})
it.each([400, 401, 404, 503])('does not interpret HTTP %s as catalog success or disclose server messages', async statusCode => {
  const api = createCouponAPI(async () => ({ statusCode, data: { code: 'COUPONS_UNAVAILABLE', message: 'private database failure' } }))
  await expect(api.list({ platform: 'JD' })).rejects.toMatchObject({ kind: statusCode === 404 ? 'unavailable' : 'error' })
  await expect(api.list({ platform: 'JD' })).rejects.not.toThrow('private database failure')
})
it.each([
  { items: [item], nextCursor: '' }, // wrong platform
  { items: [], nextCursor: 3 },
  { items: null, nextCursor: '' },
  { items: [{ ...item, platform: 'JD', id: 'JD:coupon-1', discountMinor: -1 }], nextCursor: '' },
  { items: [{ ...item, platform: 'JD', id: 'JD:coupon-1', expiresAt: 'invalid' }], nextCursor: '' },
  { items: [{ ...item, platform: 'JD', id: 'JD:coupon-1', claimMode: 'UNKNOWN' }], nextCursor: '' },
])('rejects malformed or mismatched catalog data instead of displaying it', async data => {
  await expect(createCouponAPI(async () => reply(data)).list({ platform: 'JD' })).rejects.toMatchObject({ kind: 'error' })
})
it('rejects a detail for a different coupon', async () => {
  await expect(createCouponAPI(async () => reply(item)).detail({ platform: 'MEITUAN', cityCode: '110100' }, 'MT:other')).rejects.toMatchObject({ kind: 'error' })
})
it('rejects wrong city and business material even on an HTTP success', async () => {
  await expect(createCouponAPI(async () => reply({ items: [item], nextCursor: '' })).list({ platform: 'MEITUAN', cityCode: '310100' })).rejects.toMatchObject({ kind: 'error' })
})
it.each([null, { code: 0 }, { code: '0', data: { items: [], nextCursor: '' } }])('rejects an invalid service envelope', async data => {
  await expect(createCouponAPI(async () => ({ statusCode: 200, data })).list({ platform: 'JD' })).rejects.toMatchObject({ kind: 'error' })
})
it('does not send invalid context or an invalid detail identifier', async () => {
  const request = vi.fn(async () => reply(item))
  const api = createCouponAPI(request)
  await expect(api.list({ platform: 'JD', cityCode: '\n' })).rejects.toMatchObject({ kind: 'error' })
  await expect(api.detail({ platform: 'JD' }, '\n')).rejects.toMatchObject({ kind: 'error' })
  expect(request).not.toHaveBeenCalled()
})
it.each([{ ...item, claimMode: ['PLATFORM_ACTIVITY'] }, { ...item, scope: ['ACTIVITY'] }])('rejects non-string enums even when string coercion would match', async data => {
  await expect(createCouponAPI(async () => reply(data)).detail({ platform: 'MEITUAN', cityCode: '110100' }, 'MT:coupon-1')).rejects.toMatchObject({ kind: 'error' })
})
it('accepts existing GET contract identifiers and optional scope/city names', async () => {
  const material = { ...item, id: 'city-coupon', scope: 'SHOP', scopeExternalId: 'shop-1', scopeName: '', cityName: '' }
  expect(await createCouponAPI(async () => reply(material)).detail({ platform: 'MEITUAN', cityCode: '110100' }, 'city-coupon')).toEqual(material)
  expect(await createCouponAPI(async () => reply({ items: [material], nextCursor: '' })).list({ platform: 'MEITUAN', cityCode: '110100' })).toEqual({ items: [material], nextCursor: '' })
})
it('distinguishes a missing/expired coupon from a missing API route', async () => {
  const api = createCouponAPI(async () => ({ statusCode: 404, data: { code: 'COUPON_NOT_FOUND', message: 'coupon not found' } }))
  await expect(api.detail({ platform: 'JD' }, 'JD:not-found')).rejects.toMatchObject({ kind: 'not-found' })
})
it('uses uni-app GET transport with a bounded timeout and no fabricated authorization', async () => {
  const request = vi.fn(options => { options.success(reply({ items: [], nextCursor: '' })); return {} })
  vi.stubGlobal('uni', { request })
  expect(await createCouponAPI().list({ platform: 'JD' })).toEqual({ items: [], nextCursor: '' })
  expect(request.mock.calls[0][0]).toMatchObject({ url: '/api/v1/coupons?platform=JD&limit=20', method: 'GET', timeout: 10000 })
  expect(request.mock.calls[0][0].header).toBeUndefined()
})
it('turns network failure into a sanitized error', async () => {
  await expect(createCouponAPI(async () => { throw new Error('private network detail') }).cities('JD')).rejects.toMatchObject({ kind: 'error' })
})
it.each([
  { ...item, expiresAt: '2020-01-01T00:00:00Z' },
  { ...item, actionLabel: '领取成功' },
])('does not accept expired material or a claim label inconsistent with its mode', async data => {
  await expect(createCouponAPI(async () => reply(data)).detail({ platform: 'MEITUAN', cityCode: '110100' }, 'MT:coupon-1')).rejects.toMatchObject({ kind: 'error' })
})
it.each([{ items: [{ code: '', name: '北京' }] }, { items: [{ code: '110100', name: '' }] }, { items: [{ code: '110100', name: '北京' }, { code: '110100', name: '另一名称' }] }])('rejects unusable city choices', async data => {
  await expect(createCouponAPI(async () => reply(data)).cities('JD')).rejects.toMatchObject({ kind: 'error' })
})
it('handles an actual uni-app request failure without exposing transport details', async () => {
  vi.stubGlobal('uni', { request: (options: any) => { options.fail({ errMsg: 'private timeout' }); return {} } })
  await expect(createCouponAPI().list({ platform: 'JD' })).rejects.toMatchObject({ kind: 'error' })
})
