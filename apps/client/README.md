# 万惠宝客户端

uni-app / Vue 3 / TypeScript 独立客户端；H5 首发。使用受支持的 Node.js LTS：20.19+ 或 22.12+。

```sh
cd apps/client
npm ci
npm run dev:h5
```

开发入口 `http://127.0.0.1:5173`。开发服务器只绑定本机，关闭跨域响应和 HMR，文件访问限制在客户端目录；修改代码后手动刷新。不要暴露到公网，仅绑定本机不能消除旧 Vite 的全部风险。

```sh
npm test
npm run typecheck
npm run build:h5
```

H5 产物在 `dist/build/h5`。node_modules、产物、环境文件不进入 Git。包版本遵守 uni-app 官方 Vue 3 稳定模板的同版约束；构建链安全审计需在发布前独立完成，不使用 `--force` / `--legacy-peer-deps` 掩盖版本冲突。

2026-09-17 锁文件审计：40 项（15 low / 12 moderate / 13 high / 0 critical）；独立 Vitest 已升级 4.1.11。uni-app 插件仍精确要求 Vite 5.2.8，含跨站读取开发服务器等风险；当前防护不是所有漏洞的全面修复。安全升级、兼容回归与发布安全验收未通过前，不允许生产发布；不得将上述工程检查称为安全审计通过。

## 当前边界

已实现：首页渠道区与 AI / 推广双卡、五个消费者导航、「我的」推广入口；H5 支持入口和底栏 Enter / Space 操作。分类、省钱、订单、AI 与推广业务目前显示未接入说明，不代表功能闭环。微信端虽可编译，SVG 导航图标与实际设备行为仍须单独适配验收。

工程阶段不内置演示订单、商品、返现、收益或成功转链。真实身份提供方、协议内容、获批渠道和资金规则未提供时，对应功能明确显示未接入，不可提交真实业务。

`npm run build:mp-weixin` 仅供后续编译检查，不能代替真实微信 AppID、合法域名配置、开发者工具和设备验收；manifest 中不填假 AppID。

规范与任务见 `docs/superpowers/specs/2026-09-17-wanhui-client-design.md`、`docs/superpowers/plans/2026-09-17-wanhui-client-foundation.md`。
