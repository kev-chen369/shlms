<script setup lang="ts">
import { computed, ref, watch, onUnmounted } from 'vue'
import { useCouponCatalog } from '../features/coupons/catalog'
defineProps<{ compact?: boolean }>()
const catalog = useCouponCatalog(), state = catalog.state
const cityOpen = ref(false), businessDraft = ref(state.context.business)
const businessVersion = ref(0)
let expiryTimer: ReturnType<typeof setInterval> | undefined
watch(() => state.items.length, length => {
  if (length && !expiryTimer) expiryTimer = setInterval(() => catalog.pruneExpired(), 1000)
  else if (!length && expiryTimer) { clearInterval(expiryTimer); expiryTimer = undefined }
}, { immediate: true })
onUnmounted(() => { if (expiryTimer) clearInterval(expiryTimer) })
const cityName = computed(() => state.cities.find(city => city.code === state.context.cityCode)?.name || state.context.cityCode || '不限城市活动')
watch(() => state.context, () => { businessVersion.value++; businessDraft.value = state.context.business; cityOpen.value = false }, { flush: 'sync' })
function toggleCities() { cityOpen.value = !cityOpen.value }
function chooseCity(cityCode: string) { catalog.setContext({ ...state.context, cityCode }); cityOpen.value = false }
function applyBusiness() { catalog.setContext({ ...state.context, business: businessDraft.value.trim() }) }
function commitBusiness(event: unknown) {
  const value = (event as { detail?: { value?: unknown } })?.detail?.value
  if (typeof value === 'string') businessDraft.value = value
}
function confirmBusiness(event: unknown) { commitBusiness(event); applyBusiness() }
function money(minor: number) { return `¥${Math.floor(minor / 100)}.${String(minor % 100).padStart(2, '0')}` }
function scopeName(scope: string) { return ({ PRODUCT: '商品', CATEGORY: '品类', SHOP: '店铺', ACTIVITY: '活动' } as Record<string, string>)[scope] }
function dateText(date: string) { return new Date(date).toLocaleString('zh-CN', { hour12: false }) }
const statusText = computed(() => state.error || state.expiredNotice || ({ idle: '等待读取券目录', loading: '正在读取优惠…', empty: '暂无可用优惠' } as Record<string, string>)[state.status] || '')
</script>

