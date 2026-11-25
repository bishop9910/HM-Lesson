# AGENTS.md · HM-Lesson 协作与工程规范

> 本文件是**所有 AI Agent 与人类协作者的强制约束**，优先级高于任何单次对话中的口头建议。
> 违反本文件的操作视为错误交付，必须回滚并说明原因。
> 最后更新：2026-11-22（部署目标改为微信云托管）

---

## 0. 项目一句话

把委托方原“飞书零代码家校系统”重构为**微信云托管**上的交付系统：**Go Kratos v3 单服务 `hm-api`**（HTTP + WebSocket），数据库与对象存储使用云托管/腾讯云配套能力，家长与老师共用**一个微信小程序**（uni-app + Vue3 + TS + pnpm；家长端主包 + 教师模式 `pkg-teacher` 分包，Q1=B）。

**不再使用**：Debian 自建服务器、Docker Compose 生产部署、Caddy/Nginx 反向代理、自建 SeaweedFS/MinIO（生产）、服务器域名与域名 ICP 备案依赖（小程序侧；小程序备案已完成）。

当前状态：**M1 骨架 + 多老师认证 + 云托管首次部署 + M2 CRM/Excel 导入 + M3 任务主链路（2026-11-24）+ 微信订阅消息（2026-11-25）+ M4 一对一预约与课时（2026-11-25）+ 家长账号体系（2026-11-25）+ M5 学员删除与内部系统账号（2026-11-25）**。Kratos v3 已接入，完成手机号+密码登录、JWT、owner/assistant 成员管理、Argon2id、`migrations/*.sql` 版本化迁移与启动自动 up、`/api/v1/dashboard/summary`、`/api/v1/auth/bind-request` 家长绑定申请；M2 已落盘学员档案/跟进/绑定审批/6 位绑定码/guardian 登录/家长门户接口与页面；M3 已落盘教师任务列表/新建/发布/关闭/统计/逐份批阅/打回/批改图、家长任务/提交/拍照/档案、发布与批阅站内弹窗通知；订阅消息（`000006`）已落盘一次性额度 `subscription_grants` + 投递队列 `notification_deliveries` + `pkg/wx` + 进程内 worker + 家长门户「开启提醒」，模板走 `HM_WX_SUBSCRIBE_TEMPLATES`，未配置时 no-op（ADR-029），云托管内必须 `HM_WX_CALL_MODE=cloud` 走云调用通道；M4（`000007`，线上修复 `000008`）已落盘 `slots`/`bookings`/`lesson_accounts`/`lesson_ledger`、教师时段与预约审核、消课幂等与未到课不扣课时、家长预约与课时视图、四个小程序页面（时段/预约审核/课时/家长预约）；家长账号体系（`000009`，ADR-030）已落盘 `guardians.password_hash`/`last_login_at`、手机号+密码登录、审批设初始密码、家长与老师自助改密、owner 家长账号总览/重置密码/物理删除、学员状态同步停用与 4 个页面；M5（`000010`，ADR-031）已落盘 `staff.hidden` 与学员删除 `DELETE /api/v1/students/{id}`（owner、两次确认、有课时流水需 `?force=1`、连带清作业/预约/绑定并让家长回到未绑定），系统账号用户名固定 `system`、隐藏、仅内部测试与调试用、初始密码取 `HM_SYSTEM_PASSWORD` 且可自助改密。`go build/vet/test` 通过，云托管已连接 MySQL 5.7.18 实测迁移、健康检查与 owner 登录；`vue-tsc` 与 `mp-weixin` 构建通过。**生产已验证** M2/M3 主链路、逾期提交、任务删除、站内通知与批改图字段、M4 预约/课时/幂等/未到课/时区 UTC；**待部署验证** `000009`/`000010` 在真实 MySQL 5.7 上 up/down；**待真机验证**相机拍摄/`wx.cloud.uploadFile`、小程序弹窗点按、订阅消息服务通知、家长密码登录与账号清理、学员删除与系统账号登录。不要假设任何业务接口、表数据、页面已完成。

## 1. 开始任何任务前

