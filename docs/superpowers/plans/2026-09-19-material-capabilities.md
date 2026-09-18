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

### M7-01b：物料身份、适用范围与卡片投影

Files：创建 `internal/material/model.go` / `model_test.go`；更新主计划、架构及测试记录。接口 `Validate(record Record) error`、`CardFor(record Record, context Context, now time.Time) (Card, Decision)`。Card可展示不等于可生成；调用方仍须独立检查身份、能力与推广位。

Record：ID为规范小写UUID，Platform JD / TB / MT，Type PRODUCT / ACTIVITY，ExternalMaterialID(128字节)、CanonicalURL(2048，仅内部)、Title(256)、Status DRAFT / ACTIVE / SUSPENDED / REMOVED，StartsAt / EndsAt / SourceUpdatedAt，RuleVersion80 / EvidenceRef128，Region{Mode NATIONWIDE / CITIES, CityCodes最多64个唯一非空32字节code}，Business可选40字节，Terminals至少1个、最多2个唯一H5 / WX_MINI。首期MT只活动；商品与活动不要求价格，不把活动external ID解释成SKU。

所有文本UTF-8有效、无首尾空白或控制字符。CanonicalURL只校验HTTPS绝对URL语法、无userinfo / 反斜杠 / 字面空白，不声明DNS / 重定向 / 官方白名单已验证，且永不在Card中公开。SourceUpdatedAt与EndsAt必填，活动StartsAt必填，非空窗口必须start<end；三个时间须可序列化为JSON时间，防止超出年份 / 时区范围。历史 / 待开始材料可保存，卡片判断按start<=now<end、源更新时间不在未来、状态ACTIVE。

Context含Platform / Type / CityCode / Business / Terminal。NATIONWIDE明确覆盖所选城市，CITIES要求明确匹配城市；Business为空表示业务不限，地域仍由Region独立决定，否则业务精确匹配。终端必须显式包含。此物料范围与M7-01a能力Key非通配的语义不同，能力必须另外查可信声明。

Card字段白名单：ID / Platform / Type / Title / StartsAt / EndsAt / SourceUpdatedAt / RuleVersion / Region / Business / Terminals；PRODUCT未设置StartsAt时使用可省略pointer，不输出0001年伪开始时间；不包含CanonicalURL / EvidenceRef / ExternalMaterialID / 价格 / 预计收益。任何拒绝都返回零Card和固定Reason：INVALID_MATERIAL / INVALID_CONTEXT / SCOPE_MISMATCH / NOT_ACTIVE / NOT_STARTED / EXPIRED / SOURCE_NOT_CURRENT / REGION_MISMATCH / BUSINESS_MISMATCH / TERMINAL_MISMATCH。复制slice及StartsAt值防止调用方改写内部规则。

- [x] 写失败测试：商品正常卡片、无价格活动、MT商品拒绝、平台 / 类型 / 日期 / 状态 / 地域 / 业务 / 终端不适用、未知与缺失字段、URL / 证据不公开。
- [x] `go test -count=1 ./internal/material` 观察缺少接口RED；实现Record / Context / Card / Decision及校验 / 投影，不改京东适配器或数据库。
- [x] 补边界、无价格字段与slice隔离、序列化安全证据，全量Go test / vet / build与链接 / diff检查，独立审查后逐项commit `feat(M7-01b): add typed material scope and safe card projection`。

2026-09-19：7 个顶层测试通过，JSON 超范围年份回归先失败再修复；全量 Go test / vet / build 退出0，独立及增量审查无阻断。链接检查通过。数据库依赖测试因 PG_TEST_DSN 未配置跳过；此项不构成真实来源、网络 URL 安全、目录 API 或生成能力验收。

M7-01b～f已在主计划逐项编号；各项开始前补充该项实际仓储 / API / 前端接口及失败测试步骤，不能拿本项判定实现当目录或真实能力验收。迁移验收需要隔离PostgreSQL，真实能力验收需要负责人和获批账号；缺少时如实阻塞对应项，不阻断可独立领域建模。现有订单 / 预览 / 转链路径保持原义。

### M7-01c-1：仓储查询与游标契约

文件：internal/material/query.go、query_test.go。Query含OwnerID（可信身份解析后规范非零UUID）、Scope Context、Limit1～100、Cursor；ParseQuery返回AfterID，不接受客户端作为身份来源。平台 / 类型组合、终端及可选城市业务使用领域字段边界。游标使用版本1 JSON的raw URL base64编码，仅保存owner / scope / afterID，最多1024字符，拒绝未知字段、尾随JSON、无效UTF-8及非规范编码。用户 / 任意筛选上下文切换旧游标拒绝；Limit可以改变，不改变范围。EncodeCursor须先验证查询及规范非零UUID afterID，不嵌套旧游标。游标不是签名或授权；篡改after只影响本人范围分页位置，后续仓储 / API必须独立鉴权。

- [x] 失败测试：合法首屏与游标往返、跨用户与五种筛选切换、缺身份 / 组合 / limit、坏游标 / nil UUID / 未知字段。
- [x] 定向RED后实现最小协议，全量Go验证；独立审查后以M7-01c-1提交。不声明数据库仓储完成。

