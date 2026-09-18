import { defineConfig } from 'vite'
import uni from '@dcloudio/vite-plugin-uni'

export default defineConfig({
  plugins: [uni()],
  server: {
    host: '127.0.0.1', port: 5173, strictPort: true,
    cors: false,
    hmr: false,
    fs: { strict: true, allow: [__dirname] },
  },
})
