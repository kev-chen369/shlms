# 京东渠道适配设计

## 商品推广接入记录（2026-09-17）

目标接口：`jd.union.open.promotion.common.get`，用于通过商品 / 活动链接生成网站或 APP 的推广链接。官方入口：

- [接口文档](https://union.jd.com/openplatform/api/v2?apiName=jd.union.open.promotion.common.get)
- [账号 API 管理](https://union.jd.com/myTools/myApi)
- [京东开放平台联盟接入说明](https://jos.jd.com/jdunion)
- [联盟新版 API 接入及签名说明](https://union.jd.com/searchResultDetail?articleId=108188)

本项目优先实现商品推广。商品 / 活动转链、商品详情与佣金查询属于不同能力；生成链接成功不能作为商品报价、订单归因或返现结算已经完成的证据。

### 账号和凭据记录

用户已提供京东联盟登录账号与一项标记为 API Key 的凭据。账号索引为 `130****3946`，完整账号、登录密码、API Key 均不写入本文、代码、测试数据或 Git 历史。登录密码仅用于京东官方登录，不作为接口调用参数。

2026-09-17 已在登录后的控制台核对：用户提供的值是「我的 API」中的有效第三方查询授权 Key，控制台说明它供第三方查询明细数据，默认 365 天有效；领取新 Key 会使旧 Key 失效。它不是此转链接口签名所需的 AppKey/AppSecret，也不应发送给 `api.jd.com` 当作这两种字段。无需因本次开发领取新授权 Key。

「开放平台 → 应用管理」显示尚未创建应用；「网站管理」「APP 管理」「流量媒体管理」均未查询到已备案媒体，因此当前无可用 AppKey/AppSecret，也无该接口所需的合规 `siteId`。功能接入页虽显示 `jd.union.open.promotion.common.get` 属于基础权限、状态「已开通」，仍需创建应用并使用对应的网站 / APP 媒体配置才可实际调用。用户没有指定备案网站或 APP，本次不创建媒体、应用或推广位；真实权限和场景 2 的单品能力未验收。

仓库 `.gitignore` 排除本地 `.env`、`*.env`、`.secrets/`、`secrets/` 和私钥文件，仅保留脱敏的 `.env.example`。忽略规则不是密钥管理器，发布前仍检查暂存区；浏览器 Cookie、登录会话、完整请求签名和原始渠道错误体也不得进入接口文档。当前未创建本地凭据文件、未把任何真实凭据提交到 Git。

### 当前实现与调用边界

对应任务：推广中心计划 `M2-04a-1`。代码位于 `internal/channel/jd`。

| 输入 / 设置 | 当前处理 |
| --- | --- |
| `channel.PromotionRequest.ExternalProductID` | 接受 1～32 位、首位非零的十进制商品编号，或 `https://item.jd.com/{sku}.html`；32 位是本项目输入上限，不代表渠道公布的长度限制 |
| `ClientRequest.MaterialID` | 将支持的输入规范化为京东商品 HTTPS 地址，供后续真实 HTTP 客户端使用 |
| `ClientRequest.ExternalProductID` | 保留调用方原输入，兼容既有内部适配边界；商品编号始终为字符串，避免大整数精度损失 |
| `Config.PositionID` | 由服务端注入，缺失时拒绝调用；非空只代表本地配置存在，不代表京东已核验该推广位 |
| `Config.SubUnionEnabled` | 默认关闭；只在获批权限后启用，此时要求内部 Tracking ID 非空并映射到内部 `SubUnionID` 字段 |
| `Client` | 必须提供实现；缺失时返回未配置错误。`HTTPClient` 已实现但生产路由仍未装配 |

当前不接受短链、活动链接、含链接文案、券地址、携带查询串 / 片段的商品链接、非 HTTPS 链接或相似域名。此处不执行网页抓取，也不绕过已有安全展开器。后续扩展输入范围需单独核对渠道规则并补充测试。

适配器在调用前校验商品、推广位配置、子标识所需 Tracking 和上下文取消状态。普通错误输出使用固定提示，不拼接渠道原始错误内容；内部 `errors.Is/As` 仍可用于错误分类，不应把解包后的原始错误直接写日志。

### 正式协议与当前实现（M2-04a-2）

已核对上述官方 [接口详情](https://union.jd.com/openplatform/api/v2?apiName=jd.union.open.promotion.common.get) 与 [新版接入说明](https://union.jd.com/searchResultDetail?articleId=108188)：

| 内容 | 京东文档与本项目处理 |
| --- | --- |
| 网关 | `https://api.jd.com/routerjson`；正式请求按文档构造 UTF-8 编码的 HTTPS GET；旧 `router.jd.com` 已停止维护 |
| 系统参数 | `method=jd.union.open.promotion.common.get`、`app_key`、北京时间 `yyyy-MM-dd HH:mm:ss`、`format=json`、`v=1.0`、`sign_method=md5`、`sign`；若接口属性要求授权才传 `access_token`，本客户端未配置授权流，不自行填入第三方授权 Key |
| 业务参数 | `360buy_param_json` 为 JSON 字符串，内部为 `promotionCodeReq`；`materialId` 和 `siteId` 必填；`positionId` 可选且为数值；`sceneId` 必填。本实现先只做京东主站单品场景 `sceneId=2`，且要求明确配置已获批；未获批则创建客户端失败 |
| 媒体限制 | 官方要求 `siteId` 为备案网站 ID / APP ID / 合规流量媒体 ID，同时特别指出该网站 / APP 接口不得填导购媒体 ID，投放来源与备案不一致会使订单无效；本次将网站或 APP 的正整数 ID 由服务端配置注入，不接受前端指定 |
| 单品场景 | 文档说明 `sceneId=2` 支持京东主站商品 ID / 商品链接，需另申请权限；其他场景只支持联盟商品 ID / 联盟链接。因此不能把已构造的 `item.jd.com` 链接以默认场景 1 悄悄提交 |
| 子渠道 | `subUnionId` 可选，需向京东申请权限，限字母 / 数字 / `_` / `-` 共 1～80 字符；只有配置确认获批才加入请求。未实现 `pid`、`ext1`、优惠券二合一、短口令及活动页 |
| 签名 | 将除 `sign` 外所有系统与业务参数按参数名升序，原始值按 `key+value` 拼接，首尾各加 AppSecret，计算 MD5 并大写；AppSecret 本身不发给网关。请求禁用跨域重定向，5 秒超时，不记录签名 URL / 原始响应 |
| 返回 | 读取 `jd_union_open_promotion_common_get_responce.getResult.code=200` 与 `data.clickURL`；兼容文档对象形式和 JSON 字符串内层；拒绝错误响应、非 HTTPS 与非 `union-click.jd.com` 域名。官方示例含 HTTP URL，本项目为安全起见对其保留待核验状态，不升级成 HTTPS 或冒充成功 |

此接口详情未确认稳定外部请求号或按号查询能力。超时、网络失败、非 200 HTTP、无法解析结果及不安全的链接均归类为结果未知且绝不自动重发；业务明确拒绝单独归类。现有 `conversion.Executor` 的查询恢复边界不能直接装配这个无查询能力的网关，生产转链状态机仍保持关闭。普通错误不包含账号、AppSecret、签名、完整渠道响应。

`cmd/api` 目前没有装配京东 HTTP 客户端，本次适配准备不开放商品转链成功路径，也不将渠道位设为 READY。已有推广预览 / 转链状态链路继续依赖真实商品报价、渠道位核验和生产执行器装配。面向用户的推广页面在其他研发分支，本次没有擅自合并分支。

### 验证与验收

- 本地测试：商品大编号、URL 规范化、相似域名 / 凭据 / 端口 / 非 HTTPS / 编码路径 / 查询串 / 片段拒绝；缺少依赖、缺少 Tracking、取消请求均零渠道调用；子标识默认不发送；错误消息脱敏。
- 客户端契约测试：固定北京时间、规范 JSON 与签名校验、`sceneId=2` 权限开关、缺 siteId / 不合规子标识零网络调用、明确拒绝与未知结果、HTTPS 主机校验、不跟随签名请求重定向、无自动重试。使用本地 TLS 测试服务器，不向京东发送生产请求。
- 真实验收：仍未执行。需要创建对应备案网站 / APP 与开放平台应用，取得其 AppKey/AppSecret、确认场景 2 权限及有效商品，再记录真实成功响应的脱敏证据和人工打开结果。
- 本次任务状态、实际验证命令和结果见[研发计划](./superpowers/plans/2026-09-16-promotion-center.md)。测试替身成功不代表京东生产转链成功。

## 订单与佣金范围

商品查询、详情与券、推广转链、订单和佣金增量同步均通过实际获批的联盟能力实现；接口名称及 `subunionid` 权限以账户控制台为准。

创建内部 `tracking_id`，在获批范围内映射渠道子标识，同时保存 PID/推广位、用户、商品、来源和时间。不能回传子标识时，仅可用独立推广位与时间窗辅助匹配，标为低置信度并复核。

按更新时间滑动窗口增量拉取，窗口重叠以容忍延迟；先保存原始订单，再标准化。游标和水位持久化，重复数据由唯一键吸收。

验收覆盖凭据、搜索、转链、测试订单归因、退款/取消、佣金调整、结算和重放。没有生产账户与真实结算证据时只能标记 `SANDBOX_READY`。
