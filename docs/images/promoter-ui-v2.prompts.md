# 推广中心用户端 V2 生成提示词

工具：内置 imagegen。风格参考：mobile-ui-overview-v2.png。四组新生成，钱包组采用一次局部修正后的版本。

## 开通、工作台与推广位

输出：`promoter-onboarding-positions-v2.png`

```text
Use case: ui-mockup. Create a NEW high fidelity 4-screen mobile UI board for 惠省生活「推广中心」, a promoter-facing affiliate marketing workspace, not a consumer cashback storefront. Input image 1 is ONLY the existing consumer V2 style reference. Match warm white #F7F8F4, deep emerald #075E46, mint panels, restrained burnt orange money accents, sharp readable simplified Chinese, generous padding and fine-line icons. Reduce decorative illustrations, prioritize useful controls and data. Equal complete 390x844 phone screens in one high-resolution landscape board, no phone hardware, aligned, small module title and numbered labels, tiny footer "推广中心 · 界面概念稿 · 演示数据". Main tabs when specified EXACTLY 工作台 / 活动 / 订单 / 我的. Secondary pages have back arrows and no tab bar. All demonstration. Only 京东 currently enabled; do not show unavailable platforms as enabled. Use plain text platform labels not official logos. No guaranteed income, automatic mass sharing, customer contact data, raw Tracking ID or provider IDs, cartoon money or team hierarchy. One primary action per screen. Promoter money label "我的预计收益" or "我的实际收益", NEVER display the full channel commission as the user's take. User-facing "推广位" means a named placement such as 通勤好物群, not an actual connected group. Share action means explicit user copy/share, not delivery guaranteed. All text readable.
Board title: 惠省生活 · 开通、工作台与推广位
Exactly FOUR screens.
01 申请开通: brand 惠省生活 small, title "开通推广中心", short "生成专属链接，查看推广订单与收益"; no income slogan. Three short capability rows 推广位管理 / 商品转链 / 订单与收益. Form 推广名称 placeholder 请输入推广名称; 推广场景 chips 微信群 selected / 朋友圈 / 内容账号; explanation 不收集群成员信息. Unchecked checkbox "同意推广服务协议"; primary 提交申请; small 审核通过后可使用. Minimal entry from consumer 我的.
02 推广工作台: title 推广中心, text link 返回省钱首页, badge 已开通. Filter 近7天 / 全部推广位. Summary cards 转链次数36, 分享操作80, 有效订单18, 我的预计收益¥18.00; small "仅本期订单预估，未结算不可提现". Strong compact emerald action "粘贴链接，开始转链". Three shortcuts 推广位 / 我的素材 / 结算记录. Section 最近转链 two rows beige mug "轻量保温杯 500ml" 京东 · 通勤好物群 · 有效 / 复制链接 and headphones 京东 · 朋友圈精选 · 有效 / 查看. All data modest, no fake graph. Tabs 工作台 active.
03 推广位管理: title 推广位, primary top 新建; search 推广位名称; three cards 通勤好物群 默认 · 已启用 / 场景 微信群 / 京东 可用 / actions 编辑,停用; 朋友圈精选 已启用 / 场景 朋友圈 / 京东 可用 / 设为默认; 旧活动 已停用 / 历史记录可查看 / 查看. Notice "停用后不能新建推广链接，历史订单仍保留". No delete button, no secret identifiers, no customer names.
04 新建推广位: title 新建推广位; name input 通勤好物群; scene select 微信群; tiny 命名用于区分推广来源，无需连接微信群. Switch 设为默认推广位 off. Channel status panel 京东 待配置 / 其他渠道 暂未开放; explanation "渠道配置完成后才可转链". Sticky 保存推广位. The screen is a pending NEW placement, no false instant enabled.
```

## 万能转链与分享

输出：`promoter-convert-share-v2.png`

