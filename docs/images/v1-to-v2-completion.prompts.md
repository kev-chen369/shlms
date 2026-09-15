# V1 未覆盖画面补绘为 V2：提示词

工具：内置 imagegen。每组输入原 V1 图作为内容参考、mobile-ui-overview-v2.png 作为风格参考。最终多平台与扩展电商采用修正版。

## 多平台综合导购

输出：`multi-platform-client-overview-v2.png`

```text
Use case: ui-mockup. NEW V2 redraw of previously uncovered V1 UI screens for Chinese app 惠省生活.
Input image 1 is the existing V1 board: CONTENT COVERAGE REFERENCE ONLY; preserve its main scenarios, not its old visual style or misleading prices/statuses. Input image 2 is V2: VISUAL STYLE REFERENCE. Match V2 warm white #F7F8F4, deep emerald #075E46 buttons/headings, mint panels, restrained orange price accents, readable Chinese sans-serif, fine line icons, generous page padding and restrained cards. No bulky phone hardware, no perspective, no mascots, coins, giant promotional slogans. Clear practical UI. All amounts are explicitly demonstration values. Tiny footer "V2 界面概念稿 · 演示数据 · 渠道以实际开放为准". Platform names are plain text badges without official logos.
For mobile boards render equal complete 390:844-proportion screens side-by-side, high-resolution landscape composition, readable text, small board heading and numbered captions only. Primary tab navigation when needed EXACTLY 首页 / 分类 / 省钱 / 订单 / 我的. Secondary pages use back button. Never use internal tracking jargon, or app-side checkout/payment, seat selection or booking form. External CTA names destination clearly. Estimated cost = coupon price minus estimated cashback; never label it actual payable price, never deduct coupon twice. Dates, product photos and financial figures are fictional demonstrations. Order cards distinguish pending estimated cashback and credited actual cashback.
Board: 惠省生活 · 多平台综合导购 V2
Draw exactly FOUR mobile screens.
01 多平台入口: title 惠省生活, city 上海, search "搜商品、外卖或电影优惠"; simple editorial mint hero "日常所需，一起省"; four equal platform shortcut tiles 京东 / 拼多多 / 美团 / 饿了么, smaller next row 淘票票 / 更多平台; 4 clear category buttons 电商购物 / 外卖红包 / 电影优惠 / 本地生活. Two cards with mug and food photography. Five-tab nav 首页 active.
02 综合搜索: search "周末省钱"; category tabs 全部 / 商品 / 外卖 / 电影; small filter 平台, order 综合推荐 (do not sort incomparable categories by one total price). Three different clearly typed results: 京东 无线降噪耳机 "券后 ¥199" "预计返 ¥12" "预计成本 ¥187"; 美团 外卖红包活动 "查看当期优惠" "领取条件以活动页为准"; 淘票票 周末电影优惠 "查看适用影院与场次", tiny "优惠活动" badge. Each row photo and view arrow. Footer "商品与活动分类展示".
03 外卖优惠对比: city 上海; two equal clean cards 美团 "外卖红包" and 饿了么 "外卖优惠"; rows 适用范围 本市参与商家 / 领取条件 以活动页为准 / 有效期 以领取页为准; respective primary actions "去美团领取" and "去饿了么领取"; lower small movie strip "淘票票 · 周末电影优惠" with 查看活动; clear "不同活动的使用门槛可能不同". No invented face values or sum of unrelated savings.
04 多平台返现订单: title 返现订单; channel filter 全部平台; scene tabs 全部 / 电商 / 外卖 / 电影; 3 cards: 京东 无线降噪耳机 status 已到账 实际返现 ¥12; 美团 外卖订单 status 待结算 预计返现 ¥2; 淘票票 电影订单 status 待确认 预计返现 ¥6; compact readable progress or update caption; link 申请查单; five-nav 订单 active. No logistics/payment controls.
```

## 扩展电商平台

输出：`expanded-ecommerce-platforms-v2.png`

