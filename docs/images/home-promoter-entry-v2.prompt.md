# 首页推广中心入口设计

![首页推广中心入口](./home-promoter-entry-v2.png)

页面顺序：品牌与位置 → 搜索 → 渠道快捷入口 → AI 帮我选/推广赚钱并排卡片 → 今日推荐 → 外卖红包 → 原消费者五栏导航。

推广赚钱卡片位于首屏渠道入口下方、推荐商品上方，点击卡片任意非独立操作区域进入推广中心。未登录先登录并保留返回目标；未开通进入申请；审核中进入进度页；已开通进入推广工作台；已停用展示状态说明和客服入口。我的页面保留推广中心入口。

图片为概念稿，数据为演示；使用内置 imagegen 依据现有 V2 首页重绘。

## 完整提示词

```text
Use case: ui-mockup. Input image 1 is the existing 5-screen 惠省生活 V2 consumer app board. Redesign and extract ONLY its HOME SCREEN as one full single portrait screen, adding a highly clear promoter-center entry in the correct hierarchy. Output ONE complete high-resolution flat mobile app screen, logical390x844 proportions; no surrounding slide, no phone hardware, no extra screens, no external annotations. Match V2 deep emerald #075E46, warm white #F7F8F4, mint #E7F3EB, charcoal body text, restrained orange prices, readable simplified Chinese sans-serif, generous20px page gutters, 14px card corners, refined compact useful density. No large advertising hero.
Exact screen content top-to-bottom:
1 native status bar 9:41.
2 header small brand 惠省生活 bold dark emerald, location 上海 with small pin and down caret, right notification bell.
3 broad rounded search field with magnifying glass, placeholder "搜商品，或粘贴商品链接".
4 FOUR equal compact channel shortcuts in ONE row 京东 / 拼多多 / 美团 / 饿了么. Use simple original small line icons in rounded tinted tiles, platform text underneath, no official logos.
5 Immediately BELOW channel shortcuts and ABOVE products: TWO same-width cards side-by-side, height about110 logical pixels. LEFT pale mint card has subtle sparkle icon, bold title EXACT "AI 帮我选", one short caption "说出需求，帮你比价", bottom tiny arrow text "去试试 →". RIGHT pale peach card (#FFF0E4) uses deep emerald shared-arrow line icon, bold title EXACT "推广赚钱", short caption "分享好物，查看收益", bottom tiny arrow text "进入推广中心 →". Right card title dark emerald, no coin piles, no guaranteed profit or reward figures, no income dashboard. This two-card row must be fully visible in first viewport, spacious and immediately legible; it is the primary change.
6 heading "今天值得买" left, "查看更多 ›" right. TWO photo product cards side by side: beige insulated travel mug and white wireless earbuds. Keep realistic softly lit photography similar to reference. Mug title "轻量保温杯 500ml", small 京东 text badge, price "券后 ¥79", separate muted orange small "预计返 ¥5". Earbuds title "无线降噪耳机", small 京东 badge, price "券后 ¥199", separate "预计返 ¥12". Keep product photography enough area but do not overwhelm shortcut row.
7 compact mint horizontal strip "外卖红包" / "先领券，再下单", right small tasteful bowl-of-food thumbnail, chevron.
8 fixed bottom native tab bar EXACT FIVE equal tabs "首页 / 分类 / 省钱 / 订单 / 我的", 首页 active emerald, other icons gray outline, clear labels. Bottom home indicator.
Constraints: remove old oversized AI promotional hero entirely. Header→search→channels→two useful shortcut cards→recommendations is exact required order. All labels clean and readable, adequate touch areas, no duplicate navigation, no other text, no on-screen technical remarks, no numbered diagram. Deliver production-quality HOME UI screenshot.
```

