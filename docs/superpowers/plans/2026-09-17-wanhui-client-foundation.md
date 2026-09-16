# 万惠宝客户端基础 Implementation Plan

> **For agentic workers:** 使用 superpowers:executing-plans 逐任务执行；用户要求自行判断，不再逐项询问。仅本地提交，不推送或部署。

**Goal:** 建立真实 uni-app H5 工程，并实现 V2 首页入口与消费者导航。

**Architecture:** `apps/client` 独立 npm 工程，Vue 单文件组件、uni-app 页面导航；Go API 不变。不存在的业务能力以明确未接入状态呈现。

**Tech Stack:** uni-app Vue 3、TypeScript、Vite、Vitest、Vue Test Utils。

**Spec:** `docs/superpowers/specs/2026-09-17-wanhui-client-design.md`

## Global Constraints

- 品牌「万惠宝」，消费者五栏：首页 / 分类 / 省钱 / 订单 / 我的。
- 不内置令牌、私钥、生产凭据，不制造审核通过、商品价格、返现或成功转链。
- 保留已有 Go 与用户文件，产物和 node_modules 不入 Git。
- 触控至少 44px；320px、390px、宽屏分别验证；微信小程序真实适配单独验收。

## M0-02c：可运行前端工程【进行中】

文件：`apps/client/package.json`、锁文件、vite.config.ts、vitest.config.ts、tsconfig.json、index.html、src/main.ts、src/App.vue、src/pages.json、src/manifest.json、src/pages/home/index.vue、tests/home.test.ts、README.md、.gitignore。

接口：`createApp()` 返回 `{ app }`；首页默认导出 Vue 组件，由 pages.json 首项承载。后续页面复用全局暖白 / 深绿样式。

- [ ] 配置官方 uni-app Vue 3 同版本包与独立 Vitest 配置，写首页品牌渲染测试：

```ts
const wrapper = mount(Home)
expect(wrapper.text()).toContain('万惠宝')
```

- [ ] 创建空 Home 组件，运行 `npm test`，确认缺少品牌断言实际失败。
- [ ] 实现最小首页和应用入口，运行 `npm test`、`npm run typecheck`、`npm run build:h5`；H5 浏览器加载无异常。

```ts
export function createApp() {
  const app = createSSRApp(App)
  return { app }
}
```

- [ ] 核对锁文件、不暂存 node_modules / dist，记录验证与限制；提交 `feat(M0-02c): establish Wanhui uni-app H5 client`。

## M1-01：首页双卡与五栏导航【未开始】

文件：src/pages/home/index.vue、src/pages/{category,savings,orders,profile,promotion,ai}/index.vue、src/components/{FeatureCard,UnavailableState}.vue、src/static/icons/*.svg、src/pages.json、tests/home.test.ts、tests/profile.test.ts、tests/navigation.test.ts。

接口：FeatureCard 接收 `title: string`、`description: string`、`action: string`、`tone: 'mint' | 'peach'`，emit `activate`；UnavailableState 接收 `title: string`、`description: string`。推广页面路由 `/pages/promotion/index`，AI 路由 `/pages/ai/index`。

- [ ] 先写双卡触发与「我的」入口测试，绑定真实组件后测试未发事件 / 未导航的实际失败：

```ts
await wrapper.find('[data-test="promotion-entry"]').trigger('click')
expect(navigateTo).toHaveBeenCalledWith({ url: '/pages/promotion/index' })
```

- [ ] 实现搜索、渠道行、双卡、商品待接入空态；所有尚无 API 的按钮用可见提示反馈，不返回假结果。
- [ ] pages.json 配置五个 tabBar 页面及 AI / 推广子页面；我的页增加推广入口。其他栏目展示对应未接入说明。
- [ ] 单元测试、类型检查、H5 构建通过；浏览器验证五栏切换、双卡、我的次入口、搜索和渠道提示，320 / 390 / 宽屏无横向溢出；查看 V2 与实现截图，保存视觉核对记录。
- [ ] 记录完成日期与准确验证命令，提交 `feat(M1-01): implement V2 homepage and promotion entries`。

## 后续交付边界

M1-02 登录与状态分流、M1-03 申请表、M1-05 推广位 UI、M2 预览 / 转链 / 分享 UI 将根据此工程逐项补执行计划，不勾选其完成。M0-02d 真实身份、M0-03 真实渠道、M0-01 资金规则继续各自保持阻塞，不以公共页面测试代替业务验收。
