<script setup lang="ts">
import UnavailableState from '../../components/UnavailableState.vue'
function openPromotion() { uni.switchTab({ url: '/pages/promotion/index' }) }
function openOrders() { uni.navigateTo({ url: '/pages/orders/index' }) }
function openAI() { uni.navigateTo({ url: '/pages/ai/index' }) }
</script>

<template>
  <view class="client-page">
    <text class="page-title">我的</text>
    <UnavailableState title="登录服务尚未接入" description="身份接入前不展示个人订单、余额或收益。" />
    <button class="entry-button" role="button" tabindex="0" data-test="promotion-secondary" @tap="openPromotion" @keydown.enter="openPromotion" @keydown.space.prevent="openPromotion">推广中心<image src="/static/icons/arrow.svg" /></button>
    <button class="entry-button" role="button" tabindex="0" data-test="orders-secondary" @tap="openOrders" @keydown.enter="openOrders" @keydown.space.prevent="openOrders">订单<image src="/static/icons/arrow.svg" /></button>
    <button class="entry-button" role="button" tabindex="0" data-test="ai-secondary" @tap="openAI" @keydown.enter="openAI" @keydown.space.prevent="openAI">AI 帮我选<image src="/static/icons/arrow.svg" /></button>
  </view>
</template>