```text
Use case: ui-mockup. Create a NEW high fidelity 4-screen mobile UI board for 惠省生活「推广中心」, a promoter-facing affiliate marketing workspace, not a consumer cashback storefront. Input image 1 is ONLY the existing consumer V2 style reference. Match warm white #F7F8F4, deep emerald #075E46, mint panels, restrained burnt orange money accents, sharp readable simplified Chinese, generous padding and fine-line icons. Reduce decorative illustrations, prioritize useful controls and data. Equal complete 390x844 phone screens in one high-resolution landscape board, no phone hardware, aligned, small module title and numbered labels, tiny footer "推广中心 · 界面概念稿 · 演示数据". Main tabs when specified EXACTLY 工作台 / 活动 / 订单 / 我的. Secondary pages have back arrows and no tab bar. All demonstration. Only 京东 currently enabled; do not show unavailable platforms as enabled. Use plain text platform labels not official logos. No guaranteed income, automatic mass sharing, customer contact data, raw Tracking ID or provider IDs, cartoon money or team hierarchy. One primary action per screen. Promoter money label "我的预计收益" or "我的实际收益", NEVER display the full channel commission as the user's take. User-facing "推广位" means a named placement such as 通勤好物群, not an actual connected group. Share action means explicit user copy/share, not delivery guaranteed. All text readable.
Board title: 惠省生活 · 万能转链与分享
Exactly FOUR screens forming one journey.
01 万能转链: back title 万能转链; textarea with clipboard icon and placeholder 粘贴商品链接或包含链接的文案; small 目前支持京东商品链接; button 粘贴; selection 推广位 通勤好物群 · 默认 chevron; 分享场景 微信群 selected / 朋友圈; primary 识别链接; secondary 最近转链. No clipboard reads without tapping.
02 确认商品: mug photo nice neutral, text 京东 / 轻量保温杯 500ml; pricing 券后价¥79; prominent 我的预计收益¥1.00; small 消费者预计返现¥5.00 shown distinctly below and "预计金额以渠道实际结算为准". Placement row 推广位 通勤好物群 with 更换. source 09.16 10:30 更新. Primary 生成推广链接; small 将保存本次推广位与收益规则. No gross commission.
03 转链成功: modest green check title 推广链接已生成; compact mug card price¥79 promoter expected¥1.00; details 推广位 通勤好物群 / 分享场景 微信群 / 有效期 09.17 12:00; readable link field "https://example.com/p/demo-001" clearly small 示例链接; primary 复制推广链接; secondary text 查看分享文案; lower link 继续转链. No fake QR. Expired status not current.
04 分享文案: segmented 链接 / 文案 active / 海报 / 二维码, latter two disabled with 未开放; editable-looking white preview card "轻量保温杯 500ml" then "券后价 ¥79，优惠以京东页面为准。" then "查看优惠：https://example.com/p/demo-001"; small "对外文案不展示你的推广收益"; concise line 推广位 通勤好物群; primary 复制文案; secondary 复制链接; muted "复制不代表已发送，请自行选择分享对象". Do not show internal tracking or promoter profit in consumer copy.
```

## 活动、推广订单与数据

输出：`promoter-activities-orders-v2.png`

```text
Use case: ui-mockup. Create a NEW high fidelity 4-screen mobile UI board for 惠省生活「推广中心」, a promoter-facing affiliate marketing workspace, not a consumer cashback storefront. Input image 1 is ONLY the existing consumer V2 style reference. Match warm white #F7F8F4, deep emerald #075E46, mint panels, restrained burnt orange money accents, sharp readable simplified Chinese, generous padding and fine-line icons. Reduce decorative illustrations, prioritize useful controls and data. Equal complete 390x844 phone screens in one high-resolution landscape board, no phone hardware, aligned, small module title and numbered labels, tiny footer "推广中心 · 界面概念稿 · 演示数据". Main tabs when specified EXACTLY 工作台 / 活动 / 订单 / 我的. Secondary pages have back arrows and no tab bar. All demonstration. Only 京东 currently enabled; do not show unavailable platforms as enabled. Use plain text platform labels not official logos. No guaranteed income, automatic mass sharing, customer contact data, raw Tracking ID or provider IDs, cartoon money or team hierarchy. One primary action per screen. Promoter money label "我的预计收益" or "我的实际收益", NEVER display the full channel commission as the user's take. User-facing "推广位" means a named placement such as 通勤好物群, not an actual connected group. Share action means explicit user copy/share, not delivery guaranteed. All text readable.
Board title: 惠省生活 · 活动、推广订单与数据
Exactly FOUR screens.
01 推广活动: title 推广活动, JD text chip 京东; city-independent ecommerce categories 全部 / 家居 / 数码 / 食品; two activity cards with tasteful product photos 秋日家居好物 and 数码精选; metadata 活动有效期09.30 / 收益按实际订单结算; each 查看活动. Selected activity compact bottom selection 推广位 通勤好物群 and primary 生成活动链接. Main tabs 活动 active. No daily guaranteed returns.
02 推广订单: title 推广订单; filters 近7天, 全部推广位, 京东. Status tabs 全部 / 待确认 / 待结算 / 已入账. Card1 mug 通勤好物群 / 订单状态 已完成 / 收益状态 待结算 / 实付¥79 / 我的预计收益¥1.00. Card2 headphones 朋友圈精选 / 订单状态 已完成 / 收益状态 已入账 / 实付¥199 / 我的实际收益¥2.00. Card3 tissue 通勤好物群 / 订单状态 已退款 / 收益状态 已失效 / 当前收益¥0.00. Small 查询不到订单？申请查单. Main tabs 订单 active.
03 订单收益详情: back title 订单收益详情; mint panel 收益待结算 / 我的预计收益¥1.00 / 渠道结算后才可入账. Mug detail 实付¥79; separate summary 订单状态 已完成 / 收益状态 待结算; progress 已归属,待结算 active,已入账 gray. Rows 推广位 通勤好物群 / 分享场景 微信群 / 下单时间09.16 10:30 / 订单号 ****4821. Expandable 收益说明: 本单消费者预计返现¥5.00 (explanatory, not promoter payout), "你的收益以本单规则及实际结算为准"; footer 联系客服. No total channel commission or platform margin.
04 推广数据: title 推广数据; date 近7天, 京东, 通勤好物群; cards 转链次数36 / 分享操作80 / 有效订单18 / 我的预计收益¥18.00; chart title 有效订单趋势 values 1,2,3,2,4,3,3 (sum18), clean line; section 数据口径 "分享操作统计复制与分享按钮操作，不代表送达" and "点击数据 — 暂无可验证数据"; avoid fake conversion rate if click count missing. Bottom link 查看推广订单. No made-up share recipient counts.
```

