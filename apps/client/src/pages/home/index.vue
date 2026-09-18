<script setup lang="ts">
import { ref, watch } from 'vue'
import PlatformTabs from '../../components/PlatformTabs.vue'
import CouponCatalog from '../../components/CouponCatalog.vue'
import { selectedPlatform, selectedPlatformName } from '../../features/platform'

const query = ref('')
const notice = ref('')
watch(selectedPlatform, () => { notice.value = '' })
function search() { notice.value = '商品搜索与链接解析暂未接入，当前不会生成商品结果。' }
function openCoupons() { uni.switchTab({ url: '/pages/coupons/index' }) }
function openPromotion() { uni.switchTab({ url: '/pages/promotion/index' }) }
function explain(kind: string) { notice.value = `${selectedPlatformName.value}${kind}未接入，授权与数据核验完成前不可跳转购买。` }
</script>

<template>
  <view class="client-page home-page">
    <view class="home-header">
      <view class="brand-block"><text class="brand">万宝单生活</text><text class="brand-caption">好物更省 · 生活更美好</text></view>
      <view class="search-bar">
        <image src="/static/icons/search.svg" class="search-icon" />
        <input v-model="query" placeholder="搜商品、商店或优惠券" aria-label="搜索商品、商店或优惠券" confirm-type="search" @confirm="search" />
        <button role="button" tabindex="0" data-test="search-submit" @tap="search" @keydown.enter.prevent="search" @keydown.space.prevent="search">搜索</button>
      </view>
    </view>
    <PlatformTabs />
    <view class="platform-hero"><text class="hero-title">{{ selectedPlatformName }}精选</text><text class="hero-description">授权完成后展示真实商品与优惠</text></view>
    <view v-if="notice" class="notice" role="status" aria-live="polite" data-test="notice">{{ notice }}</view>
    <view class="home-module">
      <view class="module-heading"><text class="module-title products">推广商品</text><button role="button" tabindex="0" data-test="products-entry" @tap="explain('商品目录')" @keydown.enter.prevent="explain('商品目录')" @keydown.space.prevent="explain('商品目录')">查看更多<image src="/static/icons/chevron.svg" /></button></view>
      <view class="module-empty" role="status"><text>{{ selectedPlatformName }}商品目录未接入</text><text class="module-description">接入获批商品后展示，不使用演示价格或返现。</text></view>
    </view>
    <view class="home-module">
      <view class="module-heading"><text class="module-title stores">优选商店</text><button role="button" tabindex="0" data-test="stores-entry" @tap="explain('店铺目录')" @keydown.enter.prevent="explain('店铺目录')" @keydown.space.prevent="explain('店铺目录')">查看更多<image src="/static/icons/chevron.svg" /></button></view>
      <view class="module-empty" role="status"><text>{{ selectedPlatformName }}店铺目录未接入</text><text class="module-description">没有可核验的店铺与优惠，暂不可进店。</text></view>
    </view>
    <view class="home-module" data-test="coupon-entry">
      <view class="module-heading"><text class="module-title coupons">领券中心</text><button role="button" tabindex="0" @tap="openCoupons" @keydown.enter.prevent="openCoupons" @keydown.space.prevent="openCoupons">查看更多<image src="/static/icons/chevron.svg" /></button></view>
      <CouponCatalog compact />
    </view>
    <view class="home-module" data-test="promotion-entry">
      <view class="module-heading"><text class="module-title promotion">推广专区</text></view>
      <view class="promotion-panel"><view><text class="promotion-title">分享优质商品</text><text class="module-description">让更多人发现好物</text><text class="module-description">渠道开通后可用</text></view><button role="button" tabindex="0" @tap="openPromotion" @keydown.enter.prevent="openPromotion" @keydown.space.prevent="openPromotion">进入推广<image src="/static/icons/arrow.svg" /></button></view>
    </view>
  </view>
</template>

<style scoped>
.home-page { padding: 0 14px calc(94px + env(safe-area-inset-bottom)); background: #f8faf8; }
.home-header { display: flex; align-items: center; gap: 14px; margin: 0 -14px; padding: 22px 14px; background: linear-gradient(110deg, #f4fff9, #e5fbed); }
.brand-block { flex-shrink: 0; }
.brand { font-size: 23px; letter-spacing: -.6px; }
.brand-caption { display: block; margin-top: 5px; font-size: 10px; color: #6d7e74; }
.search-bar { display: flex; align-items: center; gap: 6px; flex: 1; min-width: 0; min-height: 48px; padding: 0 8px; border: 1px solid #dce4df; border-radius: 28px; background: white; }
.search-icon { width: 20px; height: 20px; flex-shrink: 0; }
.search-bar input { width: 0; min-width: 0; flex: 1; font-size: 13px; }
button { margin: 0; background: transparent; min-height: 44px; color: #66786b; font-size: 13px; line-height: 44px; }
button::after { border: none; }
.search-bar button { min-width: 44px; padding: 0; color: #07594b; flex-shrink: 0; }
.platform-hero { min-height: 136px; padding: 26px 22px; margin-bottom: 14px; border-radius: 18px; background: linear-gradient(110deg, #def8ec, #e8fbef); }
.hero-title { display: block; font-size: 28px; line-height: 1.3; color: #07583a; font-weight: 750; }
.hero-description { display: block; font-size: 13px; color: #287858; margin-top: 8px; }
.home-module { padding: 14px; margin-bottom: 12px; border-radius: 18px; border: 1px solid #eef2ef; background: white; }
.module-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; min-height: 44px; }
.module-title { padding-left: 11px; border-left: 4px solid; font-size: 19px; font-weight: 750; line-height: 1.3; }
.products { border-color: #e63946; } .stores { border-color: #f49b30; } .coupons { border-color: #0b9a5a; } .promotion { border-color: #3989ee; }
.module-heading button { display: flex; align-items: center; padding: 0; gap: 3px; flex-shrink: 0; }
.module-heading image { width: 14px; height: 14px; }
.module-empty { padding: 18px 14px; margin-top: 8px; border: 1px dashed #dce7df; border-radius: 12px; background: #fcfefd; font-size: 14px; color: #344b42; }
.module-description { display: block; margin-top: 6px; font-size: 12px; line-height: 1.6; color: #78847e; }
.promotion-panel { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 16px 0 4px; }
.promotion-title { display: block; font-size: 18px; font-weight: 650; }
.promotion-panel button { display: flex; align-items: center; gap: 6px; flex-shrink: 0; min-height: 44px; padding: 0 14px; border: 1px solid #0b9a5a; color: #07594b; border-radius: 24px; }
.promotion-panel image { width: 18px; height: 18px; }
.notice { padding: 12px 14px; margin-bottom: 12px; background: #eaf6f0; border-radius: 12px; font-size: 13px; }
@media (max-width: 360px) { .home-header { flex-wrap: wrap; } .search-bar { flex-basis: 100%; } .promotion-panel { flex-wrap: wrap; } }
</style>
