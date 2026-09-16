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

## M0-02c：可运行前端工程【已完成】

文件：`apps/client/package.json`、锁文件、vite.config.ts、vitest.config.mts、tsconfig.json、index.html、src/main.ts、src/App.vue、src/pages.json、src/manifest.json、src/pages/home/index.vue、tests/home.test.ts、README.md、.gitignore。

接口：`createApp()` 返回 `{ app }`；首页默认导出 Vue 组件，由 pages.json 首项承载。后续页面复用全局暖白 / 深绿样式。

- [x] 配置官方 uni-app Vue 3 同版本包与独立 Vitest 配置，写首页品牌渲染测试：

```ts
const wrapper = mount(Home)
expect(wrapper.text()).toContain('万惠宝')
```

- [x] 创建空 Home 组件，运行 `npm test`，确认缺少品牌断言实际失败。
- [x] 实现最小首页和应用入口，运行 `npm test`、`npm run typecheck`、`npm run build:h5`；H5 浏览器加载无异常。

```ts
export function createApp() {
  const app = createSSRApp(App)
  return { app }
}
```

- [x] 核对锁文件、不暂存 node_modules / dist，记录验证与限制；本地提交 `feat(M0-02c): establish Wanhui uni-app H5 client`。

完成记录（2026-09-17）：Node 20.19.5，`npm ci`、`npm test`（1 项）、`npm run typecheck`、`npm run build:h5` 通过；Playwright Chrome 在 390×844 加载品牌与标题，无运行错误；favicon 404 已修复。只读审查反馈已处理，外部 Origin 请求修改前跨域允许 *、修改后无允许头，关闭 HMR / 收紧文件范围。`git diff --check` 通过。审计仍有 40 项风险，生产发布阻塞（M0-02e），不宣称安全通过。微信设备与真实业务未验收。

## M1-01：首页双卡与五栏导航【已完成】

文件：src/pages/home/index.vue、src/pages/{category,savings,orders,profile,promotion,ai}/index.vue、src/components/{FeatureCard,UnavailableState}.vue、src/static/icons/*.svg、src/pages.json、src/App.vue、src/platform/h5-navigation.ts、tests/home.test.ts、tests/profile.test.ts、tests/navigation.test.ts。

接口：FeatureCard 接收 `title: string`、`description: string`、`action: string`、`tone: 'mint' | 'peach'`、`icon: string`，emit `activate`；UnavailableState 接收 `title: string`、`description: string`。推广页面路由 `/pages/promotion/index`，AI 路由 `/pages/ai/index`。`enhanceH5Navigation(root: HTMLElement): () => void` 只增强框架底栏焦点 / 键盘，返回生命周期清理函数，不替换路由或操作微信 DOM。

- [x] 先写双卡触发与「我的」入口测试，绑定真实组件后测试未发事件 / 未导航的实际失败：

```ts
await wrapper.find('[data-test="promotion-entry"] button').trigger('tap')
expect(navigateTo).toHaveBeenCalledWith({ url: '/pages/promotion/index' })
```

- [x] 实现搜索、渠道行、双卡、商品待接入空态；所有尚无 API 的按钮用可见提示反馈，不返回假结果。
- [x] pages.json 配置五个 tabBar 页面及 AI / 推广子页面；我的页增加推广入口。其他栏目展示对应未接入说明。
- [x] 单元测试、类型检查、H5 构建通过；浏览器验证五栏切换、双卡、我的次入口、搜索和渠道提示，320 / 390 / 宽屏无横向溢出；查看 V2 与实现截图，保存视觉核对记录。
- [x] 记录完成日期与准确验证命令，本地提交 `feat(M1-01): implement V2 homepage and promotion entries`。

完成记录（2026-09-17）：五个新增行为实际 RED，三项入口键盘及底栏适配测试实际 RED，完成后 `npm test` 10 项、`npm run typecheck`、`npm run build:h5`、`npm run build:mp-weixin`、`git diff --check` 均通过。只读审查发现键盘与触控问题后已修复并独立复核通过。微信编译曾因空 App 脚本失败，补稳定 App 名称后恢复；仅编译证据，不是设备业务验收。

浏览器命令：`node /private/tmp/wanhui-home-qa.cjs`，使用已有 Playwright / Chrome、本机 `http://127.0.0.1:5173`，320×760 / 390×844 / 1280×900。五栏切换、双卡、我的推广次入口、搜索 / 京东未接入提示、Tab 焦点、Enter / Space 导航、44×44 搜索触控、无横向溢出、无覆盖层、无失败资源、无控制台或运行异常均通过。QA 脚本与截图不入业务仓库；真实签发方、渠道、微信设备与其 SVG 支持仍未验。

### V2 视觉核对

原图 853×1844 含系统栏、演示商品和旧品牌；它是多倍像素概念图，不作为 H5 原生 CSS 像素尺寸。验收按 390px 移动布局与 320px 窄屏，不模拟系统状态栏；宽屏限制内容与底栏最大 520px。

| 核对项 | 参考与实际证据 | 修复或明确差异 |
| --- | --- | --- |
| 结构 | V2 搜索→四渠道→双卡→推荐→外卖→底栏；实际截图顺序一致 | 开发进程重启后补验真实底栏，不以编译配置当渲染证据 |
| 品牌与首屏文案 | 万惠宝、AI 帮我选 / 推广赚钱、CTA 与五栏文字可见 | 品牌改名、城市改选择城市；搜索新增操作按钮；未接入解释为批准的真实性差异 |
| 配色 | 暖白背景、深绿标题 / 按钮、薄荷 AI、浅橙推广、四渠道浅色区 | 无额外渐变、无背景图片蒙层 |
| 字体与容器 | 26px 品牌、23px 区标题、双卡等宽、18px 卡片圆角 | 320px 双卡字号收缩但按钮和内容不横向溢出 |
| 图标与素材 | 原生导航 / 分享 / 星芒图标、五栏 selected 状态 | 未授权渠道商标改文字标识；省钱用优惠券语义图标；商品与外卖图不伪造或用整图替代UI |
| 数据区域 | 实际商品区域只有待接入说明 | 移除演示价格 / 返现，不将空态验收当真实商品联调 |
| 交互与焦点 | 点击 / Enter / Space 正确跳转、底栏键盘增强有清理 | 搜索宽度从不足44px修复到44px；仅H5增强不注入微信 DOM |

直接 `view_image` 查看原 V2 与三宽截图，修正触控与键盘问题后重新拍图并检查。按照以上有意差异忠实实现已确认的首页范围，不宣称原图全部业务数据、微信视觉或整套推广页面已实现。

## 后续交付边界

M1-02 登录与状态分流、M1-03 申请表、M1-05 推广位 UI、M2 预览 / 转链 / 分享 UI 将根据此工程逐项补执行计划，不勾选其完成。M0-02d 真实身份、M0-03 真实渠道、M0-01 资金规则继续各自保持阻塞，不以公共页面测试代替业务验收。
