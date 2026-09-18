<script setup lang="ts">
import { reactive, ref, watch, onMounted } from 'vue'
import type { createPromoterOrdersModel } from '../features/promotion/orders-model'
const props = defineProps<{ model: ReturnType<typeof createPromoterOrdersModel> }>()
const state = props.model.state
const draft = reactive({ channel: '', orderStatus: '', positionId: '', from: '', to: '' })
const filterError = ref('')
const fieldVersion = ref(0)
const channels = [{ id: '', name: '全部渠道' }, { id: 'JD', name: '京东' }, { id: 'TB', name: '淘宝' }, { id: 'MT', name: '美团' }]
const statuses = [{ id: '', name: '全部状态' }, { id: 'CREATED', name: '已创建' }, { id: 'PAID', name: '已付款' }, { id: 'CONFIRMED', name: '已确认' }, { id: 'COMMISSION_CONFIRMED', name: '佣金已确认' }, { id: 'SETTLEMENT_PENDING', name: '待结算' }, { id: 'SETTLED', name: '已结算' }, { id: 'CANCELLED', name: '已取消' }, { id: 'INVALID', name: '已失效' }, { id: 'REFUNDED', name: '已退款' }]
const statusName = (id: string) => statuses.find(item => item.id === id)?.name || id
const channelName = (id: string) => channels.find(item => item.id === id)?.name || id
const dateText = (value: string) => value.replace('T', ' ').replace(/Z$/, ' UTC')
const attributionName = (value: string) => ({ SUB_ID: 'Tracking 凭证', LINK_REQUEST: '链接请求凭证', CHANNEL_POSITION: '渠道推广位' }[value] || value)
watch(() => state.filter, filter => {
  fieldVersion.value++
  Object.assign(draft, { channel: filter.channel || '', orderStatus: filter.orderStatus || '', positionId: filter.positionId || '', from: filter.from?.slice(0, 10) || '', to: filter.to ? new Date(Date.parse(filter.to) - 86400000).toISOString().slice(0, 10) : '' })
  filterError.value = ''
}, { flush: 'sync' })
function day(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return null
  const parsed = new Date(value + 'T00:00:00Z')
  return Number.isFinite(parsed.getTime()) && parsed.toISOString().slice(0, 10) === value ? parsed : null
}
function apply() {
  const from = draft.from ? day(draft.from) : null, to = draft.to ? day(draft.to) : null
  if ((draft.from && !from) || (draft.to && !to) || (from && to && from > to) || (to && to.getUTCFullYear() >= 9999)) { filterError.value = '请检查日期：格式 YYYY-MM-DD，结束日期不得早于开始日期。'; return }
  filterError.value = ''
  props.model.setFilter({ channel: draft.channel, orderStatus: draft.orderStatus, positionId: draft.positionId.trim(), from: from?.toISOString() || '', to: to ? new Date(to.getTime() + 86400000).toISOString() : '' })
}
function reset() {
  // Retire inputs with pending throttled emissions before clearing their drafts.
  fieldVersion.value++
  Object.assign(draft, { channel: '', orderStatus: '', positionId: '', from: '', to: '' }); filterError.value = ''; props.model.setFilter({})
}
function commitField(field: 'positionId' | 'from' | 'to', event: unknown) {
  // uni H5 throttles model updates; blur contains the latest actual field value.
  const value = (event as { detail?: { value?: unknown } })?.detail?.value
  if (typeof value === 'string') draft[field] = value
}
onMounted(() => { void props.model.refresh() })
</script>
<template>
  <view class="promoter-orders" :aria-busy="state.status === 'loading'">
    <text class="notice">仅本人已归因的推广订单，不是消费者返现订单。渠道订单状态不代表收益或到账，金额暂不展示。</text>
    <view v-if="state.status === 'login-required'" class="panel" role="status">
      <text class="heading">请登录后查看本人推广订单</text><text>真实登录服务尚未接入或当前会话不可用；本页不会创建演示身份或显示测试订单。</text>
    </view>
    <template v-else>
      <view class="panel" data-test="order-filter">
        <text class="heading">筛选本人推广订单</text>
        <view class="chips" role="group" aria-label="渠道筛选">
          <button role="button" tabindex="0" v-for="item in channels" :key="item.id" :data-test="'channel-' + item.id" :aria-pressed="draft.channel === item.id" :class="{ selected: draft.channel === item.id }" @tap="draft.channel = item.id" @keydown.enter="draft.channel = item.id" @keydown.space.prevent="draft.channel = item.id">{{ item.name }}</button>
        </view>
        <view class="chips" role="group" aria-label="订单状态筛选">
          <button role="button" tabindex="0" v-for="item in statuses" :key="item.id" :data-test="'status-' + item.id" :aria-pressed="draft.orderStatus === item.id" :class="{ selected: draft.orderStatus === item.id }" @tap="draft.orderStatus = item.id" @keydown.enter="draft.orderStatus = item.id" @keydown.space.prevent="draft.orderStatus = item.id">{{ item.name }}</button>
        </view>
        <label class="field"><text>推广位编号</text><input :key="'position-' + fieldVersion" v-model="draft.positionId" @blur="commitField('positionId', $event)" maxlength="128" data-test="position-filter" aria-label="推广位编号" placeholder="不填则全部推广位" /></label>
        <text class="muted">按订单发生日期筛选（UTC），包含开始和结束当天。</text>
        <label class="field"><text>开始日期</text><input :key="'from-' + fieldVersion" v-model="draft.from" @blur="commitField('from', $event)" maxlength="10" data-test="from-filter" aria-label="开始日期 YYYY-MM-DD" placeholder="YYYY-MM-DD，可不填" /></label>
        <label class="field"><text>结束日期</text><input :key="'to-' + fieldVersion" v-model="draft.to" @blur="commitField('to', $event)" maxlength="10" data-test="to-filter" aria-label="结束日期 YYYY-MM-DD" placeholder="YYYY-MM-DD，可不填" /></label>
        <text v-if="filterError" role="alert" class="error">{{ filterError }}</text>
        <view class="actions"><button role="button" tabindex="0" data-test="order-apply" @tap="apply" @keydown.enter="apply" @keydown.space.prevent="apply">应用筛选</button><button role="button" tabindex="0" data-test="order-reset" @tap="reset" @keydown.enter="reset" @keydown.space.prevent="reset">清除筛选</button></view>
      </view>
      <view class="list" aria-live="polite">
        <text v-if="state.status === 'loading'" role="status">正在读取本人推广订单…</text>
        <text v-else-if="state.status === 'empty'" role="status">暂无本人推广订单，不代表没有渠道订单或已经完成结算。</text>
        <text v-else-if="state.error" role="alert" class="error">{{ state.error }}</text>
        <view v-for="item in state.items" :key="item.id" class="panel" data-test="promoter-order">
          <view class="summary"><text class="heading">{{ channelName(item.channel) }} · {{ item.maskedOrderId }}</text><text class="badge">{{ statusName(item.orderStatus) }}</text></view>
          <text>推广位：{{ item.positionId }}</text><text class="muted">订单发生：{{ dateText(item.orderOccurredAt) }}</text>
          <button role="button" tabindex="0" data-test="order-open" :aria-expanded="state.detailId === item.id" @tap="model.openDetail(item.id)" @keydown.enter="model.openDetail(item.id)" @keydown.space.prevent="model.openDetail(item.id)">查看订单详情</button>
        </view>
        <text v-if="state.pageError" role="alert" class="error">{{ state.pageError }}</text>
        <button role="button" tabindex="0" v-if="state.cursor" data-test="order-more" :disabled="state.loadingMore" @tap="model.loadMore()" @keydown.enter="model.loadMore()" @keydown.space.prevent="model.loadMore()">{{ state.loadingMore ? '正在加载…' : state.pageError ? '重试加载更多' : '加载更多推广订单' }}</button>
        <button role="button" tabindex="0" data-test="order-refresh" :disabled="state.status === 'loading'" @tap="model.refresh()" @keydown.enter="model.refresh()" @keydown.space.prevent="model.refresh()">刷新本人推广订单</button>
      </view>
      <view v-if="state.detailId" class="panel detail" data-test="order-detail" :aria-busy="state.detailStatus === 'loading'" aria-live="polite">
        <text class="heading">推广订单详情</text>
        <text v-if="state.detailStatus === 'loading'">正在读取订单详情…</text>
        <text v-else-if="state.detailError" role="alert" class="error">{{ state.detailError }}</text>
        <template v-if="state.detail">
          <text>{{ channelName(state.detail.channel) }} · {{ state.detail.maskedOrderId }} · {{ statusName(state.detail.orderStatus) }}</text>
          <text>推广位：{{ state.detail.positionId }}</text><text>归因方式：{{ attributionName(state.detail.attributionMethod) }}</text>
          <text>归因时间：{{ dateText(state.detail.attributedAt) }}</text><text>订单发生：{{ dateText(state.detail.orderOccurredAt) }}</text><text>状态时间：{{ dateText(state.detail.statusAt) }}</text>
          <text>退款事件数：{{ state.detail.refundEventCount }}（不是退款金额）</text>
          <text class="heading">已应用状态历史</text><text v-if="!state.detail.history.length">暂无已应用状态历史</text>
          <view v-for="(event, index) in state.detail.history" :key="index" class="history"><text>{{ event.previousStatus ? statusName(event.previousStatus) : '初始状态' }} → {{ statusName(event.status) }}</text><text class="muted">发生：{{ dateText(event.occurredAt) }}</text><text class="muted">记录：{{ dateText(event.projectedAt) }}</text></view>
        </template>
        <button role="button" tabindex="0" v-if="['error', 'unavailable'].includes(state.detailStatus)" data-test="order-detail-retry" @tap="model.openDetail(state.detailId)" @keydown.enter="model.openDetail(state.detailId)" @keydown.space.prevent="model.openDetail(state.detailId)">重试订单详情</button>
        <button role="button" tabindex="0" data-test="order-close" @tap="model.closeDetail()" @keydown.enter="model.closeDetail()" @keydown.space.prevent="model.closeDetail()">关闭订单详情</button>
      </view>
    </template>
  </view>
