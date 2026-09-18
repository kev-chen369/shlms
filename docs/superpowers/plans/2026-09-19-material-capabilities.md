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

### M7-01c-2c-1：能力只读仓储

创建internal/capability/repository.go / repository_test.go。Repository{DB *sql.DB}.Check(ctx context.Context, ownerID string, key Key, now time.Time) (Decision,error)。ownerID仅来自可信身份，规范小写非零UUID；无效owner / Key / zero now返回ErrInvalid及零Decision，nil / 关闭 / SQL失败返回ErrUnavailable（context取消保留context错误）与零Decision，不输出数据库原始错误。单条参数化SQL读取本人推广位与资格，再LEFT JOIN九维精确声明及复合绑定证据；无本人位 / 停用 / scene不符统一POSITION_UNAVAILABLE，本人资格非ENABLED返回NOT_ENABLED，无声明默认UNCONFIGURED。证据owner是负责人引用，不冒充推广员ID。完整记录只在内部交给Evaluate，不返回媒体、负责人或证据；字段完整不代表获批真实性。读取快照不是生成锁 / 事务授权；不装配生产服务，无READY写入或渠道调用。

- [x] 写真实PG失败测试：默认拒绝、完整精确READY及各维错配、跨用户 / scene / 位与会员停用、四状态、未来 / 到期证据、数据库允许但域拒绝的Unicode空白、取消 / nil / 关闭错误安全；确认缺Repository的RED。
- [x] 实现上述单查询读仓储，使用已有Evaluate；真实PG定向GREEN，无推广 / Tracking写入。
- [x] 配置私有PG_TEST_DSN全量Go test / race定向 / vet / build、diff及链接检查，独立审查后commit M7-01c-2c-1。2c父项、物料仓储2及全链c-3不勾选。

2026-09-19【已完成】：2项新增仓储顶层测试，九维错配及非默认淘宝活动正例、四种非启用会员状态、SQL错误脱敏和Unicode证据拒绝。缺Repository / Err接口编译RED后实现；测试fixture按pgx参数化单语句及完整UTC时间修正，不把fixture错误当产品缺陷。实际私有PostgreSQL17.11定向 / -race、最终全量go test -count=1 ./... / go vet ./... / go build ./...退出0，18个文档相对链接及git diff --check通过；独立审查实际TestRepository / diff通过，无阻断。最终非系统schema数量0，测试仅临时合成数据；零Tracking新增、两份READY声明及三份证据保留断言。只读能力仓储已实现，物料仓储 / API / 真实授权与生成事务复核不在此项完成范围。逐项本地commit，不push / 部署。

### M7-01c-2c-2：授权物料筛选与分页仓储

创建internal/material/repository.go / repository_test.go；能力仓储提取共享queryRow实现并增加CheckInTransaction(ctx,*sql.Tx,owner,key,now)，保持原Check契约。Repository{DB *sql.DB}.List(ctx,Query,capability.Key,now) (Page,error)。Page只含Items []Card、NextCursor string和Capability capability.Decision；错误返回零Page / ErrInvalid或ErrUnavailable，取消保留context错误。Query平台 / 类型 / 城市 / 业务 / 终端必须与CATALOG Key完全一致，媒体 / 位 / scene同领域字节及文本边界，now非零；owner仍是可信身份，旧游标绑定owner / 筛选。开启只读REPEATABLE READ，先CheckInTransaction按本人资格 / 位 / 完整能力及证据判定；拒绝即非nil空Items、无cursor，固定安全原因，不查询物料。允许后在同快照中按UUID递增keyset分页，SQL独立绑定owner启用状态、平台 / 类型、ACTIVE、start<=now<end、source<=now、地域 / 业务 / 终端。NATIONWIDE物料覆盖所选城市，空物料业务不限，与能力空值非通配严格区分。

