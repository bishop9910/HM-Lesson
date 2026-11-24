# 测试报告 · HM-Lesson

> 状态：**M1 骨架 + 多老师认证 + 云托管首次部署**（2026-11-24）。backend 已完成 Kratos v3 接入、多老师认证/成员管理、版本化迁移；前端完成教师登录/成员管理、类型检查与 mp-weixin 构建；**已在微信云托管连接 MySQL 5.7.18 实测迁移、健康检查与 owner 登录**。没有真实执行过的测试不得写“通过”。
> 维护：每完成一个里程碑追加一轮记录；没有真实执行过的测试不得写“通过”。

## 1. 环境

| 项 | 值 |
| --- | --- |
| 开发机 | Windows，Go 1.25.7，Node 22，pnpm 12 |
| 集成环境 | Debian/CI（Docker 在 Debian/CI；本机不装 Docker） |
| 数据库 | **MySQL 5.7.18**：本地 Docker 容器 / 生产云托管（Q22 已确认） |
| 对象存储 | 生产：云托管 COS（fileID）；本地 dev：MinIO 容器 |
| 部署目标 | 微信云托管（上海，单服务 `hm-api`，`min=max=1`） |

## 2. 自动化测试记录

| 日期 | 范围 | 命令 | 结果 | 备注 |
| --- | --- | --- | --- | --- |
| 2026-11-22 | backend 编译 | `go build ./...` | **通过** | GOCACHE 指向临时目录，退出码 0 |
| 2026-11-22 | backend 静态检查 | `go vet ./...` | **通过** | 退出码 0 |
| 2026-11-22 | backend 单元测试 | `go test ./... -count=1` | **通过** | `internal/conf` 2 个表驱动用例；其余包无测试 |
| 2026-11-22 | backend 健康检查 | 启动后 `curl /healthz`、`/readyz` | **通过** | 默认本地端口 `:8000`，返回 `{"status":"ok","version":"0.1.0-dev"}` / `{"status":"ready"}` |
| 2026-11-22 | 前端类型检查 | `node_modules/.bin/vue-tsc --noEmit -p tsconfig.json` | **通过** | 依赖由用户安装；修复 `request.ts` 的 PATCH/data 类型错误后退出码 0 |
| 2026-11-22 | 小程序构建 | `node_modules/.bin/uni build -p mp-weixin` | **通过** | 产物 `frontend/apps/mp/dist/build/mp-weixin`，154,394 字节；有 Sass `@import` 弃用告警，不影响构建 |
| 2026-11-23 | backend 编译/静态检查 | `go build ./...`、`go vet ./...` | **通过** | 新增 GORM/Argon2/JWT 依赖后退出码 0 |
| 2026-11-23 | 多老师认证单测 | `go test ./... -count=1` | **通过** | `pkg/password`、`pkg/token`、`internal/biz`（Login/CreateStaff/ListStaff/UpdateStaff/Bootstrap/randomID）与 `internal/conf` 全部通过 |
| 2026-11-23 | 前端类型检查 | `node_modules/.bin/vue-tsc --noEmit -p tsconfig.json` | **通过** | 新增教师登录/成员管理页后退出码 0 |
| 2026-11-23 | 小程序构建 | `node_modules/.bin/uni build -p mp-weixin` | **通过** | 新增 login/staff 分包页后构建成功；仅 Sass `@import`/legacy 告警 |
| 2026-11-24 | 迁移执行器单测 | `go test ./... -count=1` | **通过** | 新增 `internal/data` 的 `TestLoadMigrations`（2 个版本、up/down 成对）与 `TestSplitSQL`；biz/conf/password/token 同时通过；未连 MySQL |
| 2026-11-24 | MySQL 5.7.18 兼容调整 | `gofmt -l`、`go build/vet/test`、Linux amd64 交叉构建 | **通过** | 代码/单元与构建通过；真实 5.7.18 上迁移已在云托管首次启动验证 |
| 2026-11-24 | 云托管部署 | GitHub 仓库绑定 → 构建镜像 → 启动 `hm-lesson-003` | **通过** | 运行镜像 Debian 12；服务 `hm-lesson`，环境 `prod-d8gq8vzftc39caa5b` |
| 2026-11-24 | 云托管健康检查 | `GET /healthz`、`GET /readyz` | **通过** | 返回 `{"status":"ok","version":"0.1.0"}`、`{"status":"ready"}` |
| 2026-11-24 | 云托管登录 | `POST /api/v1/auth/login` | **通过** | 返回 `staff.role=owner`、`staff.status=active`、JWT；`expires_at=2026-11-24T11:37:24Z`（2h） |
| — | proto lint/生成 | `buf lint`、`make api` | 未执行 | 需要下载 buf 远程插件/依赖，未获授权 |
| — | 版本化迁移（本地） | 启动 `data.Migrate`（MySQL） | 未执行 | 本机无 Docker/MySQL；云托管启动已验证 |