</template>
<style scoped>
.promoter-orders { color: #183f36; font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
.notice, .muted { display: block; color: #65776f; font-size: 13px; }
.notice { margin-bottom: 18px; }
.panel { margin: 14px 0; padding: 16px; border: 1px solid #dbe8e1; border-radius: 16px; background: #fff; }
.panel > text, .history > text { display: block; margin-bottom: 8px; }
.heading { font-size: 16px; font-weight: 650; }
.chips, .actions, .summary { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 14px; align-items: center; }
button { margin: 8px 0; min-height: 44px; border: 1px solid #d6e5dc; border-radius: 12px; background: #fff; color: #07594b; font-size: 14px; line-height: 1.5; padding: 12px; }
button::after { border: none; }
button:focus-visible, input:focus-visible { outline: 3px solid #f0ad63; outline-offset: 2px; }
.chips button { margin: 0; flex: 1 0 80px; }
.selected, .badge { background: #e5f4ed; color: #07594b; }
.badge { border-radius: 8px; padding: 4px 8px; }
.field { display: block; margin: 12px 0; }
.field > text { display: block; margin-bottom: 6px; }
input { border: 1px solid #d6e5dc; border-radius: 10px; padding: 12px; height: 60px; min-height: 60px; font-size: 14px; }
.actions button { flex: 1; }
.error { display: block; color: #a44320; margin: 12px 0; }
.history { border-top: 1px solid #dbe8e1; padding: 12px 0; }
</style>