1. 读本文件。
2. 读 [`HANDOVER.md`](HANDOVER.md)（快速代码地图/接口/踩坑）与 [`TODO.md`](TODO.md)（当前优先级与待办）。
3. 读 [`docs/backend-requirements.md`](docs/backend-requirements.md)、[`docs/data-model.md`](docs/data-model.md)、[`docs/frontend-requirements.md`](docs/frontend-requirements.md)、[`docs/deploy-wechat-cloudrun.md`](docs/deploy-wechat-cloudrun.md) 中与你任务相关的章节。
4. 打开 [`docs/open-questions.md`](docs/open-questions.md)，确认你要做的事**不依赖未决问题**。若依赖：
   - 不要猜产品规则；
   - 在实现中留出清晰的配置点/接口边界，并在回复中明确“阻塞于 Qx”；
   - 必要时先向用户提问，再继续。
5. 已定结论（2026-11-18）：Q1=B 小程序教师模式；Q2=个人主体未认证（绑定走手机号+审批/绑定码，禁用 `getPhoneNumber`）；Q4=`github.com/bishop9910/hm-lesson`；Q5=多老师可扩，支持同时多个老师；Q6=Excel 导入；Q7=订阅消息+站内信；Q8=未到课不自动扣课时；Q9=提交即锁时段；Q10=wot-design-uni；Q12=青绿视觉；Q13=一期不做支付；Q14=导出 xlsx；Q16=Go 已装、Docker 在 Debian；Q17=GitHub Actions；Q18=一期不做暗色。
6. 2026-11-22 变更：Q15 由“Debian 单机 Compose”改为**微信云托管**；Q11 存储改为**云托管对象存储（COS/fileID）**，不再以 SeaweedFS 为生产默认；Q3 域名/备案对小程序侧**不再阻塞**（`callContainer`/`connectContainer` 不需要域名），仅在需要非小程序客户端时才涉及自定义域名。
7. 已确认（2026-11-22 第二轮）：Q19=A（个人主体 + 工具类目先提审）、Q22=MySQL 5.7.18（本地容器/生产云托管）、Q23=生产 COS/本地 MinIO、Q24=一期不用 Redis、Q25=不绑自定义域名、Q26=AppID 不变更、Q27=GitHub Actions、Q28=A（功能范围全保留，IM 除外）。
8. 用 `todo_write` 列出步骤；多步骤任务先给计划再动手。

## 2. 文档优先级（冲突时）

```text
用户当次明确指令 > AGENTS.md > docs/backend-requirements.md、docs/data-model.md、docs/frontend-requirements.md、docs/deploy-wechat-cloudrun.md
> README.md > docs/鹤鸣老师课堂-...docx（原始蓝本）> 代码注释 > 任何 Agent 的推断
```

- 原始 `docx` 是**业务事实来源**，但不是技术方案来源；平台差异按需求文档中的“迁移映射”执行。
- 发现文档之间矛盾：以更具体、更新的文档为准，并在同一次提交中修正另一处，同时在回复中说明。
- **禁止**在没有更新文档的情况下偏离需求文档中的状态机、权限矩阵、计费与预约规则。

## 3. 硬性禁令（红线）

