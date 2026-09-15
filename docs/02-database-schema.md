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
