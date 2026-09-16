# API 规范

## 通用约定

- 用户端前缀 `/api/v1`，后台前缀 `/admin/v1`。
- Bearer Token 鉴权；写接口支持 `Idempotency-Key`。
- 金额使用十进制定点数并携带币种；时间统一 ISO 8601 UTC。
- 响应为 `{ "code": 0, "message": "success", "data": {}, "requestId": "..." }`。
- 分页采用 cursor；错误码稳定，不直接暴露上游错误原文。

## 用户端接口

- `POST /auth/login/phone`、`POST /auth/login/wechat`、`POST /auth/refresh`
- `GET /users/me`、`GET /home`
- `GET /products/search`、`GET /products/{id}`、`GET /products/{id}/compare`
- `POST /promotions/link`：创建 Tracking，返回 H5/Scheme/小程序跳转信息。
- `GET /orders`、`GET /orders/{orderNo}`、`POST /orders/missing/claim`
- `GET /wallet`、`GET /wallet/transactions`、`POST /withdrawals`
- `POST /ai/chat`、`POST /ai/compare`：调用真实商品工具后返回解释。

后台对渠道、活动、订单、佣金、对账、提现、工单和风控提供资源化接口。审批需携带原因和版本号，冲突返回 409；禁止通用“直接修改余额”接口。

## 推广中心接口扩展（待实现）

新增推广身份 / 申请、推广位、只读商品预览、转链状态、分享事件、推广订单 / 收益 / 看板、结算批次以及提现查询；完整方法、路径和输入输出以[推广中心详细设计第 3 节](./21-promotion-center-detailed-design.md#3-接口契约)为准。后台补申请审核与推广员停用审计。

保留购物用途的 `POST /promotions/link`，推广用途新增 `POST /promotions/preview` 和 `POST /promotions/convert`，共享领域能力但不绕过推广权限。身份来自鉴权上下文，所有推广位和链接需验证所有权。写请求同键同输入幂等，不同输入返回 409；处理中状态可查询，不因上游超时盲目重复发起。金额对外为十进制定点字符串与币种，内部按最小货币单位整数处理。
