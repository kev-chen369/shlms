<script setup lang="ts">
import { platforms, selectedPlatform } from '../features/platform'
</script>

<template>
  <view class="platform-tabs" role="group" aria-label="选择平台">
    <button v-for="platform in platforms" :key="platform.id" role="button" tabindex="0" :data-platform="platform.id" :aria-pressed="selectedPlatform === platform.id" :class="{ selected: selectedPlatform === platform.id }" @tap="selectedPlatform = platform.id" @keydown.enter.prevent="selectedPlatform = platform.id" @keydown.space.prevent="selectedPlatform = platform.id">{{ platform.name }}</button>
  </view>
</template>

<style scoped>
.platform-tabs { display: flex; overflow-x: auto; gap: 4px; padding: 10px 0; margin: 4px 0 12px; }
button { flex-shrink: 0; min-width: 70px; min-height: 44px; margin: 0; padding: 0 12px; border-radius: 24px; background: transparent; color: #344b42; font-size: 15px; line-height: 44px; }
button::after { border: none; }
.selected { background: #fff0f0; color: #dc3340; font-weight: 700; text-decoration: underline; text-underline-offset: 8px; text-decoration-thickness: 3px; }
</style>