2026-09-19：4组新增查询测试通过，缺接口RED后GREEN；补齐实际重复键、空白 / 重排、nil afterID与最大字段往返。Go test -count=1 ./internal/material及全量test / vet / build、diff检查退出0。独立审查无阻断；仓储、迁移、API及生产授权未实现，PG_TEST_DSN未配置的数据库测试跳过。M7-01c-2继续，c-3隔离库阻塞，不将本协议当端到端目录。

### M7-01c-2a：物料目录数据库迁移

文件：migrations/000022_promotion_materials.up.sql / down.sql，internal/material/schema_test.go。消费Record领域契约，产生promotion_materials表（内部UUID主键、platform / material_type / external_material_id唯一、内部canonical_url / evidence_ref、title / status默认DRAFT、starts_at可选商品 / 必填活动、ends_at / source_updated_at、rule_version、region_mode / city_codes TEXT[]、business、terminals TEXT[]），无价格 / SKU / 收益。DB保证MT仅活动、非零UUID、基本字节边界、已知状态、有限1～9999年窗口及起止顺序、城市与终端一维唯一非空规则；共享数组校验函数只供此表约束。完整UTF-8 / URL语法与信任证据仍由领域校验和获批导入保证，不因数据库字段存在获得授权。

- [x] 写真实隔离schema测试，执行up后插入合法商品 / 无价格活动，验证平台命名空间唯一、非法范围 / 窗口CHECK失败、down无残留；依赖PG_TEST_DSN，不以SKIP当RED。
- [x] 在新建私有实例运行定向测试，观察缺迁移RED，然后实现新增迁移，不改历史SQL。
- [x] 定向 / 全量Go及完整迁移验证，记录DB版本与实跑 / 跳过范围，审查并提交M7-01c-2a。

2026-09-19：PostgreSQL17.11私有Unixsocket实例，真实缺迁移RED后GREEN；26个CHECK拒绝用例、最大城市数量 / 字节边界、身份命名空间及唯一性与down表 / helper无残留通过。时间fixture显式Z避免上海时区将10000年转换为9999年UTC；没有放宽约束。独立审查无阻断，审查者实跑原21边界，新增minor边界主执行复验通过。配置PG_TEST_DSN全量Go test / vet / build和diff退出0；既有迁移链up / 并发 / 重复验证随全量执行，本新增迁移独立up / down实测。全部新迁移完整up / down与仓储一致性仍属于c-3，不提前勾选。未执行生产迁移 / 部署，未开放目录或生成。

### M7-01c-2b：能力与不可变证据结构

文件：migrations/000023_channel_capabilities.up.sql / down.sql，internal/capability/schema_test.go。沿用Key / Evidence：channel_capabilities含非零UUID id、platform / material_type / kind / media_id / position_id / scene / terminal / city_code / business完整唯一范围，position_id引用既有promotion_positions，默认UNCONFIGURED，状态四种，evidence_id可空。channel_capability_evidence含非零UUID id、capability_id外键、五个必填128字节脱敏引用字段、verified_at / expires_at有限JSON年份且递增、recorded_by / recorded_at provenance。READY必须非空evidence_id，复合FK(id,evidence_id)绑定本声明证据；先插声明，再追加证据，最后受控指向。Key与ID禁止UPDATE改义，证据UPDATE / DELETE拒绝，停用仍保留证据历史；down先解除循环FK再删新表 / 函数，不碰旧表。DB只保证字段及绑定，不证明来源真实性 / 管理员资格，不用CURRENT_TIMESTAMP CHECK自动批准；读者仍须Evaluate核验时效。无应用写入口、角色授权或真实READY装配。

- [x] 在独立schema使用真实既有000001～000022迁移，再执行000023 up，写声明默认 / 唯一 / 平台类型能力 / 字段 / 推广位FK及READY本证据测试；缺迁移真实RED。
- [x] 实现新增表 / 精确唯一 / 复合FK / Key及证据不可变触发器，验证跨声明证据拒绝、缺字段及无效时间、停用保留及down新表 / 函数无残留、旧推广位保留。
- [x] 配置私有PG_TEST_DSN定向 / 全量Go test、vet / build / diff，独立审查后commit M7-01c-2b；完整新链up / down与仓储仍归c-3。

2026-09-19【已完成】：2项新增数据库测试含30个字段边界子用例及默认 / 唯一 / FK / 不可变 / down断言。真实TRUNCATE CASCADE绕过逐行保护RED后，增加BEFORE TRUNCATE语句级拒绝触发器，定向与全量Go test -count=1 ./...、go vet ./...、go build ./...、git diff --check退出0；独立增量审查实际复跑通过，DROP TABLE不触发TRUNCATE保护，down通过。数据库只提供基本字段与关系约束，不证明引用真实性或授权，也不防有权限者禁用触发器 / DDL；读者仍须完整域验证及Evaluate。未执行生产迁移或部署。
