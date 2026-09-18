<script setup lang="ts">
import { ref } from 'vue'
import FeatureCard from '../../components/FeatureCard.vue'
import UnavailableState from '../../components/UnavailableState.vue'

const query = ref('')
const notice = ref('')
const channels = [
  { id: 'jd', name: '京东', mark: '京', tone: 'jd' },
  { id: 'pdd', name: '拼多多', mark: '拼', tone: 'pdd' },
  { id: 'meituan', name: '美团', mark: '美', tone: 'meituan' },
  { id: 'eleme', name: '饿了么', mark: '饿', tone: 'eleme' },
]
function open(url: string) { uni.navigateTo({ url }) }
function search() { notice.value = '商品搜索与链接解析暂未接入，当前不会生成商品结果。' }
</script>

<template>
  <view class="client-page home-page">
    <view class="home-header">
      <text class="brand">万宝单生活</text>
      <button class="city-button" role="button" tabindex="0" @tap="notice = '城市定位暂未接入，未获取你的位置信息。'" @keydown.enter="notice = '城市定位暂未接入，未获取你的位置信息。'" @keydown.space.prevent="notice = '城市定位暂未接入，未获取你的位置信息。'">选择城市<image src="/static/icons/chevron.svg" /></button>
      <button class="notification-button" role="button" tabindex="0" aria-label="通知" @tap="notice = '通知服务暂未接入。'" @keydown.enter="notice = '通知服务暂未接入。'" @keydown.space.prevent="notice = '通知服务暂未接入。'"><image src="/static/icons/bell.svg" /></button>
    </view>
    <view class="search-bar">
      <image src="/static/icons/search.svg" class="search-icon" />
      <input v-model="query" placeholder="搜商品，或粘贴商品链接" aria-label="搜索商品或商品链接" confirm-type="search" @confirm="search" />
      <button role="button" tabindex="0" data-test="search-submit" @tap="search" @keydown.enter="search" @keydown.space.prevent="search">搜索</button>
    </view>
    <view class="channel-row">
      <button v-for="channel in channels" :key="channel.id" role="button" tabindex="0" class="channel-button" :class="channel.tone" :data-test="`channel-${channel.id}`" @tap="notice = `${channel.name}渠道暂未接入，授权验证完成前不可跳转购买。`" @keydown.enter="notice = `${channel.name}渠道暂未接入，授权验证完成前不可跳转购买。`" @keydown.space.prevent="notice = `${channel.name}渠道暂未接入，授权验证完成前不可跳转购买。`">
        <text class="channel-mark">{{ channel.mark }}</text><text class="channel-name">{{ channel.name }}</text>
      </button>
    </view>
    <view class="feature-row">
      <FeatureCard data-test="ai-entry" title="AI 帮我选" description="说出需求，帮你比价" action="去试试" tone="mint" icon="/static/icons/sparkles.svg" @activate="open('/pages/ai/index')" />
      <FeatureCard data-test="promotion-entry" title="推广赚钱" description="分享好物，查看收益" action="进入推广中心" tone="peach" icon="/static/icons/share.svg" @activate="open('/pages/promotion/index')" />
    </view>
    <view v-if="notice" class="notice" role="status" aria-live="polite" data-test="notice">{{ notice }}</view>
    <view class="section-heading"><text>今天值得买</text><button role="button" tabindex="0" @tap="notice = '商品推荐暂未接入，暂无可核验的商品和优惠。'" @keydown.enter="notice = '商品推荐暂未接入，暂无可核验的商品和优惠。'" @keydown.space.prevent="notice = '商品推荐暂未接入，暂无可核验的商品和优惠。'">查看更多<image src="/static/icons/chevron.svg" /></button></view>
    <UnavailableState title="商品推荐暂未接入" description="真实商品与优惠接入后展示，不使用演示价格或返现。" />
    <view class="takeaway-strip" role="button" tabindex="0" @tap="notice = '外卖优惠暂未接入，暂不可领取。'" @keydown.enter="notice = '外卖优惠暂未接入，暂不可领取。'" @keydown.space.prevent="notice = '外卖优惠暂未接入，暂不可领取。'">
      <view><text class="takeaway-title">外卖红包</text><text class="takeaway-description">先领券，再下单</text></view><image src="/static/icons/chevron.svg" />
    </view>
  </view>
</template>

<style scoped>
.home-header { display: flex; align-items: center; gap: 12px; margin-bottom: 20px; }
.city-button { display: flex; align-items: center; gap: 4px; min-height: 44px; margin: 0; padding: 0 4px; font-size: 13px; background: none; color: #344b42; line-height: 44px; }
.city-button image { width: 14px; height: 14px; transform: rotate(90deg); }
.notification-button { display: flex; align-items: center; justify-content: center; margin: 0 0 0 auto; padding: 0; width: 44px; height: 44px; background: none; }
.notification-button image { width: 24px; height: 24px; }
button::after { border: none; }
.search-bar { display: flex; align-items: center; gap: 9px; height: 50px; padding: 0 12px; background: white; border: 1px solid #dce4df; border-radius: 14px; }
.search-icon { width: 22px; height: 22px; flex-shrink: 0; }
.search-bar input { flex: 1; width: 0; font-size: 14px; min-width: 0; }
.search-bar button { padding: 0 3px; margin: 0; min-width: 44px; min-height: 44px; flex-shrink: 0; background: none; color: #07594b; font-size: 13px; line-height: 44px; }
.channel-row { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 10px; margin: 16px 0 12px; }
.channel-button { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 7px; width: 100%; height: 90px; margin: 0; padding: 0; border-radius: 16px; line-height: 1.4; }
.channel-mark { display: flex; align-items: center; justify-content: center; width: 32px; height: 32px; border-radius: 11px; font-size: 18px; font-weight: 750; color: white; }
.channel-name { font-size: 14px; font-weight: 600; color: #243b32; }
.jd { background: #ffeded; } .jd .channel-mark { background: #ee3b3e; }
.pdd { background: #fff0ef; } .pdd .channel-mark { background: #e84448; }
.meituan { background: #fff8df; } .meituan .channel-mark { background: #f6c529; color: #423907; }
.eleme { background: #eaf5ff; } .eleme .channel-mark { background: #149de0; }
.feature-row { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.notice { margin-top: 16px; padding: 12px 14px; border-radius: 12px; background: #eaf6f0; color: #466357; font-size: 13px; }
.section-heading { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin: 20px 0 12px; }
.section-heading > text { font-size: 23px; font-weight: 750; }
.section-heading button { display: flex; align-items: center; gap: 3px; min-height: 44px; padding: 0; margin: 0; color: #6b756f; font-size: 13px; background: none; line-height: 44px; }
.section-heading image { width: 14px; height: 14px; }
.takeaway-strip { display: flex; align-items: center; justify-content: space-between; margin-top: 14px; padding: 18px; min-height: 94px; border-radius: 18px; background: #e5f3ed; color: #07594b; }
.takeaway-title { display: block; font-size: 21px; font-weight: 750; }
.takeaway-description { display: block; font-size: 14px; color: #6d7e74; margin-top: 3px; }
.takeaway-strip > image { width: 24px; height: 24px; }
</style>
