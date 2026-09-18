# 测试计划

## M7-01e-3a 本人推广位契约（2026-09-19）

`positions-api.test.ts`仅替换HTTP边界，测试真实读取函数和uni GET认证 / timeout配置；覆盖缺失或非法身份零请求、身份变化丢弃旧响应、401 / 404 / 故障脱敏、status=ENABLED分页、空目录、重复位 / 超量 / 非法游标 / envelope拒绝、安全字段投影和数组隔离。已启用位与JD配置readiness不是生成授权；未知READY / TB / MT映射拒绝。现有后端文本按Unicode字符计数，中文80 / ID256合法与81 / 257非法独立测试，不把物料scope字节限制误用在位响应。初版字节误用由独立审查发现，新增合法中文回归实见RED后修正；页面选择需另验收物料范围限制。本阶段无页面改动或实际图，不代表真实身份、位授权或业务验收。

## M7-01e-2 选品状态模型（2026-09-19）

`materials-model.test.ts`使用真实Vue reactive / watch / effectScope与实际materials-api，只替换HTTP边界并提供完整合成响应。覆盖身份 / 位未选零请求，空 / blocked / error区分、平台 / 类型清空scope、位 / scene / 城市 / 业务 / 终端范围复制及旧结果隔离、相同scope不重读、未开放平台或MT商品零请求、分页失败恢复 / 并发去重 / 游标循环拒绝、能力撤销清所有cache与待响应、失效详情移除 / 旧页不可恢复、鲜详情更新摘要 / 游标作废、三路径401锁拒session、迟到同session401、身份切换清scope / watcher同步、关闭详情 / dispose / effectScope清理。

平台 / 类型由显式选择命令更新，再显式setScope提供当前平台已选位与scene；会话切换不自动使用前身份的位。详情刷新无论有效或失效都取消旧续页并清游标，需刷新目录重建分页，不借同一个cursor延续已发生变更的结果。这里是数据 / 状态验收，不是选品页面 / 浏览器或真实授权验收，模型不存预览 / 链接或生成许可。

## M7-01e-1 物料前端数据契约（2026-09-19）

`materials-api.test.ts`测试真实API适配函数，仅在网络边界提供完整合成响应；验证JD / TAOBAO / MEITUAN映射和MT仅活动、完整范围快照 / 编码、20项分页 / 1024字节安全游标及尾页省略格式、匹配详情UUID、UTF8字节与纳秒时间边界、范围匹配、白名单及数组拷贝、身份变化丢弃在途响应、401 / 404 / 失败脱敏与uni GET认证。冲突能力 / 物料可用性、越范围 / 重复卡片、未知状态及非法输入均拒绝；只返回CATALOG读取判定，不返回生成授权、价格或内部URL。

缺模块RED后实现；补普通映射对象继承属性（toString / constructor / __proto__）3项实际RED，再按自有属性白名单修复；数组和String包装对象的隐式键转换2项RED后增加严格字符串类型检查。这里验证数据契约，不是后端真实HTTP联调、状态模型、选品页面或真实设备验收；M7-01e-2 / 3另验证，图稿尚无变化。

## M7-01d-3b-2 启动配置验收（2026-09-19）

`TestLoadCatalogBindingsFile`验证普通绝对路径、未设置 / 空数组关闭、缺失 / 目录 / FIFO / symlink / 相对路径 / 超限 / 非法内容安全拒绝；`TestLoadAPIConfig`验证环境选择与启动配置保留，`TestMainRejectsBrokenCatalogConfig`执行真实main子进程，错误配置失败且无监听或输入泄漏。`TestRuntimeMaterialsDeploymentFileEntrypoint`编译临时API并用隔离PG / RSA签名实际HTTP读取，验证配置真正传入列表 / 详情、未设置拒绝、不热重载、数据库撤销及零Tracking / 转链请求。临时丢弃main绑定的mutation实际RED，恢复后GREEN；不是源码文本测试。

用私有PG_TEST_DSN运行cmd/api、material、app定向三轮race和全量Go / vet / build；本机Unix进程与合成数据不替代正式身份提供方、媒体来源、最小权限、真实设备或渠道生成验收。前端选品仍待M7-01e，现有京东适配器不变。

