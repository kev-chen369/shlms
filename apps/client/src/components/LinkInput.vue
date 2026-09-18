<script setup lang="ts">
import { ref } from 'vue'
// #ifdef H5
import { onMounted, onUnmounted } from 'vue'
const rootRef = ref<HTMLElement | { $el: HTMLElement } | null>(null)
let inputObserver: MutationObserver | undefined
// uni-app puts fallthrough attributes on uni-textarea, not its native input.
function labelNativeInput() {
  const value = rootRef.value
  const wrapper = value && ('$el' in value ? value.$el : value)
  const input = wrapper?.querySelector('textarea')
  input?.setAttribute('aria-label', '商品链接或文案')
}
onMounted(() => {
  const value = rootRef.value
  const root = value && ('$el' in value ? value.$el : value)
  if (root) {
    // uni replaces the native node after clear / disabled changes; observe
    // this component's subtree, never the whole document or attribute changes.
    inputObserver = new MutationObserver(labelNativeInput)
    inputObserver.observe(root, { childList: true, subtree: true })
  }
  labelNativeInput()
})
onUnmounted(() => inputObserver?.disconnect())
// #endif

const props = defineProps<{ ready: boolean; readClipboard: () => Promise<string> }>()
const emit = defineEmits<{ preview: [content: string] }>()
const content = ref('')
const inputVersion = ref(0)
const pending = ref(false)
const message = ref('')
function inputBytes(value: string) {
  return Array.from(value).reduce((total, char) => {
    const code = char.codePointAt(0)!
    return total + (code <= 0x7f ? 1 : code <= 0x7ff ? 2 : code <= 0xffff ? 3 : 4)
  }, 0)
}

async function paste() {
  if (pending.value) return
  pending.value = true
  message.value = ''
  try {
    const value = await props.readClipboard()
    if (!value.trim()) message.value = '剪贴板没有内容，请手动输入商品链接。'
    else if (inputBytes(value.trim()) > 4096) message.value = '内容不能超过 4096 个 UTF-8 字节，原输入已保留。'
    else { content.value = value; message.value = '已粘贴，请核对内容。' }
  } catch {
    message.value = '无法读取剪贴板，请手动粘贴；原输入已保留。'
  } finally { pending.value = false }
}
function clear() {
  if (pending.value) return
  // Dispose the native edit and its pending throttled uni input notification.
  inputVersion.value++
  content.value = ''
  message.value = ''
}
function preview() {
  if (!props.ready || pending.value) return
  const value = content.value.trim()
  if (!value) { message.value = '请输入商品链接或包含链接的文案。'; return }
  if (inputBytes(value) > 4096) { message.value = '内容不能超过 4096 个 UTF-8 字节。'; return }
  message.value = ''
  emit('preview', value)
}
</script>

<template>
  <view ref="rootRef" class="link-input" :aria-busy="pending">
    <label for="product-content" class="input-label">商品链接或文案</label>
    <textarea :key="inputVersion" id="product-content" v-model="content" aria-label="商品链接或文案" placeholder="粘贴商品链接或包含链接的文案" :maxlength="4096" :disabled="pending" />
    <view class="input-actions">
      <button data-action="paste" role="button" tabindex="0" :aria-disabled="pending" :disabled="pending" @click="paste" @keydown.enter.prevent="paste" @keydown.space.prevent="paste">{{ pending ? '正在读取…' : '粘贴' }}</button>
      <button data-action="clear" role="button" tabindex="0" :aria-disabled="pending" :disabled="pending" @click="clear" @keydown.enter.prevent="clear" @keydown.space.prevent="clear">清空</button>
    </view>
    <text v-if="message" class="input-message" role="status" aria-live="polite">{{ message }}</text>
    <view v-if="!ready" class="readiness-notice" role="status">登录、推广位与渠道尚未完成联调。可以准备内容，暂不可识别商品或生成推广链接。</view>
    <button class="preview-button" data-action="preview" role="button" :tabindex="!ready || pending ? -1 : 0" :aria-disabled="!ready || pending" :disabled="!ready || pending" @click="preview" @keydown.enter.prevent="preview" @keydown.space.prevent="preview">识别商品链接</button>
    <text class="input-help">仅在点击「粘贴」时读取剪贴板；不会自动转链。商品渠道和链接安全由服务端核验。</text>
  </view>
</template>

<style scoped>
.link-input { padding: 20px; background: white; border: 1px solid #e5ece7; border-radius: 18px; }
.input-label { display: block; font-size: 16px; font-weight: 650; margin-bottom: 12px; }
textarea { box-sizing: border-box; width: 100%; min-height: 150px; padding: 14px; border: 1px solid #dce7df; border-radius: 12px; background: #f7faf8; font-size: 14px; color: #123c34; }
.input-actions { display: flex; gap: 12px; margin-top: 14px; }
button { min-height: 44px; padding: 0 20px; margin: 0; font-size: 14px; line-height: 44px; border-radius: 12px; background: #eaf6f0; color: #07594b; }
button::after { border: none; }
button[disabled] { background: #eff2f0; color: #78847e; }
.input-message, .input-help { display: block; margin-top: 12px; font-size: 13px; line-height: 1.6; overflow-wrap: anywhere; }
.input-help { color: #78847e; }
.readiness-notice { margin-top: 20px; padding: 14px; background: #fff1e7; color: #6b5240; border-radius: 12px; font-size: 13px; }
.preview-button { width: 100%; margin-top: 18px; background: #07594b; color: white; }
textarea:focus-visible { outline: 2px solid #07594b; outline-offset: 3px; }
</style>
