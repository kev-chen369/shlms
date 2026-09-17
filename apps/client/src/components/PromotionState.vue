<script setup lang="ts">
import type { PromotionViewState } from '../features/promotion/profile'
import { computed } from 'vue'
import UnavailableState from './UnavailableState.vue'

const props = defineProps<{ state: PromotionViewState }>()
const display = computed(() => {
  const state = props.state
  if (state.kind === 'login-required') return { title: '推广业务尚未开放', description: '登录服务尚未接入，暂不可申请、管理推广位或生成真实推广链接。' }
  if (state.kind === 'loading') return { title: '正在读取推广身份', description: '以服务端状态为准，请稍候。' }
  if (state.kind === 'error') return { title: '推广信息暂不可用', description: '未取得有效身份信息，不开放推广操作。' }
  const states = {
    NOT_APPLIED: { title: '尚未申请推广员', description: '申请入口将于真实登录与协议接入后开放，当前不能提交。' },
    PENDING: { title: '推广申请审核中', description: '审核通过前不能生成推广链接，请以实际审核结果为准。' },
    REJECTED: { title: '推广申请未通过', description: '请查看审核原因；重新申请需使用已接入的实际申请流程。' },
    ENABLED: { title: '推广资格已开通', description: '资格开通不代表渠道可转链；推广位、渠道授权和可用状态仍须核验。' },
    DISABLED: { title: '推广资格已停用', description: '停止新转链；历史记录保留，但外部旧链接行为以渠道与正式政策为准。' },
  }
  return states[state.profile.status]
})
const visibleReason = computed(() => {
  const state = props.state
  return state.kind === 'profile' && ['REJECTED', 'DISABLED'].includes(state.profile.status) ? state.profile.reason : ''
})
</script>

<template>
  <view :aria-busy="state.kind === 'loading'">
    <UnavailableState :title="display.title" :description="display.description" />
    <view v-if="visibleReason" class="user-reason"><text class="reason-label">原因</text><text>{{ visibleReason }}</text></view>
  </view>
</template>

<style scoped>
.user-reason { padding: 16px; margin-top: 14px; border-radius: 14px; background: #fff1e7; color: #6b5240; font-size: 14px; white-space: pre-wrap; overflow-wrap: anywhere; word-break: break-word; }
.reason-label { display: block; margin-bottom: 6px; font-weight: 650; }
</style>
