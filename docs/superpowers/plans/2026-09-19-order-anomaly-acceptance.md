# M3-05 订单异常与指标一致性验收矩阵

任务 M3-05a；范围为既有证据复核和验收拆分，不是订单、财务或真实渠道验收通过。

**目标：** 证明重复 / 乱序 / 退款 / 无归因 / 渠道位及回补不会破坏订单归属、历史与看板口径。

**架构：** 使用真实 `order.Store.Save` → `ProjectionStore.Apply` → `AttributionStore.Apply` → `order.ReadStore.ListOwned` / `GetOwned` 与 `dashboard.ReadStore.Get`，在同一隔离 PostgreSQL schema 内联测。原始事件加密、投影去重及归因均走仓储，禁止用直接写入最终统计表冒充联测。数据库合成样例和真实渠道、资金验收分开。

**技术：** Go、PostgreSQL、现有迁移与仓储测试。依据：[推广中心计划 M3-05](./2026-09-16-promotion-center.md)、[订单状态机](../../07-order-state-machine.md)、[推广详细设计](../../21-promotion-center-detailed-design.md)。

## 2026-09-19 执行证据

运行 `go test -count=1 -v ./internal/order ./internal/dashboard`：`TestAttributionInputValidation`、`TestInvalidFilter` 执行通过；下表 9 个数据库用例均因 `PG_TEST_DSN is not set` 跳过。包结果 PASS 不能证明数据库事务、迁移或真实订单链路通过。此前计划中的隔离 PostgreSQL 证据为历史记录，本次未重现，不能推翻也不能代替本次验收。

本机 PATH、常见 PostgreSQL 安装路径及缓存没有发现可用的 postgres / initdb / pg_ctl；Docker info 与 context inspect 对同一运行探针复查仍未返回，docker socket 的 `curl --max-time 3` 探针退出 28、零响应。已终止本次只读探针进程，没有停止 Docker 或用户服务、没有修改业务数据库。没有拉镜像或自动操作生产迁移。

## 已有测试的覆盖与缺口

| 既有用例（初次复核SKIP，PROC-11随后实跑通过） | 文件与实际覆盖 | 尚未证明的范围 |
|---|---|---|
| TestRawEvidenceEncryptedAndIdempotent | `internal/order/evidence_test.go`：12 路并发原始事件同键只创建一次，加密、输入冲突与篡改 | 全链路最终订单 / 看板不多计 |
| TestRawEvidenceRejectsInvalidInputAndMissingKey | 同文件：非法输入与缺少密钥拒绝 | 数据库实际执行及完整链路 |
| TestProjectionPreservesOrderAndRefundHistory | `internal/order/projection_test.go`：重复、旧事件、非法回退、部分 / 全额退款、重放历史 | 退款后列表 / 看板、跨日期最早时间回补 |
| TestProjectionRequiresSavedEvidenceAndSerializesConcurrentReplay | 同文件：未存证据拒绝、12 路并发、先退款后订单、退款映射冲突 | 多入口联测及去重后的统计 |
| TestAttributionExactTrackingPendingAndConflict | `internal/order/attribution_test.go`：精确 Tracking、重放与证据冲突 | 匿名消费者边界、统计与读取联测 |
| TestAttributionUnknownEvidenceWaitsForReview | 同文件：未知凭据保留待核验，没有自动猜测归属 | 待核验订单不进入本人列表 / 看板、回补流程 |
| TestAttributionLinkRequestRequiresSuccessAndUniqueRequestID | 同文件：非成功请求不归因、成功唯一请求归因、重复请求 ID 拒绝 | 共享外部渠道位与账户隔离 |
| TestReadOwnedOrdersFiltersAndMasks | `internal/order/read_test.go`：本人归属、脱敏、公有 ID、筛选、分页及订单发生时间 | 随事件回补 / 退款后的同一链路计数 |
| TestCountsOwnRecordsAndDates | `internal/dashboard/read_test.go`：合成最终记录的本人 / 日期 / 渠道 / 推广位及退款排除 | 从原始事件生成记录、计数随乱序 / 退款 / 回补一致 |

## M3-05b：隔离数据库联测验收（环境已解除阻塞，新增联测未实现）

拟新增 `internal/order/anomaly_integration_test.go`，沿用 `orderTestDB` 的随机 schema 和清理，不改写历史迁移，不向需要保留数据的数据库运行测试。读取现有仓储公开方法；不增加测试专用生产入口。

