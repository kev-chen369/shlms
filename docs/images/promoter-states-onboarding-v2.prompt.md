# 推广身份与推广位状态稿生成记录

任务：M0-05a。2026-09-16。使用内置 imagegen，以 `promoter-onboarding-positions-v2.png` 为视觉参考生成四屏新稿；随后仅将第 4 屏京东待配置行中误生成的红白吉祥物图标改为通用灰绿标签图标，未改变其他页面结构。最终稿为 [promoter-states-onboarding-v2.png](./promoter-states-onboarding-v2.png)。原始生成文件保留在 Codex 生成目录，仓库仅存修正后的项目稿。

最终生成提示词（样式参考稿，不修改参考图）：

> Use case: ui-mockup. Asset type: four-panel high-fidelity mobile UI state sheet for the 惠省生活 promotion center design documentation. Input image 1: style reference only. Match its clean white mobile app cards, deep emerald green primary buttons, soft mint highlights, thin cool-gray dividers, typography hierarchy, generous spacing, and four equal phone-screen columns. Create an entirely new sheet. Exactly four portrait screens: P-01 申请审核中 with 审核中、审核结果以站内通知为准、查看申请进度、联系客服 and no conversion; P-01 审核未通过 with 请完善推广场景说明、重新申请、查看原因; P-02 推广资格已停用 with 暂不可新建推广链接、查看历史订单与收益、联系客服; P-03 暂无推广位 with 还没有推广位、创建后仍需等待渠道配置、新建推广位、京东 · 待配置. Practical product UI, no earnings amounts, no approval promises, no live channel availability, no watermark.

定向修正提示词：

> Use case: precise-object-edit. Edit only the small red-and-white mascot/logo icon in the 京东 · 待配置 row of the fourth phone screen. Replace it with a generic muted gray-green outline channel-tag symbol. Preserve all other text, screens, layout, typography, colors and dimensions; no third-party logo or mascot.

验收说明：四种状态与 P-01/P-02/P-03 编号相符；待审、拒绝、停用均无可用转链按钮；停用保留历史入口；无推广位引导创建且明确渠道仍待配置。图中申请时间、图标和状态仅为演示，前端实现以真实接口为准。
