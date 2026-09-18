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

2026-09-18 分支整合后，`000010_promotion_share_events` 是共同的分享事件表，按用户和事件号去重，且通过复合外键绑定转链归属；复制上报只是客户端报告。券与订单迁移顺延为 000011～000021，SQL 内容不变。此编号适用于全新数据库；已执行旧功能分支 000010～000021 的数据库需要 PROC-04b 专项核对与数据迁移，不能直接运行新迁移或改写已应用账本。

`000011_coupon_catalog` 增加只读券目录底表：平台、领取模式、适用范围摘要、金额、城市 / 业务、规则版本、脱敏渠道证据引用及更新时间 / 到期时间。公开列表仅查询 `enabled=true`、核验时间不在未来且尚未过期的记录；迁移不附带演示券，也没有客户端写入接口。渠道导入、证据复核、失效同步、商品 / 店铺关系与真实领券记录属于 M6 后续任务；本表的 `enabled` 只能由受控运营流程在真实核验后设置。金额单位为分，币种固定 CNY。

`000012_coupon_targets` 为店铺 / 品类券补外部范围标识及名称，并新增 `coupon_products` 有效商品映射。公开查询按券和商品各自的启用、核验及有效期过滤；商品券没有有效商品、店铺 / 品类券缺范围标识时不展示。城市 / 业务限制在列表、详情和商品查询中一致生效，缺城市不展示限定城市的券。下架由取消启用或到期体现；真实渠道状态更新、商品报价与导入写入流程尚未接通。

`000013_coupon_sync_events` 记录券同步的脱敏证据引用、核验时间及发布 / 撤销事件。内部同步服务只接受注入的可信渠道核验器返回的快照，以事务同时更新券、有效商品和事件；同证据重放不重复写，旧核验时间不能覆盖新状态，撤销同步禁用商品映射。生产启动不装配核验器或同步入口，目前仍不会自动产生真实券。正式渠道适配与数据库写权限收敛待 M6-02b-2b 验收。

`000014_coupon_claims` 保存消费者领券请求的用户归属、券、幂等键、请求指纹和 `PENDING / QUERY_REQUIRED / CLAIMED / FAILED` 状态。每人同键唯一；只有 `CLAIMED` 可附脱敏回执引用，终态不能被较晚失败覆盖。本人状态查询已接入运行时；领取 POST 仅在注入获批适配器时注册，生产目前尚无该适配器。该表记录不等于平台账户已收到券。

## 推广中心增量模型（待迁移）

`000004_promotion_positions` 已实现内部推广位和操作回执表。默认位通过每人部分唯一索引限制为最多一个，禁用记录不得为默认；写入前锁同一推广身份行，协调身份停用与推广位变更。创建 / 编辑 / 切换 / 停用与幂等回执同事务，回执按用户 + 操作 + 幂等键唯一。切换默认同时递增旧默认位版本；不提供删除记录接口。渠道映射尚未落库，内部位状态不能视为渠道 readiness。迁移仅在隔离测试 schema 验证，down 会删数据，不作为常规生产回退。

`000005_channel_positions` 增加京东内部位 → 渠道账户 / 外部位映射及追加型配置事件。重复外部标识在同渠道 / 账户下禁止绑定多个内部位，以免无法精确归属；当前映射只能是 `PENDING_VERIFICATION`，尚无真实渠道核验步骤。配置变更锁定推广身份和内部位，同事务写审计；表所有者仍可做 DDL / TRUNCATE，生产应用角色应收紧权限。000005 的 down 删除映射及事件，仅用于隔离测试或受控回滚。先前段落所说“渠道映射尚未落库”是 000004 阶段的历史状态。

`000006_admin_authorizations` 新增 `admin_principals` 与 `admin_permissions`，只接受四种明确权限。管理员 ID 与验签用户 sub 使用同一规范 ID；新账号默认未启用。每次后台请求从数据库读取启用状态和权限，不接受客户端或令牌中的角色声明作为后台授权。生产账号开通 / 撤销需独立受控操作并留痕，不能让普通客户端调用表写入。

实现进展（2026-09-16）：`000002_promoter_applications` 已提供身份 / 申请表与 up/down 迁移；按用户隔离申请幂等键，同人最多一份 PENDING 申请，当前申请外键约束归属。提交仓储在同一事务内锁定身份行、检查重复请求、插入申请并更新身份；后台审核未来必须同事务更新申请和身份状态。已在独立 PostgreSQL 测试 schema 验证，尚未在业务环境执行迁移；其余增量模型未实现。down 会删除申请和身份数据，仅用于隔离测试或有备份的明确回滚，不用于常规生产回退。

新增 `promoter_profiles`、`promoter_applications`、`promotion_positions`、`promotion_previews`、`share_artifacts`、`share_records`、`promoter_earning_records`、`settlement_batches`、`settlement_items`；扩展渠道推广位、Tracking、链接和钱包流水来源。字段和约束见[推广中心详细设计第 4 节](./21-promotion-center-detailed-design.md#4-数据与状态)。

同人最多一份待审申请、最多一个启用默认推广位；收益与结算业务唯一键防重复入账。推广位停用不删历史记录，旧购物 Tracking 不伪造推广员归属。统一钱包以来源流水区分收益，新增迁移须兼容历史余额并完成对账。当前已有 Tracking 迁移不代表推广模型已落库。
