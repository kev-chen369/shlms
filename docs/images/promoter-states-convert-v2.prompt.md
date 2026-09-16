# 推广转链状态稿生成记录

任务：M0-05b。2026-09-16。使用内置 imagegen，以 `promoter-convert-share-v2.png` 为视觉参考，分别生成四屏预览/处理状态稿和三屏结果/恢复状态稿。最终图片：[转链与预览状态](./promoter-states-convert-v2.png)、[结果与恢复状态](./promoter-states-result-v2.png)。两张图仅为设计概念，商品、时间及渠道信息均为演示。

第一组提示词（视觉参考，不编辑参考图）：

> Use case: ui-mockup. Asset type: new four-screen V2 mobile UI state sheet for 惠省生活 promotion center. Match the existing four-column landscape board, white rounded phone screens, deep emerald action color, mint panels and restrained orange warnings. Create exactly four new screens: P-05 京东渠道待配置—“当前推广位暂不可生成链接”, disabled “生成推广链接”, “查看推广位”; P-06 预览已过期—“请重新获取商品信息”, “重新预览”, disabled create; P-06 价格或规则已变化—show abstract before/after placeholders, “请确认最新信息后再生成”, “刷新预览”, disabled create; P-07 正在确认链接—“请稍后查看处理结果”, “处理中”, “刷新状态”, no copy link or URL. No fabricated amounts, live channel promises, third-party logos or watermark.

第二组提示词：

> Use case: ui-mockup. Asset type: new three-screen V2 mobile UI state sheet for 惠省生活 promotion center. Same visual system as reference. Create exactly three screens: P-07 结果待核验—“渠道结果尚未确认”, “请刷新状态，勿重复提交”, “待查询”, “刷新状态”, no new Create or copy action; P-07 生成失败—“未生成可分享链接”, “渠道明确拒绝本次请求”, “重新预览”, “返回转链”, new request required; P-07 链接已过期—“请重新预览商品并生成新链接”, “重新预览”, “查看历史记录”. No fabricated URL, no active copy link, no watermark.

定向修正：第二组原稿在过期状态中把站内展示有效期误写成外部链接失效，使用内置 imagegen 仅修改第 3 屏提示为“展示有效期已结束”“外部链接状态以渠道为准”，其余两屏和布局保持不变。外部旧链接能否停用需以渠道能力与正式产品规则为准。

验收边界：渠道待配置、预览过期与改价均不可直接生成；PENDING / PROCESSING 只能刷新原请求状态；结果不确定不得盲目重发；确定失败后重新预览并用新请求；页面不展示个人预计收益作为已到账资金。
