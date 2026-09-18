<script setup lang="ts">
import { reactive, ref, watch, computed, onMounted } from 'vue'
import type { createPromoterDashboardModel } from '../features/promotion/dashboard-model'
const props = defineProps<{ model: ReturnType<typeof createPromoterDashboardModel> }>()
const state = props.model.state
const draft = reactive({ channel: '', positionId: '', from: '', to: '' })
const filterError = ref(''), fieldVersion = ref(0)
const channels = [{ id: '', name: '全部渠道' }, { id: 'JD', name: '京东' }, { id: 'TB', name: '淘宝' }, { id: 'MT', name: '美团' }]
watch(() => state.filter, filter => {
  fieldVersion.value++
  Object.assign(draft, { channel: filter.channel || '', positionId: filter.positionId || '', from: filter.from || '', to: filter.to || '' })
  filterError.value = ''
}, { flush: 'sync', immediate: true })
onMounted(() => { void props.model.refresh() })
function calendarDay(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return NaN
  const time = Date.parse(value + 'T00:00:00Z')
  return Number.isFinite(time) && new Date(time).toISOString().slice(0, 10) === value ? time : NaN
}
function apply() {
  const from = draft.from ? calendarDay(draft.from) : null, to = draft.to ? calendarDay(draft.to) : null
  if ((from !== null && !Number.isFinite(from)) || (to !== null && !Number.isFinite(to)) || (from !== null && to !== null && (from > to || (to - from) / 86400000 + 1 > 366))) {
    filterError.value = '请检查日期：使用有效 YYYY-MM-DD，开始不晚于结束，包含首尾最多 366 天。'; return
  }
  filterError.value = ''
  props.model.setFilter({ ...(draft.from ? { from: draft.from } : {}), ...(draft.to ? { to: draft.to } : {}), ...(draft.channel ? { channel: draft.channel } : {}), ...(draft.positionId.trim() ? { positionId: draft.positionId.trim() } : {}) })
}
function reset() {
  fieldVersion.value++
  Object.assign(draft, { channel: '', positionId: '', from: '', to: '' }); filterError.value = ''
  props.model.setFilter({})
}
function commitField(field: 'positionId' | 'from' | 'to', event: unknown) {
  const value = (event as { detail?: { value?: unknown } })?.detail?.value
  if (typeof value === 'string') draft[field] = value
}
// Use IANA local calendar dates, not a fixed +08 offset or an instant minus 24 hours.
function shanghaiDay(value: string) {
  const parts = new Intl.DateTimeFormat('en', { timeZone: 'Asia/Shanghai', calendar: 'gregory', numberingSystem: 'latn', year: 'numeric', month: '2-digit', day: '2-digit' }).formatToParts(new Date(value))
  const part = (type: string) => parts.find(item => item.type === type)?.value || ''
  return `${part('year').padStart(4, '0')}-${part('month')}-${part('day')}`
}
const period = computed(() => {
  if (!state.counts) return ''
  try {
    const from = shanghaiDay(state.counts.from), exclusive = shanghaiDay(state.counts.toExclusive)
    const to = new Date(calendarDay(exclusive) - 86400000).toISOString().slice(0, 10)
    return `${from} 至 ${to}（包含首尾，Asia/Shanghai）`
  } catch { return '上海日期显示暂不可用，请刷新重试。' }
})
</script>
<template>
  <view class="dashboard">
    <text class="muted notice">仅本人推广统计。复制上报不是点击；订单数不代表收益或到账，不展示金额或转化率。</text>
    <text v-if="state.status === 'login-required'" role="status">请登录后查看本人推广统计；当前没有可用的登录身份。</text>
    <template v-else>
      <view class="panel" data-test="dashboard-filter">
        <text class="heading">筛选本人推广统计</text>
        <view class="chips">
          <button v-for="item in channels" :key="item.id" role="button" tabindex="0" :data-test="'dashboard-channel-' + item.id" :aria-pressed="draft.channel === item.id" :class="{ selected: draft.channel === item.id }" @tap="draft.channel = item.id" @keydown.enter="draft.channel = item.id" @keydown.space.prevent="draft.channel = item.id">{{ item.name }}</button>
        </view>
        <label class="field"><text>推广位编号</text><input :key="'position-' + fieldVersion" v-model="draft.positionId" @blur="commitField('positionId', $event)" data-test="dashboard-position" maxlength="128" aria-label="推广位编号" placeholder="不填则全部推广位" /></label>
        <text class="muted">按上海自然日筛选，包含开始和结束当天。不填日期使用服务端默认范围；部分日期由服务端补全。</text>
        <label class="field"><text>开始日期</text><input :key="'from-' + fieldVersion" v-model="draft.from" @blur="commitField('from', $event)" data-test="dashboard-from" maxlength="10" aria-label="开始日期 YYYY-MM-DD" placeholder="YYYY-MM-DD，可不填" /></label>
        <label class="field"><text>结束日期</text><input :key="'to-' + fieldVersion" v-model="draft.to" @blur="commitField('to', $event)" data-test="dashboard-to" maxlength="10" aria-label="结束日期 YYYY-MM-DD" placeholder="YYYY-MM-DD，可不填" /></label>
        <text v-if="filterError" role="alert" class="error">{{ filterError }}</text>
        <view class="actions"><button role="button" tabindex="0" data-test="dashboard-apply" @tap="apply" @keydown.enter="apply" @keydown.space.prevent="apply">应用筛选</button><button role="button" tabindex="0" data-test="dashboard-reset" @tap="reset" @keydown.enter="reset" @keydown.space.prevent="reset">清除筛选</button></view>
      </view>
      <view aria-live="polite" :aria-busy="state.status === 'loading'">
        <text v-if="state.status === 'loading'" role="status">正在读取本人推广统计…</text>
        <text v-else-if="state.error" role="alert" class="error">{{ state.error }}</text>
        <view v-if="state.counts" class="panel" data-test="dashboard-counts">
          <text class="heading">服务端统计快照</text><text data-test="dashboard-period">{{ period }}</text>
          <text class="muted">快照时刻：{{ state.counts.asOf }}（保留服务端精度）</text>
          <text v-if="state.status === 'empty'" role="status">本范围暂无统计记录。</text>
          <view class="metric" data-test="metric-links"><text>成功转链</text><text class="number">{{ state.counts.successfulLinks }}</text><text class="muted">当前成功的请求，按请求更新时间统计。</text></view>
          <view class="metric" data-test="metric-copies"><text>复制上报</text><text class="number">{{ state.counts.copyReports }}</text><text class="muted">已存储且去重的复制事件；不是分享送达、点击或成交。</text></view>
          <view class="metric" data-test="metric-orders"><text>有效归因订单</text><text class="number">{{ state.counts.validOrders }}</text><text class="muted">按最早订单发生时间统计，当前已归因，排除失效、取消与退款状态；不是结算金额。</text></view>
        </view>
        <button role="button" tabindex="0" data-test="dashboard-refresh" :disabled="state.status === 'loading'" @tap="model.refresh()" @keydown.enter="model.refresh()" @keydown.space.prevent="model.refresh()">刷新本人推广统计</button>
      </view>
    </template>
  </view>