## 分层

- 单元：状态映射、金额换算、佣金规则、幂等键和风控规则。
- 契约：用脱敏样例验证 Adapter 的请求、响应、签名与错误映射。
- 集成：数据库事务、Outbox/MQ、缓存、重试和并发钱包更新。
- 端到端：注册到提现及对账的完整链路。

## 必测负路径

重复订单/消息、乱序状态、超时限流、无效签名、权限不足、全额/部分退款、佣金下调、反作弊失效、余额不足、重复提现、深链失败、活动过期、渠道熔断与恢复。

## 发布门槛

测试环境全链路通过、生产配置检查通过、无高危安全问题、可回滚、监控告警已演练。只有代码测试通过不得标记 `PRODUCTION_READY`，还需实际渠道账户和结算证据。

## 推广中心专项验收

关联[详细设计](./21-promotion-center-detailed-design.md)和[研发计划](./superpowers/plans/2026-09-16-promotion-center.md)，按 P-00～P-16 记录用例与证据：

- 首页位置与身份分流、登录安全回跳、申请幂等、审核拒绝 / 停用、推广位跨用户隔离与并发默认位。
- 链接识别 SSRF、重定向和 DNS 地址变化、未配置渠道、过期预览、价格变动二次确认、同键不同输入与上游超时恢复。
- 预览有效期不得冻结于事务开始：让预留真实阻塞在渠道映射行锁上，等待预览自然过期后释放锁，应返回 PREVIEW_EXPIRED 且零新增记录；旧回执重放保持同一请求，新幂等键仍拒绝。
- 转链预留事务内渠道 readiness：缺映射 / 待核验 / 报价后账户或外部位变化不得写入；观察 PostgreSQL 阻塞关系验证并发失效，原请求重放不新增 Tracking。生产迁移仍只允许待核验，READY 成功用例仅使用随机测试 schema 合成配置，不能代替渠道核验证据。
- 同人同键不同推广位：两组共 12 路并发、使用不同推广位 / 预览 / 商品，只有一组回放同一回执，另一组全部幂等冲突；数据库必须只有一条请求及 Tracking，推广位、预览、商品和用户归属一致。仅变更推广位但保留预览和键也应冲突；跨用户推广位或预览混用时零新增记录。
- 执行领取与停用并发：观察 PostgreSQL 阻塞关系，停用身份或内部位提交后必须拒绝领取，保留 PENDING / 原版本 / 零尝试且无租约。请求行锁等待结束后才开始执行租约；注入领取更新失败应完整回滚、释放资格锁，并允许后续正常领取。
- 调用取消后的结果归档：创建中取消调用者，不确定结果仍持久化 QUERY_REQUIRED，已确认成功仍持久化链接；恢复查询返回已确认成功时取消调用者也须落库。创建次数及尝试次数始终为一，查询使用原稳定请求号，终态不可再次创建或恢复。注入 Gateway 的流程证据不代替真实京东验收。
- 执行前过期：预览在复核报价期间自然过期，以及预览 / 复核报价在 PostgreSQL 请求行锁等待期间自然过期，均应归档 FAILED_FINAL / INVALID_RESULT，零渠道创建、无链接、无租约且不可再执行 / 恢复。复核后拒绝尚未领取，尝试次数为零；锁等待后拒绝已经领取，尝试次数为一但不代表已调用渠道。
- 分享操作去重、公开文案不泄露收益、复制失败、未实现素材不可用。
- 分享事件：同用户 / eventId 12 路同输入重放同一 recordedAt，另一个事件 12 路不同动作竞争只胜出一组，其余冲突；变更链接 / 动作 / 场景均冲突，不同用户可用同一 eventId。真实 PostgreSQL 拒绝跨用户、所有非成功状态、控制字符，注入插入失败后零事件且可正常重试，复合外键拒绝伪造归属，000010 up/down/up 验证。HTTP 验证鉴权、键与 eventId 一致、大小 / JSON / 注入字段校验及错误脱敏，已验签运行时身份接线；重复上报不增加 Tracking / 请求或改变版本 / 尝试次数。不将客户端遥测当作真实复制、送达、点击或收益证据。
- 分享内容接口：鉴权与跨用户 / 错误记录 ID 隔离，所有非成功状态不得暴露链接；仅 link / text，重复或注入查询拒绝，qr / poster 返回未开放。响应字段白名单不含私有指纹、幂等键、深链及收益；危险链接和数据库错误安全脱敏。真实 PostgreSQL 合成历史成功验证本人重复读取复用原 Tracking，停用与预览过期不改写历史，零新增 Tracking / 请求且尝试次数 / 版本不变，不作为真实渠道成功或复制送达证据。
- 重复 / 乱序订单、匿名买家、无法归因、部分退款、订单与收益双状态、看板指标及时区一致。
  - 具体仓储覆盖、缺口与分层验收见 [M3-05 订单异常验收矩阵](./superpowers/plans/2026-09-19-order-anomaly-acceptance.md)。数据库用例 SKIP 不等于已执行；隔离数据库联测与真实渠道 / 财务验收分别记录。