```text
Use case: ui-mockup. NEW V2 redraw of previously uncovered V1 UI screens for Chinese app 惠省生活.
Input image 1 is the existing V1 board: CONTENT COVERAGE REFERENCE ONLY; preserve its main scenarios, not its old visual style or misleading prices/statuses. Input image 2 is V2: VISUAL STYLE REFERENCE. Match V2 warm white #F7F8F4, deep emerald #075E46 buttons/headings, mint panels, restrained orange price accents, readable Chinese sans-serif, fine line icons, generous page padding and restrained cards. No bulky phone hardware, no perspective, no mascots, coins, giant promotional slogans. Clear practical UI. All amounts are explicitly demonstration values. Tiny footer "V2 界面概念稿 · 演示数据 · 渠道以实际开放为准". Platform names are plain text badges without official logos.
For mobile boards render equal complete 390:844-proportion screens side-by-side, high-resolution landscape composition, readable text, small board heading and numbered captions only. Primary tab navigation when needed EXACTLY 首页 / 分类 / 省钱 / 订单 / 我的. Secondary pages use back button. Never use internal tracking jargon, or app-side checkout/payment, seat selection or booking form. External CTA names destination clearly. Estimated cost = coupon price minus estimated cashback; never label it actual payable price, never deduct coupon twice. Dates, product photos and financial figures are fictional demonstrations. Order cards distinguish pending estimated cashback and credited actual cashback.
Board: 惠省生活 · 扩展电商平台 V2
Draw exactly FOUR mobile screens.
01 电商频道: title 电商省钱, search 搜商品或粘贴链接, four elegant platform tiles 淘宝/天猫 / 拼多多 / 唯品会 / 苏宁易购; section categories 品牌好物 / 家电数码 / 日用精选 / 高返专区; two product photography cards generic cream air fryer and cleaning item, air fryer "券后 ¥259" "预计返 ¥20"; short bottom "更多渠道按实际开放展示". Five-nav 分类 active.
02 跨平台比价: search "空气炸锅 5L", heading "同款同规格比较", subtitle "型号 AF-5 · 奶油白 · 5L"; show four same-model matching cream air fryer cards, plain badges 淘宝/天猫 / 拼多多 / 唯品会 / 苏宁易购. EXACT values: 淘宝/天猫 券后259 预计返20 预计成本239; 拼多多 券后249 预计返10 预计成本239; 唯品会 券后269 预计返25 预计成本244; 苏宁易购 券后249 预计返22 预计成本227. Currency ¥ for all. Update 10:30, small "仅比较本次已查询渠道". No all-network lowest claim.
03 优惠详情: photo same cream air fryer, title 空气炸锅 AF-5 5L, plain 淘宝/天猫 badge; coupon price ¥259 prominently, original ¥399 strikethrough, coupon ¥140 tagged 已计入券后价; estimate panel 预计返现 ¥20 / 预计成本 ¥239. Other channels comparison rows 拼多多 ¥239 / 唯品会 ¥244 / 苏宁易购 ¥227 all under label 预计成本. Sticky primary "领券去淘宝", caption 将在合作平台完成购买.
04 电商返现订单: filters 全部 / 待结算 / 已到账 / 失效; four platform cards same model with dates: 淘宝/天猫 待结算 预计返现¥20; 拼多多 待确认 预计返现¥10; 唯品会 已到账 实际返现¥25; 苏宁易购 待结算 预计返现¥22. Each includes product thumbnail, platform badge and status text. Five-nav 订单 active. Keep consistent same-item comparisons, no misleading confirmed money.
```

## 本地生活与旅行

输出：`local-life-travel-platforms-v2.png`

