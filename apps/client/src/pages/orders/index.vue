<script setup lang="ts">
import UnavailableState from '../../components/UnavailableState.vue'
</script>

<template>
  <view class="client-page">
    <text class="page-title">订单</text>
    <UnavailableState title="订单服务尚未接入" description="真实身份与订单服务尚未接入，不展示演示订单。" />
  </view>
</template>
