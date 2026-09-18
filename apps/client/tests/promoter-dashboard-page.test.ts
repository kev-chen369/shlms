import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import DashboardPage from '../src/pages/promotion/dashboard.vue'
import PromotionPage from '../src/pages/promotion/index.vue'
import pages from '../src/pages.json'
afterEach(() => vi.unstubAllGlobals())
it('independent dashboard route defaults to signed out, with no request or fake count', async () => {
  const request = vi.fn(); vi.stubGlobal('uni', { request })
  const wrapper = mount(DashboardPage); await flushPromises()
  expect(wrapper.text()).toContain('本人推广统计'); expect(wrapper.text()).toContain('请登录后')
  expect(request).not.toHaveBeenCalled(); expect(wrapper.find('[data-test=dashboard-counts]').exists()).toBe(false)
  expect(pages.pages.some(page => page.path === 'pages/promotion/dashboard')).toBe(true); expect(pages.pages.some(page => page.path === 'pages/orders/index')).toBe(true); wrapper.unmount()
})
it.each(['Enter', ' '])('promotion entry uses keyboard %s to open the independent dashboard', async key => {
  const navigateTo = vi.fn(); vi.stubGlobal('uni', { navigateTo })
  const wrapper = mount(PromotionPage); await wrapper.get('[data-test=promoter-dashboard-entry]').trigger('keydown', { key })
  expect(navigateTo).toHaveBeenLastCalledWith({ url: '/pages/promotion/dashboard' }); wrapper.unmount()
})
it('dashboard returns to the promotion tab with keyboard', async () => {
  const switchTab = vi.fn(); vi.stubGlobal('uni', { switchTab })
  const wrapper = mount(DashboardPage); await wrapper.get('[data-test=dashboard-back]').trigger('keydown', { key: 'Enter' })
  expect(switchTab).toHaveBeenCalledWith({ url: '/pages/promotion/index' }); wrapper.unmount()
})