## 3. 手工验收（TC-01 ~ TC-17）

| 编号 | 场景 | 结果 | 证据/备注 |
| --- | --- | --- | --- |
| TC-01 | 家长绑定与审批后可见 | 未执行 | |
| TC-02 | 家长只能看到本人学员 | 未执行 | |
| TC-03 | 任务下发按人生成提交记录 | 未执行 | |
| TC-04 | 作业提交（文字+多图+文件） | 未执行 | |
| TC-05 | 打回→重交→再批阅 | 未执行 | |
| TC-06 | 逾期提交仍可交并标记 | 未执行 | |
| TC-07 | 家长只看到可预约时段 | 未执行 | |
| TC-08 | 并发预约同一时段只有一条成功 | 未执行 | 高危 |
| TC-09 | 驳回释放时段并通知 | 未执行 | |
| TC-10 | 改期=取消后重提 | 未执行 | |
| TC-11 | 标记完成才写课时流水 | 未执行 | 高危 |
| TC-12 | 余额 ≤0 禁止预约 | 未执行 | |
| TC-13 | 资料按分组可见 | 未执行 | |
| TC-14 | 跨学员越权返回 404 | 未执行 | 高危 |
| TC-15 | 回执确认与催办 | 未执行 | |
| TC-16 | 导出 xlsx 字段与筛选一致 | 未执行 | |
| TC-17 | Excel 导入预览/错误/回滚 | 未执行 | 非蓝本，新增 |

## 4. 未执行原因与计划

1. M1 只有健康检查、多老师认证单测与前端骨架；业务用例（TC-01~TC-17）需等 M2 起实现。
2. 本机无 Docker，版本化迁移实测与集成测试在 Debian/CI 上执行（Q16）；数据库已确认 MySQL 5.7.18。
3. 云托管契约测试（100 KiB/1000 KiB/15 s、fileID、WS 首帧认证）需真实云环境或 mock 网关，待云托管 dev 环境建立后接入。

## 5. 缺陷记录

| 编号 | 现象 | 严重度 | 状态 | 根因/修复 |
| --- | --- | --- | --- | --- |
| D-01 | `vue-tsc` 报 `uni.request` 不接受 `PATCH` 与 `unknown` data | 低 | 已修复 | `uni.request` 类型不含 PATCH；H5 调试降级为 POST + `X-HTTP-Method-Override`，data 收窄为 `RequestData` |
| D-02 | `uni build -p mp-weixin` 报 `Rollup failed to resolve import "nostics" from pinia` | 中 | 已修复 | pinia 4.1.0 的传递依赖在 pnpm 符号链接布局下未被 Rollup 解析；将 `nostics@1.4.0` 加为小程序直接依赖并补 `pnpm-lock.yaml`；本地已用 Junction 验证构建，正式链接需在 `frontend/` 跑一次 `pnpm install` |
| D-03 | Sass 报 `@import` 与 legacy JS API 弃用 | 低 | 待处理 | 迁移到 `@use` 需改 tokens 与各页面引用；不影响构建，排入 M2 |