</template>
<style scoped>
.dashboard { color: #183f36; font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
.muted { display: block; color: #65776f; font-size: 13px; }
.notice { margin-bottom: 18px; }
.panel { margin: 14px 0; padding: 16px; border: 1px solid #dbe8e1; border-radius: 16px; background: #fff; }
.panel > text { display: block; margin-bottom: 8px; }
.heading { font-size: 16px; font-weight: 650; }
.chips, .actions { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 14px; }
button { margin: 8px 0; min-height: 44px; border: 1px solid #d6e5dc; border-radius: 12px; background: #fff; color: #07594b; font-size: 14px; line-height: 1.5; padding: 12px; }
button::after { border: none; }
button:focus-visible, input:focus-visible { outline: 3px solid #f0ad63; outline-offset: 2px; }
.chips button { margin: 0; flex: 1 0 80px; }
.selected { background: #e5f4ed; }
.field { display: block; margin: 12px 0; }
.field > text { display: block; margin-bottom: 6px; }
input { border: 1px solid #d6e5dc; border-radius: 10px; padding: 12px; height: 60px; min-height: 60px; font-size: 14px; }
.actions button { flex: 1; }
.error { display: block; color: #a44320; margin: 12px 0; }
.metric { border-top: 1px solid #dbe8e1; padding: 14px 0; }
.number { display: block; font-size: 28px; font-weight: 650; }
</style>