- 收益重复入账、批次对账、统一钱包迁移、并发提现、重复支付回调、失败解冻及余额不足追偿。
- 首页到真实订单、结算、提现及退款的端到端证据；图稿、模拟接口、测试通过与生产闭环分别记录。
- M7-01a：`go test -count=1 ./internal/capability` 验证可信能力声明精确范围、各能力 / 类型独立、默认 / 待核验 / 停用 / 未知关闭、完整证据与有效时间、错误字段边界及安全投影。纯领域测试无数据库，不证明真实批准 / 调用引用真实性；无生产装配或READY写入口。目录 / 仓储迁移 / API / 前端与真实获批验收仍分别执行。
- M7-01b：`go test -count=1 ./internal/material` 验证商品 / 无价格活动、平台类型与地域 / 业务 / 终端、起止及源更新时间、未知 / 缺项 / 字段 / JSON时间边界、零卡片拒绝与安全字段投影、slice隔离。URL仅校验HTTPS语法，不执行网络 / 证明官方授权；真实源导入、数据库唯一约束及目录API仍独立验收，不因可展示而开放转链。
- M7-01c-1：同包查询测试验证首屏 / 后续游标、用户与五维筛选切换、limit和字段上限、UUID边界、未知 / 重复 / 重排 / 尾随JSON与非规范base64。游标仅分页位置，后续接口须可信身份解析，仓储须独立绑定本人范围；当前无SQL / HTTP装配，不构成数据库或API验收。
- M7-01c-2a：TestMaterialSchemaConstraintsAndDown在配置PG_TEST_DSN的隔离PostgreSQL真实执行000022 up、合法商品 / 活动及身份唯一性、26项CHECK边界与合法最大城市规则、down表 / helper清理。不把DDL或引用存在当来源授权，Record.Validate及只读仓储 / API仍独立验证。
- M7-01c-2b：能力隔离schema执行真实既有迁移及000023；字段 / 默认 / 精确唯一 / 本证据FK、Key不可变、证据UPDATE / DELETE / TRUNCATE CASCADE拒绝，停用保留及down清理。不是数据库角色 / DDL防绕过或真实审核验收。
- M7-01c-2c-1：TestRepositoryExactScopeAndEligibility / TestRepositoryFailsClosedOnInvalidInputAndStorage真实PG读取本人资格 / 位及精确能力，覆盖九维错配、空城市业务非通配、非默认淘宝活动范围、四种非启用会员状态、能力状态、证据起止与Unicode空白、取消 / nil / SQL失败 / 关闭脱敏；返回Decision白名单，零Tracking新增及声明 / 证据保留。不是真实负责人身份或媒体授权验证、最终生成事务检查、公开目录API或物料仓储验收。
- M7-01c-2c-2：真实完整迁移后执行5项TestCatalogRepository，验证能力先于目录查询（缺物料表仍可拒绝）、商品 / 京东淘宝美团无价格活动、安全投影、所有范围与日期过滤、非法URL / Unicode证据跨批跳过、limit改变与UUID分页真hasMore、换身份 / 范围游标拒绝、停用 / 到期、nil / 关闭 / SQL故障 / 取消零Page和零新增Tracking / 转链请求。后批故障通过STABLE只读SQL函数和实际数据view注入，验证前批成功后异常不返回部分页面；强制误返部分Page的mutation实测RED后恢复GREEN。TestCapabilityTransactionReaderLifecycle验证nil / 已结束tx及调用者生命周期，同事务桥接读取不提交调用者事务。完整并发快照与新迁移全链up / down属于c-3，真实授权与公开API仍另验收。
- M7-01c-3a：TestActualMigrationChainUpDownUp实际Run / Verify全部up、逆序执行真实down（每版仅测试schema事务同步删除精确账本行）、业务表 / 视图 / 序列及函数无残留、再次完整up及重复零应用。TestRunActualMigrationsConcurrentlyAndRepeatedly四路并发核对新物料 / 能力 / 证据表；动态版本数量，-race -count3复验。不修改生产迁移器，不提供生产down/reset，不证明生产数据回滚无损；目录并发快照仍另验收。
- M7-01c-3b：TestCatalogReadSnapshotAcrossCommittedChanges以实际物料relation锁 / pg_blocking_pids观察读取阻塞，分别提交能力、会员、位、未来新证据及六种物料变更；ID2域非法使limit1跨批读ID3。当前读取旧卡片 / 旧授权及真hasMore，新请求和旧游标续页读取已提交状态，材料 / 证据保留、零Tracking / 转链请求。定向-race -count3；降低隔离级别的mutation实际捕获混合快照并恢复。只读一致性不等于立即中止在途读取或生成事务锁，不替代真实渠道验收。
- M7-01d-1：TestCatalogDetail六项真实数据库测试验证详情安全字段、商品 / 三平台无价格活动、11种物料拒绝、缺失与跨平台类型同安全原因、本人资格 / 位 / 能力 / 证据到期拒绝、缺表证明先gate、UUID及范围 / kind / now校验、nil / SQL失败 / 关闭 / 取消零Detail。详情锁并发提交会员 / 标题 / 有效期变更，旧请求同快照，新请求最新判定；临时READ COMMITTED实测三case混合快照RED再恢复。只读元数据不等于转链权限，HTTP鉴权、可信配置、来源授权与生成最终检查仍另验收。
- M7-01d-2a：TestReadService三项实际PG测试验证配置复制后调用者更改MediaID不改变读取，真实分页 / 详情、撤销后旧游标和详情重新拒绝、跨用户不可读；错媒体不复用另一READY，四维配置分别错配及空配置在物料表缺失时仍安全拒绝。构造非法 / 重复配置或nil DB拒绝，读取非法owner / Scope / scene / position / cursor / limit / ID及取消均不返回数据，nil服务安全错误。配置字段齐全不代表批准，HTTP白名单 / 验证器与真实媒体仍独立验收。
- M7-01d-2b：TestPromoterMaterialRoutes四项HTTP契约测试实际运行路由与handler，读取接口替身提供白名单数据 / 安全域拒绝及错误。验证可信身份、Scope / 位 / scene、默认20与显式分页游标、安全JSON及no-store；缺服务或Users不注册、未登录 / 解析失败先401且零读取、POST拒绝、未知 / 解码后伪造owner / media / status / kind / now、重复 / 非法参数与Scope / 文本 / 分页、详情禁止cursor / limit。200拒绝不保留item，400 / 503错误data:null不泄漏原始错误或错误伴随的部分数据。仅HTTP边界，不代替真实验证器和PG仓储链路；后续d-3独立验收。
- M7-01d-3a：TestRuntimeMaterials三项真实应用联测，实际全部迁移 / 私有PG、临时RSA2048签名 / 真实Verifier、ReadService与Repository、本机httptest HTTP服务和http.Client读取，无身份 / reader替身。覆盖本人分页 / 详情、401未登录 / 错签名、跨owner拒绝和游标400、详情UUID400、注入拒绝 / 缺失安全原因、no-store及无内部字段、零Tracking / 转链请求和证据保留；九类授权与物料变更后新请求拒绝、非法 / 重复配置启动失败、空绑定或错媒体不能使用READY、真实缺表503脱敏。nil部署配置是合法关闭态；配置文件加载属3b，签名为测试密钥，不代表第三方身份提供方、来源真实性或真实业务验收。