```text
Use case: ui-mockup. NEW V2 redraw of previously uncovered V1 UI screens for Chinese app 惠省生活.
Input image 1 is the existing V1 board: CONTENT COVERAGE REFERENCE ONLY; preserve its main scenarios, not its old visual style or misleading prices/statuses. Input image 2 is V2: VISUAL STYLE REFERENCE. Match V2 warm white #F7F8F4, deep emerald #075E46 buttons/headings, mint panels, restrained orange price accents, readable Chinese sans-serif, fine line icons, generous page padding and restrained cards. No bulky phone hardware, no perspective, no mascots, coins, giant promotional slogans. Clear practical UI. All amounts are explicitly demonstration values. Tiny footer "V2 界面概念稿 · 演示数据 · 渠道以实际开放为准". Platform names are plain text badges without official logos.
For mobile boards render equal complete 390:844-proportion screens side-by-side, high-resolution landscape composition, readable text, small board heading and numbered captions only. Primary tab navigation when needed EXACTLY 首页 / 分类 / 省钱 / 订单 / 我的. Secondary pages use back button. Never use internal tracking jargon, or app-side checkout/payment, seat selection or booking form. External CTA names destination clearly. Estimated cost = coupon price minus estimated cashback; never label it actual payable price, never deduct coupon twice. Dates, product photos and financial figures are fictional demonstrations. Order cards distinguish pending estimated cashback and credited actual cashback.
Board: 惠省生活 · 本地生活与旅行 V2
Draw exactly FIVE mobile screens, wide high-resolution board.
01 生活旅行: city 上海, search 搜电影、酒店、出行优惠; scenic tasteful photo banner "周末出发，轻松一点"; platform entries 美团 / 饿了么 / 淘票票 / 携程 / 同程; categories 外卖 / 电影 / 酒店 / 机票火车票 / 景区 / 本地生活. Two modest photo activities "周末观影" and "城市短住". Five-nav 省钱 active. No fake guaranteed discounts.
02 电影优惠: back title 电影优惠, city 上海, plain 淘票票 badge; original fictional movie poster abstract landscape "周末放映", no famous real film/actor; section "当期观影活动", rules cards 适用影院与场次 以合作平台为准 / 票价与优惠 在合作平台确认 / 返现 以实际订单结算为准; action "去淘票票选场次"; "选座与购票将在淘票票完成". Do not show seat map or on-site ticket payment.
03 酒店比价: title 酒店优惠, 上海, date selector "09.20–09.21 · 1晚", occupancy "1间 · 2成人"; hotel "湖畔示例酒店" and same room type "高级大床房 · 含早餐 · 相同取消条件"; two comparison cards 携程 房费¥500 预计返¥20 预计成本¥480 and 同程 房费¥490 预计返¥15 预计成本¥475; price basis 1间1晚含税费; update10:30; simple button each 查看详情; caption 房态与退改条款以合作平台确认为准.
04 出行优惠: title 出行优惠; segmented 机票 / 火车票 / 景区 with 火车票 active; clean travel photo, plain 同程 badge; title "出行优惠活动"; fields 适用线路 查看活动 / 优惠条件 以领取页面为准 / 退改规则 以合作平台为准; main "去同程查看车次"; smaller link "查看携程出行优惠"; do not imply general ticket cashback; prices pending at partner. No station form checkout or promised bookings.
05 生活旅行订单: title 返现订单, scene filters 全部 / 外卖 / 电影 / 酒店 / 出行; four rows 美团 外卖订单 待结算 预计返¥2; 淘票票 电影订单 已到账 实际返¥6; 携程 湖畔示例酒店 待结算 预计返¥20; 同程 出行活动订单 待确认 预计返¥3; small thumbnails, clear channel and statuses; support 申请查单; five-nav 订单 active. Note 钱款退款与行程管理请前往合作平台. Treat money as demonstrative not guaranteed entitlement.
```

## 运营后台总览

输出：`admin-dashboard-overview-v2.png`

