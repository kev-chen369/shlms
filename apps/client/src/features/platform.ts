import { computed, ref } from 'vue'
export const platforms = [
  { id: 'JD', name: '京东' }, { id: 'MEITUAN', name: '美团' },
  { id: 'TAOBAO', name: '淘宝' }, { id: 'PDD', name: '拼多多' }, { id: 'ELEME', name: '饿了么' },
] as const
export type Platform = typeof platforms[number]['id']
export const selectedPlatform = ref<Platform>('JD')
export const selectedPlatformName = computed(() => platforms.find(item => item.id === selectedPlatform.value)!.name)
