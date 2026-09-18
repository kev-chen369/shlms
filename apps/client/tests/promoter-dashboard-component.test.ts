import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import { afterEach, expect, it } from 'vitest'
import PromoterDashboard from '../src/components/PromoterDashboard.vue'
import { createPromoterDashboardModel } from '../src/features/promotion/dashboard-model'
const counts = { timeZone: 'Asia/Shanghai', from: '2026-08-19T16:00:00Z', toExclusive: '2026-09-18T16:00:00Z', asOf: '2026-09-18T12:00:00.123456789Z', successfulLinks: 5, copyReports: 8, validOrders: 2 }
const cleanup: (() => void)[] = []
afterEach(() => cleanup.splice(0).forEach(fn => fn()))
function harness(token: string | null = 'provided-test-identity', snapshot = counts, initialFilter = {}) {
  const session = ref(token), calls: string[] = []; let mode = 'ready'
  const model = createPromoterDashboardModel(session, async url => {
    calls.push(url); if (mode === 'error' || mode === '401') return { statusCode: mode === '401' ? 401 : 503, data: { message: 'private secret' } }
    const q = new URL(url, 'http://localhost').searchParams
    const data = { ...snapshot, from: q.has('from') ? q.get('from') + 'T00:00:00+08:00' : snapshot.from, toExclusive: q.has('to') ? new Date(Date.parse(q.get('to') + 'T00:00:00+08:00') + 86400000).toISOString() : snapshot.toExclusive, ...(mode === 'zero' ? { successfulLinks: 0, copyReports: 0, validOrders: 0 } : {}) }
    return { statusCode: 200, data: { code: 0, data } }
  })
  if (Object.keys(initialFilter).length) model.setFilter(initialFilter)
  const wrapper = mount(PromoterDashboard, { props: { model } }); cleanup.push(() => { wrapper.unmount(); model.dispose() })
  return { wrapper, model, session, calls, mode: (value: string) => { mode = value } }
}
it('signed-out dashboard has an honest explanation and no filter, token form, requests or fake metrics', async () => {
  const { wrapper, calls } = harness(null); await flushPromises(); expect(wrapper.text()).toContain('请登录后查看本人推广统计')
  expect(wrapper.find('[data-test=dashboard-filter]').exists()).toBe(false); expect(wrapper.find('[data-test=dashboard-counts]').exists()).toBe(false); expect(wrapper.find('input').exists()).toBe(false); expect(calls).toEqual([])
})
it('mounting a model with an existing filter displays that applied context', async () => {
  const { wrapper } = harness('provided-test-identity', counts, { channel: 'TB', positionId: 'p2', from: '2026-09-01', to: '2026-09-02' }); await flushPromises()
  expect((wrapper.get('[data-test=dashboard-position]').element as HTMLInputElement).value).toBe('p2'); expect((wrapper.get('[data-test=dashboard-from]').element as HTMLInputElement).value).toBe('2026-09-01')
  expect(wrapper.get('[data-test=dashboard-channel-TB]').attributes('aria-pressed')).toBe('true')
})
it.each([
  ['1991-03-15T16:00:00Z', '1991-04-14T15:00:00Z', '1991-03-16', '1991-04-14'],
  ['1991-08-17T15:00:00Z', '1991-09-16T16:00:00Z', '1991-08-18', '1991-09-16'],
  ['1899-12-31T15:54:17Z', '1900-01-30T15:54:17Z', '1900-01-01', '1900-01-30'],
])('uses Shanghai calendar boundaries across DST and historical offset seconds %s', async (from, toExclusive, startDay, endDay) => {
  const { wrapper } = harness('provided-test-identity', { ...counts, from, toExclusive }); await flushPromises()
  const period = wrapper.get('[data-test=dashboard-period]').text(); expect(period).toContain(startDay); expect(period).toContain(endDay)
})
it('changing identity clears unapplied drafts and validation errors', async () => {
  const { wrapper, session } = harness(); await flushPromises()
  await wrapper.get('[data-test=dashboard-position]').setValue('old-private-position'); await wrapper.get('[data-test=dashboard-from]').setValue('invalid')
  await wrapper.get('[data-test=dashboard-apply]').trigger('tap'); session.value = 'another-identity'; await flushPromises()
  expect((wrapper.get('[data-test=dashboard-position]').element as HTMLInputElement).value).toBe(''); expect((wrapper.get('[data-test=dashboard-from]').element as HTMLInputElement).value).toBe('')
  expect(wrapper.find('[role=alert]').exists()).toBe(false)
})
it('renders approved counts, Shanghai natural dates and exact snapshot without treating copy as clicks or income', async () => {
  const { wrapper } = harness(); await flushPromises()
  expect(wrapper.get('[data-test=metric-links]').text()).toContain('5'); expect(wrapper.get('[data-test=metric-copies]').text()).toContain('8'); expect(wrapper.get('[data-test=metric-orders]').text()).toContain('2')
  expect(wrapper.text()).toContain('2026-08-20'); expect(wrapper.text()).toContain('2026-09-18'); expect(wrapper.text()).toContain('2026-09-18T12:00:00.123456789Z')
  expect(wrapper.text()).toContain('复制上报不是点击'); expect(wrapper.text()).toContain('不代表收益或到账')
})
it('applies channel, position and inclusive Shanghai day filters with keyboard, and clears them', async () => {
  const { wrapper, calls } = harness(); await flushPromises()
  await wrapper.get('[data-test=dashboard-channel-TB]').trigger('keydown', { key: ' ' }); await wrapper.get('[data-test=dashboard-position]').setValue('p2')
  await wrapper.get('[data-test=dashboard-from]').setValue('2026-09-01'); await wrapper.get('[data-test=dashboard-to]').setValue('2026-09-02')
  await wrapper.get('[data-test=dashboard-apply]').trigger('keydown', { key: 'Enter' }); await flushPromises()
  expect(calls.at(-1)).toBe('/api/v1/promoter/dashboard?from=2026-09-01&to=2026-09-02&channel=TB&positionId=p2')
  expect(wrapper.get('[data-test=dashboard-period]').text()).toContain('2026-09-01')
  await wrapper.get('[data-test=dashboard-reset]').trigger('keydown', { key: ' ' }); await flushPromises(); expect(calls.at(-1)).toBe('/api/v1/promoter/dashboard')
})
it.each([['2026-02-30', '2026-09-02'], ['2026-09-03', '2026-09-02'], ['2025-01-01', '2026-09-02']])('rejects invalid or excessive inclusive date range %s..%s without clearing valid metrics', async (from, to) => {
  const { wrapper, calls } = harness(); await flushPromises()
  await wrapper.get('[data-test=dashboard-from]').setValue(from); await wrapper.get('[data-test=dashboard-to]').setValue(to); await wrapper.get('[data-test=dashboard-apply]').trigger('tap'); await flushPromises()
  expect(calls).toHaveLength(1); expect(wrapper.text()).toContain('日期'); expect(wrapper.find('[data-test=dashboard-counts]').exists()).toBe(true)
})
it('commits final uni blur values before apply and validates latest date', async () => {
  const { wrapper, calls } = harness(); await flushPromises()
  wrapper.get('[data-test=dashboard-from]').element.dispatchEvent(new CustomEvent('blur', { detail: { value: 'invalid' } }))
  await wrapper.get('[data-test=dashboard-apply]').trigger('tap'); await flushPromises(); expect(calls).toHaveLength(1); expect(wrapper.text()).toContain('日期')
})
it('real zeros retain range and snapshot; service error has no zero fallback and keyboard refresh recovers', async () => {
  const { wrapper, model, mode } = harness(); await flushPromises(); mode('zero'); await model.refresh(); await flushPromises()
  expect(wrapper.text()).toContain('本范围暂无统计记录'); expect(wrapper.get('[data-test=dashboard-period]').text()).toContain('2026-08-20'); expect(wrapper.get('[data-test=metric-links]').text()).toContain('0')
  mode('error'); await model.refresh(); await flushPromises(); expect(wrapper.text()).toContain('暂不可用'); expect(wrapper.text()).not.toContain('private secret'); expect(wrapper.find('[data-test=dashboard-counts]').exists()).toBe(false)
  mode('ready'); await wrapper.get('[data-test=dashboard-refresh]').trigger('keydown', { key: 'Enter' }); await flushPromises(); expect(wrapper.find('[data-test=dashboard-counts]').exists()).toBe(true)
})
it('exit and 401 remove private metrics, filters and snapshot', async () => {
  const { wrapper, model, session, mode } = harness(); await flushPromises(); session.value = null; await flushPromises()
  expect(wrapper.find('[data-test=dashboard-counts]').exists()).toBe(false); expect(wrapper.find('[data-test=dashboard-filter]').exists()).toBe(false)
  session.value = 'new-identity'; await flushPromises(); mode('401'); await model.refresh(); await flushPromises(); expect(wrapper.text()).toContain('请登录后'); expect(wrapper.find('[data-test=dashboard-period]').exists()).toBe(false)
})
