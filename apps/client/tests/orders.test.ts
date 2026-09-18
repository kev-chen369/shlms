import { mount } from '@vue/test-utils'
import { afterEach, expect, it, vi } from 'vitest'
import Orders from '../src/pages/orders/index.vue'

afterEach(() => vi.unstubAllGlobals())
it.each(['tap', 'Enter', ' '])('returns to the personal tab from secondary orders using %s', async action => {
  const switchTab = vi.fn()
  vi.stubGlobal('uni', { switchTab })
  const wrapper = mount(Orders)
  expect(wrapper.find('[data-test="orders-back"]').exists()).toBe(true)
  const button = wrapper.get('[data-test="orders-back"]')
  if (action === 'tap') await button.trigger('tap')
  else await button.trigger('keydown', { key: action })
  expect(switchTab).toHaveBeenCalledExactlyOnceWith({ url: '/pages/profile/index' })
  expect(wrapper.text()).toContain('订单服务尚未接入')
})
