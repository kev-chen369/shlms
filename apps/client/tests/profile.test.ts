import { mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import Profile from '../src/pages/profile/index.vue'

afterEach(() => vi.unstubAllGlobals())
it.each([
  ['orders-secondary', '/pages/orders/index'], ['ai-secondary', '/pages/ai/index'],
])('keeps %s as a secondary destination after V3 tab migration', async (entry, url) => {
  const navigateTo = vi.fn()
  vi.stubGlobal('uni', { navigateTo })
  const wrapper = mount(Profile)
  expect(wrapper.find(`[data-test="${entry}"]`).exists()).toBe(true)
  await wrapper.get(`[data-test="${entry}"]`).trigger('keydown', { key: 'Enter' })
  expect(navigateTo).toHaveBeenCalledExactlyOnceWith({ url })
})
it('opens the same promotion destination from the personal page', async () => {
  const switchTab = vi.fn()
  vi.stubGlobal('uni', { switchTab, navigateTo: vi.fn() })
  const wrapper = mount(Profile)
  expect(wrapper.find('[data-test="promotion-secondary"]').exists()).toBe(true)
  await wrapper.find('[data-test="promotion-secondary"]').trigger('tap')
  expect(switchTab).toHaveBeenCalledExactlyOnceWith({ url: '/pages/promotion/index' })
  expect(wrapper.text()).toContain('登录服务尚未接入')
})
it('supports keyboard activation of the personal promotion entry', async () => {
  const switchTab = vi.fn()
  vi.stubGlobal('uni', { switchTab, navigateTo: vi.fn() })
  const wrapper = mount(Profile)
  await wrapper.find('[data-test="promotion-secondary"]').trigger('keydown', { key: 'Enter' })
  expect(switchTab).toHaveBeenCalledExactlyOnceWith({ url: '/pages/promotion/index' })
})