每批limit+1原始记录仍经CardFor完整域校验及白名单投影；非法URL / Unicode证据等不返回。跨批继续直到limit+1合法卡片或耗尽，避免非法前缀导致漏项 / 空页 / 假hasMore；NextCursor是最后一张返回卡片ID，仅确实存在后续合法卡片时生成。所有扫描 / JSON数组转换 / row关闭 / commit错误均脱敏，失败不返回部分页面。事务快照不是生成授权锁，后续调用仍最终复核；无网络 / 导入 / 写入 / API装配，来源真实性、角色权限仍独立门槛。

- [x] 写真实完整迁移fixture测试：无能力空态、精确READY列表、安全字段、筛选与窗口边界、非法前缀 / 中间记录分页、游标换人换范围拒绝、资格 / 位 / 能力停用与到期、新请求可见变更、错误 / 取消零Page；缺接口RED。
- [x] 实现只读快照能力桥接与列表，实测正例 / 拒绝 / 持久化零副作用，定向GREEN。
- [x] 私有PG_TEST_DSN定向-race及全量Go test / vet / build、diff / 链接，独立审查后commit M7-01c-2c-2。完整up / down / 并发快照验收仍归c-3，API与真实渠道门槛不提前完成。

2026-09-19【已完成】：5项目录仓储测试及1项事务生命周期测试；缺接口编译RED后实现，真实PostgreSQL17.11定向 / -race与最终配置PG_TEST_DSN全量go test -count=1 ./... / go vet ./... / go build ./...、diff退出0，18文档相对链接通过。曾因测试跨包Key位置literal导致vet失败，改具名字段；故障注入VOLATILE函数在view下形成EXPLAIN可见SubqueryScan→Sort→Limit提前抛错，禁用顺序扫描未解决，恢复默认planner并将纯只读函数正确声明STABLE，先验证前两条成功再验证后批SQL失败。临时mutation把查询错误误返已有page，真实RED捕获ID1 / READY部分返回；精确恢复fail()后GREEN，未保留变异。缺目录表且能力停用仍返回空态；Card白名单、slice独立和零新增Tracking / 请求通过。独立审查与后批故障增量实跑无阻断，最终非系统schema数0。仅私有合成数据，未执行生产迁移、渠道调用或网络；c-2 / c-2c父项等待c-3全链和并发快照综合验收，不提前勾选。下一项c-3；公开API及真实授权仍未完成。

### M7-01c-3a：完整迁移链回归

修改internal/dbmigrate/runner_test.go的新表存在核对；新增chain_test.go使用既有isolatedDB及真实readMigrations / Run。先Run全部23版、Verify，逐一逆序读取同名down，在测试事务中执行down并删除对应测试账本行（非生产down runner），确认仅保留空schema_migrations，无业务表 / 视图 / 序列或函数，再Run全部up、Verify及重复Run零应用；全部版数动态读取不硬编码未来版本。现有4路并发执行也核对三张新表。此项只测DDL及临时账本，已有23 populated proof/down证据另见2b，不声称生产无损回滚。私有PG_TEST_DSN定向count3 / -race及全量test / vet / build、diff / 链接，独立审查后逐项commit；3b快照并发及父项仍未完成。

- [x] 实际完整up / reverse down / up断言和新表核对，不用文件文本存在当执行证据。
- [x] 配置私有PG_TEST_DSN定向重复 / race与全量Go验证，独立审查后以M7-01c-3a提交。

2026-09-19【已完成】：真实PostgreSQL17.11两个定向测试-race -count3全部通过，全量配置PG_TEST_DSN go test -count=1 ./... / go vet ./... / go build ./...与diff退出0，14相对文档链接通过；独立审查实际复跑完整链与四路并发通过，无阻断。没有新生产实现或修复，无虚构RED，新增测试是既有迁移的综合验收。无生产down / 账本修改或部署，3b及父项仍未完成。

### M7-01c-3b：读取阻塞期间的快照一致性

