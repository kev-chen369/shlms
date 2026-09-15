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
