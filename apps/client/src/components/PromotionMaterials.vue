<script setup lang="ts">
import type { createMaterialsModel } from '../features/promotion/materials-model'
import type { MaterialCard } from '../features/promotion/materials-api'
const props = defineProps<{ model: ReturnType<typeof createMaterialsModel> }>()
const state = props.model.state
const platforms = [{ id: 'JD' as const, name: '京东' }, { id: 'TAOBAO' as const, name: '淘宝' }, { id: 'MEITUAN' as const, name: '美团' }]
const types = [{ id: 'PRODUCT' as const, name: '商品' }, { id: 'ACTIVITY' as const, name: '活动' }]
const reasons: Record<string, string> = { UNCONFIGURED: '目录未配置', PENDING_VERIFICATION: '目录待核验', SUSPENDED: '目录已停用', INVALID_DECLARATION: '目录声明无效', SCOPE_MISMATCH: '当前范围不适用', INVALID_EVIDENCE: '目录证据无效', VERIFICATION_EXPIRED: '目录核验已过期', POSITION_UNAVAILABLE: '本人推广位不可用', NOT_ENABLED: '推广身份未启用' }
function typeDisabled(type: 'PRODUCT' | 'ACTIVITY') { return state.platform === 'MEITUAN' && type === 'PRODUCT' }
function selectType(type: 'PRODUCT' | 'ACTIVITY') { if (!typeDisabled(type)) props.model.selectType(type) }
function facts(item: MaterialCard) {
  return [
    `类型：${item.type === 'ACTIVITY' ? '活动' : '商品'} · 来源：${item.platform === 'JD' ? '京东' : item.platform === 'TB' ? '淘宝' : '美团'}`,
    ...(item.startsAt ? [`开始：${item.startsAt}`] : []), `有效期至：${item.endsAt}`,
    `地域：${item.region.mode === 'NATIONWIDE' ? '全国' : item.region.cityCodes?.join('、')}`,
    `适用终端：${item.terminals.map(t => t === 'H5' ? 'H5' : '微信小程序').join('、')}`,
    `业务：${item.business || '不限业务'}`, `来源更新：${item.sourceUpdatedAt}`, `规则版本：${item.ruleVersion}`,
  ]
}
</script>
<template>
  <view class="materials">
    <view class="intro"><text class="heading">推广选品</text><text>选择商品或活动，核对信息与本人推广位。</text></view>
    <text class="notice">目录可读取不代表可生成；推广链接生成尚未接入，暂不可生成。</text>
    <text v-if="state.status === 'login-required'" role="status">请登录后查看本人推广物料；当前没有可用的登录身份。</text>
    <template v-else>
      <view class="chips" aria-label="物料平台">
        <button v-for="platform in platforms" :key="platform.id" role="button" tabindex="0" :data-test="'materials-platform-' + platform.id" :aria-pressed="state.platform === platform.id" :class="{ selected: state.platform === platform.id }" @tap="model.selectPlatform(platform.id)" @keydown.enter="model.selectPlatform(platform.id)" @keydown.space.prevent="model.selectPlatform(platform.id)">{{ platform.name }}</button>
      </view>
      <view class="chips" aria-label="物料类型">
        <button v-for="type in types" :key="type.id" role="button" :tabindex="typeDisabled(type.id) ? -1 : 0" :data-test="'materials-type-' + type.id" :aria-pressed="state.type === type.id" :disabled="typeDisabled(type.id)" :class="{ selected: state.type === type.id }" @tap="selectType(type.id)" @keydown.enter="selectType(type.id)" @keydown.space.prevent="selectType(type.id)">{{ type.name }}</button>
      </view>
      <text v-if="state.platform === 'MEITUAN'" class="muted">美团首期仅授权活动；任意商品推广未开放。</text>
      <slot name="position" />
      <view aria-live="polite" :aria-busy="state.status === 'loading' || state.loadingMore">
        <text v-if="state.status === 'position-required'" role="status">请选择本人推广位与投放场景后读取目录。</text>
        <text v-else-if="state.status === 'loading'" role="status">正在读取授权物料目录…</text>
        <text v-else-if="state.status === 'empty'" role="status">本范围暂无适用物料，不提供演示商品。</text>
        <text v-else-if="state.status === 'blocked'" role="status" class="warning">{{ reasons[state.reason] || '当前目录不可用' }}；请核对身份、推广位及渠道配置。</text>
        <text v-else-if="state.status === 'unavailable'" role="status">推广物料服务或当前类型尚未接入。</text>
        <text v-if="state.error" role="alert" class="error">{{ state.error }}</text>
        <text v-if="state.notice" class="muted">{{ state.notice }}</text>
        <view v-for="item in state.items" :key="item.id" class="panel" data-test="material-card">
          <text class="heading">{{ item.title }}</text>
          <text v-for="fact in facts(item)" :key="fact" class="muted">{{ fact }}</text>
          <view class="actions">
            <button role="button" tabindex="0" data-test="material-view" @tap="model.openDetail(item.id)" @keydown.enter="model.openDetail(item.id)" @keydown.space.prevent="model.openDetail(item.id)">查看{{ item.type === 'ACTIVITY' ? '活动' : '商品' }}</button>
            <button disabled data-test="material-generate">生成推广链接</button>
          </view>
          <text class="muted">生成能力尚未接入；不创建链接或Tracking。</text>
        </view>
        <view v-if="state.detailId" class="panel" data-test="material-detail" :aria-busy="state.detailStatus === 'loading'">
          <text class="heading">最新物料详情</text>
          <text v-if="state.detailStatus === 'loading'" role="status">正在重新核对物料…</text>
          <template v-if="state.detail?.item">
            <text class="heading">{{ state.detail.item.title }}</text>
            <text v-for="fact in facts(state.detail.item)" :key="fact" class="muted">{{ fact }}</text>
            <button disabled>生成推广链接（暂未接入）</button>
          </template>
          <text v-if="state.detailError" role="alert" class="error">{{ state.detailError }}</text>
          <button v-if="state.detailStatus === 'error' || state.detailStatus === 'unavailable'" role="button" tabindex="0" data-test="material-detail-retry" @tap="model.openDetail(state.detailId)" @keydown.enter="model.openDetail(state.detailId)" @keydown.space.prevent="model.openDetail(state.detailId)">重新核对详情</button>
          <button role="button" tabindex="0" data-test="material-detail-close" @tap="model.closeDetail()" @keydown.enter="model.closeDetail()" @keydown.space.prevent="model.closeDetail()">关闭详情</button>
        </view>
        <text v-if="state.pageError" role="alert" class="error">{{ state.pageError }}</text>
        <button v-if="state.cursor" role="button" tabindex="0" data-test="materials-more" :disabled="state.loadingMore" @tap="model.loadMore()" @keydown.enter="model.loadMore()" @keydown.space.prevent="model.loadMore()">{{ state.loadingMore ? '正在读取…' : state.pageError ? '重试加载更多' : '加载更多物料' }}</button>
        <button v-if="state.scope" role="button" tabindex="0" data-test="materials-refresh" :disabled="state.status === 'loading'" @tap="model.refresh()" @keydown.enter="model.refresh()" @keydown.space.prevent="model.refresh()">刷新授权目录</button>
      </view>
    </template>
  </view>