<template>
  <view class="coupon-catalog">
    <view class="context-caption">{{ cityName }} · {{ state.context.business || '不限业务活动' }}</view>
    <view v-if="!compact && state.status !== 'unopened'" class="context-controls">
      <button role="button" tabindex="0" data-test="coupon-city-toggle" :aria-expanded="cityOpen" @tap="toggleCities" @keydown.enter.prevent="toggleCities" @keydown.space.prevent="toggleCities">城市：{{ cityName }}<image src="/static/icons/chevron.svg" /></button>
      <view v-if="cityOpen" class="city-options" role="group" aria-label="选择适用城市">
        <button role="button" tabindex="0" data-city="" :aria-pressed="state.context.cityCode === ''" @tap="chooseCity('')" @keydown.enter.prevent="chooseCity('')" @keydown.space.prevent="chooseCity('')">不限城市活动</button>
        <button v-for="city in state.cities" :key="city.code" role="button" tabindex="0" :data-city="city.code" :aria-pressed="state.context.cityCode === city.code" @tap="chooseCity(city.code)" @keydown.enter.prevent="chooseCity(city.code)" @keydown.space.prevent="chooseCity(city.code)">{{ city.name }}</button>
      </view>
      <text v-if="state.citiesStatus === 'loading'" role="status" class="hint">正在读取可用城市…</text>
      <view v-else-if="['error', 'unavailable'].includes(state.citiesStatus)" class="city-error"><text class="hint">城市选项暂不可用；未选城市时只展示不限城市活动。</text><button role="button" tabindex="0" @tap="catalog.loadCities" @keydown.enter.prevent="catalog.loadCities" @keydown.space.prevent="catalog.loadCities">重试城市</button></view>
      <view class="business-control"><input :key="businessVersion" v-model="businessDraft" data-test="coupon-business" aria-label="适用业务代码，可选" placeholder="适用业务代码（可选）" maxlength="40" @blur="commitBusiness" @confirm="confirmBusiness" /><button role="button" tabindex="0" data-test="coupon-business-apply" @tap="applyBusiness" @keydown.enter.prevent="applyBusiness" @keydown.space.prevent="applyBusiness">应用</button></view>
      <text class="hint">选择城市仍包含不限城市活动；业务留空仅展示不限业务活动。</text>
    </view>
    <view v-if="state.status !== 'ready'" class="catalog-status" role="status" aria-live="polite" data-test="coupon-status">{{ statusText }}<text v-if="state.status === 'empty'" class="hint">{{ state.nextCursor ? '本页优惠已到期，可加载更多或刷新目录。' : '当前已读取范围没有可用券活动，不展示演示券。' }}</text></view>
    <text v-else-if="state.expiredNotice" role="status" class="hint">{{ state.expiredNotice }}</text>
    <view v-for="item in (compact ? state.items.slice(0, 2) : state.items)" :key="item.id" class="coupon-card" data-test="coupon-card">
      <view class="coupon-value"><text class="amount">{{ money(item.discountMinor) }}</text><text class="hint">{{ item.thresholdMinor ? `满${money(item.thresholdMinor)}可用` : '无金额门槛' }}</text></view>
      <view class="coupon-copy"><text class="coupon-title">{{ item.title }}</text><text class="hint">{{ scopeName(item.scope) }}{{ item.scopeName ? ` · ${item.scopeName}` : '' }}</text><text class="hint">有效期至 {{ dateText(item.expiresAt) }}</text><button role="button" tabindex="0" data-test="coupon-detail-open" :aria-expanded="state.detailId === item.id" @tap="catalog.openDetail(item.id)" @keydown.enter.prevent="catalog.openDetail(item.id)" @keydown.space.prevent="catalog.openDetail(item.id)">查看规则<image src="/static/icons/chevron.svg" /></button></view>
    </view>
    <text v-if="state.items.length" class="hint readonly-notice">仅供查看，领取与购买暂未接通；最终优惠以平台结算为准。</text>
    <button v-if="['error', 'unavailable', 'not-found', 'empty', 'ready'].includes(state.status)" role="button" tabindex="0" data-test="coupon-retry" class="catalog-action" @tap="catalog.refresh" @keydown.enter.prevent="catalog.refresh" @keydown.space.prevent="catalog.refresh">{{ ['ready', 'empty'].includes(state.status) ? '刷新目录' : '重试目录' }}</button>
    <view v-if="!compact && state.pageError" role="status" class="catalog-status">{{ state.pageError }}，可重试加载更多。</view>
    <button v-if="!compact && state.nextCursor" role="button" tabindex="0" data-test="coupon-more" class="catalog-action" :disabled="state.loadingMore" @tap="catalog.loadMore" @keydown.enter.prevent="catalog.loadMore" @keydown.space.prevent="catalog.loadMore">{{ state.loadingMore ? '加载中…' : '加载更多优惠' }}</button>
    <view v-if="state.detailId" class="coupon-detail" data-test="coupon-detail" role="region" aria-label="券详情">
      <view class="detail-heading"><text class="coupon-title">券规则</text><button role="button" tabindex="0" data-test="coupon-detail-close" @tap="catalog.closeDetail" @keydown.enter.prevent="catalog.closeDetail" @keydown.space.prevent="catalog.closeDetail">关闭</button></view>
      <view v-if="state.detailStatus !== 'ready'" role="status" aria-live="polite">{{ state.detailStatus === 'loading' ? '正在读取规则…' : state.detailError }}</view>
      <view v-if="state.detail" class="detail-fields"><text>{{ state.detail.title }}</text><text>优惠金额：{{ money(state.detail.discountMinor) }}</text><text>使用门槛：{{ state.detail.thresholdMinor ? `满${money(state.detail.thresholdMinor)}` : '无金额门槛' }}</text><text>适用范围：{{ scopeName(state.detail.scope) }} {{ state.detail.scopeName }}</text><text>适用城市：{{ state.detail.cityName || state.detail.cityCode || '不限城市活动' }}</text><text>适用业务：{{ state.detail.business || '不限业务活动' }}</text><text>有效期至：{{ dateText(state.detail.expiresAt) }}</text><text>数据更新：{{ dateText(state.detail.updatedAt) }}</text><text>规则版本：{{ state.detail.ruleVersion }}</text><text>领取方式：{{ state.detail.actionLabel }}（暂不可操作）</text><text class="hint readonly-notice">领取与购买暂未接通，不表示券已领取；最终优惠以平台结算为准。</text></view>
      <button v-if="['error', 'unavailable'].includes(state.detailStatus)" role="button" tabindex="0" class="catalog-action" @tap="catalog.openDetail(state.detailId)" @keydown.enter.prevent="catalog.openDetail(state.detailId)" @keydown.space.prevent="catalog.openDetail(state.detailId)">重试规则</button>
      <view v-if="state.detailStatus === 'ready' && state.detail && state.detail.scope !== 'ACTIVITY'" class="applicable-products" data-test="coupon-products" role="region" aria-label="适用商品">
        <text class="coupon-title">适用商品</text>
        <text class="hint">仅展示已核验的商品身份与有效期，不提供实时价格或购买入口；适用资格仍须平台结算复核。</text>
        <view v-if="state.productsStatus !== 'ready'" data-test="coupon-products-status" class="hint" role="status" aria-live="polite">{{ state.productsError ? (state.productsStatus === 'unavailable' ? '适用商品服务尚未接入' : '适用商品读取失败，请稍后重试') : state.productsStatus === 'loading' ? '正在读取适用商品…' : '暂无可用适用商品' }}</view>
        <text v-if="state.productsNotice" class="hint" role="status">{{ state.productsNotice }}</text>
        <view v-for="product in state.products" :key="product.externalProductId" data-test="coupon-product" class="applicable-product">
          <text class="product-title">{{ product.title }}</text><text class="hint">商品 ID：{{ product.externalProductId }}</text><text class="hint">有效期至：{{ dateText(product.expiresAt) }}</text><text class="hint">数据更新：{{ dateText(product.updatedAt) }}</text>
        </view>
        <text v-if="state.productsPageError" class="hint" role="status">适用商品加载失败，可重试加载更多。</text>
        <button v-if="state.productsStatus !== 'loading'" role="button" tabindex="0" data-test="coupon-products-retry" class="catalog-action" @tap="catalog.refreshProducts" @keydown.enter.prevent="catalog.refreshProducts" @keydown.space.prevent="catalog.refreshProducts">{{ ['error', 'unavailable'].includes(state.productsStatus) ? '重试适用商品' : '刷新适用商品' }}</button>
        <button v-if="state.productsCursor" role="button" tabindex="0" data-test="coupon-products-more" class="catalog-action" :disabled="state.loadingMoreProducts" @tap="catalog.loadMoreProducts" @keydown.enter.prevent="catalog.loadMoreProducts" @keydown.space.prevent="catalog.loadMoreProducts">{{ state.loadingMoreProducts ? '正在加载适用商品…' : '加载更多适用商品' }}</button>
      </view>
    </view>
  </view>