新增internal/material/snapshot_test.go，复用真实catalogDB及当前Repository.List，无模拟仓储 / 时钟或生产同步hook。各case三条实际材料：ID1合法、ID2域非法证据、ID3合法，limit1须跨批读ID3才能返回真NextCursor。writer事务持有promotion_materials ACCESS EXCLUSIVE锁，启动List；用pg_locks实际目标relation OID / 未granted及pg_blocking_pids观察List阻塞，证明其能力读取已先完成，不以sleep猜测。writer分别提交能力SUSPENDED、会员DISABLED、位DISABLED、未来新证据指向、物料SUSPENDED / 地域 / 业务 / 终端 / 到期 / 标题变化，释放锁；当前List必须仍旧ID1 / 原标题及真hasMore，后续新请求分别安全拒绝、目录空或新标题。每case检查三条材料 / 证据保留与零Tracking / 转链请求，context20秒、buffered结果、事务回滚收尾。此证据是只读快照一致性，不表示撤销瞬间取消所有进行中读取或生成锁；真实渠道 / 生产门槛仍独立。

- [x] 实跑10种变更的真实锁观察、跨批旧快照与新请求可见断言。
- [x] 私有PG_TEST_DSN定向-race -count3与全量Go验证 / 文档链接 / diff，独立审查并提交M7-01c-3b后复核c-2 / c-3父项。

2026-09-19【已完成】：真实PostgreSQL17.11十case定向-race -count3全部通过，旧游标续页新增断言同样三轮通过；最终配置PG_TEST_DSN全量go test -count=1 ./... / go vet ./... / go build ./...、diff退出0，18相对文档链接通过，独立审查实际十case通过无阻断。临时降为READ COMMITTED的mutation实际捕获会员变更后READY / 空页、标题变更后READY / 新标题混合快照RED；精确恢复原RepeatableRead，未保留生产变更。最后非系统schema数0，测试合成数据已清理。结合3a及c-1 / c-2各子项现有与本轮全量实跑证据，父c-2 / c-2c / c-3 / M7-01c范围复核完成，仅结构、只读目录列表仓储及隔离验证；详情 / 目录HTTP、前端、生产角色 / 来源批准、真实渠道与生成最终复核仍未完成。下一项M7-01d，不push / 部署。

### M7-01d-1【已完成】：详情只读仓储

在既有目录仓储上增加 `Get(ctx, ownerID, scope, key, materialID, now)`，不复用列表limit / cursor，不将内部Record序列化。返回Detail的可选item安全Card、capability Decision及availability Decision。规范本人UUID / 物料UUID、CATALOG与全部范围参数先校验；只读REPEATABLE READ先检查同快照本人资格 / 位 / 能力，再按ID + 平台 + 类型及独立本人启用条件读取。能力拒绝时不查物料、item为空、availability为CAPABILITY_UNAVAILABLE；无行及跨平台类型统一MATERIAL_UNAVAILABLE。同范围已找到记录由CardFor检查状态 / 时效 / 地域 / 业务 / 终端及内部字段完整性；拒绝只有安全原因，item为空。SQL / commit失败不返回部分详情，取消保留context错误。无网络、写入、生成或公开路由。

- [x] 真实PG详情白名单 / 拒绝 / 缺失范围 / 输入与存储错误测试先RED，最小实现后GREEN。
- [x] 定向race、全量Go / vet / build及diff验证；只读独立审查后记录证据并逐任务提交。

HTTP可信媒体与场景解析属于d-2，启动及鉴权集成属于d-3；仓储验证不替代API或真实渠道 / 来源验收。当前特性分支无其他tracked修改，按此前自行判断授权在原工作区继续，保留.DS_Store和所有其他工作树；不自动push / 部署。

