<script setup lang="ts">
import PromotionState from '../../components/PromotionState.vue'
</script>

<template>
  <view class="client-page">
    <text class="page-title">推广中心</text>
    <PromotionState :state="{ kind: 'login-required' }" />
  </view>
</template>
