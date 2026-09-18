import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, expect, it } from 'vitest'
import CouponCatalog from '../src/components/CouponCatalog.vue'
import { createCouponAPI } from '../src/features/coupons/api'
import { createCouponCatalog, couponCatalogKey } from '../src/features/coupons/catalog'

const coupon = { id: 'c1', platform: 'JD', claimMode: 'BUNDLED_OFFER', actionLabel: '领券购买', title: '商品券', scope: 'PRODUCT', scopeExternalId: '', scopeName: '', currency: 'CNY', discountMinor: 500, thresholdMinor: 2000, cityCode: '', cityName: '', business: '', ruleVersion: 'v1', updatedAt: '2026-09-18T00:00:00Z', expiresAt: '2030-01-01T00:00:00Z' }
const product = { externalProductId: 'sku-1', title: '适用商品一', updatedAt: '2026-09-18T00:00:00Z', expiresAt: '2028-01-01T00:00:00Z' }
const wrappers: ReturnType<typeof mount>[] = []
afterEach(() => wrappers.splice(0).forEach(wrapper => wrapper.unmount()))
function harness() {
  let mode = 'ready'
  const catalog = createCouponCatalog(createCouponAPI(async url => {
    if (url.includes('/products') && mode === 'error') return { statusCode: 503, data: { message: 'private error' } }
    const data = url.includes('coupon-cities') ? { items: [] } : url.includes('/products') ? { items: mode === 'empty' ? [] : [{ ...product, externalProductId: url.includes('cursor=next') ? 'sku-2' : 'sku-1' }], nextCursor: mode === 'empty' || url.includes('cursor=next') ? '' : 'next' } : url.includes('/coupons/') ? coupon : { items: [coupon], nextCursor: '' }
    return { statusCode: 200, data: { code: 0, message: 'success', data } }
  }))
  const wrapper = mount(CouponCatalog, { global: { provide: { [couponCatalogKey as symbol]: catalog } } }); wrappers.push(wrapper)
  return { wrapper, catalog, mode: (value: string) => { mode = value } }
}
it('renders applicable product identities with validity and keyboard pagination, not purchase controls', async () => {
  const { wrapper } = harness(); await flushPromises()
  await wrapper.get('[data-test=coupon-detail-open]').trigger('keydown', { key: 'Enter' }); await flushPromises()
  const region = wrapper.get('[data-test=coupon-products]')
  expect(region.text()).toContain('适用商品一'); expect(region.text()).toContain('sku-1')
  expect(region.text()).toContain('有效期至'); expect(region.text()).toContain('不提供实时价格或购买入口')
  await wrapper.get('[data-test=coupon-products-more]').trigger('keydown', { key: ' ' }); await flushPromises()
  expect(wrapper.findAll('[data-test=coupon-product]')).toHaveLength(2)
  expect(wrapper.find('[data-test=coupon-products-more]').exists()).toBe(false)
  await wrapper.get('[data-test=coupon-detail-close]').trigger('tap'); await flushPromises()
  expect(wrapper.find('[data-test=coupon-products]').exists()).toBe(false)
})
it('shows sanitized product errors and keyboard retry recovery without clearing valid coupon rules', async () => {
  const { wrapper, mode } = harness(); mode('error'); await flushPromises()
  await wrapper.get('[data-test=coupon-detail-open]').trigger('tap'); await flushPromises()
  expect(wrapper.get('[data-test=coupon-products]').text()).toContain('读取失败')
  expect(wrapper.text()).not.toContain('private error'); expect(wrapper.text()).toContain('规则版本：v1')
  mode('ready'); await wrapper.get('[data-test=coupon-products-retry]').trigger('keydown', { key: 'Enter' }); await flushPromises()
  expect(wrapper.findAll('[data-test=coupon-product]')).toHaveLength(1)
})
it('renders an honest empty product state without fabricated product links', async () => {
  const { wrapper, mode } = harness(); mode('empty'); await flushPromises()
  await wrapper.get('[data-test=coupon-detail-open]').trigger('tap'); await flushPromises()
  expect(wrapper.get('[data-test=coupon-products]').text()).toContain('暂无可用适用商品')
  expect(wrapper.findAll('[data-test=coupon-product]')).toHaveLength(0)
  expect(wrapper.find('[data-test=coupon-products-more]').exists()).toBe(false)
})
