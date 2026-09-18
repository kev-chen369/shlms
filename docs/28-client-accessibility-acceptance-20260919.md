# 现有 H5 页面关联与可用性验收（REL-01a）

2026-09-19，本机 Chrome / uni H5；不表示完整 P-00～P-16 产品、微信设备、真实业务或正式无障碍符合性验收完成。

## 设计、页面、接口与用例关联

| 设计关联 | 实际页面（`apps/client/src/`） | 当前接口 / 行为 | 主要用例与未完成范围 |
|---|---|---|---|
| P-00，V3 首页 | `pages/home/index.vue` | 平台切换、四模块、领券只读接口、四栏导航 | `tests/home.test.ts` 等现有首页测试；`tests/browser/home-text-resize.browser.cjs` 实际布局。商品搜索 / 店铺 / 真实推广内容未接通 |
| 非推广 P 页；V3 领券设计 | `pages/coupons/index.vue` | `/api/v1/coupon-cities`、`/api/v1/coupons`、详情与适用商品只读 | coupon API / model / component 用例；本轮浏览器空目录及平台键盘操作。领取 / 购买 / 外跳未开放 |
| P-02 的入口骨架，不是完整工作台 | `pages/promotion/index.vue` | 本人订单、统计、准备链接入口；默认真实身份未接入 | orders / dashboard 页面键盘用例；开通、推广位及转链业务未验收 |
| 非推广 P 页；我的次级入口 | `pages/profile/index.vue` | 推广 / 消费者订单 / AI 导航；身份与余额未接入提示 | 当前页面导航测试及本轮键盘进入推广；不展示假余额或收益 |
| P-05 的本地准备子集 | `pages/promotion/convert.vue` | 主动读取剪贴板、输入 / 清空；识别关闭 | `tests/link-input.test.ts`，REL-01a-1；不是 P-06～P-08 或真实转链 |
| P-10；P-11 仅归因详情子集 | `pages/promotion/orders.vue` | `/api/v1/promoter/orders` 及本人详情；UTC 日期 | `tests/promoter-orders-page.test.ts`、组件 / 模型 / API；M3-02c-2b 浏览器。无收益双状态 / 金额，不等于 P-11 收益详情完成 |
| P-12 的只读计数子集 | `pages/promotion/dashboard.vue` | `/api/v1/promoter/dashboard`；上海自然日、精确快照、三计数 | `tests/promoter-dashboard-page.test.ts`、组件 / 模型 / API；M3-04b-2b-2 浏览器。没有可信点击、收益或转化率 |

上表路径以客户端工程为根；测试具体文件以仓库当前文件为准。推广接口只有身份提供方注入后才读取，当前默认无身份，不开放 token 表单；开发代理不转发推广 API。本次七页检查使用默认未登录状态及本机空券 HTTP 测试数据；已登录统计 / 订单模拟场景另见对应任务的浏览器记录，不能扩大本次字体覆盖。

## 本轮真实浏览器检查

环境：`http://127.0.0.1:5173`，320×844、390×844、1280×844；Browser 插件及 skill 未提供，使用已有 Playwright 1.62.1 / 本机 Chrome，无依赖安装。

七页面共 21 组：正常字号与 CSS 文本放大 200% 后检查文档横向尺寸；放大文本、按钮、输入 / 文本域 / placeholder 和底栏标签采用先快照原计算字号再逐项放大，避免继承导致重复放大。可见按钮 / role=button 触控区域均至少 44×44px，实际键盘完成平台切换 / 搜索未开放提示 / 进入统计 / 我的进入推广 / 链接清空 / 订单与统计返回。标题按页面配置核对（链接准备页标题为「准备推广链接」），非空、无框架覆盖、无运行错误。

此模拟不是浏览器整页缩放、操作系统字体偏好、屏幕阅读器或微信设备测试；字号为 px 的所有文本并非均由选择器覆盖。未逐项检查所有静态字段、嵌套详情状态或辅助技术阅读顺序，不作全量符合性声明。平台横向滚动条保留，是既有设计而非文档横向溢出；大字号标题换行和搜索区移到下一行是有意适配。实际直接查看 320px 首页及 390px 修复后的首页截图。

## 两项独立修复

- REL-01a-1：快速原生输入尚未进入节流模型时清空不生效；版本 key 重建文本域，组件根范围监听异步原生节点替换并重设名称。11 项 LinkInput 用例及实际快速两次输入 → Space 清空 → network idle / 原生标签回归通过，独立提交 cc08fc1。
- REL-01a-2：390px / 200% 文本下搜索输入仅 18.984px，品牌不收缩且 header 仅 <=360px 可换行。真实 Node / Playwright 回归先 RED；改为内容驱动换行、搜索区域保留 190px 布局宽度，当前三个宽度输入约 190 / 260 / 149px，均达到测试要求的 80px 可编辑宽度，无文档溢出。截图复查及高度回归又观察 RED：200% 字体 26px 但 native input 高仅 18.1875px；输入改为 44px 高，避免文字裁切。默认字号 390px 仍可并排展示，不改 V3 模块 / 四栏 / 业务开关。

可重复布局回归：先运行本机 dev:h5，确保已有 Playwright / Chrome 可用，然后在 `apps/client` 运行 `npm run test:layout`。Playwright 可从当前项目依赖或 `CLIENT_QA_PLAYWRIGHT_MODULE` 指定现有模块路径，Chrome 可用 `CLIENT_QA_CHROME_EXECUTABLE` 指定，目标可通过 `CLIENT_QA_URL` 指定。缺依赖则明确失败，不自动下载或跳过。不要以 jsdom 单元测试代替真实布局。

## 保留的发布门槛

REL-01b 的系统字体 / 微信真机 / 辅助技术与完整业务状态、产品视觉验收仍阻塞。身份、正式推广数据、收益、结算、提现及真实订单回流未满足，REL-01 和整体发布门槛未完成；数据库用例跳过情况另见 M3-05 验收矩阵。本轮不推送、部署或迁移数据库。
