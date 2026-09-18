# 隔离 PostgreSQL 验证环境与实跑记录

任务：PROC-11。2026-09-19 新建本机私有测试实例，非现有业务库、非生产部署。

## 环境与来源

- PostgreSQL 17.11，aarch64 macOS，UTF-8 / C locale，服务端时区 Asia/Shanghai。
- 私有运行目录 `/private/tmp/shlms-pg-runtime.lE8YYN`，数据目录为其 `data` 子目录；只监听该目录内 Unix socket，端口标识55439，`listen_addresses` 为空。不启用 Homebrew service 或开机服务。
- 原 Homebrew 安装进程在依赖下载阶段等待；复用已完成的 PostgreSQL / ICU / krb5 二进制下载包，在私有目录解压，引用现有稳定基础库，不升级现有依赖。PostgreSQL 原下载包 SHA-256：`508af59730184c810e393b98835fd3ae282bb8298157e6c2d92399d6ea6aaf2b`，与 Homebrew 官方 bottle 记录一致，不是 PostgreSQL 官方 macOS 安装包。
- 只对私有副本重定位 Mach-O 占位依赖、重新进行本地 ad-hoc 签名；修正私有共享数据 / 内置模块目录。系统 shell 清除 DYLD_LIBRARY_PATH 已通过探针确认，因此不依赖该变量传入子进程。私有短路径别名为 `/private/tmp/pgq.m9Zjsl/s`（share）和 `/private/tmp/pgq.m9Zjsl/l`（模块）。
- 初始两次初始化明确失败（时区目录 / 字典模块），initdb 自动移除了未完成数据目录；修正后完整 bootstrap、post-bootstrap、同步与 pg_ctl start 退出0。
- 私有实例成功后终止本轮自己启动的安装进程及确认归属的下载子进程，安装不是成功完成；未终止其他 Homebrew / Codex / Docker 进程，未信任第三方 tap。
- `brew list --versions`复核：readline仍8.3.3、xz仍5.8.3；postgresql@17 / icu4c@78 / krb5没有全局安装。独立审查者通过pg_ctl及SQL确认私有实例实际运行、版本 / 数据目录 / 空TCP监听和时区，无阻断。

## 实跑证据

`psql` 查询确认版本、空 TCP 监听地址、私有 data 目录与 Asia/Shanghai。测试角色 shlms_test 仅存在于这个新实例，不使用真实身份或渠道凭据。测试帮助函数使用独立 schema；清理效果需另审计，不能仅凭测试PASS认定无残留。

配置临时测试连接后运行 `go test -count=1 -json ./...`，汇总为489个pass事件（含子用例）、0 fail、0 skip；此前同一实例首次跑全量仅物料的年份fixture失败：未标UTC的10000年日期按上海时区转为9999年UTC。修正fixture显式Z后通过，不删除边界测试或放宽迁移约束。随后补充城市与时间边界并重新实跑定向 / 全量。

`go vet ./...`、`go build ./...`、`git diff --check` 均退出0。全量包括真实迁移并发 / 重复执行、摘要核验、事务回滚及各仓储SQL测试，不再把缺少PG_TEST_DSN的跳过当验证。

额外清理审计发现3个仅本轮tracking_test_*合成schema残留：tracking测试defer db.Close先于t.Cleanup执行，随后DROP错误被忽略。此缺陷未被既有PASS断言覆盖，已编号PROC-12，需真实失败回归后修正清理顺序；本记录不宣称全实例零残留。物料schema的down表 / helper及其schema清理已独立验证。

## 本机复验方式

下面仅适用于仍存在的本轮私有实例。临时目录可能被系统清理；不可将命令改指向保留业务数据的库。若实例停止，先检查 pg_ctl status / 进程，不因一次观察超时重启。

```sh
/private/tmp/shlms-pg-runtime.lE8YYN/postgresql@17/17.11/bin/pg_ctl -D /private/tmp/shlms-pg-runtime.lE8YYN/data status
PG_TEST_DSN='postgres://shlms_test@/postgres?host=/private/tmp/shlms-pg-runtime.lE8YYN&port=55439' go test -count=1 ./...
```

隔离实例使用私有目录内trust认证，仅供合成测试数据；不能将此配置用于生产或外部监听。项目内不保存真实DSN、密码或原始订单数据。按需停止只操作本实例：

```sh
/private/tmp/shlms-pg-runtime.lE8YYN/postgresql@17/17.11/bin/pg_ctl -D /private/tmp/shlms-pg-runtime.lE8YYN/data -m fast -w stop
```

## 不代表完成的范围

物料迁移的up / 约束 / down另按M7-01c-2a验收；能力证据、只读仓储、完整新迁移链及目录API仍继续。订单异常新增全链路用例M3-05b尚待研发，不仅凭旧测试通过补勾。真实身份、获批渠道、收益 / 财务对账与微信真机未验收；未运行生产迁移或部署。