## 我的推广、结算与提现

输出：`promoter-wallet-settlement-v2.png`

```text
Use case: ui-mockup. Create a NEW high fidelity 4-screen mobile UI board for 惠省生活「推广中心」, a promoter-facing affiliate marketing workspace, not a consumer cashback storefront. Input image 1 is ONLY the existing consumer V2 style reference. Match warm white #F7F8F4, deep emerald #075E46, mint panels, restrained burnt orange money accents, sharp readable simplified Chinese, generous padding and fine-line icons. Reduce decorative illustrations, prioritize useful controls and data. Equal complete 390x844 phone screens in one high-resolution landscape board, no phone hardware, aligned, small module title and numbered labels, tiny footer "推广中心 · 界面概念稿 · 演示数据". Main tabs when specified EXACTLY 工作台 / 活动 / 订单 / 我的. Secondary pages have back arrows and no tab bar. All demonstration. Only 京东 currently enabled; do not show unavailable platforms as enabled. Use plain text platform labels not official logos. No guaranteed income, automatic mass sharing, customer contact data, raw Tracking ID or provider IDs, cartoon money or team hierarchy. One primary action per screen. Promoter money label "我的预计收益" or "我的实际收益", NEVER display the full channel commission as the user's take. User-facing "推广位" means a named placement such as 通勤好物群, not an actual connected group. Share action means explicit user copy/share, not delivery guaranteed. All text readable.
Board title: 惠省生活 · 我的推广、结算与提现
Exactly FOUR screens.
01 我的推广: small user nickname 小惠 / 推广员 已开通, link 切回消费者版. Emerald balance card "统一钱包可提现" ¥128.60 with 去提现; small "包含已入账购物返现与推广收益". Beneath TWO distinct cells "推广预计收益 ¥32.00" and "推广待结算 ¥248.60"; info "预计与待结算不可提现". Section links 推广资金明细 / 结算记录 / 到账账户 / 帮助与客服. Footer "提现使用统一钱包，不重复入账". Main tabs 我的 active.
02 结算记录: title 推广结算记录; channel 京东, month 2026年9月; summary 本月推广收益已入账¥18.00. Card1 批次 JS0915 / 已入账 / 推广收益¥12.00 / 结算订单12笔 / 入账时间09.15; Card2 批次 JS0910 / 已入账 / 推广收益¥6.00 / 结算订单6笔. Separate muted strip 待结算预计¥248.60 not in credited sum. Every batch 查看明细 action; note "结算入账不等于已提现". No channel gross commission presented as earnings.
03 提现申请: title 提现申请; banner 统一钱包可提现¥128.60; source hint 购物返现与推广收益统一提现. Amount input ¥100.00; verified account 本人银行卡 尾号1234; calculation 手续费¥0.00 示例 / 预计到账¥100.00; primary 确认申请; clear note "提交后冻结¥100.00，剩余可提现¥28.60". No unapproved minimum amount or instant arrival promise.
04 提现记录: title 提现记录; filters 全部 / 处理中 / 已完成; three example rows distinct transactions: 09.16 处理中 ¥100.00 / 金额已冻结 / 查看详情; 09.10 已到账 ¥50.00 / 银行卡尾号1234 / 查看详情; 09.08 提现失败 ¥20.00 / 金额已解冻 / 查看原因. Top explanatory 不同日期独立记录. Small help link 提现遇到问题？联系客服. No claim current100 failed or already arrived. Clean ledger readability.
```

## 钱包组局部修正

```text
Use case: precise-object-edit / text-localization. Input is the EDIT TARGET full 4-screen 惠省生活 我的推广、结算与提现 board. Preserve the 4 screens, palette, hierarchy, data and all amounts exactly. Make ONLY three small changes:
1. On screen 01 bottom mint banner replace engineering-sounding "提现使用统一钱包，不重复入账" with user-facing "购物返现与推广收益，一处查看". Replace its smaller subtitle with "明细分开记录，余额统一提现". Keep banner layout.
2. On screen 03 remove the four preset amount chips ¥50 / ¥100 / ¥200 / ¥500 below the amount input; leave modest whitespace in that strip. Keep the main ¥100.00 input and all other rows, available128.60 and remaining28.60.
3. On screen 04 at bottom remove the unconfirmed customer service hours "工作时间：9:00 - 21:00", keep the contact-customer-service link. Do not invent new service commitments.
Return the complete board, no cropping. All other contents unchanged.
```

