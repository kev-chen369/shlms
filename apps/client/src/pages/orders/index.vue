<script setup lang="ts">
import UnavailableState from '../../components/UnavailableState.vue'
function returnToProfile() { uni.switchTab({ url: '/pages/profile/index' }) }
</script>

<template>
  <view class="client-page">
    <text class="page-title">订单</text>
    <UnavailableState title="订单服务尚未接入" description="真实身份与订单服务尚未接入，不展示演示订单。" />
    <button class="entry-button" role="button" tabindex="0" data-test="orders-back" @tap="returnToProfile" @keydown.enter.prevent="returnToProfile" @keydown.space.prevent="returnToProfile">返回我的<image src="/static/icons/arrow.svg" /></button>
  </view>
</template>
