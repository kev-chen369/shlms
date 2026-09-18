import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import { afterEach, expect, it } from 'vitest'
import PromoterOrders from '../src/components/PromoterOrders.vue'
import { createPromoterOrdersModel } from '../src/features/promotion/orders-model'
const id = '11111111-1111-1111-1111-111111111111'
const item = { id, channel: 'JD', maskedOrderId: '****1234', orderStatus: 'PAID', positionId: 'p1', attributionMethod: 'SUB_ID', attributedAt: '2026-09-18T08:00:00Z', orderOccurredAt: '2026-09-18T08:00:00Z', statusAt: '2026-09-18T08:00:00Z' }
const cleanup: (() => void)[] = []
afterEach(() => cleanup.splice(0).forEach(fn => fn()))
function harness(token: string | null = 'test-supplied-identity') {
  const session = ref(token), calls: string[] = []; let mode = 'ready'
  const model = createPromoterOrdersModel(session, async url => {
    calls.push(url)
    if (mode === 'error' || mode === '401') return { statusCode: mode === '401' ? 401 : 503, data: { message: 'private secret' } }
    if (mode === '404') return { statusCode: 404, data: { code: 'ORDER_NOT_FOUND' } }
    if (mode === 'unavailable') return { statusCode: 404, data: { code: 'ROUTE_NOT_FOUND' } }
    const params = new URL(url, 'http://localhost').searchParams
    const data = url.includes(id) ? { ...item, history: [{ previousStatus: 'CREATED', status: 'PAID', occurredAt: item.statusAt, projectedAt: item.statusAt }], refundEventCount: 2 } : { items: mode === 'empty' ? [] : [{ ...item, id: params.has('cursor') ? '22222222-2222-2222-2222-222222222222' : id, channel: params.get('channel') || 'JD', positionId: params.get('positionId') || 'p1', orderStatus: params.get('orderStatus') || 'PAID' }], nextCursor: mode === 'empty' || params.has('cursor') ? '' : 'next' }
    return { statusCode: 200, data: { code: 0, data } }
  })
  const wrapper = mount(PromoterOrders, { props: { model } })
  cleanup.push(() => { wrapper.unmount(); model.dispose() })
  return { wrapper, model, session, calls, mode: (value: string) => { mode = value } }
}
it('signed-out page explains identity and distinguishes promoter orders without issuing requests or token controls', async () => {
  const { wrapper, calls } = harness(null); await flushPromises()
  expect(wrapper.text()).toContain('请登录后查看本人推广订单'); expect(wrapper.text()).toContain('不是消费者返现订单')
  expect(wrapper.find('[data-test=order-filter]').exists()).toBe(false); expect(wrapper.find('input').exists()).toBe(false); expect(calls).toEqual([])
})
it('renders masked summaries and keyboard detail with complete status history but no income inference', async () => {
  const { wrapper } = harness(); await flushPromises()
  expect(wrapper.get('[data-test=promoter-order]').text()).toContain('****1234'); expect(wrapper.text()).toContain('已付款')
  await wrapper.get('[data-test=order-open]').trigger('keydown', { key: 'Enter' }); await flushPromises()
  expect(wrapper.get('[data-test=order-detail]').text()).toContain('已创建 → 已付款')
  expect(wrapper.get('[data-test=order-detail]').text()).toContain('退款事件数：2'); expect(wrapper.text()).toContain('不代表收益或到账')
  await wrapper.get('[data-test=order-close]').trigger('keydown', { key: ' ' }); await flushPromises()
  expect(wrapper.find('[data-test=order-detail]').exists()).toBe(false)
})
it('applies channel, status, position and inclusive UTC date filters and clears them with keyboard', async () => {
  const { wrapper, calls } = harness(); await flushPromises()
  await wrapper.get('[data-test=channel-TB]').trigger('tap'); await wrapper.get('[data-test=status-CONFIRMED]').trigger('tap')
  await wrapper.get('[data-test=position-filter]').setValue('p2'); await wrapper.get('[data-test=from-filter]').setValue('2026-09-18'); await wrapper.get('[data-test=to-filter]').setValue('2026-09-18')
  await wrapper.get('[data-test=order-apply]').trigger('keydown', { key: 'Enter' }); await flushPromises()
  expect(calls.at(-1)).toBe('/api/v1/promoter/orders?channel=TB&positionId=p2&orderStatus=CONFIRMED&from=2026-09-18T00%3A00%3A00.000Z&to=2026-09-19T00%3A00%3A00.000Z&limit=20')
  expect(wrapper.get('[data-test=promoter-order]').text()).toContain('已确认')
  await wrapper.get('[data-test=order-reset]').trigger('keydown', { key: ' ' }); await flushPromises(); expect(calls.at(-1)).toBe('/api/v1/promoter/orders?limit=20')
})
it.each(['2026-02-30', 'invalid'])('rejects invalid date %s without clearing existing orders or requesting', async date => {
  const { wrapper, calls } = harness(); await flushPromises(); const count = calls.length
  await wrapper.get('[data-test=from-filter]').setValue(date); await wrapper.get('[data-test=order-apply]').trigger('tap'); await flushPromises()
  expect(wrapper.text()).toContain('日期'); expect(calls).toHaveLength(count); expect(wrapper.findAll('[data-test=promoter-order]')).toHaveLength(1)
})
it('rejects reversed date range without requesting', async () => {
  const { wrapper, calls } = harness(); await flushPromises()
  await wrapper.get('[data-test=from-filter]').setValue('2026-09-19'); await wrapper.get('[data-test=to-filter]').setValue('2026-09-18')
  await wrapper.get('[data-test=order-apply]').trigger('tap'); await flushPromises(); expect(calls).toHaveLength(1); expect(wrapper.text()).toContain('日期')
})
it('pagination errors preserve list and cursor, and keyboard retry appends another masked order', async () => {
  const { wrapper, mode } = harness(); await flushPromises(); mode('error')
  await wrapper.get('[data-test=order-more]').trigger('tap'); await flushPromises(); expect(wrapper.findAll('[data-test=promoter-order]')).toHaveLength(1)
  expect(wrapper.text()).toContain('读取失败'); expect(wrapper.text()).not.toContain('private secret')
  mode('ready'); await wrapper.get('[data-test=order-more]').trigger('keydown', { key: ' ' }); await flushPromises(); expect(wrapper.findAll('[data-test=promoter-order]')).toHaveLength(2)
  expect(wrapper.find('[data-test=order-more]').exists()).toBe(false)
})
it('shows recoverable empty and sanitized errors; logout removes filters, cards and detail', async () => {
  const { wrapper, model, session, mode } = harness(); await flushPromises(); mode('empty'); await model.refresh(); await flushPromises()
  expect(wrapper.text()).toContain('暂无本人推广订单'); mode('error'); await model.refresh(); await flushPromises(); expect(wrapper.text()).toContain('读取失败')
  mode('ready'); await wrapper.get('[data-test=order-refresh]').trigger('keydown', { key: 'Enter' }); await flushPromises()
  await wrapper.get('[data-test=order-open]').trigger('tap'); await flushPromises(); session.value = null; await flushPromises()
  expect(wrapper.find('[data-test=order-detail]').exists()).toBe(false); expect(wrapper.find('[data-test=order-filter]').exists()).toBe(false); expect(wrapper.findAll('[data-test=promoter-order]')).toHaveLength(0)
})
it('404 detail has an explanation and close, without resurrecting the removed summary', async () => {
  const { wrapper, mode } = harness(); await flushPromises(); mode('404')
  await wrapper.get('[data-test=order-open]').trigger('tap'); await flushPromises(); expect(wrapper.get('[data-test=order-detail]').text()).toContain('不存在或不可查看')
  expect(wrapper.findAll('[data-test=promoter-order]')).toHaveLength(0); await wrapper.get('[data-test=order-close]').trigger('tap'); expect(wrapper.find('[data-test=order-detail]').exists()).toBe(false)
})
it('unavailable detail retains summary and allows retry when service recovers', async () => {
  const { wrapper, mode } = harness(); await flushPromises(); mode('unavailable')
  await wrapper.get('[data-test=order-open]').trigger('tap'); await flushPromises(); expect(wrapper.get('[data-test=order-detail]').text()).toContain('尚未接入')
  expect(wrapper.findAll('[data-test=promoter-order]')).toHaveLength(1); mode('ready')
  await wrapper.get('[data-test=order-detail-retry]').trigger('keydown', { key: ' ' }); await flushPromises(); expect(wrapper.text()).toContain('已创建 → 已付款')
})
it('401 removes private summaries and filters and does not silently refresh rejected identity', async () => {
  const { wrapper, model, calls, mode } = harness(); await flushPromises(); mode('401')
  await wrapper.get('[data-test=order-open]').trigger('tap'); await flushPromises()
  expect(wrapper.text()).toContain('请登录后'); expect(wrapper.find('[data-test=order-filter]').exists()).toBe(false); expect(wrapper.findAll('[data-test=promoter-order]')).toHaveLength(0)
  const count = calls.length; await model.refresh(); expect(calls).toHaveLength(count)
})
it('commits the final blur value before apply even when uni input model emission is throttled', async () => {
  const { wrapper, calls } = harness(); await flushPromises()
  await wrapper.get('[data-test=from-filter]').setValue('2026-09-18')
  wrapper.get('[data-test=from-filter]').element.dispatchEvent(new CustomEvent('blur', { detail: { value: '2026-02-30' } }))
  await wrapper.get('[data-test=order-apply]').trigger('tap'); await flushPromises()
  expect(calls).toHaveLength(1); expect(wrapper.text()).toContain('请检查日期')
  wrapper.get('[data-test=from-filter]').element.dispatchEvent(new CustomEvent('blur', { detail: { value: '' } }))
  wrapper.get('[data-test=position-filter]').element.dispatchEvent(new CustomEvent('blur', { detail: { value: 'final-position' } }))
  wrapper.get('[data-test=to-filter]').element.dispatchEvent(new CustomEvent('blur', { detail: { value: '2026-09-18' } }))
  await wrapper.get('[data-test=order-apply]').trigger('tap'); await flushPromises()
  expect(calls.at(-1)).toBe('/api/v1/promoter/orders?positionId=final-position&to=2026-09-19T00%3A00%3A00.000Z&limit=20')
})
