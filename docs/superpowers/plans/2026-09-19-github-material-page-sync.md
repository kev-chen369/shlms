# PROC-17：选品页面代码、文档与图同步

日期：2026-09-19。用户要求更新线上代码、文档与图。

- [x] PROC-17a【已完成】2026-09-19：复验三个已提交任务 M7-01e-3a、M7-01e-3b-1、M7-01e-3b-2，更新同步说明 / 文档与图索引并归档直接查看过的实际 390px 页面图。`npm test` 27文件485项、`npm run typecheck`、`npm run build:h5`、`npm run build:mp-weixin`、`npm run test:layout` 7项及两项选品 browser 用例退出0。图为真实页面渲染 / 合成身份与 HTTP，不是生产业务验收。未提交的 M7-01e-3c 多尺寸完整验收不纳入本次完成范围。文档相对链接、PNG签名 / 非零尺寸及 diff 在提交前复核。
- [x] PROC-17b【已完成】2026-09-19：发布工作树 `/private/tmp/shlms-publish-3dDC0k` 从远端 main 89b3e69 快进合并至 3b427e1，20个 tracked 文件，包含三个实现提交和 PROC-17a 文档 / 实际图。`git diff --check 89b3e69..HEAD` 通过；发布与研发 HEAD tracked 树均为 cb00f3404c2f483e9e746cc5b5142f43b0bac34a。`git push git@github.com:kev-chen369/shlms.git HEAD:main` 非强制成功；独立 `git ls-remote ... refs/heads/main` 返回 3b427e18aaca82f3ba07dfe2e8aca0fafd9d4996。未完成验收文件、其主计划状态差异、用户 .DS_Store 及既有依赖 symlink 均未提交。该记录独立提交后再推送并核对，不将记录提交自身哈希写回形成循环。未部署应用、未迁移生产库。
- [ ] PROC-17c【阻塞】生产 H5 / 小程序部署。仓库尚未提供可执行生产发布入口、目标环境及运行配置。依赖：部署目标和既有发布流程；下一步：按提供的流程部署并验证实际 URL。GitHub 同步不等同于生产发布，不运行生产迁移或开启真实渠道。

任务完成后分别记录验证结果并立即提交；保留未完成验收与用户 .DS_Store 文件。
