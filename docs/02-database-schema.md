# 数据库与数据模型

## 核心表

- 身份：`users`、`user_identities`、`referral_relations`、`roles`、`permissions`
- 渠道：`channels`、`channel_accounts`、`channel_positions`、`channel_api_logs`
- 商品：`products`、`channel_products`、`coupons`、`price_history`
- 归因：`tracking_records`、`promotion_links`、`click_logs`
- 交易：`raw_channel_orders`、`orders`、`order_items`、`order_status_logs`
- 资金：`commission_records`、`cashback_records`、`wallets`、`wallet_transactions`、`withdrawals`
- 对账：`reconciliation_tasks`、`reconciliation_differences`
- 治理：`risk_cases`、`support_tickets`、`admin_audit_logs`、`outbox_events`

## 约束

- `orders(channel_id, external_order_id)` 唯一。
- `wallet_transactions(business_type, business_id, account_id)` 唯一。
- 金额记录币种；订单保存佣金规则快照，不回算历史订单。
- 渠道原始响应持久化，敏感字段脱敏或加密。
- 钱包流水只追加，余额由事务内流水驱动，并使用版本号防并发覆盖。

对订单更新时间、用户订单、Tracking、提现状态和对账日期建立组合索引。API 日志与原始订单按月分区，清理任务必须留痕。

## 推广中心增量模型（待迁移）

`000004_promotion_positions` 已实现内部推广位和操作回执表。默认位通过每人部分唯一索引限制为最多一个，禁用记录不得为默认；写入前锁同一推广身份行，协调身份停用与推广位变更。创建 / 编辑 / 切换 / 停用与幂等回执同事务，回执按用户 + 操作 + 幂等键唯一。切换默认同时递增旧默认位版本；不提供删除记录接口。渠道映射尚未落库，内部位状态不能视为渠道 readiness。迁移仅在隔离测试 schema 验证，down 会删数据，不作为常规生产回退。

实现进展（2026-09-16）：`000002_promoter_applications` 已提供身份 / 申请表与 up/down 迁移；按用户隔离申请幂等键，同人最多一份 PENDING 申请，当前申请外键约束归属。提交仓储在同一事务内锁定身份行、检查重复请求、插入申请并更新身份；后台审核未来必须同事务更新申请和身份状态。已在独立 PostgreSQL 测试 schema 验证，尚未在业务环境执行迁移；其余增量模型未实现。down 会删除申请和身份数据，仅用于隔离测试或有备份的明确回滚，不用于常规生产回退。

新增 `promoter_profiles`、`promoter_applications`、`promotion_positions`、`promotion_previews`、`share_artifacts`、`share_records`、`promoter_earning_records`、`settlement_batches`、`settlement_items`；扩展渠道推广位、Tracking、链接和钱包流水来源。字段和约束见[推广中心详细设计第 4 节](./21-promotion-center-detailed-design.md#4-数据与状态)。

同人最多一份待审申请、最多一个启用默认推广位；收益与结算业务唯一键防重复入账。推广位停用不删历史记录，旧购物 Tracking 不伪造推广员归属。统一钱包以来源流水区分收益，新增迁移须兼容历史余额并完成对账。当前已有 Tracking 迁移不代表推广模型已落库。
