# AI 省钱生活平台研发文档 V1.2

本目录是根据已确认产品方案重建的研发文档集。平台定位为“AI 消费决策 + 全渠道优惠 + CPS/CPA 佣金 + 用户返现”，V1 以第三方导购成交为主。

## 阅读顺序

- [产品总设计](./产品总设计_V1.1.md)
- [API](./01-api-spec.md) / [数据库](./02-database-schema.md)
- 渠道：[京东](./03-channel-jd.md)、[拼多多](./04-channel-pdd.md)、[美团](./05-channel-meituan.md)、[饿了么](./06-channel-eleme.md)
- 核心：[订单状态机](./07-order-state-machine.md)、[佣金与钱包](./08-commission-wallet.md)
- 保障：[后台](./09-admin-console.md)、[测试](./10-test-plan.md)、[部署](./11-deployment.md)、[监控](./12-monitoring.md)
- 产品：[前端规格](./13-frontend-ui-spec.md)、[业务流程](./14-business-processes.md)
- UI 设计图：[用户端五屏总览](./images/mobile-ui-overview-v1.png)、[多平台客户端](./images/multi-platform-client-overview-v1.png)、[扩展电商平台](./images/expanded-ecommerce-platforms-v1.png)、[本地生活与旅行平台](./images/local-life-travel-platforms-v1.png)、[运营后台总览](./images/admin-dashboard-overview-v1.png)、[购买与返现旅程](./images/purchase-cashback-journey-v1.png)、[钱包与邀请流程](./images/wallet-withdrawal-referral-v1.png)

V1 验收主链路：用户注册 → 商品搜索 → Tracking → 推广链接 → 第三方下单 → 订单归因 → 佣金确认 → 返现可用 → 提现 → 对账。

> 渠道接口、主体要求和权限以实际获批控制台为准；文档不存放真实密钥。
