<script setup lang="ts">
import PromotionState from '../../components/PromotionState.vue'
function openLinkInput() { uni.navigateTo({ url: '/pages/promotion/convert' }) }
function openOrders() { uni.navigateTo({ url: '/pages/promotion/orders' }) }
function openDashboard() { uni.navigateTo({ url: '/pages/promotion/dashboard' }) }
</script>

<template>
  <view class="client-page">
    <text class="page-title">推广中心</text>
    <PromotionState :state="{ kind: 'login-required' }" />
    <button class="entry-button" role="button" tabindex="0" data-test="promoter-dashboard-entry" @tap="openDashboard" @keydown.enter="openDashboard" @keydown.space.prevent="openDashboard">本人推广统计 <text>→</text></button>
    <button class="entry-button" role="button" tabindex="0" data-test="promoter-orders-entry" @tap="openOrders" @keydown.enter="openOrders" @keydown.space.prevent="openOrders">本人推广订单 <text>→</text></button>
    <button class="entry-button" role="button" tabindex="0" @tap="openLinkInput" @keydown.enter="openLinkInput" @keydown.space.prevent="openLinkInput">准备商品链接 <text>→</text></button>
  </view>
</template>
