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

推广位增量实现（2026-09-16）：

- `GET /api/v1/promotion-positions`：本人列表，status 可选 ENABLED / DISABLED，limit 默认 20、范围 1～100，cursor 绑定用户与筛选；按内部 ID 升序分页，不宣称时间排序。空列表 items=[]，返回 nextCursor。停用推广身份仍可读本人历史。
- `POST /api/v1/promotion-positions`：name、scene、可选 isDefault；新建内部位版本为 1，未指定默认不自动选为默认。
- `PATCH /api/v1/promotion-positions/{id}`：name、scene、version；当前按完整名称 / 场景更新，不支持通过此接口修改默认标记或渠道归属。
- `POST /api/v1/promotion-positions/{id}/default`、`.../{id}/disable`：仅接受 version，分别切换默认 / 停用并清除默认，不删除历史。

写操作必须提供 Idempotency-Key 及 JSON，name / scene 去首尾空白后 1～80 字符、请求体最多 4 KiB。只有已启用推广身份可写，停用后连旧写请求重放也返回 403；同人同操作同键的输入变化返回 409。默认切换同时更新旧默认位的版本，其他页面收到版本冲突须刷新。找不到或不属于本人统一 404，不泄露他人资源。

响应含 id、name、scene、status、isDefault、version、createdAt；当前渠道映射尚未实现，channels 固定标注 JD / WAITING_CONFIGURATION，canConvert=false。幂等重放返回原操作回执，实际当前默认 / 停用状态以重新查询列表为准。缺少 PositionService 或用户解析器时不注册路由，生产启动仍未接入。

后台列表增量实现：`GET /admin/v1/promoter-applications` 要求独立管理员认证及 `promoter:read` 权限。参数 status 可选 PENDING / ENABLED / REJECTED / DISABLED（省略为全部），limit 默认 20、范围 1～100，cursor 为服务端返回的 nextCursor。返回 `{items: [{applicationId,userId,displayName,scene,consentedAt,status,version}],nextCursor}`，每人仅当前申请，按同意时间与申请 ID 倒序；不提供全历史审计列表。筛选变化须清空 cursor，重复 / 未知查询参数或无效游标返回 400，空结果 items 为 []。分页不冻结跨请求数据快照，审核时必须带列表中的 version，409 后刷新。缺少列表服务或管理员依赖时不注册路由，尚未生产装配。

后台增量实现：`POST /admin/v1/promoter-applications/{id}/review` 接受 `{approve: boolean, reason: string, version: integer}`；`POST /admin/v1/promoters/{id}/disable` 接受 `{reason: string, version: integer}`。均要求 JSON、Idempotency-Key 和独立管理员认证。权限分别为 promoter:review / promoter:disable，版本必须大于 0，原因去首尾空白后 1～500 字符（用户可见，不得填内部机密）。响应包含 userId、applicationId、status、version；相同请求重放返回原操作回执，当前状态需另行查询。身份缺失 401、权限不足 / 自审 403、目标不存在 404、版本 / 状态 / 幂等冲突 409、内部异常 503。缺少管理员解析器或管理服务时路由不注册，尚未接入生产启动。

实现进展（2026-09-16）：`GET /api/v1/promoter/profile` 已实现查询服务和可注入路由，返回 status、reason、applicationId、capabilities。未申请返回 NOT_APPLIED；未认证返回 401，读取失败返回 503 / PROMOTER_UNAVAILABLE，响应禁止缓存。只能查询鉴权本人，拒绝依赖返回的其他用户记录，不公开审核通过的内部备注。缺少身份或查询依赖时不注册路由。该接口尚未装配进生产启动入口，数据库仓储和真实认证仍待实现；其他接口仍为目标契约。

增量实现：申请 PostgreSQL 仓储及 `POST /api/v1/promoter/applications` 已实现，需注入身份与 ApplicationService 才开放路由。请求使用 application/json 和 Idempotency-Key（1～128 个非空白 ASCII 字符），字段为 displayName、scene、agreementVersion、agreed（必须 true）；服务端要求明确配置当前协议版本，不提供虚构默认协议。姓名 / 场景去首尾空白后限制 80 字符，请求体最多 4 KiB，禁止客户端指定用户、申请 ID、同意时间及未知字段。协议未同意 / 版本不符为 422，幂等冲突或当前状态不允许申请为 409，读取 / 写入异常脱敏为 503。

提交成功返回 200：applicationId、status=PENDING、consentedAt，是原始提交回执，重放不会变成新的审核结果；当前审核状态应查询 profile。相同用户和键在规范化输入一致时返回原 ID 与同意时间，拒绝后重新申请需新键。协议更新后旧版本请求需重新确认当前协议，此时旧键不可用于新版本申请。生产认证及启动装配仍未完成。

`GET /api/v1/promoter/applications/current` 已实现，返回本人当前申请的 applicationId、displayName、scene、agreementVersion、consentedAt、status、reason、canReapply。status 反映当前推广身份，停用为 DISABLED；仅 REJECTED 可重新申请。无申请返回 404 / APPLICATION_NOT_FOUND，未登录 401，存储失败 503；只公开拒绝 / 停用的用户可见原因，审批内部备注不得写入该字段。单条 JOIN 保证申请与身份来自同一查询快照；不接受 userId 查询他人。接口需身份和 CurrentApplicationService 依赖，仍未接入生产启动入口。

新增推广身份 / 申请、推广位、只读商品预览、转链状态、分享事件、推广订单 / 收益 / 看板、结算批次以及提现查询；完整方法、路径和输入输出以[推广中心详细设计第 3 节](./21-promotion-center-detailed-design.md#3-接口契约)为准。后台补申请审核与推广员停用审计。

保留购物用途的 `POST /promotions/link`，推广用途新增 `POST /promotions/preview` 和 `POST /promotions/convert`，共享领域能力但不绕过推广权限。身份来自鉴权上下文，所有推广位和链接需验证所有权。写请求同键同输入幂等，不同输入返回 409；处理中状态可查询，不因上游超时盲目重复发起。金额对外为十进制定点字符串与币种，内部按最小货币单位整数处理。
