# 万宝单生活研发文档 V1.2

本目录是根据已确认产品方案重建的研发文档集。平台定位为“AI 消费决策 + 全渠道优惠 + CPS/CPA 佣金 + 用户返现”，V1 以第三方导购成交为主。

2026-09-17 确认 APP 对外名称为「万宝单生活」。旧设计稿中的「惠省生活」是历史展示名称，图稿更新见[研发计划 DOC-08](./superpowers/plans/2026-09-16-promotion-center.md)；名称更新不代表已创建京东联盟 APP 或取得备案 ID。

## 阅读顺序

- [最新 GitHub 同步范围与实际页面图](./30-github-sync-materials-accessibility-20260919.md)：物料 / 能力领域模型、券与转链页面回归、五张实际截图及未完成边界。
- [统计页面同步记录](./29-github-sync-accessibility-dashboard-20260919.md)：本人统计页面、输入与大字号修复、复验结果及未完成边界。

- [现有 H5 页面关联与可用性验收](./28-client-accessibility-acceptance-20260919.md)：P 编号 / 页面 / API / 用例映射，七页面字体 / 键盘检查及两项回归修复；不是完整业务或微信真机验收。

- [最新代码、文档与实际页面图同步](./27-github-sync-dashboard-orders-20260919.md)：独立本人推广订单页面、看板接口 / 状态模型及实际页面截图，区分已实现、未实现与真实业务验收。
- [推广中心详细设计（含首页与 16 个页面图）](./21-promotion-center-detailed-design.md)：入口、状态、接口、数据与资金边界。
- [推广中心研发计划](./superpowers/plans/2026-09-16-promotion-center.md)：M0～M5 依赖、开发任务和验收门槛。
- [万单宝式模型分析与推广中心用户端](./20-promoter-model-user-ui-design.md)：业务规则分析、P-01～P-16 功能关联与 16 个界面。
- [研发功能与 V2 界面开发关联](./19-feature-v2-ui-development-map.md)：按功能编号找到画面、详细设计和开发顺序。
- [V1 → V2 设计覆盖与补绘](./18-v1-to-v2-design-coverage.md)：原 7 张 V1 图逐项对照，新增多平台、扩展电商、生活旅行和运营后台设计。
- [用户端 V2 补充界面图册](./17-user-ui-v2-gallery.md)：8 组、34 个页面与状态。
- [用户端界面重设计 V2](./16-user-ui-redesign-v2.md)：最新五屏概念稿与设计说明。
- [产品总设计](./产品总设计_V1.1.md)
- [API](./01-api-spec.md) / [数据库](./02-database-schema.md)
- 渠道：[京东](./03-channel-jd.md)、[拼多多](./04-channel-pdd.md)、[美团](./05-channel-meituan.md)、[饿了么](./06-channel-eleme.md)
- 核心：[订单状态机](./07-order-state-machine.md)、[佣金与钱包](./08-commission-wallet.md)
- 保障：[后台](./09-admin-console.md)、[测试](./10-test-plan.md)、[部署](./11-deployment.md)、[监控](./12-monitoring.md)
- 产品：[前端规格](./13-frontend-ui-spec.md)、[业务流程](./14-business-processes.md)、[研发功能与 UI 参考图映射](./15-feature-ui-reference-map.md)
- UI 设计图：[用户端五屏总览](./images/mobile-ui-overview-v1.png)、[多平台客户端](./images/multi-platform-client-overview-v1.png)、[扩展电商平台](./images/expanded-ecommerce-platforms-v1.png)、[本地生活与旅行平台](./images/local-life-travel-platforms-v1.png)、[运营后台总览](./images/admin-dashboard-overview-v1.png)、[购买与返现旅程](./images/purchase-cashback-journey-v1.png)、[钱包与邀请流程](./images/wallet-withdrawal-referral-v1.png)

V1 验收主链路：用户注册 → 商品搜索 → Tracking → 推广链接 → 第三方下单 → 订单归因 → 佣金确认 → 返现可用 → 提现 → 对账。

> 渠道接口、主体要求和权限以实际获批控制台为准；文档不存放真实密钥。
