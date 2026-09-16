import vue from '@vitejs/plugin-vue'

export default {
  plugins: [vue({ template: { compilerOptions: { isCustomElement: (tag) => ['view', 'text', 'image', 'scroll-view'].includes(tag) } } })],
  test: { environment: 'jsdom', include: ['tests/**/*.test.ts'] },
}