```text
Use case: ui-mockup. NEW V2 redraw of previously uncovered V1 UI screens for Chinese app 惠省生活.
Input image 1 is the existing V1 board: CONTENT COVERAGE REFERENCE ONLY; preserve its main scenarios, not its old visual style or misleading prices/statuses. Input image 2 is V2: VISUAL STYLE REFERENCE. Match V2 warm white #F7F8F4, deep emerald #075E46 buttons/headings, mint panels, restrained orange price accents, readable Chinese sans-serif, fine line icons, generous page padding and restrained cards. No bulky phone hardware, no perspective, no mascots, coins, giant promotional slogans. Clear practical UI. All amounts are explicitly demonstration values. Tiny footer "V2 界面概念稿 · 演示数据 · 渠道以实际开放为准". Platform names are plain text badges without official logos.
For mobile boards render equal complete 390:844-proportion screens side-by-side, high-resolution landscape composition, readable text, small board heading and numbered captions only. Primary tab navigation when needed EXACTLY 首页 / 分类 / 省钱 / 订单 / 我的. Secondary pages use back button. Never use internal tracking jargon, or app-side checkout/payment, seat selection or booking form. External CTA names destination clearly. Estimated cost = coupon price minus estimated cashback; never label it actual payable price, never deduct coupon twice. Dates, product photos and financial figures are fictional demonstrations. Order cards distinguish pending estimated cashback and credited actual cashback.
Board: 惠省生活 · 运营后台总览 V2
Draw ONE complete desktop admin dashboard screen, landscape 16:10 or 16:9, very high resolution crisp Chinese, no mobile frames. Transform V1 admin overview into restrained modern V2 dashboard with white sidebar and mint selected item; no decorative hero welcome banner, no browser URL or browser chrome.
Sidebar title 惠省生活 / 运营后台, navigation 经营总览 selected / 渠道管理 / 订单中心 / 佣金与返现 / 钱包与提现 / 对账中心 / 风险预警 / 客服工单 / 权限与审计.
Topbar title 经营总览; date range 2026.09.01–09.15, button 刷新, avatar 运营管理员, tiny 演示数据.
Top four KPI cards: 成交金额 ¥286,530; 有效订单 1,243; 已结算佣金 ¥28,650; 结算后毛利 ¥5,730. Below small unified note "结算后毛利 = 已结算佣金 − 用户返现 − 成本及推广奖励", sample values if shown 28,650−20,055−2,865=5,730. No incoherent margins exceeding commissions.
Middle grid: left compact chart 近7日有效订单 with emerald line and axes, beside it conversion funnel as clean horizontal bars "访问 10,000 / 推广点击 4,000 / 下单 1,600 / 有效订单 1,243" monotonically narrowing. Right 待办与风险 with 3 separate rows 待审提现 8 / 待处理对账差异 3 / 订单同步延迟 1, each 查看 action and appropriate badge.
Next section 渠道状态 table with columns 渠道 / 状态 / 最近同步 / 结算核对; rows 京东 正常 10:30 已核对; 拼多多 正常 10:29 已核对; 美团 延迟 10:05 待核对; 饿了么 正常 10:28 已核对. Use status icon + text not color alone.
Bottom 最新返现订单 table, 3 readable rows, columns 时间 / 订单号 / 渠道 / 商品或活动 / 实付金额 / 返现 / 状态 / 操作: 10:30 HS0915001 京东 保温杯 ¥79 预计¥5 待结算 查看; 10:25 HS0915002 拼多多 日用品 ¥26.50 预计¥2.80 待确认 查看; 10:20 HS0915003 京东 无线耳机 ¥199 实际¥12 已到账 查看. No direct edit balance or one-click payment buttons. Plain organized operational workspace, coherent figures, no rounded-card excess.
```

## 局部修正：multi-platform-client-overview-v2

```text
Use case: precise-object-edit. Input image is the full 4-screen 多平台综合导购 V2 board, EDIT TARGET. Keep layout, four screens, all amounts and other UI unchanged. Correct ONLY the progress timelines in screen 04 on the right: first 京东 earbuds order is 已到账 with 实际返现¥12, so ALL three milestones must be GREEN CHECKS connected by green lines; labels 已下单 / 待结算 / 已到账, final completion date 05-22. Second 美团 order is 待结算, show first completed green check, second active outlined green circle, third light-gray pending hollow circle (not an X), labels 已下单 / 待结算 / 已到账; omit estimated future dates. Third 淘票票 order is 待确认, show first green check, second active outlined green circle, third gray pending hollow circle, labels 已下单 / 待确认 / 待结算; no predicted future dates. Preserve status badges 已到账/待结算/待确认 and actual/estimated cashback. Return full four-screen board without cropping, all other screens unchanged.
```

## 局部修正：expanded-ecommerce-platforms-v2

```text
Use case: text-localization. Edit the full four-screen 扩展电商平台 V2 image. Change ONLY one wrong caption: in SECOND screen 跨平台比价, FIRST product row 淘宝/天猫, right-hand mint price box above ¥239 currently says 预计返现. Replace it with EXACT Chinese text 预计成本. Keep ¥239 and separate orange 预计返现 ¥20 intact. Preserve every other pixel-like detail as closely as possible, all four screens, all other text, amounts, layout, colors and composition. Output full corrected board, no crop.
```