1. **禁止擅自安装或下载任何东西**：`go install`、`docker pull`、`pnpm add`、`apt install`、下载二进制、拉取大镜像、`buf dep update`、`wxcloud` CLI 下载等，必须先告知用户“要装什么、为什么、多大、大概多久”，得到同意后再执行。下载明显缓慢时立即停止并请用户手动安装。参见 Q16/Q27。
2. **禁止编造**：不写“已验证”“测试通过”除非真的运行过；不虚构命令输出、版本号、接口返回、控制台截图。
3. **禁止提交密钥**：`.env`、证书、私钥、微信 AppSecret、数据库密码、云托管私钥、TCR 凭据一律不入库；只提交 `.env.example`。生产密码不得有默认值。
4. **禁止跳层**：Kratos 分层为 `api → service → biz → data`。`service` 不得直接 import `data`；`biz` 不得 import `transport`；`data` 不得依赖 `service`。
5. **禁止手改生成物**：`*.pb.go`、`*.pb.validate.go`、`*_grpc.pb.go`、`*_http.pb.go`、OpenAPI 生成的前端类型。改 `.proto` 后重新生成。
6. **禁止绕过权限**：任何按学员维度的查询，必须经统一的 `guardian scope` 校验；不得只在 handler 判断、不得信任前端传的 `student_id`。见 §7。
7. **禁止擅自改生产表结构**：所有表变更必须写成 `backend/migrations/NNNN_*.up.sql` + `.down.sql`（ADR-003）；改列/删列必须在空库验证，禁止在生产库执行破坏性 DDL 而没有备份与用户确认。
8. **禁止 `any` / `@ts-ignore` / `//nolint` 滥用**：确需使用时必须写原因；`any` 在评审中视为缺陷。
9. **禁止删除或改写用户数据语义**：退课=归档、流失=状态，不得物理删除学员/任务/流水。课时余额只能由流水推导。**唯一例外（2026-11-25，ADR-031，用户明确要求）**：owner 可经 `DELETE /api/v1/students/{id}` 物理删除学员档案（前端两次确认）；存在课时流水时必须显式 `?force=1` 才能连流水一起删除，默认路径不得静默丢账。
10. **禁止扩大范围**：不做需求文档“范围外”的功能（即时通讯/聊天/客服会话/群聊、支付、录播、短信、自动排课最优解），除非用户明确要求并更新文档。多老师 / `assistant` 已是本期能力，不属于范围外。
11. **禁止在未获授权时执行破坏性 git 操作**：`push --force`、`reset --hard` 到远端、删除分支、改写历史。
12. **禁止把临时脚本/调试文件留在仓库根目录**；用完删除或放 `scripts/dev/` 并说明。
13. **禁止依赖云托管公网域名上线**：小程序必须走 `wx.cloud.callContainer` / `wx.cloud.connectContainer`；生产要关闭“公网访问”。默认公网域名只用于临时联调。
14. **禁止把文件写本地磁盘当持久化**：云托管实例无持久盘、随时缩容。上传、导出、缓存文件必须写对象存储；日志只写 stdout。
15. **禁止请求返回后依赖后台 goroutine 跑长任务**：实例可能被回收。长任务落 `jobs` 表，由定时触发器或请求内分片推进；必须幂等可重试。
16. **禁止单次请求超限**：`callContainer` 请求体 ≤100 KiB、响应 ≤1000 KiB、超时 ≤15 s。大文件走对象存储，大列表必须分页，导出走异步任务 + 文件。
17. **禁止信任客户端传入的微信身份 header**：`X-WX-OPENID` 等只能由网关注入；业务必须校验 `X-WX-SOURCE` 存在，绝不能把前端自报的 openid 当身份。

## 4. 技术栈锁定

| 层 | 选型 | 版本策略 |
| --- | --- | --- |
| 部署 | **微信云托管**（上海地域，单服务 `hm-api`） | 容器端口 80；MVP `min=max=1`；关闭公网访问 |
| 语言 | Go | 1.25.x（Kratos v3 要求） |
| RPC 框架 | github.com/go-kratos/kratos/v3 | v3.0.x，锁定 minor |
| 协议 | Protobuf + buf | buf 1.75+；`buf lint` + `buf breaking` |
| 传输 | HTTP/JSON + WebSocket（小程序 `callContainer` / `connectContainer`）；gRPC 仅内部/工具 | 同一 proto 生成多套 |
| 数据库 | **MySQL 5.7.18**：本地 Docker 容器 / 生产云托管 MySQL（Q22 已确认） | 见 §6.3 |
| 建表 | `backend/migrations/*.up.sql` / `.down.sql`（embed） | 启动时 `GET_LOCK` 后按版本执行；`cmd/migrate` 手工 up/down/version |
| ORM/驱动 | GORM v2 + `mysql` 驱动 | 复杂查询写原生 SQL |
| 缓存/限流 | **一期不用 Redis**：MySQL 计数表 + 进程内 LRU（Q24） | — |
| 对象存储 | **生产云托管 COS（fileID）；本地 MinIO 容器**（Q23 已确认） | — |
| 日志 | 标准库 `log/slog`（Kratos v3） | JSON，stdout，结构化 |
| 配置 | Kratos config（file + env 覆盖） | 云托管环境变量注入 |
| 依赖注入 | google/wire | 生成 `wire_gen.go` |
| CI/CD | GitHub Actions → 构建镜像 → TCR → `wxcloud` 部署；或控制台持续集成 | Q27 |
| 前端 | uni-app（Vue3 + Vite）+ TypeScript | 官方 vite-ts 模板；**单小程序，家长端 + 教师模式** |
| 包管理 | pnpm workspace | 锁定 `pnpm-lock.yaml` |
| UI | wot-design-uni | 1.14.x（Q10 已确认） |
| 状态 | Pinia | 按域拆分 |
| 测试 | Go testing + testify + testcontainers；Vitest + Vue Test Utils | 见 §8 |

