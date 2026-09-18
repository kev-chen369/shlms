<script setup lang="ts">
import { inject, ref } from 'vue'
import PromoterOrders from '../../components/PromoterOrders.vue'
import { createPromoterOrdersModel } from '../../features/promotion/orders-model'
import { promoterSessionKey } from '../../features/promotion/session'
const session = inject(promoterSessionKey, ref<string | null>(null))
const model = createPromoterOrdersModel(session)
function back() { uni.switchTab({ url: '/pages/promotion/index' }) }
</script>
<template>
  <view class="client-page">
    <text class="page-title">本人推广订单</text>
    <PromoterOrders :model="model" />
    <button class="entry-button" role="button" tabindex="0" @tap="back" @keydown.enter="back" @keydown.space.prevent="back">返回推广中心</button>
  </view>
</template>
