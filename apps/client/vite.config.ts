import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

export default defineConfig({
  plugins: [uni()],
  server: {
    host: '127.0.0.1', port: 5173, strictPort: true,
    cors: false,
    hmr: false,
    proxy: { '^/api/v1/(coupon-cities|coupons)(\\?|$)|^/api/v1/coupons/[^/?]+(/products)?(\\?|$)': { target: 'http://127.0.0.1:8080', changeOrigin: false } },
    fs: { strict: true, allow: [__dirname] },
  },
})