2026-09-19：缺Get / Detail接口实际编译RED后新增实现。真实私有PG17.11六项详情测试及三轮race通过；商品安全11字段、三平台WX_MINI活动、11类域拒绝、缺失与跨平台类型同原因、本人 / 位 / 能力 / 证据到期、无能力缺表仍返回拒绝、规范ID / 范围 / kind / now、nil / SQL失败 / 关闭 / cancel错误零Detail、零Tracking / 请求断言通过。首轮到期reason测试误写EVIDENCE_EXPIRED，经policy现有契约证实改VERIFICATION_EXPIRED，生产策略不改。新增真实pg_locks / writer PID条件观察详情阻塞，提交会员 / 标题 / expiry变更，旧读取完整旧快照，新请求最新结果；临时READ COMMITTED实测三case混合快照RED，恢复后三轮GREEN。最终配置PG_TEST_DSN全量Go test / vet / build、9相关文档相对链接 / diff退出0；独立审查真实恢复后六项通过，无Critical / Important问题。新增78行详情实现，不改List、JD适配器或历史SQL；无公开路由、来源批准、生成权限或生产部署验收。M7-01d父项继续，下一项d-2。

### M7-01d-2a【已完成】：可信目录读取服务

CatalogBinding以平台 / 类型 / 终端 / 场景四维精确映射服务端MediaID；不设默认媒体 / 通配 / 自动READY。NewReadService复制配置到私有map，拒绝歧义重复绑定及非法组合 / 文本字段，DB不能为空，空配置是合法关闭态。ReadInput只有可信OwnerID、五维Scope、所选内部PositionID及Scene，无media / now / 状态字段。服务List接cursor / limit，Get只接materialID；读取前验证完整输入、分页绑定和取消。缺绑定返回非nil空列表或无item详情、UNCONFIGURED能力拒绝，不调用物料SQL；有绑定构造完整CATALOG Key，以服务端time.Now逐次调用真实仓储复核本人位、资格和证据。

所选位 / scene不是客户端授权声明，仓储仍验证本人、启用和实际位scene。媒体配置只是来源选择，并不代替获批负责人 / 来源 / 实调用证明；仓储读取可信证据记录并校验结构 / 时效，真实性另行审核。游标仍按既有owner / 五维Scope协议绑定分页位置，不携带缓存授权；换位 / 场景或配置改变后每次重新检查完整Key，前端切换时必须清理旧游标。

- [x] 缺服务接口RED后实现；真实PG验证可信配置复制、正确媒体 / 分页 / 详情、错媒体与四维配置隔离、撤销和跨用户、空配置缺表无读取、非法 / 重复配置与输入及取消零数据。
- [x] 定向race / 全量Go / vet / build / 文档链接 / diff，独立只读复审后本地逐任务提交。

M7-01d-2b仍负责HTTP鉴权与参数白名单，d-3负责启动装配与验证器集成；当前服务未注册路由或配置真实媒体，不push / 部署。

2026-09-19：缺CatalogBinding / NewReadService / ReadInput真实编译RED后新增服务和3项真实PG测试GREEN；私有PG17.11定向-race -count3及配置PG_TEST_DSN全量go test -count=1 ./... / vet / build均退出0，11计划相对链接及diff通过。独立审查实际三项通过，无Critical / Important；修正文档真实性边界minor措辞，未改变策略或源批准。全量及审查完成后非系统schema数0。服务输入不含media / READY / now，复制静态部署映射、精确无通配，逐次数据库复核，无API / 新迁移 / 真实渠道 / 前端装配。逐任务本地提交，M7-01d-2和父项继续，下一项d-2b。

### M7-01d-2b【已完成】：鉴权HTTP契约

Dependencies新增Materials List / Get接口，两条GET只在服务和Users同时存在时注册。handler先鉴权，再严格url.ParseQuery白名单和每字段唯一校验；公共参数platform / type / terminal / cityCode / business / positionId / scene，列表另允许cursor / limit，详情拒绝分页。ReadInput.Valid提取既有请求形状判定供服务与HTTP共用，ParseQuery校验规范owner、Scope与分页；UUID详情检查仍由读取服务保证。无客户端owner / media / READY / now输入，无默认平台 / 类型 / 场景或位。列表limit默认20，详情校验范围使用1但不接受分页参数。

成功与安全空态200，认证失败401、无效输入400、存储 / context失败503 MATERIALS_UNAVAILABLE，错误data为null。所有读取响应no-store；不返回错误伴随的部分数据，不补造price / URL / 生成权限。路由缺依赖404，无POST写入。完整请求响应边界见[API说明](../../01-api-spec.md)。