新增任何依赖前：说明用途、体积/维护状态、是否可用标准库替代，经用户确认。

## 5. 仓库结构（目标形态）

```text
HM-Lesson/
├── AGENTS.md
├── README.md
├── docs/
│   ├── 鹤鸣老师课堂-飞书家校系统搭建需求与实施清单.docx
│   ├── backend-requirements.md
│   ├── data-model.md
│   ├── frontend-requirements.md
│   ├── deploy-wechat-cloudrun.md
│   ├── open-questions.md
│   └── decisions.md            # 架构决策记录（ADR，新增决策必须写）
├── backend/
│   ├── api/
│   │   ├── proto/hm/v1/*.proto # 唯一定义源
│   │   └── openapi/            # buf 生成，供前端出 TS 类型
│   ├── gen/hm/v1/              # 生成物（禁止手改）
│   ├── cmd/hm/                 # 单进程入口（api + 内嵌 job 调度）
│   ├── migrations/             # 版本化 SQL 迁移（embed，up/down）
│   ├── internal/               # biz、data、service、server、conf
│   ├── pkg/                    # authz、storage、wx、notify、xlsx、idgen、jobrunner
│   ├── Dockerfile              # 云托管构建用，单阶段/多阶段均可
│   ├── buf.yaml / buf.gen.yaml
│   ├── Makefile
│   └── go.mod
├── frontend/
│   ├── apps/mp/                # 单小程序：家长端（主包）+ 教师模式（pkg-teacher 分包）
│   ├── packages/{api-types,shared}
│   └── pnpm-workspace.yaml
├── deploy/
│   ├── cloudrun/               # 云托管部署说明/配置、环境变量样例
│   └── local/                  # 本地开发 compose（MySQL 5.7.18 + MinIO）
├── .github/workflows/          # CI + 云托管部署
└── scripts/                    # 仅开发/运维脚本，需可重复执行
```

**一期是单体多模块**：一个 `backend` 进程内按 `auth`、`crm`、`task`、`booking`、`lesson`、`class`、`material`、`notice`、`file`、`dashboard`、`export`、`notify`、`settings` 划分 biz 模块；模块间只通过 ID 与显式用例方法交互。**不做独立 worker 容器**：定时任务在同一进程内以 DB 锁选主执行，MVP `min=max=1`。

## 6. 后端规范

### 6.1 Proto

- 包名 `hm.v1`；目录 `api/proto/hm/v1/*.proto`，按模块拆分文件（common/auth/crm/task/booking/lesson/class/material/notice/file/dashboard/notify/settings）。
- `go_package = "github.com/bishop9910/hm-lesson/backend/gen/hm/v1;hmv1"`（Q4 已定）；生成产物落 `backend/gen/hm/v1/`。
- 每个对外 RPC 必须有 `google.api.http` 注解；需要登录的加自定义 `hm.api.auth` 注解，由中间件统一鉴权。
- 字段命名 snake_case；枚举值带前缀（`TASK_STATUS_PENDING`）；禁止复用已删除的字段号，用 `reserved`。
- 错误：业务错误用 `errors.BadRequest/Forbidden/NotFound/Conflict` + 稳定 `reason`（大写下划线），前端按 reason 映射文案。**reason 一旦发布不得改名**。
- 生成：`make api`（buf generate）；CI 跑 `buf lint` 与 `buf breaking --against .git#branch=main`。
- 时间统一 `google.protobuf.Timestamp`（UTC 存库）；金额用 `int64` 分；ID 用 string（UUIDv7）。

