# 研发功能、V2 界面与详细设计关联

本文将产品功能编号、界面图、研发文档和开发顺序对应起来。历史 V1 图对照见 [18-v1-to-v2-design-coverage.md](./18-v1-to-v2-design-coverage.md)。界面是视觉与信息层级参考；状态、价格和渠道权限以详细设计与实际获批能力为准。登录入口采用简化版 V3。

## 用户端功能

| 编号 | 功能 | 对应界面 | 详细设计 | 开发顺序 |
| --- | --- | --- | --- | --- |
| F-01 | 手机号/微信登录、游客入口 | [手机号 V3](./images/user-ui-login-simple-v3.png)、[微信与恢复 V3](./images/user-ui-auth-companion-simple-v3.png) | [API](./01-api-spec.md)、[数据模型](./02-database-schema.md) | 身份基础 |
| F-02 | 首页、分类、省钱频道 | [核心五屏](./images/mobile-ui-overview-v2.png)、[分类与省钱](./images/user-ui-discovery-v2.png)、[多平台入口](./images/multi-platform-client-overview-v2.png) | [前端规格](./13-frontend-ui-spec.md)、[业务流程](./14-business-processes.md) | 用户端首屏 |
| F-03 | 商品/活动搜索与跨平台比较 | [核心搜索](./images/mobile-ui-overview-v2.png)、[混合搜索](./images/multi-platform-client-overview-v2.png)、[电商同款比价](./images/expanded-ecommerce-platforms-v2.png)、[酒店比价](./images/local-life-travel-platforms-v2.png) | [API](./01-api-spec.md)、[渠道](./03-channel-jd.md)、[业务流程](./14-business-processes.md) | 渠道数据可用后 |
| F-04 | AI 选购与解释 | [AI 四屏](./images/user-ui-ai-v2.png) | [产品总设计](./产品总设计_V1.1.md)、[API](./01-api-spec.md) | 真实查询后 |
| F-05 | 商品/活动详情 | [商品详情](./images/mobile-ui-overview-v2.png)、[电商详情](./images/expanded-ecommerce-platforms-v2.png)、[生活旅行详情](./images/local-life-travel-platforms-v2.png) | [前端规格](./13-frontend-ui-spec.md)、[渠道设计](./05-channel-meituan.md) | 搜索后 |
| F-06 | Tracking、推广链接与第三方跳转 | [跳转三屏](./images/user-ui-purchase-v2.png)、[商品详情](./images/mobile-ui-overview-v2.png) | [API](./01-api-spec.md)、[京东适配](./03-channel-jd.md)、[业务流程](./14-business-processes.md)、[MVP 计划](./superpowers/plans/2026-09-15-mvp-foundation.md) | 已有内存域基础；持久化进行中 |
| F-07 | 收藏、分享 | [收藏](./images/user-ui-profile-v2.png)、[商品详情](./images/mobile-ui-overview-v2.png) | [数据模型](./02-database-schema.md)、[业务流程](./14-business-processes.md) | 用户端后续 |
| F-08 | 多平台返现订单 | [核心订单](./images/mobile-ui-overview-v2.png)、[多平台订单](./images/multi-platform-client-overview-v2.png)、[电商订单](./images/expanded-ecommerce-platforms-v2.png)、[生活旅行订单](./images/local-life-travel-platforms-v2.png) | [订单状态机](./07-order-state-machine.md) | 渠道订单同步后 |
| F-09 | 订单归因、确认、结算 | [订单详情](./images/user-ui-orders-v2.png)、[后台总览](./images/admin-dashboard-overview-v2.png) | [订单状态机](./07-order-state-machine.md)、[数据模型](./02-database-schema.md) | Tracking 后 |
| F-10 | 漏单申诉 | [订单申诉四屏](./images/user-ui-orders-v2.png) | [API](./01-api-spec.md)、[业务流程](./14-business-processes.md) | 订单基础后 |
| F-11 | 预计、待结算与到账返现 | [核心订单](./images/mobile-ui-overview-v2.png)、[订单详情](./images/user-ui-orders-v2.png) | [佣金与钱包](./08-commission-wallet.md) | 渠道结算后 |
| F-12 | 钱包与不可变流水 | [钱包首页](./images/mobile-ui-overview-v2.png)、[钱包明细](./images/user-ui-wallet-v2.png) | [佣金与钱包](./08-commission-wallet.md)、[数据模型](./02-database-schema.md) | 佣金确认后 |
| F-13 | 提现、冻结、失败解冻 | [钱包提现五屏](./images/user-ui-wallet-v2.png) | [佣金与钱包](./08-commission-wallet.md)、[后台设计](./09-admin-console.md) | 钱包交易后 |
| F-14 | 一级邀请奖励 | [邀请与奖励](./images/user-ui-profile-v2.png) | [佣金与钱包](./08-commission-wallet.md)、[产品总设计](./产品总设计_V1.1.md) | 主链路后 |