- [x] handler契约测试缺Dependencies.Materials先RED，最小实现后覆盖真实路由默认 / 显式分页、可信owner与Scope传递、依赖 / 身份缺失、鉴权错误不泄露、POST拒绝、严格参数、空态与脱敏错误 / 部分结果丢弃。
- [x] 定向race及配置PG_TEST_DSN全量Go / vet / build，文档链接 / diff与独立审查后逐任务提交。

本任务采用读取接口测试替身验证HTTP边界，不替代真实数据库 / 签名验证器的HTTP链路，后者属d-3；不改app启动或JD渠道适配器，不push / 部署。

2026-09-19：缺Materials依赖字段实际编译RED后新增实现，四项实际handler / router测试GREEN；首轮错误用例误以为error应省略data，经writeError现有统一契约证实data:null，修改测试并加强错误伴随部分数据不泄露及明确空态原因断言，未改全局契约。定向-race -count3与私有PG17.11配置PG_TEST_DSN全量Go test / vet / build、17文档相对链接 / diff退出0，独立审查四项实际通过无Critical / Important；非系统schema数0。HTTP只读范围父d-2依据a / b及最终全量证据完成，d-3真实验证器 / 仓储链路和详情UUID拒绝、运行配置及启动装配仍未验收；M7-01d及整体目标继续。本地逐任务提交，无push / 部署。

### M7-01d-3a【已完成】：应用装配与签名 / PG / HTTP联测

app.Config增加显式CatalogBindings，NewHandler在既有公钥 / 数据库 / 迁移验证之后创建真实material.ReadService并注入路由；非法 / 重复配置启动失败，nil绑定是合法关闭态，不读物料且不授READY。cmd/api尚未配置加载，默认传空绑定，受控文件加载属于3b；不修改认证策略、自动迁移或渠道适配器。

测试使用runtimeDB独立schema和实际全部迁移、临时RSA2048密钥签发RS256 at+jwt、真实Verifier、ReadService / Repository及httptest本机HTTP服务器；经http.Client实际发GET，无reader或身份替身。合成物料和证据仅私有PG，使用服务端当前时间窗口，不声称真实授权或身份提供方验收。

- [x] 缺CatalogBindings编译RED后装配；真实本人首屏 / 后页 / 详情，未登录 / 错签名、跨用户 / 跨owner游标、非法 / 零 / 大写物料UUID、伪造配置参数及missingID；白名单 / no-store、零Tracking / 请求及物料证据保留。
- [x] 会员 / 位 / CATALOG撤销、证据到期和物料状态 / 日期 / 地域 / 业务 / 终端变更后，两API立即新请求复判，拒绝无item / cursor；错误配置、空绑定 / 错媒体默认拒绝、SQL失败安全503。
- [x] 三轮定向race、全量Go / vet / build、文档链接 / diff与独立只读审查后逐任务提交。

历史read-only快照保证完整旧读取；这里“新请求复判”不是撤销中止在途请求或生成事务最终锁。来源真实性、角色 / DDL权限、正式账户、设备、生成最终检查及前端选品仍待对应任务。

2026-09-19：3项真实应用测试及九类状态子项独立实跑通过，最终私有PG_TEST_DSN定向-race -count3和全量go test -count=1 ./... / vet / build均退出0，20相关文档相对链接与diff通过。首轮Page重复JSON解码使omitempty NextCursor残留于测试对象，造成尾页误判RED；按实际生产省略字段语义重置每页解码对象后GREEN，不改生产游标或序列化。独立审查实际三项 / 九子项无Critical / Important，强化跨人 / 空配置结果必须items[]或无item，三轮race再通过；最终全量实跑后非系统schema数0。只增加应用7行装配，不改Verifier或渠道 / 历史SQL、不启用生成、不对现有数据库执行迁移。3b受控配置加载与综合父项复核未完成，本地逐任务提交，无push / 部署。