### 6.2 分层

- `service`：参数校验、鉴权上下文提取、调用 biz、DTO 转换。不含业务规则。
- `biz`：用例编排、状态机、事务边界、领域错误。不碰 SQL。
- `data`：repo 实现、SQL、缓存、外部客户端。返回领域对象，不返回 proto。
- 事务：在 biz 定义 `TxManager`，repo 接收 `ctx` 中的 tx；跨聚合操作必须显式事务。
- 所有外部调用（微信接口、云存储、数据库、Redis）设超时与重试上限；不得无限重试。
- 幂等：写接口支持 `Idempotency-Key`（提交作业、预约、消课、回执、导入提交）。
- **请求路径内不做重活**：单请求目标 p95 < 1 s；导出、批量通知、大批量导入等落 `jobs`，接口只建任务并返回 id。

### 6.3 数据库

- 迁移脚本放 `backend/migrations/NNNN_<snake_case>.up.sql` 与 `.down.sql`，成对提供；用 `//go:embed` 打进二进制，由启动流程按版本执行（ADR-003）。
- 每张表：`id` 主键（UUIDv7，`char(36)`）、`created_at`、`updated_at`、需要时 `deleted_at`（仅归档类实体）。时间统一 UTC。
- 外键、唯一约束、CHECK 必须显式命名（`fk_`、`uq_`、`ck_`）。
- 枚举用 `text/varchar + CHECK`，不用数据库原生 enum。
- **MySQL 5.7.18（Q22 已确认，本地容器与生产云托管同一套 SQL）**：
  - 库/表名全小写（`lower_case_table_names` 创建后不可改）；字符集 `utf8mb4`，排序 `utf8mb4_unicode_ci`；
  - **不支持部分唯一索引**：预约冲突用“生成列 + 唯一索引”或事务内 `SELECT ... FOR UPDATE`，ADR-011 的实现方式需同步改写；
  - 不支持 `RETURNING`、`ON CONFLICT`、`FILTER`、`ILIKE`、`JSONB`、`timestamptz`、`numeric[]`；用 `LAST_INSERT_ID()`、`ON DUPLICATE KEY`、`JSON`、`datetime(3)`；
  - **MySQL 5.7 会解析但忽略 `CHECK`**：所有枚举/范围校验必须在 Go 侧再做一遍（biz/校验层），不能只依赖数据库；`TEXT` 不能有默认值，需要默认值时用 `VARCHAR`。
- 索引命名 `idx_<table>_<cols>`；“一个时段一个有效预约”必须有数据库级约束，不能只靠应用判断。
- 迁移必须在**空库**上从零 up/down/up 验证；每次只前进，破坏性变更需备份与评审。
- 禁止在迁移里写业务数据（种子放 `seeds/`，仅 dev）。
- 启动迁移必须用**同一连接**的 `GET_LOCK('hm_migrate', 30)`（`db.Connection`），按版本执行后释放；失败 fail fast。`cmd/migrate` 提供手工 up/down/version。

### 6.4 日志与可观测

- `slog` JSON 写 stdout；字段：`trace_id`、`request_id`、`actor_id`、`route`、`latency_ms`、`error_reason`。
- **禁止**打印手机号、微信 openid 全量、成绩、作业正文；需要时脱敏（`138****0000`）。
- 关键审计事件写 `audit_logs` 表（谁、何时、对什么、做了什么、前后值摘要），不依赖日志文件。
- 健康检查：`GET /healthz`（不查库）与 `GET /readyz`（查库/存储），云托管探针用前者，避免冷启动误杀。

## 7. 安全与权限（最高优先级）

