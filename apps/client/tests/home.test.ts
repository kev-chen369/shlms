import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import Home from '../src/pages/home/index.vue'

describe('consumer home', () => {
  afterEach(() => vi.unstubAllGlobals())
  it('identifies the client by the currently approved public brand', () => {
    const wrapper = mount(Home)
    expect(wrapper.text()).toContain('万宝单生活')
    expect(wrapper.text()).not.toContain('万惠宝')
    expect(wrapper.text()).not.toContain('惠省生活')
  })
  it.each([
    ['promotion-entry', '/pages/promotion/index'],
    ['ai-entry', '/pages/ai/index'],
  ])('opens the destination from %s', async (entry, url) => {
    const navigateTo = vi.fn()
    vi.stubGlobal('uni', { navigateTo })
    const wrapper = mount(Home)
    expect(wrapper.find(`[data-test="${entry}"]`).exists()).toBe(true)
    await wrapper.find(`[data-test="${entry}"] button`).trigger('tap')
    expect(navigateTo).toHaveBeenCalledExactlyOnceWith({ url })
  })
  it('explains unavailable search instead of fabricating results', async () => {
    const wrapper = mount(Home)
    expect(wrapper.find('[data-test="search-submit"]').exists()).toBe(true)
    await wrapper.find('[data-test="search-submit"]').trigger('tap')
    expect(wrapper.find('[data-test="notice"]').text()).toContain('暂未接入')
    expect(wrapper.text()).not.toContain('¥79')
  })
  it.each(['Enter', ' '])('activates the promotion feature using the %s key', async key => {
    const navigateTo = vi.fn()
    vi.stubGlobal('uni', { navigateTo })
    const wrapper = mount(Home)
    await wrapper.find('[data-test="promotion-entry"] button').trigger('keydown', { key })
    expect(navigateTo).toHaveBeenCalledExactlyOnceWith({ url: '/pages/promotion/index' })
  })
  it('explains unavailable channels without redirecting to an unapproved URL', async () => {
    const navigateTo = vi.fn()
    vi.stubGlobal('uni', { navigateTo })
    const wrapper = mount(Home)
    expect(wrapper.find('[data-test="channel-jd"]').exists()).toBe(true)
    await wrapper.find('[data-test="channel-jd"]').trigger('tap')
    expect(wrapper.find('[data-test="notice"]').text()).toContain('京东渠道暂未接入')
    expect(navigateTo).not.toHaveBeenCalled()
  })
})
