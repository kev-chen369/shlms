# UI 设计图

## 文件

- `qa-promoter-dashboard-filter-text200-320.png`、`qa-promoter-dashboard-input-text200-320.png`、`qa-promoter-dashboard-counts-text200-320.png`、`qa-promoter-orders-filter-text200-320.png`、`qa-promoter-orders-input-text200-320.png`、`qa-promoter-orders-detail-text200-320.png`：2026-09-19 实际 H5 320px / 200% CSS 文本筛选、输入、计数与归因详情截图。仅测试端合成身份与 HTTP 测试数据，不代表生产身份 / 订单、收益、系统字体或正式无障碍验收；见[最新同步说明](../29-github-sync-accessibility-dashboard-20260919.md)。

- `client-promoter-orders-login-390-20260919.png`、`client-promoter-orders-list-390-20260919.png`、`client-promoter-orders-detail-390-20260919.png`：实际 H5 本人推广订单未登录 / 列表 / 详情页面。列表与详情使用 QA 测试身份及本机 HTTP 测试数据，无收益金额，不代表真实订单或生产身份验收；见[当前同步说明](../27-github-sync-dashboard-orders-20260919.md)。

- `client-coupon-products-390-20260919.png`、`client-coupon-products-320-20260919.png`：M6-02b-3b 实际 H5 券详情适用商品区域，390 / 320px 视口；本机 HTTP 测试数据，只读，没有实时价格 / 购买入口，不代表真实渠道或领券验收。详见[最新同步记录](../26-github-sync-products-orders-20260919.md)。
- `client-home-readonly-20260919.png`、`client-coupons-readonly-20260919.png`：2026-09-19 实际 uni-app H5 首页与领券页，390px 屏宽。本机 HTTP 测试服务经真实 uni 请求和开发代理提供测试券，非真实可领优惠；用于 M1-01d-2 页面验收，不取代 V3 概念稿或微信真机验收。券只读，领取与购买未接通。详见[同步记录](../25-github-sync-20260919.md)。

- `home-platform-modules-h5-v3.png`：2026-09-17 多平台切换首页 H5 概念稿；京东选中态、四个内容模块和「首页 / 领券 / 推广 / 我的」四栏底导航。DOC-04 已确认此结构为当前设计基线，旧 V2 五栏仅作历史归档；全部商品、店铺、价格和券额均为演示，不代表客户端已完成或业务可用。
- `coupon-center-h5-v3.png`：与当前首页一致的 H5「领券」一级页，京东选中态、精选券 / 品类券 / 即将过期及四栏底导航；金额与商品均为演示，不代表真实可领。
- `promotion-center-h5-v3.png`：与当前首页一致的 H5「推广」一级页，已开通推广身份但京东渠道位待配置，生成入口禁用，订单与收益为空态；四栏底导航选中推广。
- `promotion-materials-h5-v3.png`：推广选品详细页，平台京东 / 淘宝 / 美团和商品 / 活动切换，每个物料独立查看 / 生成操作；示例渠道均未就绪，不展示可复制的真实链接。详见[详细设计](../23-multiplatform-promotion-ui.md)。
- `promotion-product-confirm-v3.png`：V3 商品确认与推广位选择状态图，京东报价待核验 / 渠道位待配置，生成链接禁用；新图使用 APP 对外名称「万宝单生活」。
- `promotion-activity-detail-v3.png`：V3 美团外卖活动详情示例，地域 / 终端 / 时段待核验且无可用推广位，活动链接生成禁用；非真实活动。
- `promotion-link-states-v3.png`：V3 推广链接处理中、渠道结果待核验、确定失败三屏交互示意；非成功状态均不可复制链接。

