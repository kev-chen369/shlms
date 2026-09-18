import { afterEach, expect, it, vi } from 'vitest'
import { createCouponAPI } from '../src/features/coupons/api'

const product = { externalProductId: 'sku-1', title: '适用商品', updatedAt: '2026-09-18T00:00:00Z', expiresAt: '2030-01-01T00:00:00Z' }
const reply = (data: unknown) => ({ statusCode: 200, data: { code: 0, message: 'success', data } })
afterEach(() => vi.unstubAllGlobals())

it('reads applicable product identities using the encoded coupon and current context', async () => {
  const request = vi.fn(async () => reply({ items: [product], nextCursor: 'next' }))
  const page = await createCouponAPI(request).products({ platform: 'TAOBAO', cityCode: '110100', business: '外卖' }, '券/1', 'a+b/=')
  expect(page).toEqual({ items: [product], nextCursor: 'next' })
  expect(request).toHaveBeenCalledExactlyOnceWith('/api/v1/coupons/%E5%88%B8%2F1/products?cityCode=110100&business=%E5%A4%96%E5%8D%96&limit=20&cursor=a%2Bb%2F%3D')
})
it('accepts an empty final page without inventing applicable products', async () => {
  expect(await createCouponAPI(async () => reply({ items: [], nextCursor: '' })).products({ platform: 'JD' }, 'coupon-1')).toEqual({ items: [], nextCursor: '' })
})
it.each([
  { items: [{ ...product, externalProductId: '' }], nextCursor: '' },
  { items: [{ ...product, title: '\nprivate' }], nextCursor: '' },
  { items: [{ ...product, updatedAt: 'invalid' }], nextCursor: '' },
  { items: [{ ...product, expiresAt: 'invalid' }], nextCursor: '' },
  { items: [{ ...product, expiresAt: '2000-01-01T00:00:00Z' }], nextCursor: '' },
  { items: [{ externalProductId: 'sku-1', title: '缺少日期' }], nextCursor: '' },
  { items: [product, product], nextCursor: '' },
  { items: Array.from({ length: 21 }, (_, i) => ({ ...product, externalProductId: `sku-${i}` })), nextCursor: '' },
  { items: [], nextCursor: 'next' },
  { items: [product], nextCursor: 3 },
  { items: null, nextCursor: '' },
])('rejects malformed, duplicate or expired applicable product pages', async data => {
  await expect(createCouponAPI(async () => reply(data)).products({ platform: 'JD' }, 'coupon-1')).rejects.toMatchObject({ kind: 'error' })
})
it.each(['PDD', 'ELEME'] as const)('does not request products for unopened %s', async platform => {
  const request = vi.fn(async () => reply({ items: [product], nextCursor: '' }))
  await expect(createCouponAPI(request).products({ platform }, 'coupon-1')).rejects.toMatchObject({ kind: 'unopened' })
  expect(request).not.toHaveBeenCalled()
})
it('rejects invalid identifiers, context and cursors before transport', async () => {
  const request = vi.fn(async () => reply({ items: [product], nextCursor: '' })), api = createCouponAPI(request)
  await expect(api.products({ platform: 'JD' }, '')).rejects.toMatchObject({ kind: 'error' })
  await expect(api.products({ platform: 'JD', business: '\n' }, 'c1')).rejects.toMatchObject({ kind: 'error' })
  await expect(api.products({ platform: 'JD' }, 'c1', 'x'.repeat(1001))).rejects.toMatchObject({ kind: 'error' })
  expect(request).not.toHaveBeenCalled()
})
it.each([
  [404, 'COUPON_NOT_FOUND', 'not-found'], [404, 'UNKNOWN_ROUTE', 'unavailable'], [503, 'INTERNAL', 'error'],
] as const)('classifies HTTP %s with %s without leaking internal messages', async (statusCode, code, kind) => {
  const api = createCouponAPI(async () => ({ statusCode, data: { code, message: 'private database details' } }))
  await expect(api.products({ platform: 'JD' }, 'c1')).rejects.toMatchObject({ kind })
  await expect(api.products({ platform: 'JD' }, 'c1')).rejects.not.toThrow('private database details')
})
it('uses bounded GET transport without fabricated authorization or claim operations', async () => {
  const request = vi.fn(options => { options.success(reply({ items: [product], nextCursor: '' })); return {} })
  vi.stubGlobal('uni', { request })
  expect((await createCouponAPI().products({ platform: 'JD' }, 'c1')).items[0].externalProductId).toBe('sku-1')
  expect(request.mock.calls[0][0]).toMatchObject({ url: '/api/v1/coupons/c1/products?limit=20', method: 'GET', timeout: 10000 })
  expect(request.mock.calls[0][0].header).toBeUndefined()
})
