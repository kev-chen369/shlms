<script setup lang="ts">
import UnavailableState from '../../components/UnavailableState.vue'
</script>

<template>
  <view class="client-page">
    <text class="page-title">推广中心</text>
    <UnavailableState title="推广业务尚未开放" description="登录服务尚未接入，暂不可申请、管理推广位或生成真实推广链接。" />
  </view>
</template>