- 家长身份 = `guardians` 行；可见范围 = 其绑定的 `student_guardians`，且 `guardians.status='active'` 的学员。
- 统一入口：`authz.RequireStudentAccess(ctx, studentID)`。**所有**按学员查询的 repo 方法必须接收已校验的 `studentID` 集合，禁止“查到再判断”。
- 家长 token 只含 `guardian_id`，不含 `student_id` 列表；每次请求实时查绑定（可缓存 60s）。
- 越权返回 `404`（不泄露存在性），不返回 `403`。
- 微信身份：只信网关注入的 `X-WX-OPENID`/`X-WX-APPID`/`X-WX-SOURCE`；服务启动时打印实际 header 名（框架可能转小写/驼峰）确认；禁止把客户端上传的同名 header 当身份。
- 生产关闭云托管“公网访问”；关闭后仅小程序/公众号可调。若临时开启公网，必须加 IP/来源限制与独立鉴权，并在上线前关闭。
- WebSocket：`connectContainer` 建立后，客户端首帧发送应用层 token 认证；服务端同时可用网关 openid（若存在）做二次校验。iOS「高性能+」可能读不到 openid，**token 认证是必需路径**，不能只依赖 header。
- 资料访问：分类 × 分组矩阵在服务端判定；文件用 fileID/对象键鉴权，下载发短期临时链接或服务端流式转发，且记 `download_logs`。
- 上传：客户端直传对象存储；服务端校验扩展名白名单、MIME、大小、图片解码；对象键由服务端生成，禁止用用户文件名当路径。
- 限流：登录/绑定/提交/预约按 openid + IP 双维度；无 Redis 时用 MySQL 计数表 + 进程内令牌桶，多实例前必须换 Redis。
- 教师密码：**Argon2id**（不落明文、不可逆），参数随机器校准并在文档记录；连续失败 5 次锁定 15 分钟。
- 教师账号：`staff.role ∈ {owner, assistant}`；一期即支持多个老师同时使用，owner 可创建/停用成员，assistant 可登录并处理作业、预约、资料；`assistant` 不得访问跟进、收费、课时调整、导出、成员管理。
- 家长绑定：**手机号 + 老师审批**（主）/ **绑定码**（辅）。因为个人主体不能用 `getPhoneNumber`，**禁止**实现手机号快速验证；绑定页由家长手动输入手机号。
- 教师模式页面必须放 `pkg-teacher` 分包，家长端页面不得 import 教师模块；进入教师模式前服务端校验 `role`，前端只做展示层隐藏。
- 小程序端禁止出现“管理端 Web”假设：所有教师操作必须在手机尺寸下可用（表格转卡片、批量操作用选择模式）。
- 依赖安全：CI 跑 `govulncheck`；前端 `pnpm audit --prod` 仅报告不自动修。

## 8. 测试规范

| 层 | 要求 |
| --- | --- |
| 单元 | biz 状态机、课时计算、权限判定必须有表驱动测试；新增逻辑覆盖率不低于 70% |
| 集成 | testcontainers 起 MySQL 5.7.18 + MinIO；repo 与事务测试；空库迁移测试 |
| 接口 | 每个 P0 RPC 至少 happy path + 一个权限/状态错误用例（httptest） |
| 云托管契约 | `callContainer` 头解析、100 KiB/1000 KiB 边界、15 s 超时、fileID 上传下载、WS 首帧认证 |
| E2E | 关键链路：绑定→看到任务→提交→批阅→家长可见；预约→审核→完成→扣课时 |
| 前端 | 工具函数/store 单测；构建通过；主包体积检查 |
| 手工 | 需求文档 TC-01~TC-17 逐条走查，结果记入 `docs/test-report.md` |

- 测试必须能在本地一条命令跑：`make test`、`pnpm test`。
- 本机无 Docker 时，集成测试在 Debian/CI 跑；Windows 只跑单元与契约测试（见 README §9）。
- 不允许 `t.Skip` 掩盖失败；不允许依赖执行顺序；测试数据自清理。
- 提交前必须跑：`go build ./...`、`go test ./...`、`buf lint`、`golangci-lint run`、`vue-tsc --noEmit`、`pnpm lint`。

## 9. 前端规范（摘要，详见前端文档）

