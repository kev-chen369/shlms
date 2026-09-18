# 统一物料与能力实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按已确认V3设计建立可信平台能力与商品 / 活动物料边界，最终提供本人只读目录，不提前解锁取链。

**Architecture:** 沿用Go模块化单体，能力领域判定与物料模型分离；后续仓储只读可信配置，API仅返回安全投影。能力判定不是用户资格 / 推广位最终事务检查的替代物，不能凭通过领域测试装配真实渠道。

**Tech Stack:** Go / PostgreSQL / uni-app Vue；不新增依赖。

**Spec:** [V3详细设计](../../23-multiplatform-promotion-ui.md)、[架构复核](../../24-multichannel-promotion-architecture-v3.md)。

## Global Constraints

- 平台京东 / 淘宝 / 美团独立；商品 / 活动不同类型，不更改既有京东单品契约。
- 没有授权目录时显示空态和原因，不填演示商品；未获批能力不开放生成。
- 只从可信服务端仓储取得能力声明，不接受客户端 / 普通管理员自填READY。
- 后续迁移新增，不改000007或其他已存在迁移；本计划不授权生产数据库迁移。
- 每个可独立验收项按主计划编号更新状态、验证并立即本地commit；push另按用户请求。

## M7-01a：能力声明与判定（本轮独立交付）

Files：创建 `internal/capability/policy.go` 与 `policy_test.go`，更新主计划状态。

Interfaces：`Evaluate(record Record, requested Key, now time.Time) Decision`。Key精确绑定Platform(JD/TB/MT)、MaterialType(PRODUCT/ACTIVITY)、Kind(CATALOG/PRODUCT_PREVIEW/PRODUCT_LINK/ACTIVITY_LINK/RESULT_LOOKUP/ORDER_ATTRIBUTION)、MediaID、PositionID、Scene、Terminal(H5/WX_MINI)、CityCode、Business。字段全部精确匹配，空CityCode / Business不是任意城市 / 业务通配符。

Record含Key、Status(UNCONFIGURED/PENDING_VERIFICATION/READY/SUSPENDED)、Evidence；Evidence含OwnerID、MediaApprovalRef、SourceApprovalRef、InterfaceVersion、RealCallEvidenceRef、VerifiedAt / ExpiresAt。引用只保存脱敏证据标识，不放签名密钥 / 原始错误。Decision仅Allowed / Reason，不公开内部证据。

首期美团仅承认ACTIVITY声明，PRODUCT须另有官方协议 / 获批证据及设计扩展；内部填配置不能跳过此限制。声明适用范围全部精确，不凭平台某一能力READY推出另一能力READY。

字符串边界：MediaID / PositionID / 证据字段128字节、Scene80、CityCode32 / Business40；必填标识非空、无首尾空白 / 控制字符、有效UTF-8。类型能力组合必须一致；未知值拒绝。时间由服务端显式传入，zero拒绝；READY需完整证据、verifiedAt<=now<expiresAt且有效窗口递增。判定不证明证据真实性，信任只能由后续仓储权限 / 审核产生；当前不装配任何生产服务。

- [x] 写失败测试：例如 `Evaluate(Record{}, Key{}, now).Allowed` 必须false；可信完整同Key READY才允许，跨平台 / 类型 / 媒体 / 位 / 场景 / 终端 / 城市 / 业务分别拒绝。
- [x] `go test -count=1 ./internal/capability` 观察缺少生产类型 / 判定函数的RED。
- [x] 实现最小Key / Record / Evidence / Decision及Validate判定，无路由 / READY写入口 / 渠道适配改动。
- [x] 拒绝未知状态 / 类型能力组合、空 / 控制 / 超限证据、future / expired / zero时间；到期边界不允许，无证据泄漏。
- [x] 定向及Go全量test / vet / build、diff / 文档链接检查通过，独立审查，记录模拟与真实限制后commit `feat(M7-01a): add scoped fail-closed capability policy`。

2026-09-19本项实现 / 定向6个顶层测试通过，36个平台 / 类型 / 能力组合以独立字面允许表验证；全量Go命令通过，PG_TEST_DSN未配置的数据库集成SKIP不视为执行。独立审查无阻断，审查者定向 / diff通过。本地提交关联M7-01a，不将本项允许判定当真实获批证据或开放能力。

## 后续任务与验收依赖

M7-01b～f已在主计划逐项编号；各项开始前补充该项实际仓储 / API / 前端接口及失败测试步骤，不能拿本项判定实现当目录或真实能力验收。迁移验收需要隔离PostgreSQL，真实能力验收需要负责人和获批账号；缺少时如实阻塞对应项，不阻断可独立领域建模。现有订单 / 预览 / 转链路径保持原义。