每项先写带手工期望值的真实仓储测试、观察失败或对已有行为作明确回归证明，缺行为时再最小修正，完成后独立提交任务实现 / 测试 / 状态。恢复数据库后先执行已有 9 项，任何失败先定位原因，不通过跳过或削弱断言解决。

- [ ] M3-05b-1【未开始】重复 / 乱序 / 时间回补与指标一致性。固定上海 2026-09-18 当天范围；先 PAID，再重放原事件及同状态新事件，归因、列表、看板都只计一个订单。后到的早期 CREATED 跨入前一自然日时，状态仍 PAID，但 `order_occurred_at` 为真实最早发生时间；前日看板为 1、原日为 0、列表日期筛选一致，原始事件和投影历史保留。相同 eventId 不同载荷必须冲突，不能偷偷重新归属。
- [ ] M3-05b-2【未开始】退款和先退款后订单。部分退款记录只增加一个 refund event，订单状态不误变 REFUNDED，当前有效计数仍 1；全额退款后有效计数 0，订单仍能以 REFUNDED 查询本人详情、保留历史，后到 PAID 不恢复有效订单。退款重放不多记录。先退款缺订单返回 ErrMissingOrder、不可半写；补订单再重放原退款才能成功。退款金额只是仓储合成事件，不推导钱包冲正或可提现收入。
- [ ] M3-05b-3【未开始】无归因和匿名消费者。NONE / 未知 subId 保留 PENDING_REVIEW，所有本人列表与看板不展示该订单；有效推广者 Tracking 可归因推广者，但不得创建 / 推断消费者身份、返现或钱包记录。另一个推广者的列表 / 详情 / 看板不可读取。既有待核验记录的不同凭据 Apply 会返回冲突，不能假装已存在人工回补接口；先设计受权审计回补方案，另编号实现后再验收。
- [ ] M3-05b-4【未开始】共享渠道位与账户隔离。`channel_positions` 已有 `(channel,account_id,external_position_id)` 唯一约束：同账户同外部位映射到两人应拒绝，而不是任选一个归属；不同账户相同外部位分别映射时，只能以可信 AccountID 精确归因，错误账户 / 非 READY 不归属。合成 READY 仅测试使用，不能给生产渠道置 READY。消费者在多人使用同一外部位时仍不可推断。
- [ ] M3-05b-5【未开始】边界 / 无效 / 取消及读取一致性。开始时刻包含、结束时刻排除；同一订单 COUNT 只一条，INVALID / CANCELLED / REFUNDED 不计有效。改变状态与最早发生时间后本人读取、筛选、计数与元数据一致；历史 23 / 25 小时自然日使用实际 IANA 边界。不把 UTC 订单日期和上海看板自然日当作同一日期，需要传入对应的真实时刻范围比较。

PROC-11补充证据（2026-09-19）：新建私有PostgreSQL17.11仅Unixsocket实例，已有order / dashboard共11项（含上述9个数据库用例）全部RUN / PASS、0 SKIP；Go全量test / vet / build通过。环境见[隔离库记录](../../31-isolated-postgresql-acceptance-20260919.md)。上文初次复核SKIP保持为历史，不代表当前仍缺库。

下一步：在该可丢弃私有实例研发M3-05b-1～5新增联测与独立断言；各用例保持随机schema / 清理，不仅凭旧用例通过标为完成。不得将生产DATABASE_URL自动当作PG_TEST_DSN；真实业务M3-05c仍阻塞。

验证命令（测试连接由专用运行环境提供，不在文档写真实 DSN）：

```sh
go test -count=1 -v ./internal/order ./internal/dashboard
go test -race -count=1 ./internal/order ./internal/dashboard
go test -count=1 ./...
go vet ./...
git diff --check
```

验收要求：指定数据库用例确实 RUN / PASS、零 SKIP；断言覆盖数据库行数、归属、历史、退款与过滤结果，不仅检查错误为 nil；并发用例使用真实事务与数据库约束。数据库证明完成后才能勾选 M3-05b 子项。

## M3-05c：真实业务验收（阻塞）

需正式获批渠道原始事件、可信归因凭据、身份提供方及规则版本，逐渠道验证实际重复 / 退款 / 回补，保存脱敏样例和对应服务端回执。M0 资金决策与 M4 收益快照完成后，核对订单和收益双状态、退款冲正及财务对账。真实业务证据没有提供，M3-05 与全链路发布门槛保持未完成。

## 后续可安全推进的并行门槛

数据库恢复前可继续 REL-01 的现有页面 P 编号映射、字体放大 / 键盘 / 触控区域及窄屏验证；不以这些前端证据替代本矩阵数据库或资金要求。
