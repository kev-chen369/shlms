import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import { afterEach, expect, it, vi } from 'vitest'
import MaterialsPage from '../src/pages/promotion/materials.vue'
import PromotionPage from '../src/pages/promotion/index.vue'
import { promoterSessionKey } from '../src/features/promotion/session'
import pages from '../src/pages.json'
afterEach(() => vi.unstubAllGlobals())
it('registered selection route has honest signed-out state and no network', async () => {
  const request = vi.fn(); vi.stubGlobal('uni', { request })
  const wrapper = mount(MaterialsPage); await flushPromises()
  expect(wrapper.text()).toContain('请登录后'); expect(request).not.toHaveBeenCalled()
  expect(pages.pages.some(page => page.path === 'pages/promotion/materials')).toBe(true)
  expect(pages.tabBar.list.map(item => item.text)).toEqual(['首页', '领券', '推广', '我的']); wrapper.unmount()
})
it.each(['Enter', ' '])('opens selection entry using %s', async key => {
  const navigateTo = vi.fn(); vi.stubGlobal('uni', { navigateTo })
  const wrapper = mount(PromotionPage); await wrapper.get('[data-test=promoter-materials-entry]').trigger('keydown', { key })
  expect(navigateTo).toHaveBeenCalledWith({ url: '/pages/promotion/materials' }); wrapper.unmount()
})
it('reads actual positions then requires explicit position confirmation before material query', async () => {
  const calls: string[] = []
  vi.stubGlobal('uni', { request: (options: { url: string; success: (v: unknown) => void }) => {
    calls.push(options.url)
    options.success({ statusCode: 200, data: { code: 0, data: options.url.includes('/promotion-positions') ? { items: [{ id: 'p1', name: '社群位', scene: 'group', status: 'ENABLED', isDefault: true, canConvert: false, channels: [{ channel: 'JD', readiness: 'WAITING_CONFIGURATION' }] }] } : { items: [], capability: { allowed: false, reason: 'UNCONFIGURED' } } } })
  } })
  const wrapper = mount(MaterialsPage, { global: { provide: { [promoterSessionKey as symbol]: ref('provided-identity') } } }); await flushPromises()
  expect(calls).toHaveLength(1); expect(wrapper.text()).toContain('社群位')
  await wrapper.get('[data-test=position-choice]').trigger('keydown', { key: 'Enter' })
  expect(wrapper.text()).toContain('投放场景：group'); expect(calls).toHaveLength(1)
  await wrapper.get('[data-test=position-confirm]').trigger('keydown', { key: ' ' }); await flushPromises()
  expect(calls[1]).toContain('positionId=p1&scene=group'); expect(wrapper.text()).toContain('目录未配置')
  wrapper.unmount()
})
