import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import Home from '../src/pages/home/index.vue'
import { selectedPlatform } from '../src/features/platform'

describe('consumer home', () => {
  afterEach(() => { vi.unstubAllGlobals(); selectedPlatform.value = 'JD' })
  it('identifies the client by the currently approved public brand', () => {
    const wrapper = mount(Home)
    expect(wrapper.text()).toContain('万宝单生活')
    expect(wrapper.text()).not.toContain('万惠宝')
    expect(wrapper.text()).not.toContain('惠省生活')
  })
  it.each([
    ['promotion-entry', '/pages/promotion/index'],
    ['coupon-entry', '/pages/coupons/index'],
  ])('opens the destination from %s', async (entry, url) => {
    const switchTab = vi.fn()
    vi.stubGlobal('uni', { switchTab, navigateTo: vi.fn() })
    const wrapper = mount(Home)
    expect(wrapper.find(`[data-test="${entry}"]`).exists()).toBe(true)
    await wrapper.find(`[data-test="${entry}"] button`).trigger('tap')
    expect(switchTab).toHaveBeenCalledExactlyOnceWith({ url })
  })
  it('explains unavailable search instead of fabricating results', async () => {
    const wrapper = mount(Home)
    expect(wrapper.find('[data-test="search-submit"]').exists()).toBe(true)
    await wrapper.find('[data-test="search-submit"]').trigger('tap')
    expect(wrapper.find('[data-test="notice"]').text()).toContain('暂未接入')
    expect(wrapper.text()).not.toContain('¥79')
  })
  it.each(['Enter', ' '])('activates the promotion feature using the %s key', async key => {
    const switchTab = vi.fn()
    vi.stubGlobal('uni', { switchTab, navigateTo: vi.fn() })
    const wrapper = mount(Home)
    await wrapper.find('[data-test="promotion-entry"] button').trigger('keydown', { key })
    expect(switchTab).toHaveBeenCalledExactlyOnceWith({ url: '/pages/promotion/index' })
  })
  it('changes the platform context without claiming authorization or opening external links', async () => {
    const navigateTo = vi.fn()
    vi.stubGlobal('uni', { navigateTo })
    const wrapper = mount(Home)
    await wrapper.find('[data-platform="TAOBAO"]').trigger('tap')
    expect(wrapper.text()).toContain('淘宝精选')
    expect(wrapper.find('[data-platform="TAOBAO"]').attributes('aria-pressed')).toBe('true')
    expect(wrapper.text()).toContain('未接入')
    expect(navigateTo).not.toHaveBeenCalled()
  })
  it('shows the four V3 modules and clears old notices when switching platforms', async () => {
    const wrapper = mount(Home)
    for (const heading of ['推广商品', '优选商店', '领券中心', '推广专区']) expect(wrapper.text()).toContain(heading)
    expect(wrapper.text()).not.toContain('AI 帮我选')
    await wrapper.get('[data-test="products-entry"]').trigger('tap')
    expect(wrapper.get('[data-test="notice"]').text()).toContain('京东')
    await wrapper.get('[data-platform="MEITUAN"]').trigger('keydown', { key: ' ' })
    expect(wrapper.text()).toContain('美团精选')
    expect(wrapper.find('[data-test="notice"]').exists()).toBe(false)
  })
})