- `home-promoter-entry-v2.png`：首页新增“AI 帮我选 / 推广赚钱”并排入口；[交互说明与提示词](./home-promoter-entry-v2.prompt.md)。
- 推广中心用户端：[模型分析与图册](../20-promoter-model-user-ui-design.md)，4 组、16 个核心页面。
- `promoter-onboarding-positions-v2.png`：申请开通、推广工作台、推广位管理、新建推广位。
- `promoter-states-onboarding-v2.png`：审核中、拒绝、停用、无推广位四屏状态稿；[生成与修正记录](./promoter-states-onboarding-v2.prompt.md)。
- `promoter-states-convert-v2.png`：渠道待配置、预览过期、价格变化、生成中四屏状态稿；[提示词与边界](./promoter-states-convert-v2.prompt.md)。
- `promoter-states-result-v2.png`：结果待核验、确定失败、展示有效期结束三屏状态稿；[提示词与修正记录](./promoter-states-convert-v2.prompt.md)。
- `promoter-convert-share-v2.png`：万能转链、商品确认、转链结果、分享素材。
- `promoter-activities-orders-v2.png`：推广活动、推广订单、订单收益详情、推广数据。
- `promoter-wallet-settlement-v2.png`：我的推广、结算记录、提现申请、提现记录。
- V1 场景补绘为 V2：[覆盖对照与图册](../18-v1-to-v2-design-coverage.md)，新增以下 4 组、14 个界面。
- `multi-platform-client-overview-v2.png`：多平台综合导购，4 屏。
- `expanded-ecommerce-platforms-v2.png`：扩展电商平台，4 屏。
- `local-life-travel-platforms-v2.png`：本地生活与旅行，5 屏。
- `admin-dashboard-overview-v2.png`：运营后台总览，1 屏。
- `user-ui-auth-companion-simple-v3.png`：简化版微信登录与登录未完成两屏；[说明与提示词](./user-ui-auth-companion-simple-v3.prompt.md)。
- `user-ui-login-simple-v3.png`：登录页简化版，最新手机号登录视觉方案；[说明与提示词](./user-ui-login-simple-v3.prompt.md)。
- 用户端 V2 补充图：详见 [完整图册](../17-user-ui-v2-gallery.md)，包含以下 8 组。
- `user-ui-auth-v2.png`：登录与授权，3 屏。
- `user-ui-discovery-v2.png`：分类与省钱频道，4 屏。
- `user-ui-ai-v2.png`：AI 选购助手，4 屏。
- `user-ui-purchase-v2.png`：购买跳转，3 屏。
- `user-ui-orders-v2.png`：订单详情与申诉，4 屏。
- `user-ui-wallet-v2.png`：钱包与提现，5 屏。
- `user-ui-profile-v2.png`：收藏邀请与服务，5 屏。
- `user-ui-states-v2.png`：通用页面状态，6 屏。
- `mobile-ui-overview-v2.png`：用户端 V2 重设计，覆盖首页、搜索比价、商品详情、返现订单和我的钱包；详见 [设计说明](../16-user-ui-redesign-v2.md)。
- `mobile-ui-overview-v1.png`：用户端首页、搜索、商品详情、订单和个人中心五屏总览。
- `admin-dashboard-overview-v1.png`：运营管理后台经营指标、渠道、订单、佣金、对账和风控总览。
- `purchase-cashback-journey-v1.png`：搜索比价、商品详情、跳转确认和返现订单四步购买旅程。
- `wallet-withdrawal-referral-v1.png`：钱包首页、资金明细、提现申请和一级邀请奖励四屏流程。
- `multi-platform-client-overview-v1.png`：京东、美团、饿了么和淘票票的首页入口、跨平台搜索、场景详情及统一订单总览。
- `expanded-ecommerce-platforms-v1.png`：淘宝/天猫、拼多多、唯品会和苏宁易购的电商频道、全网比价、优惠详情及订单总览。
- `local-life-travel-platforms-v1.png`：美团、饿了么、淘票票、携程和同程的生活旅行频道、场景比价、优惠详情及订单总览。

## 设计基线

- 当前对外名称：万宝单生活（DOC-08）；既有图稿中的「惠省生活 / 万惠宝」为历史名称，尚未逐张重绘，不代表当前客户端名称。
- 风格：清爽、可信、现代的生活优惠平台
- 主色：翠绿色、薄荷绿和暖白色
- 强调色：用于券、返现和预警的橙色/红色
- 数据：全部为虚构演示数据，不代表真实经营结果
- 边界：不包含真实账户、客户信息、渠道密钥或第三方品牌标识
- 渠道识别：概念稿以平台名称和识别色标签区分渠道，不复制第三方官方标志

图片为 AI 生成的产品概念稿，用于产品讨论和研发对齐；具体字号、间距、组件状态和交互仍以 `docs/13-frontend-ui-spec.md` 及后续可编辑设计源文件为准。
