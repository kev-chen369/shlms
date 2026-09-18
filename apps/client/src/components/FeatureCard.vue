<script setup lang="ts">
defineProps<{ title: string; description: string; action: string; tone: 'mint' | 'peach'; icon: string }>()
defineEmits<{ activate: [] }>()
</script>

<template>
  <view class="feature-card" :class="tone">
    <view class="feature-heading"><image :src="icon" class="feature-icon" /><text>{{ title }}</text></view>
    <text class="feature-description">{{ description }}</text>
    <button class="feature-action" role="button" tabindex="0" @tap="$emit('activate')" @keydown.enter="$emit('activate')" @keydown.space.prevent="$emit('activate')">{{ action }}<image src="/static/icons/arrow.svg" /></button>
  </view>
</template>

<style scoped>
.feature-card { padding: 18px 14px; border-radius: 18px; height: 158px; color: #07594b; }
.mint { background: #eaf6f0; }
.peach { background: #fff1e7; }
.feature-heading { display: flex; align-items: center; gap: 7px; font-size: 21px; font-weight: 750; white-space: nowrap; }
.feature-icon { width: 23px; height: 23px; flex-shrink: 0; }
.feature-description { display: block; margin-top: 9px; font-size: 13px; color: #63716a; }
.feature-action { display: flex; align-items: center; justify-content: center; gap: 6px; width: 100%; min-height: 44px; margin-top: 17px; padding: 0 5px; border: 0; background: #ffffffcf; border-radius: 24px; color: #07594b; font-size: 14px; font-weight: 650; line-height: 44px; }
.feature-action::after { border: 0; }
.feature-action image { width: 19px; height: 19px; }
@media (max-width: 350px) { .feature-card { padding: 16px 10px; } .feature-heading { font-size: 18px; gap: 4px; } .feature-description { font-size: 12px; } .feature-action { font-size: 12px; } }
</style>
