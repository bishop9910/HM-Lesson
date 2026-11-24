# 测试报告 · HM-Lesson

> 状态：**M1 骨架 + 多老师认证 + 云托管首次部署 + M2 CRM/Excel 导入 + M3 任务主链路**（2026-11-24）。backend 已完成 Kratos v3 接入、多老师认证/成员管理、版本化迁移、真实汇总、绑定申请、M2 学员/跟进/绑定审批/绑定码/guardian 登录/家长门户/家长端学员详情/Excel 导入，以及 M3 任务/提交/批阅/打回/模板/家长任务与档案接口；前端完成教师学员/跟进/审批/导入/任务/批阅页与家长登录/门户/任务/档案页；**云托管已连接 MySQL 5.7.18 实测迁移、健康检查、owner 登录、dashboard、学员档案编辑、家长端学员详情与 Excel 全链路**。M3 尚未部署实测。没有真实执行过的测试不得写“通过”。
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
| 2026-11-24 | 迁移执行器单测 | `go test ./... -count=1` | **通过** | `TestLoadMigrations` 覆盖 4 个版本（init/teaching/import_batch/task_templates）、up/down 成对；`TestSplitSQL` 通过；biz/conf/password/token 同时通过；未连 MySQL |
| 2026-11-24 | MySQL 5.7.18 兼容调整 | `gofmt -l`、`go build/vet/test`、Linux amd64 交叉构建 | **通过** | 代码/单元与构建通过；真实 5.7.18 上迁移已在云托管首次启动验证 |
| 2026-11-24 | 云托管部署 | GitHub 仓库绑定 → 构建镜像 → 启动 `hm-lesson-003` | **通过** | 运行镜像 Debian 12；服务 `hm-lesson`，环境 `prod-d8gq8vzftc39caa5b` |
| 2026-11-24 | 云托管健康检查 | `GET /healthz`、`GET /readyz` | **通过** | 返回 `{"status":"ok","version":"0.1.0"}`、`{"status":"ready"}` |
| 2026-11-24 | 云托管登录 | `POST /api/v1/auth/login` | **通过** | 返回 `staff.role=owner`、`staff.status=active`、JWT；`expires_at=2026-11-24T11:37:24Z`（2h） |
| 2026-11-24 | 临时公网 URL 复测 | `GET /healthz`、`/readyz`、`POST /auth/login` | **通过** | `hm-lesson-353410-10-1540142477.sh.run.tcloudbase.com`：200/200/200；owner 登录成功 |
| 2026-11-24 | 公网 401 守卫 | 无 token 访问 students/guardians/imports | **通过** | 均返回 401 `UNAUTHORIZED`；带 token 后 staff 列表正常 |
| 2026-11-24 | 公网 Excel 导入全链路 | template → preview → commit → students → rollback | **通过** | 模板 8,552 字符 base64；1 行预览 `to_create=1`；提交 `created_rows=1`；学员列表可查；回滚后列表为空 |
| 2026-11-24 | 公网 dashboard | `GET /api/v1/dashboard/summary` | **失败** | 500 `读取汇总失败`；根因 SQL 查了 `submissions.status`（该表无 status，应为 `task_assignments.status='submitted'`）；已在本机修复，待推送部署复测 |
| 2026-11-24 | 公网 dashboard（部署后复测） | `GET /api/v1/dashboard/summary` | **通过** | 2026-11-24 19:55 实测 200，返回 `students_total=0/submissions_pending=0/bookings_pending=0/low_balance_students=0/staff_total=1`；说明 `d1d8fbc` 已真正部署 |
| 2026-11-24 | 公网家长端学员详情 | `GET /api/v1/portal/students/{id}`（guardian token） | **通过** | 200，返回剩余课时/作业/流水；用另一学员 id 返回 404 `STUDENT_NOT_FOUND`，绑定校验生效 |
| 2026-11-24 | 公网学员档案编辑 | `PATCH /api/v1/students/{id}` | **通过** | 200，年级/学校/分组/标签/回访/备注全部更新成功，再次 GET 一致 |
| 2026-11-24 | 家长端隐私字段 | 检查 `/portal/home` 与 `/portal/students/{id}` 响应 | **已修复（待部署）** | 原响应复用了教师 DTO，包含 `note`/`tags`/`next_follow_up_at`；已改为 `portalStudentDTO`，只返回 id/姓名/年级/学校/分组/状态 |
| 2026-11-24 | 首页/工作台接入真实汇总 | `GET /api/v1/dashboard/summary` | **通过（构建）** | 后端查询 `students/submissions/bookings/lesson_accounts/staff` 真实计数；前端 `vue-tsc` 与 `uni build` 通过；云托管接口待下次部署后验证 |
| 2026-11-24 | 家长绑定申请落库 | `POST /api/v1/auth/bind-request` | **通过（单测）** | `internal/biz` 覆盖手机号校验、空仓库；真实云托管待部署后验证 |
| 2026-11-24 | M2 CRM 后端 | `gofmt -l`、`go build ./...`、`go vet ./...`、`go test ./... -count=1` | **通过** | 新增 `internal/biz/crm.go`、`internal/data/crm.go`、`internal/service/crm.go`；编译/静态检查/既有测试通过；CRM 尚无专门单测 |
| 2026-11-24 | M2 CRM 前端 | `vue-tsc --noEmit`、`uni build -p mp-weixin` | **通过（构建）** | 新增学员列表/详情、绑定审批、家长登录/门户页；产物 201,973 字节；仅 Sass `@import`/legacy 告警 |
| 2026-11-24 | Excel 导入后端 | `go build/vet/test` | **通过** | `pkg/xlsximport` 解析/模板单测通过；`internal/data` 迁移数 3；导入接口未连 MySQL 实测 |
| 2026-11-24 | Excel 导入前端 | `vue-tsc --noEmit`、`uni build -p mp-weixin` | **通过（构建）** | 新增 `pkg-teacher/pages/import/index.vue`；模板下载/选择文件/预览/提交/回滚；仅 Sass 告警 |
| 2026-11-24 | M2 学员档案编辑 | `go build/vet/test`、`vue-tsc`、`uni build` | **通过（构建）** | 学员详情增加年级/学校/分组/标签/下次回访/备注编辑；后端复用 PATCH；未连 MySQL 实测 |
| 2026-11-24 | M2 家长端学员详情 | `go build/vet/test`、`vue-tsc`、`uni build` | **通过（构建）** | 新增 `GET /api/v1/portal/students/{id}` 与 `pages/portal/student.vue`：剩余课时、作业状态、课时流水；未连 MySQL 实测 |
| 2026-11-24 | M3 任务后端 | `gofmt -l`、`go build ./...`、`go vet ./...`、`go test ./... -count=1` | **通过** | 新增 `internal/biz/task.go`、`internal/data/task.go`、`internal/service/task.go` 与任务 biz 表驱动测试；任务接口未连 MySQL 实测 |
| 2026-11-24 | M3 任务模板迁移 | `go test ./internal/data -count=1` | **通过** | 新增 `000004_task_templates.up/down.sql`；`TestLoadMigrations` 期望 4 个版本 |
| 2026-11-24 | M3 前端 | `vue-tsc --noEmit`、`uni build -p mp-weixin` | **通过（构建）** | 新增教师任务/详情/待批阅、家长任务/详情/档案、模板选择；产物 261,425 字节 / 81 文件；仅 Sass 告警 |
| — | proto lint/生成 | `buf lint`、`make api` | 未执行 | 需要下载 buf 远程插件/依赖，未获授权 |
| — | 版本化迁移（本地） | 启动 `data.Migrate`（MySQL） | 未执行 | 本机无 Docker/MySQL；云托管启动已验证 |

