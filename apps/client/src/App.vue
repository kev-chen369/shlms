<script setup lang="ts">
defineOptions({ name: 'WanhuiApp' })
// #ifdef H5
import { onMounted, onUnmounted } from 'vue'
import { enhanceH5Navigation } from './platform/h5-navigation'
let dispose: (() => void) | undefined
onMounted(() => { dispose = enhanceH5Navigation(document.body) })
onUnmounted(() => dispose?.())
// #endif
</script>

<style>
page {
  background: #f7faf8;
  color: #123c34;
  font-family: -apple-system, BlinkMacSystemFont, 'PingFang SC', 'Microsoft YaHei', sans-serif;
  font-size: 15px;
  line-height: 1.5;
}
view, text, button, input { box-sizing: border-box; }
.client-page {
  width: 100%;
  max-width: 520px;
  min-height: 100vh;
  margin: 0 auto;
  padding: 24px 18px calc(94px + env(safe-area-inset-bottom));
}
.brand { font-size: 26px; font-weight: 750; }
.page-title { display: block; font-size: 26px; font-weight: 750; margin-bottom: 24px; }
.entry-button { display: flex; align-items: center; justify-content: space-between; min-height: 52px; width: 100%; margin-top: 18px; padding: 0 18px; border-radius: 14px; font-size: 16px; line-height: 52px; background: #eaf6f0; color: #07594b; }
.entry-button::after { border: none; }
.entry-button image { width: 20px; height: 20px; }
.uni-tabbar { max-width: 520px; margin: 0 auto; }
button:focus-visible, input:focus-visible, [role=button]:focus-visible { outline: 2px solid #07594b; outline-offset: 3px; }
.uni-tabbar__item:focus-visible { outline-offset: -3px; }
</style>
