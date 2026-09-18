import { afterEach, expect, it, vi } from 'vitest'
import { createCouponAPI, type Request } from '../src/features/coupons/api'
import { createCouponCatalog } from '../src/features/coupons/catalog'

const coupon = { id: 'c1', platform: 'JD', claimMode: 'BUNDLED_OFFER', actionLabel: '领券购买', title: '商品券', scope: 'PRODUCT', scopeExternalId: '', scopeName: '', currency: 'CNY', discountMinor: 500, thresholdMinor: 2000, cityCode: '', cityName: '', business: '', ruleVersion: 'v1', updatedAt: '2026-09-18T00:00:00Z', expiresAt: '2030-01-01T00:00:00Z' }
const product = { externalProductId: 'sku-1', title: '适用商品一', updatedAt: '2026-09-18T00:00:00Z', expiresAt: '2028-01-01T00:00:00Z' }
afterEach(() => vi.useRealTimers())
function harness() {
  const pending: { url: string; resolve: (value: Awaited<ReturnType<Request>>) => void }[] = []
  const request = vi.fn((url: string) => new Promise<Awaited<ReturnType<Request>>>(resolve => pending.push({ url, resolve })))
  const catalog = createCouponCatalog(createCouponAPI(request))
  const complete = (index: number, data: unknown, statusCode = 200) => pending[index].resolve({ statusCode, data: statusCode === 200 ? { code: 0, message: 'success', data } : data })
  const settle = async () => { for (let i = 0; i < 12; i++) await Promise.resolve() }
  const ready = async () => { catalog.selectPlatform('JD'); complete(1, { items: [coupon, { ...coupon, id: 'c2' }], nextCursor: '' }); await settle(); catalog.openDetail('c1'); complete(2, coupon); await settle() }
  return { catalog, request, pending, complete, settle, ready }
}
it('only requests products after the current detail has been validated', async () => {
  const { catalog, request, pending, ready, complete, settle } = harness()
  await catalog.refreshProducts(); expect(request).not.toHaveBeenCalled()
  await ready()
  expect(pending[3].url).toBe('/api/v1/coupons/c1/products?limit=20')
  expect(catalog.state.productsStatus).toBe('loading')
  complete(3, { items: [product], nextCursor: 'next' }); await settle()
  expect(catalog.state.products).toEqual([product])
  expect(catalog.state.productsStatus).toBe('ready')
})
it.each(['close', 'detail', 'city', 'platform'] as const)('discards pending products after %s changes', async change => {
  const { catalog, ready, complete, settle } = harness(); await ready()
  if (change === 'close') catalog.closeDetail()
  if (change === 'detail') catalog.openDetail('c2')
  if (change === 'city') catalog.setContext({ platform: 'JD', cityCode: '110100' })
  if (change === 'platform') catalog.selectPlatform('TAOBAO')
  expect(catalog.state.products).toEqual([]); expect(catalog.state.productsCursor).toBe('')
  complete(3, { items: [product], nextCursor: 'old' }); await settle()
  expect(catalog.state.products).toEqual([]); expect(catalog.state.productsCursor).toBe('')
})
it('preserves products and cursor on pagination failure and retries without parallel requests', async () => {
  const { catalog, ready, complete, settle, pending } = harness(); await ready()
  complete(3, { items: [product], nextCursor: 'next' }); await settle()
  catalog.loadMoreProducts(); catalog.loadMoreProducts(); expect(pending).toHaveLength(5)
  complete(4, { message: 'private failure' }, 503); await settle()
  expect(catalog.state.products).toEqual([product]); expect(catalog.state.productsCursor).toBe('next')
  expect(catalog.state.productsPageError).not.toContain('private')
  catalog.loadMoreProducts(); expect(pending[5].url).toBe('/api/v1/coupons/c1/products?limit=20&cursor=next')
  complete(5, { items: [{ ...product, externalProductId: 'sku-2' }], nextCursor: '' }); await settle()
  expect(catalog.state.products.map(p => p.externalProductId)).toEqual(['sku-1', 'sku-2'])
})
it.each(['duplicate', 'loop'] as const)('rejects %s pages without replacing good products', async kind => {
  const { catalog, ready, complete, settle } = harness(); await ready()
  complete(3, { items: [product], nextCursor: 'next' }); await settle(); catalog.loadMoreProducts()
  complete(4, { items: [kind === 'duplicate' ? product : { ...product, externalProductId: 'sku-2' }], nextCursor: kind === 'loop' ? 'next' : '' }); await settle()
  expect(catalog.state.products).toEqual([product]); expect(catalog.state.productsCursor).toBe('next')
  expect(catalog.state.productsPageError).not.toBe('')
})
it('hides expired products and preserves further pagination', async () => {
  const { catalog, ready, complete, settle } = harness(); await ready()
  complete(3, { items: [product], nextCursor: 'next' }); await settle()
  vi.useFakeTimers(); vi.setSystemTime(new Date('2029-01-01T00:00:00Z')); catalog.pruneExpired()
  expect(catalog.state.products).toEqual([]); expect(catalog.state.productsStatus).toBe('empty')
  expect(catalog.state.productsNotice).not.toBe(''); expect(catalog.state.productsCursor).toBe('next')
  catalog.loadMoreProducts(); complete(4, { items: [{ ...product, externalProductId: 'sku-2', expiresAt: '2030-01-01T00:00:00Z' }], nextCursor: '' }); await settle()
  expect(catalog.state.products[0].externalProductId).toBe('sku-2')
})
it('invalidates products and in-flight pagination when the coupon itself expires', async () => {
  const { catalog, ready, complete, settle } = harness(); await ready()
  complete(3, { items: [product], nextCursor: 'next' }); await settle(); catalog.loadMoreProducts()
  vi.useFakeTimers(); vi.setSystemTime(new Date('2030-01-01T00:00:00Z')); catalog.pruneExpired()
  complete(4, { items: [{ ...product, expiresAt: '2031-01-01T00:00:00Z' }], nextCursor: '' }); await settle()
  expect(catalog.state.detailStatus).toBe('not-found'); expect(catalog.state.products).toEqual([])
  expect(catalog.state.productsCursor).toBe('')
})
it('treats a products not-found as coupon revocation rather than an empty success', async () => {
  const { catalog, ready, complete, settle } = harness(); await ready()
  complete(3, { code: 'COUPON_NOT_FOUND', message: 'private' }, 404); await settle()
  expect(catalog.state.detailStatus).toBe('not-found'); expect(catalog.state.detail).toBeNull()
  expect(catalog.state.items.map(i => i.id)).toEqual(['c2']); expect(catalog.state.products).toEqual([])
})
it('does not restore a revoked coupon from an older pending catalog page', async () => {
  const { catalog, complete, settle } = harness(); catalog.selectPlatform('JD')
  complete(1, { items: [coupon], nextCursor: 'next-coupons' }); await settle()
  catalog.loadMore(); catalog.openDetail('c1'); complete(3, coupon); await settle()
  complete(4, { code: 'COUPON_NOT_FOUND' }, 404); await settle()
  complete(2, { items: [coupon], nextCursor: '' }); await settle()
  expect(catalog.state.items).toEqual([]); expect(catalog.state.status).toBe('empty')
  expect(catalog.state.nextCursor).toBe('next-coupons'); expect(catalog.state.loadingMore).toBe(false)
})
it('does not request a products endpoint for an activity or an unvalidated platform detail', async () => {
  const { catalog, complete, settle, pending } = harness(); catalog.selectPlatform('JD')
  complete(1, { items: [coupon], nextCursor: '' }); await settle(); catalog.openDetail('c1')
  complete(2, { ...coupon, scope: 'ACTIVITY' }); await settle(); expect(pending).toHaveLength(3)
  catalog.openDetail('c1'); complete(3, { ...coupon, platform: 'TB' }); await settle()
  expect(catalog.state.detailStatus).toBe('error'); expect(pending).toHaveLength(4)
})
it('refresh clears stale products and allows a clean retry after service error', async () => {
  const { catalog, ready, complete, settle } = harness(); await ready()
  complete(3, { items: [product], nextCursor: 'next' }); await settle(); catalog.refreshProducts()
  expect(catalog.state.products).toEqual([]); expect(catalog.state.productsCursor).toBe('')
  complete(4, { message: 'private' }, 503); await settle(); expect(catalog.state.productsStatus).toBe('error')
  catalog.refreshProducts(); complete(5, { items: [], nextCursor: '' }); await settle()
  expect(catalog.state.productsStatus).toBe('empty'); expect(catalog.state.productsError).toBe('')
})
