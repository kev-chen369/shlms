import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import CouponCatalog from '../src/components/CouponCatalog.vue'
import Home from '../src/pages/home/index.vue'
import Coupons from '../src/pages/coupons/index.vue'
import { createCouponAPI, type Request } from '../src/features/coupons/api'
import { createCouponCatalog, couponCatalogKey } from '../src/features/coupons/catalog'
import { selectedPlatform } from '../src/features/platform'
const item = {
  id: 'coupon-1', platform: 'JD', claimMode: 'PLATFORM_ACTIVITY', actionLabel: '去平台领取 / 购买',
  title: '接口测试券', scope: 'ACTIVITY', scopeExternalId: '', scopeName: '', currency: 'CNY',
  discountMinor: 500, thresholdMinor: 2000, cityCode: '', cityName: '', business: '',
  ruleVersion: 'v1', updatedAt: '2026-09-18T00:00:00Z', expiresAt: '2030-01-01T00:00:00Z',
}
const wrappers: ReturnType<typeof mount>[] = []
afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); selectedPlatform.value = 'JD' })
function mountCatalog(transport: Request = async (url: string) => ({ statusCode: 200, data: { code: 0, message: 'success', data: url.includes('coupon-cities') ? { items: [{ code: '110100', name: '北京' }] } : url.includes('/coupons/') ? item : { items: [item], nextCursor: '' } } }), compact = false) {
  const request = vi.fn(transport)
  const catalog = createCouponCatalog(createCouponAPI(request))
  const wrapper = mount(CouponCatalog, { props: { compact }, global: { provide: { [couponCatalogKey as symbol]: catalog } } })
  wrappers.push(wrapper)
  return { wrapper, catalog, request }
}
it('renders service data and read-only details, never a claim-success control', async () => {
  const { wrapper } = mountCatalog()
  await flushPromises()
  expect(wrapper.text()).toContain('接口测试券')
  expect(wrapper.text()).toContain('¥5.00')
  await wrapper.get('[data-test="coupon-detail-open"]').trigger('tap'); await flushPromises()
  expect(wrapper.get('[data-test="coupon-detail"]').text()).toContain('去平台领取 / 购买')
  expect(wrapper.get('[data-test="coupon-detail"]').text()).toContain('领取与购买暂未接通')
  await wrapper.get('[data-test="coupon-detail-close"]').trigger('keydown', { key: ' ' })
  expect(wrapper.find('[data-test="coupon-detail"]').exists()).toBe(false)
  expect(wrapper.text()).not.toContain('领取成功')
})
it('applies server-provided city and explicit business scope using accessible controls', async () => {
  const { wrapper, catalog, request } = mountCatalog()
  await flushPromises()
  await wrapper.get('[data-test="coupon-city-toggle"]').trigger('keydown', { key: 'Enter' })
  await wrapper.get('[data-city="110100"]').trigger('tap'); await flushPromises()
  expect(catalog.state.context.cityCode).toBe('110100')
  await wrapper.get('[data-test="coupon-business"]').setValue('food')
  await wrapper.get('[data-test="coupon-business-apply"]').trigger('keydown', { key: ' ' }); await flushPromises()
  expect(request.mock.calls.at(-1)?.[0]).toContain('cityCode=110100&business=food')
})
it('renders empty catalog and unopened platform without invented coupon amounts', async () => {
  const request = vi.fn(async () => ({ statusCode: 200, data: { code: 0, message: 'success', data: { items: [], nextCursor: '' } } }))
  const { wrapper } = mountCatalog(request)
  await flushPromises()
  expect(wrapper.text()).toContain('暂无可用优惠')
  selectedPlatform.value = 'PDD'; await flushPromises()
  expect(wrapper.text()).toContain('该平台券目录尚未开放')
  expect(wrapper.text()).not.toContain('¥')
  expect(wrapper.find('[data-test="coupon-retry"]').exists()).toBe(false)
})
it('exposes a sanitized read failure and retry that can recover', async () => {
  let unavailable = true
  const request = vi.fn(async (url: string) => unavailable ? { statusCode: 503, data: { message: 'private failure' } } : { statusCode: 200, data: { code: 0, message: 'success', data: url.includes('coupon-cities') ? { items: [] } : { items: [item], nextCursor: '' } } })
  const { wrapper } = mountCatalog(request)
  await flushPromises()
  expect(wrapper.text()).toContain('券目录读取失败')
  expect(wrapper.text()).not.toContain('private failure')
  unavailable = false
  await wrapper.get('[data-test="coupon-retry"]').trigger('tap'); await flushPromises()
  expect(wrapper.text()).toContain('接口测试券')
})
it('keeps homepage summary compact without a second set of context controls', async () => {
  const request = vi.fn(async (url: string) => ({ statusCode: 200, data: { code: 0, message: 'success', data: url.includes('coupon-cities') ? { items: [] } : { items: [item, { ...item, id: 'c2' }, { ...item, id: 'c3' }], nextCursor: 'next' } } }))
  const { wrapper } = mountCatalog(request, true)
  await flushPromises()
  expect(wrapper.findAll('[data-test="coupon-card"]')).toHaveLength(2)
  expect(wrapper.find('[data-test="coupon-city-toggle"]').exists()).toBe(false)
  expect(wrapper.find('[data-test="coupon-more"]').exists()).toBe(false)
})
it.each([Home, Coupons])('connects the actual page to the provided read-only catalog', async Page => {
  const catalog = createCouponCatalog(createCouponAPI(async url => ({ statusCode: 200, data: { code: 0, message: 'success', data: url.includes('coupon-cities') ? { items: [] } : { items: [item], nextCursor: '' } } })))
  const wrapper = mount(Page, { global: { provide: { [couponCatalogKey as symbol]: catalog } } })
  wrappers.push(wrapper)
  await flushPromises()
  expect(wrapper.find('[data-test="coupon-card"]').text()).toContain('接口测试券')
  expect(catalog.state.status).toBe('ready')
})
