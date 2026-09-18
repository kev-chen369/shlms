import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import { afterEach, expect, it } from 'vitest'
import PromotionMaterials from '../src/components/PromotionMaterials.vue'
import { createMaterialsModel } from '../src/features/promotion/materials-model'
const item = { id: '11111111-1111-1111-1111-111111111111', platform: 'JD', type: 'PRODUCT', title: '合成目录商品', endsAt: '2027-01-01T00:00:00Z', sourceUpdatedAt: '2026-09-19T00:00:00Z', ruleVersion: 'v1', region: { mode: 'NATIONWIDE' }, business: '', terminals: ['H5'] }
const scope = { platform: 'JD' as const, type: 'PRODUCT' as const, terminal: 'H5' as const, positionId: 'p1', scene: 'group' }
const cleanup: (() => void)[] = []
afterEach(() => cleanup.splice(0).forEach(fn => fn()))
function harness(token: string | null = 'provided-identity') {
  const session = ref(token), calls: string[] = []; let mode = 'ready'
  const model = createMaterialsModel(session, async url => {
    calls.push(url)
    if (mode === 'error') return { statusCode: 503, data: { message: 'private-secret' } }
    const capability = { allowed: mode !== 'blocked', reason: mode === 'blocked' ? 'UNCONFIGURED' : 'READY' }
    const nextItem = mode === 'next' ? { ...item, id: '22222222-2222-2222-2222-222222222222', title: '合成下一页商品' } : item
    const data = url.includes('/' + item.id + '?') ? { capability, availability: { available: mode !== 'expired', reason: mode === 'expired' ? 'EXPIRED' : 'AVAILABLE' }, ...(mode === 'expired' ? {} : { item }) } : { capability, items: mode === 'empty' || mode === 'blocked' ? [] : [nextItem], nextCursor: mode === 'page' ? 'YQ' : '' }
    return { statusCode: 200, data: { code: 0, data } }
  })
  const wrapper = mount(PromotionMaterials, { props: { model } })
  cleanup.push(() => { wrapper.unmount(); model.dispose() })
  return { wrapper, model, session, calls, mode: (next: string) => { mode = next } }
}
it('signed out has no cards, controls that request, or demo material', async () => {
  const { wrapper, calls } = harness(null); await flushPromises()
  expect(wrapper.text()).toContain('请登录后'); expect(wrapper.find('[data-test=material-card]').exists()).toBe(false)
  expect(wrapper.find('[data-test=materials-refresh]').exists()).toBe(false); expect(calls).toEqual([])
})
it('requires explicit position and scene before reading', async () => {
  const { wrapper, calls } = harness(); await flushPromises()
  expect(wrapper.text()).toContain('请选择本人推广位与投放场景'); expect(calls).toEqual([])
})
it('renders safe real summary and keeps generation disabled even when catalog READY', async () => {
  const { wrapper, model } = harness(); model.setScope(scope); await flushPromises()
  expect(wrapper.get('[data-test=material-card]').text()).toContain('合成目录商品')
  expect(wrapper.text()).toContain('目录可读取不代表可生成')
  expect(wrapper.get('[data-test=material-generate]').attributes('disabled')).toBeDefined()
  expect(wrapper.text()).not.toContain('预计收益'); expect(wrapper.find('image').exists()).toBe(false)
})
it.each(['Enter', ' '])('fresh detail opens and closes using keyboard %s', async key => {
  const { wrapper, model, calls } = harness(); model.setScope(scope); await flushPromises()
  await wrapper.get('[data-test=material-view]').trigger('keydown', { key }); await flushPromises()
  expect(calls).toHaveLength(2); expect(wrapper.get('[data-test=material-detail]').text()).toContain('v1')
  await wrapper.get('[data-test=material-detail-close]').trigger('keydown', { key })
  expect(wrapper.find('[data-test=material-detail]').exists()).toBe(false)
})
it('platform change clears cards and scope, defaults MT to activities without requesting', async () => {
  const { wrapper, model, calls } = harness(); model.setScope(scope); await flushPromises()
  await wrapper.get('[data-test=materials-platform-MEITUAN]').trigger('tap'); await flushPromises()
  expect(model.state.type).toBe('ACTIVITY'); expect(model.state.scope).toBeNull()
  expect(wrapper.find('[data-test=material-card]').exists()).toBe(false); expect(calls).toHaveLength(1)
  expect(wrapper.get('[data-test=materials-type-PRODUCT]').attributes('disabled')).toBeDefined()
})
it('type selection clears prior context and requests nothing until position reselected', async () => {
  const { wrapper, model, calls } = harness(); model.setScope(scope); await flushPromises()
  await wrapper.get('[data-test=materials-type-ACTIVITY]').trigger('keydown', { key: 'Enter' })
  expect(model.state.scope).toBeNull(); expect(model.state.type).toBe('ACTIVITY'); expect(calls).toHaveLength(1)
})
it.each(['Enter', ' '])('disabled MT product ignores keyboard %s without leaving activity scope', async key => {
  const { wrapper, model, calls } = harness(); model.selectPlatform('MEITUAN'); await flushPromises()
  wrapper.get('[data-test=materials-type-PRODUCT]').element.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true }))
  expect(model.state.type).toBe('ACTIVITY'); expect(calls).toEqual([])
})
it('distinguishes empty and denied catalogs without fake cards', async () => {
  const { wrapper, model, mode } = harness(); mode('empty'); model.setScope(scope); await flushPromises()
  expect(wrapper.text()).toContain('暂无适用物料'); mode('blocked'); await model.refresh(); await flushPromises()
  expect(wrapper.text()).toContain('目录未配置'); expect(wrapper.find('[data-test=material-card]').exists()).toBe(false)
})
it('pagination failure retains cards, offers retry, and sanitizes error', async () => {
  const { wrapper, model, mode } = harness(); mode('page'); model.setScope(scope); await flushPromises()
  mode('error'); await wrapper.get('[data-test=materials-more]').trigger('tap'); await flushPromises()
  expect(wrapper.find('[data-test=material-card]').exists()).toBe(true); expect(wrapper.text()).toContain('读取失败')
  expect(wrapper.text()).not.toContain('private-secret'); expect(wrapper.find('[data-test=materials-more]').exists()).toBe(true)
  mode('next'); await wrapper.get('[data-test=materials-more]').trigger('keydown', { key: ' ' }); await flushPromises()
  expect(wrapper.findAll('[data-test=material-card]')).toHaveLength(2); expect(wrapper.text()).toContain('合成下一页商品')
  expect(wrapper.find('[data-test=materials-more]').exists()).toBe(false); expect(wrapper.text()).not.toContain('读取失败')
})
it('expired fresh detail is explained and removes stale card', async () => {
  const { wrapper, model, mode } = harness(); model.setScope(scope); await flushPromises(); mode('expired')
  await wrapper.get('[data-test=material-view]').trigger('tap'); await flushPromises()
  expect(wrapper.text()).toContain('已失效或不适用'); expect(wrapper.find('[data-test=material-card]').exists()).toBe(false)
})
it('identity loss removes summary and detail', async () => {
  const { wrapper, model, session } = harness(); model.setScope(scope); await flushPromises()
  await wrapper.get('[data-test=material-view]').trigger('tap'); await flushPromises(); session.value = null; await flushPromises()
  expect(wrapper.find('[data-test=material-detail]').exists()).toBe(false); expect(wrapper.find('[data-test=material-card]').exists()).toBe(false)
})
