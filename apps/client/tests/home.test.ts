import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import Home from '../src/pages/home/index.vue'

describe('consumer home', () => {
  it('identifies the client as Wanhui rather than the legacy brand', () => {
    const wrapper = mount(Home)
    expect(wrapper.text()).toContain('万惠宝')
    expect(wrapper.text()).not.toContain('惠省生活')
  })
})