## 渠道与后台

| 编号 | 功能 | 对应界面 | 详细设计 |
| --- | --- | --- | --- |
| C-01 | 京东商品、转链、订单 | [核心五屏](./images/mobile-ui-overview-v2.png)、[电商频道](./images/expanded-ecommerce-platforms-v2.png) | [京东](./03-channel-jd.md) |
| C-02 | 拼多多商品、订单 | [电商频道](./images/expanded-ecommerce-platforms-v2.png) | [拼多多](./04-channel-pdd.md) |
| C-03 | 美团外卖活动 | [分类与省钱](./images/user-ui-discovery-v2.png)、[外卖对比](./images/multi-platform-client-overview-v2.png) | [美团](./05-channel-meituan.md) |
| C-04 | 饿了么活动 | [分类与省钱](./images/user-ui-discovery-v2.png)、[外卖对比](./images/multi-platform-client-overview-v2.png) | [饿了么](./06-channel-eleme.md) |
| C-05 | 扩展电商渠道概念 | [扩展电商](./images/expanded-ecommerce-platforms-v2.png) | [前端规格](./13-frontend-ui-spec.md)：未获批入口默认关闭 |
| C-06 | 电影导购概念 | [电影优惠](./images/local-life-travel-platforms-v2.png) | [产品边界](./产品总设计_V1.1.md)：合作平台选座购票 |
| C-07 | 酒店与出行概念 | [生活旅行](./images/local-life-travel-platforms-v2.png) | [前端规格](./13-frontend-ui-spec.md)：按实际授权开放 |
| A-01～A-06 | 经营、渠道、订单、佣金、提现、风控 | [运营后台 V2](./images/admin-dashboard-overview-v2.png) | [后台](./09-admin-console.md)、[监控](./12-monitoring.md)、[佣金与钱包](./08-commission-wallet.md) |
| Q-01～Q-03 | API、异常状态、发布与监控 | [通用状态](./images/user-ui-states-v2.png)、[运营后台](./images/admin-dashboard-overview-v2.png) | [API](./01-api-spec.md)、[测试](./10-test-plan.md)、[部署](./11-deployment.md)、[监控](./12-monitoring.md) |

## 开发执行顺序

1. 复用 [MVP Foundation](./superpowers/plans/2026-09-15-mvp-foundation.md) 已完成的 Go HTTP、Tracking、京东适配和转链领域代码，验证基础测试。
2. 完成 Tracking 的 PostgreSQL 迁移与仓储；解决并发重复创建的幂等冲突；用真实数据库集成测试验证。
3. 接入获批京东账户的正式接口能力，建立真实商品检索、转链和订单同步证据。无凭据时只实现适配边界与可运行的非生产演示。
4. 在真实渠道数据可用后实现 V2 对应的搜索、详情和第三方跳转页；价格更新时间、优惠券与预计返现分别标识。
5. 再实现订单归因、佣金、钱包、提现和对账；资金操作以不可变流水和渠道结算证据为前提。
6. 扩展电商、电影及旅行渠道按主体和接口授权逐一打开；概念图中的入口不等于已可交易功能。

本轮代码先落实第 1～2 步。第 3 步需要渠道账户权限、推广位及已轮换的凭据；第 4～6 步的正式交易页面不能用图稿虚构金额上线。