- `<script setup lang="ts">`，props/emits 强类型；禁止 Options API。
- 请求只能走 `utils/request.ts`；微信端必须走 `wx.cloud.callContainer`，H5 调试才用 `uni.request`；禁止组件内直连。
- 所有页面必须有加载/空/错误三态；提交按钮必须有 loading 与防重复。
- 颜色、间距、字号只用设计令牌；禁止散落 hex。
- 主包 ≤1.5MB；教师分包 ≤4MB；新增依赖先评估体积。
- 小程序端禁止引入 ECharts、富文本编辑器、Node polyfill。
- 图片/附件必须先 `wx.cloud.uploadFile` 拿 fileID 再提交，禁止 base64 塞 JSON；列表分页，单页响应控制在 200 KB 内。

## 10. Git 与协作

- 分支：`main` 保护；功能分支 `feat/<scope>-<slug>`，修复 `fix/<scope>-<slug>`。
- 提交：Conventional Commits（`feat(booking): ...`），一次提交只做一件事；不提交格式化噪音。
- PR：说明“做了什么/为什么/怎么验证/影响面/回滚方式”；涉及建表/权限/计费/部署必须单独标注。
- 不主动 commit/push，除非用户要求；需要时先 `git status` 确认变更范围。
- 不修改与本任务无关的文件；发现无关问题记录到 issue/清单，不顺手改。

## 11. 变更管理与决策记录

- 任何**产品规则**变化（状态机、权限矩阵、课时规则、预约规则、通知策略）必须先改需求文档，再改代码。
- 任何**架构选择**（拆服务、换库、引入中间件、改部署平台）写入 `docs/decisions.md`，格式：背景 / 选项 / 决定 / 理由 / 影响 / 回滚。
- 任何**未决问题**的答案，先更新 `docs/open-questions.md` 状态，再落地。
- 文档与代码同 PR 提交；文档过期视为缺陷。

## 12. 交付完成定义（DoD）

一个任务只有同时满足以下条件才算完成：

- [ ] 代码符合分层与命名规范，无 `any`/无跳层/无硬编码密钥
- [ ] proto 已生成，`buf lint` 通过，前端类型已更新
- [ ] 迁移成对且空库 up/down/up 通过，MySQL 5.7.18 方言；涉及 DB 的改动已验证
- [ ] 单测 + 集成测试通过，新增逻辑有覆盖
- [ ] 权限路径有显式测试（越权返回 404）
- [ ] 前端三态齐全，构建通过，主包/分包体积未超限
- [ ] 云托管部署配置、环境变量样例、健康检查已同步（涉及部署）
- [ ] 文档已同步（需求/数据模型/接口/决策/问题清单/部署）
- [ ] 回复中给出：改动摘要、验证方式与真实输出、遗留问题、回滚方式

## 13. 与用户沟通的规则

- 需要用户定夺的，**一次集中问**，写进 `docs/open-questions.md`，不要在实现中途反复打断。
- 报告进度时区分“已完成并验证”“已完成未验证”“未开始”，不混用。
- 出错时先给现象、根因、修复方案，再动手；不要静默重试或换方案掩盖。
- 用户说“下载太慢/别浪费时间”时，立即停止下载并请用户手动安装，转而做不依赖该工具的工作。

## 14. 上线前确认（其余已闭）

| 编号 | 事项 | 影响 | 状态 |
| --- | --- | --- | --- |
| Q19 | 上线类目与主体路径 | **A：个人主体 + 工具类目先提审**；先去云托管控制台确认能否开通，被拒再变更 |
| Q21 | 教师模式页面范围（是否接受手机批阅/看板） | 交互取舍 | 默认按前端文档实现，可再调 |

已关闭：Q1（小程序教师模式）、Q2（按无手机号组件设计）、Q3（小程序侧不需要服务器域名/域名 ICP 备案；**小程序备案已于 2026-11-22 确认完成**，域名未做 ICP 备案也不影响上线）、Q4（module 路径）、Q5（多老师可同时使用）、Q6（Excel 导入）、Q7、Q8、Q9、Q10、Q11（改为云存储）、Q12、Q13、Q14、Q15（改为云托管）、Q16、Q17、Q18、Q20（生产不再用 SeaweedFS 预签名）。

其余见 [`docs/open-questions.md`](docs/open-questions.md)。



