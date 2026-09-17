<script setup lang="ts">
import LinkInput from '../../components/LinkInput.vue'

function readClipboard(): Promise<string> {
  return new Promise((resolve, reject) => {
    uni.getClipboardData({ success: result => resolve(result.data), fail: reject })
  })
}
</script>

<template>
  <view class="client-page">
    <text class="page-title">准备推广链接</text>
    <LinkInput :ready="false" :read-clipboard="readClipboard" />
    <text class="future-formats">二维码与海报暂未开放</text>
  </view>
</template>

<style scoped>
.future-formats { display: block; margin-top: 20px; text-align: center; color: #78847e; font-size: 13px; }
</style>
