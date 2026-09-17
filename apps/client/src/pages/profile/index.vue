<script setup lang="ts">
import UnavailableState from '../../components/UnavailableState.vue'
function openPromotion() { uni.navigateTo({ url: '/pages/promotion/index' }) }
</script>

<template>
  <view class="client-page">
    <text class="page-title">我的</text>
    <UnavailableState title="登录服务尚未接入" description="身份接入前不展示个人订单、余额或收益。" />
    <button class="entry-button" role="button" tabindex="0" data-test="promotion-secondary" @tap="openPromotion" @keydown.enter="openPromotion" @keydown.space.prevent="openPromotion">推广中心<image src="/static/icons/arrow.svg" /></button>
  </view>
</template>