</template>

<style scoped>
.coupon-catalog { margin-top: 8px; }
.context-caption, .hint { display: block; font-size: 12px; line-height: 1.6; color: #5f7169; overflow-wrap: anywhere; }
.context-caption { margin-bottom: 8px; }
button { margin: 0; min-height: 44px; font-size: 13px; line-height: 44px; color: #07594b; background: transparent; padding: 0 12px; }
button::after { border: none; }
button image { width: 14px; height: 14px; }
.context-controls { margin-bottom: 16px; padding: 12px; border: 1px solid #dce7df; border-radius: 14px; background: white; }
.context-controls > button { display: flex; align-items: center; gap: 6px; width: 100%; justify-content: space-between; text-align: left; }
.city-options { display: flex; flex-wrap: wrap; gap: 4px; margin: 8px 0; }
.city-options button { border: 1px solid #dce7df; border-radius: 22px; }
.city-options button[aria-pressed=true] { background: #e5fbed; text-decoration: underline; }
.city-error { margin-bottom: 8px; }
.business-control { display: flex; gap: 8px; align-items: center; margin: 8px 0; }
.business-control input { flex: 1; width: 0; min-width: 0; font-size: 13px; border-bottom: 1px solid #dce7df; min-height: 44px; }
.catalog-status { padding: 18px 12px; border-radius: 12px; border: 1px dashed #dce7df; background: #fcfefd; font-size: 14px; line-height: 1.6; }
.catalog-action { margin-top: 8px; width: 100%; border: 1px solid #dce7df; border-radius: 22px; }
.coupon-card { display: flex; align-items: stretch; margin-bottom: 10px; border: 1px solid #ffcfad; border-radius: 14px; background: linear-gradient(110deg, #fff6ed, #fffdfa); overflow: hidden; }
.coupon-value { flex: 0 0 100px; padding: 20px 10px; border-right: 1px dashed #ffc8a2; text-align: center; }
.amount { display: block; font-size: 23px; font-weight: 750; color: #ed5b13; overflow-wrap: anywhere; }
.coupon-copy { flex: 1; min-width: 0; padding: 12px; }
.coupon-title { display: block; font-size: 16px; font-weight: 650; overflow-wrap: anywhere; }
.coupon-copy button { display: flex; align-items: center; gap: 4px; padding-left: 0; }
.readonly-notice { margin: 8px 0; }
.coupon-detail { margin-top: 16px; padding: 14px; border: 1px solid #dce7df; border-radius: 14px; background: white; font-size: 14px; }
.detail-heading { display: flex; justify-content: space-between; align-items: center; gap: 8px; }
.detail-fields > text { display: block; margin: 6px 0; overflow-wrap: anywhere; }
.applicable-products { border-top: 1px solid #dce7df; margin-top: 16px; padding-top: 16px; }
.applicable-product { padding: 12px 0; border-bottom: 1px solid #dce7df; overflow-wrap: anywhere; }
.product-title { display: block; font-size: 14px; font-weight: 650; margin-bottom: 4px; }
@media (max-width: 360px) { .coupon-value { flex-basis: 86px; padding: 16px 8px; } .amount { font-size: 20px; } .coupon-copy { padding: 10px; } }
</style>