## 3. 手工验收（TC-01 ~ TC-17）

| 编号 | 场景 | 结果 | 证据/备注 |
| --- | --- | --- | --- |
| TC-01 | 家长绑定与审批后可见 | 未执行 | |
| TC-02 | 家长只能看到本人学员 | 未执行 | |
| TC-03 | 任务下发按人生成提交记录 | 未执行（代码/构建通过） | 发布事务按 `student_ids` 一人一条 assignment；待云托管部署实测 |
| TC-04 | 作业提交（文字+多图） | 未执行（代码/构建通过） | 家长文字 + 图片 fileID；待部署实测 |
| TC-05 | 打回→重交→再批阅 | 未执行（代码/构建通过） | 打回理由必填；重交 version+1；待部署实测 |
| TC-06 | 逾期提交仍可交并标记 | 未执行（代码/构建通过） | `is_late=now>due`；待部署实测 |
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
| TC-17 | Excel 导入预览/错误/回滚 | 单测/构建通过，端到端未执行 | 非蓝本，新增；模板/解析单测通过，云托管 000003 迁移与真实导入待部署验证 |

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
| D-04 | 公网 `GET /api/v1/dashboard/summary` 返回 500 | 高 | 已修复并复测 | 查询误用 `submissions.status`（`submissions` 无 status 列），改为 `task_assignments.status='submitted'`；2026-11-24 19:55 公网复测 200 |
| D-05 | 家长端复用教师 DTO，泄露 `note/tags/next_follow_up_at` | 高 | 已修复，待部署 | 新增 `portalStudentDTO`，只返回 id/姓名/年级/学校/分组/状态 |



