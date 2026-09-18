import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import OrdersPage from '../src/pages/promotion/orders.vue'
import PromotionPage from '../src/pages/promotion/index.vue'
import pages from '../src/pages.json'
afterEach(() => vi.unstubAllGlobals())
it('independent route renders signed-out explanation without requesting or replacing the consumer orders route', async () => {
  const request = vi.fn(); vi.stubGlobal('uni', { request })
  const wrapper = mount(OrdersPage); await flushPromises()
  expect(wrapper.text()).toContain('本人推广订单'); expect(wrapper.text()).toContain('请登录后')
  expect(request).not.toHaveBeenCalled(); expect(pages.pages.some(page => page.path === 'pages/promotion/orders')).toBe(true)
  expect(pages.pages.some(page => page.path === 'pages/orders/index')).toBe(true); wrapper.unmount()
})
it('promoter entry keyboard navigation points to independent route, not consumer orders', async () => {
  const navigateTo = vi.fn(); vi.stubGlobal('uni', { navigateTo })
  const wrapper = mount(PromotionPage)
  await wrapper.get('[data-test=promoter-orders-entry]').trigger('keydown', { key: 'Enter' })
  expect(navigateTo).toHaveBeenLastCalledWith({ url: '/pages/promotion/orders' }); wrapper.unmount()
})
