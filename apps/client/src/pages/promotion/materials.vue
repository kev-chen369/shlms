<script setup lang="ts">
import { inject, ref, reactive, watch, onMounted } from 'vue'
import PromotionMaterials from '../../components/PromotionMaterials.vue'
import { createMaterialsWorkbench } from '../../features/promotion/materials-workbench'
import { promoterSessionKey } from '../../features/promotion/session'
const session = inject(promoterSessionKey, ref<string | null>(null))
let terminal: 'H5' | 'WX_MINI' = 'H5'
// #ifdef MP-WEIXIN
terminal = 'WX_MINI'
// #endif
const workbench = createMaterialsWorkbench(session, terminal)
const state = workbench.state, materialState = workbench.materials.state
const draft = reactive({ cityCode: '', business: '' }), fieldVersion = ref(0)
watch(() => [session.value, materialState.platform, materialState.type], () => { draft.cityCode = ''; draft.business = ''; fieldVersion.value++ }, { flush: 'sync' })
onMounted(() => { void workbench.refreshPositions() })
function commit(field: 'cityCode' | 'business', event: unknown) {
  const value = (event as { detail?: { value?: unknown } })?.detail?.value
  if (typeof value === 'string') draft[field] = value
}
function back() { uni.switchTab({ url: '/pages/promotion/index' }) }
const readiness: Record<string, string> = { WAITING_CONFIGURATION: '渠道待配置', WAITING_VERIFICATION: '渠道待核验', UNAVAILABLE: '不可用' }
</script>
<template>
  <view class="client-page">
    <text v-if="state.status === 'login-required'" role="status">请登录后查看本人推广选品；当前没有可用的登录身份。</text>
    <PromotionMaterials v-else :model="workbench.materials">
      <template #position>
        <view class="positions" aria-live="polite" :aria-busy="state.status === 'loading' || state.loadingMore">
          <text class="heading">选择本人推广位</text>
          <text class="muted">内部位存在及京东配置状态不代表目录或生成授权；目录由服务端按完整范围再次核验。</text>
          <text v-if="materialState.platform !== 'JD'" role="status">当前接口没有{{ materialState.platform === 'TAOBAO' ? '淘宝' : '美团' }}推广位映射，暂未开放；不能使用京东位。</text>
          <template v-else>
            <text v-if="state.status === 'loading'" role="status">正在读取本人已启用推广位…</text>
            <text v-if="state.status === 'empty'" role="status">暂无本人已启用推广位，请先完成推广资格与位配置。</text>
            <text v-if="state.status === 'unavailable'" role="status">推广位服务尚未接入。</text>
            <view v-for="item in state.items" :key="item.id" class="choice">
              <button role="button" :tabindex="workbench.applicable(item) ? 0 : -1" data-test="position-choice" :aria-pressed="state.selectedId === item.id" :disabled="!workbench.applicable(item)" :class="{ selected: state.selectedId === item.id }" @tap="workbench.choose(item.id)" @keydown.enter="workbench.choose(item.id)" @keydown.space.prevent="workbench.choose(item.id)">{{ item.name }}{{ item.isDefault ? '（默认位，仍需选择）' : '' }}</button>
              <text class="muted">场景：{{ item.scene }} · {{ readiness[item.channels[0].readiness] }}</text>
              <text v-if="!workbench.applicable(item)" class="muted">位或场景不适用当前物料读取范围，请选择其他位。</text>
            </view>
            <text v-if="state.pageError" role="alert" class="error">{{ state.pageError }}</text>
            <button v-if="state.cursor" role="button" tabindex="0" :disabled="state.loadingMore" data-test="positions-more" @tap="workbench.morePositions()" @keydown.enter="workbench.morePositions()" @keydown.space.prevent="workbench.morePositions()">{{ state.loadingMore ? '正在读取…' : state.pageError ? '重试更多推广位' : '更多本人推广位' }}</button>
            <button role="button" tabindex="0" :disabled="state.status === 'loading'" data-test="positions-refresh" @tap="workbench.refreshPositions()" @keydown.enter="workbench.refreshPositions()" @keydown.space.prevent="workbench.refreshPositions()">刷新本人推广位</button>
            <template v-if="state.selectedId">
              <text>投放场景：{{ state.scene }}（使用所选位的服务端场景）</text>
              <label class="field"><text>城市编码（可不填）</text><input :key="'city-' + fieldVersion" v-model="draft.cityCode" maxlength="32" aria-label="城市编码" @blur="commit('cityCode', $event)" @confirm="commit('cityCode', $event)" /></label>
              <label class="field"><text>业务范围（可不填）</text><input :key="'business-' + fieldVersion" v-model="draft.business" maxlength="40" aria-label="业务范围" @blur="commit('business', $event)" @confirm="commit('business', $event)" /></label>
              <button role="button" tabindex="0" data-test="position-confirm" @tap="workbench.apply(draft.cityCode, draft.business)" @keydown.enter="workbench.apply(draft.cityCode, draft.business)" @keydown.space.prevent="workbench.apply(draft.cityCode, draft.business)">确认推广位与场景，读取目录</button>
            </template>
          </template>
          <text v-if="state.error" role="alert" class="error">{{ state.error }}</text>
        </view>
      </template>
    </PromotionMaterials>
    <button class="entry-button" role="button" tabindex="0" data-test="materials-back" @tap="back" @keydown.enter="back" @keydown.space.prevent="back">返回推广中心</button>
  </view>
</template>
<style scoped>
.positions { padding: 16px; margin: 16px 0; border: 1px solid #dbe8e1; border-radius: 16px; background: #fff; color: #183f36; font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
.positions > text, .field > text { display: block; margin-bottom: 8px; }
.heading { font-size: 16px; font-weight: 650; }
.muted { display: block; color: #65776f; font-size: 13px; }
.choice { margin: 12px 0; }
.field { display: block; margin: 12px 0; }
input { border: 1px solid #d6e5dc; border-radius: 10px; padding: 12px; height: 60px; min-height: 60px; font-size: 14px; }
button { margin: 8px 0; min-height: 44px; min-width: 44px; padding: 12px; border: 1px solid #d6e5dc; border-radius: 12px; background: #fff; color: #07594b; font-size: 14px; line-height: 1.5; }
button::after { border: none; }
button:focus-visible, input:focus-visible { outline: 3px solid #f0ad63; outline-offset: 2px; }
.selected { background: #e5f4ed; border-color: #098556; }
.error { display: block; color: #a44320; margin: 12px 0; }
</style>
