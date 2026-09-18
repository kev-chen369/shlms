<script setup lang="ts">
defineProps<{ title: string; description: string }>()
</script>

<template>
  <view class="unavailable-state" role="status">
    <image src="/static/icons/bag.svg" class="state-icon" />
    <text class="state-title">{{ title }}</text>
    <text class="state-description">{{ description }}</text>
  </view>
</template>

<style scoped>
.unavailable-state { display: flex; flex-direction: column; align-items: center; padding: 30px 16px; background: white; border: 1px solid #e5ece7; border-radius: 18px; text-align: center; }
.state-icon { width: 34px; height: 34px; opacity: .55; margin-bottom: 12px; }
.state-title { font-size: 16px; font-weight: 650; }
.state-description { margin-top: 7px; font-size: 13px; color: #78847e; line-height: 1.6; }
</style>