</template>
<style scoped>
.materials { color: #183f36; font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
.intro { padding: 20px; margin-bottom: 16px; border-radius: 20px; background: linear-gradient(110deg, #dcf7e9, #eefbf5); }
.intro > text, .panel > text { display: block; margin-bottom: 8px; }
.heading { font-size: 20px; font-weight: 650; }
.notice, .warning { display: block; margin: 12px 0; color: #9a4b12; }
.muted { display: block; color: #65776f; font-size: 13px; }
.panel { margin: 16px 0; padding: 16px; border-radius: 16px; background: #fff; border: 1px solid #dbe8e1; }
.chips, .actions { display: flex; flex-wrap: wrap; gap: 8px; margin: 12px 0; }
button { margin: 8px 0; min-height: 44px; min-width: 44px; padding: 12px; font-size: 14px; line-height: 1.5; border: 1px solid #d6e5dc; border-radius: 12px; background: #fff; color: #07594b; }
button::after { border: none; }
button:focus-visible { outline: 3px solid #f0ad63; outline-offset: 2px; }
.chips button, .actions button { flex: 1 0 80px; margin: 0; }
.selected { background: #e5f4ed; border-color: #098556; }
button[disabled] { color: #737d78; background: #eef1ef; }
.error { display: block; color: #a44320; margin: 12px 0; }
</style>
