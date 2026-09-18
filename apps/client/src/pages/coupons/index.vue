<script setup lang="ts">
import PlatformTabs from '../../components/PlatformTabs.vue'
import UnavailableState from '../../components/UnavailableState.vue'
import { selectedPlatformName } from '../../features/platform'
</script>

<template>
  <view class="client-page">
    <text class="page-title">领券中心</text>
    <PlatformTabs />
    <UnavailableState :title="`${selectedPlatformName}客户端券目录未接入`" description="当前客户端尚未接入已核验券目录，不展示演示券，也不能领取或跳转下单。" />
  </view>
</template>
