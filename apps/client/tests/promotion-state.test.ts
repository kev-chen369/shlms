import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PromotionState from '../src/components/PromotionState.vue'
import type { PromotionViewState } from '../src/features/promotion/profile'

describe('promotion eligibility display', () => {
  it.each([
    ['login-required', '推广业务尚未开放'], ['loading', '正在读取推广身份'], ['error', '推广信息暂不可用'],
  ] as const)('renders %s without offering conversion', (kind, title) => {
    const wrapper = mount(PromotionState, { props: { state: { kind } } })
    expect(wrapper.text()).toContain(title)
    expect(wrapper.findAll('button')).toHaveLength(0)
  })
  it.each([
    ['NOT_APPLIED', '尚未申请推广员', true, false, false],
    ['PENDING', '推广申请审核中', false, false, false],
    ['REJECTED', '推广申请未通过', true, false, false],
    ['ENABLED', '推广资格已开通', false, true, true],
    ['DISABLED', '推广资格已停用', false, false, true],
  ] as const)('renders %s and does not confuse membership with channel authorization', (status, title, canApply, canPromote, canReadHistory) => {
    const state: PromotionViewState = { kind: 'profile', profile: { status, reason: '用户可见原因', applicationId: 'application-1', capabilities: { canApply, canPromote, canReadHistory } } }
    const wrapper = mount(PromotionState, { props: { state } })
    expect(wrapper.text()).toContain(title)
    expect(wrapper.findAll('button')).toHaveLength(0)
    if (status === 'ENABLED') expect(wrapper.text()).toContain('不代表渠道可转链')
    if (status === 'DISABLED' || status === 'REJECTED') expect(wrapper.text()).toContain('用户可见原因')
    else expect(wrapper.text()).not.toContain('用户可见原因')
  })
  it('renders user-visible rejection text as text, not executable markup', () => {
    const wrapper = mount(PromotionState, { props: { state: { kind: 'profile', profile: {
      status: 'REJECTED', reason: '<img src=x onerror=alert(1)>', applicationId: 'application-1',
      capabilities: { canApply: true, canPromote: false, canReadHistory: false },
    } } } })
    expect(wrapper.text()).toContain('<img src=x onerror=alert(1)>')
    expect(wrapper.find('img').exists()).toBe(false)
  })
})
