<script setup lang="ts">
import { inject, ref } from 'vue'
import PromoterDashboard from '../../components/PromoterDashboard.vue'
import { createPromoterDashboardModel } from '../../features/promotion/dashboard-model'
import { promoterSessionKey } from '../../features/promotion/session'
const session = inject(promoterSessionKey, ref<string | null>(null))
const model = createPromoterDashboardModel(session)
function back() { uni.switchTab({ url: '/pages/promotion/index' }) }
</script>
<template>
  <view class="client-page">
    <text class="page-title">本人推广统计</text>
    <PromoterDashboard :model="model" />
    <button class="entry-button" role="button" tabindex="0" data-test="dashboard-back" @tap="back" @keydown.enter="back" @keydown.space.prevent="back">返回推广中心</button>
  </view>
</template>
