import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// ── 统一网关接入 ──────────────────────────────────────────────────────────────
// 开发期前端不再直连业务服务端口，一律经 Higress 网关（默认 http://127.0.0.1:8080）。
//
// 为什么必须走网关：直连 :9999 / :10000 会绕开网关层能力——
// JWT 校验（网关侧 jwt-auth wasm 插件）、限流、CORS、GeoIP 注入、灰度、访问日志全部失效，
// 而应用侧仍会独立校验 JWT，等于两层校验只剩一层。
//
// 为什么两个前缀要带不同的 Host：Higress 的路由按 (host, path) 匹配，
// B 端在 api.devops.local、C 端在 user-api.devops.local（见 deploy/higress/ingresses/）。
// 合并到同一 host 会让两边的 /.well-known/jwks.json 撞车（同 host 下最长前缀匹配无法区分），
// 所以保持分域，在边缘层（vite proxy / nginx）把两个域合并成一个对外入口。
//
// 可用环境变量覆盖：
//   VITE_GATEWAY_TARGET           网关地址（默认 http://127.0.0.1:8080）
//   VITE_GATEWAY_SYSTEM_HOST      B 端 Host（默认 api.devops.local）
//   VITE_GATEWAY_USER_HOST        C 端 Host（默认 user-api.devops.local）
//   VITE_DIRECT_SERVICE=1         临时直连业务服务（排障用，不建议长期开启）
const gatewayTarget = process.env.VITE_GATEWAY_TARGET || 'http://127.0.0.1:8080'
const systemHost = process.env.VITE_GATEWAY_SYSTEM_HOST || 'api.devops.local'
const userHost = process.env.VITE_GATEWAY_USER_HOST || 'user-api.devops.local'
const direct = process.env.VITE_DIRECT_SERVICE === '1'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    // Bind IPv4 explicitly: Vite 8 listens on ::1 by default, and tools that
    // probe 127.0.0.1 (curl, health checks, container-side probes) will fail
    // to connect with ECONNREFUSED. This bit us during gateway verification.
    host: '127.0.0.1',
    proxy: {
      // devops-system：用户权限系统（/system/api/v1/...）
      //
      // 只代理 /system/api 而不是整个 /system：侧边栏的业务路由也是 /system/xxx
      // （菜单 path 拼出来的），若整个前缀都代理，刷新深链接时浏览器会真的去请求
      // /system/user，被后端网关守卫拒掉并返回 401「requests must be made through
      // the gateway」——因为文档请求不带 Authorization 头。收窄到 API 前缀后，
      // 页面路径会落到 Vite 的 SPA fallback 上，刷新才能正常回到应用。
      '/system/api': direct
        ? {
            target: process.env.VITE_API_PROXY_TARGET || 'http://127.0.0.1:9999',
            changeOrigin: true,
          }
        : {
            target: gatewayTarget,
            changeOrigin: true,
            // changeOrigin 会把 Host 改写成 target 的 host，必须显式写回网关路由用的域名
            headers: { Host: systemHost },
          },
      // devops-user：用户微服务（/user/api/v1/...）
      '/user/api': direct
        ? {
            target: process.env.VITE_USER_PROXY_TARGET || 'http://127.0.0.1:10000',
            changeOrigin: true,
          }
        : {
            target: gatewayTarget,
            changeOrigin: true,
            headers: { Host: userHost },
          },
    },
  },
  build: {
    chunkSizeWarningLimit: 1600,
    rollupOptions: {
      input: {
        // 后台管理 SPA（默认入口）
        main: fileURLToPath(new URL('./index.html', import.meta.url)),
        // C 端门户（Three.js 营销首页 + 登录/注册/个人资料）
        portal: fileURLToPath(new URL('./portal.html', import.meta.url)),
      },
    },
  },
})
