# 后端需求规格 · HM-Lesson

> 对应业务蓝本：[`鹤鸣老师课堂-飞书家校系统搭建需求与实施清单.docx`](<鹤鸣老师课堂-飞书家校系统搭建需求与实施清单.docx>)（下称“蓝本”）
> 技术基线：Go 1.25+ / Kratos v3 / 微信云托管（上海，单服务 `hm-api`）/ MySQL 5.7.18（本地容器 / 生产云托管，Q22 已确认）/ 云托管对象存储（COS/fileID，生产；本地 MinIO）/ 一期不用 Redis（Q24）
> 读者：后端开发、测试、后续接手的 AI Agent
> 关联：[README](../README.md) · [data-model.md](data-model.md) · [frontend-requirements.md](frontend-requirements.md) · [deploy-wechat-cloudrun.md](deploy-wechat-cloudrun.md) · [open-questions.md](open-questions.md)

---

## 1. 范围与约束

### 1.1 一期范围（P0/P1）

| 模块 | 优先级 | 本文对应 |
| --- | --- | --- |
| M1 客户分层与跟进 | P0 | §3.1 |
| M2 任务下发与作业提交批阅 | P0 | §3.2 |
| M3 一对一预约排课 | P0 | §3.3 |
| M4 课时台账与班课管理 | P0 | §3.4 / §3.5 |
| M5 资料库与权限隔离 | P1 | §3.6 |
| M6 通知与回执 | P1 | §3.7 |
| M7 一对一沟通 | P1 | **已砍掉**：不建 IM/聊天/客服会话；线下微信沟通 + 跟进留痕（§3.1.4） |
| M8 家长门户 | P1 | 由小程序承载，后端提供 `/portal/*` 接口 |
| M9 看板统计与导出 | P1 | §3.8 / §3.9 |
| M10 自动化与提醒中枢 | P0 | §6 |

### 1.2 明确不做

直播、商城/在线支付、**即时通讯（聊天/会话/客服消息/群聊）**、自动最优排课、短信兜底、学情自动分析、录播挂载、多租户 SaaS。助教角色与成员表结构**一期预留但不开放登录**（Q5：以后可扩）。

### 1.3 硬约束

1. 单组织部署；不做多租户，但 `staff` 支持多老师（`owner` / `assistant`），所有写入带 `created_by` / `updated_by`（Q5：现在一人，结构按可扩展做）。
2. 家长只能看到自己绑定学员的数据，**服务端每个查询强制带归属条件**，不依赖前端隐藏。
3. 业务数据不物理删除（学员、任务、预约、流水）；归档用状态字段。
4. 所有时间存 UTC（MySQL `datetime(3)` / PG `timestamptz`），业务日切与提醒按 `Asia/Shanghai`。
5. 所有金额/课时用定点小数（MySQL `decimal(12,2)` / PG `numeric(12,2)`），禁止 `float32/float64`。
6. 外部依赖（微信 API、云存储、数据库、可选 Redis）不可用时，核心写入仍应成功，通知降级为待重试。
7. 微信主体当前为**个人、未认证**（Q2）：不得依赖手机号快速验证；家长绑定用“手动手机号 + 老师审批 / 绑定码”。主体升级后只增加能力分支，不改数据模型。
8. 部署在**微信云托管**（Q15 变更）：容器只监听 80；无持久本地盘；无公网域名依赖；单服务内同时跑 HTTP/WS/job；生产关闭公网访问。
9. 小程序经 `wx.cloud.callContainer` 调 API：**请求 ≤100 KiB、响应 ≤1000 KiB、超时 ≤15 s**；设计接口与分页必须遵守。
10. 文件不经过 API 正文：客户端 `wx.cloud.uploadFile` 直传云存储，只提交 `fileID`；服务端用 COS SDK 校验与读取。

## 2. 架构与代码结构

### 2.1 进程划分

云托管只跑**一个容器**；不再拆 `api` / `job` 两个进程。

| 进程 | 入口 | 职责 | 副本 |
| --- | --- | --- | --- |
| `hm` | `cmd/hm` | HTTP/JSON + WebSocket + 内嵌 job 调度；启动时 `data.Migrate` 版本化迁移；同步业务与定时任务同进程 | MVP `min=max=1`；扩容前必须补分布式锁 |

> 不引入消息队列。Outbox 表 + 行锁/状态机足够支撑本项目量级；`FOR UPDATE SKIP LOCKED` 在 MySQL 5.7.18 与 PG 均可用。长任务（导出、批量通知）落 `jobs`，由内嵌调度器或请求内分片推进，**不依赖请求返回后的 goroutine**。

### 2.2 目录约定（Kratos v3）

```text
backend/
├── api/
│   ├── proto/hm/v1/*.proto        # 唯一定义源
│   └── openapi/hm.swagger.json    # buf 生成，CI 校验
├── cmd/hm/main.go                 # api + ws + 内嵌 job 调度 + 启动迁移
├── internal/
│   ├── biz/                       # 领域模型 + 用例（不依赖 transport）
│   ├── data/                      # GORM 仓储、事务、查询
│   ├── service/                   # proto service 实现（薄）
│   ├── server/                    # http/ws 装配、中间件、微信 header 解析
│   └── conf/                      # proto 配置 + 校验
├── pkg/
│   ├── authz/                     # JWT、密码、guardian scope、教师角色
│   ├── wx/                        # 微信身份 header、订阅消息（不含手机号快速验证）
│   ├── storage/                   # 云托管 COS（fileID）+ 本地/ MinIO dev 适配器
│   ├── notify/                    # Outbox 生产/消费
│   ├── xlsx/                      # 导出
│   ├── jobrunner/                 # 内嵌定时调度 + 数据库锁
│   └── idgen/                     # UUIDv7
├── migrations/                        # 启动时版本化迁移（embed，up/down）
├── Dockerfile                     # 云托管镜像
└── Makefile
```

分层规则：`service → biz → data`，禁止反向依赖；`service` 不写 SQL，`data` 不写业务分支；跨模块调用走 `biz` 接口，不直接查对方表（报表 SQL 例外，集中在 `data/report`）。

### 2.3 模块边界

`auth`、`crm`（学员/家长/标签/跟进）、`task`、`booking`、`lesson`、`class`、`material`、`notice`、`file`、`dashboard`、`export`、`notify`、`settings`。

每个模块一个 proto 文件与一个 `biz` 包；跨模块只通过 ID 与明确的用例方法交互（如 `lesson.ConsumeForBooking(bookingID)`）。

## 3. 模块需求与业务规则

### 3.1 M1 客户分层与跟进

#### 3.1.1 学员档案

- 一个学员一条记录；家长独立成 `guardians`，通过 `student_guardians` 多对多关联（支持父母双方各自登录，`is_primary` 标记主联系人）。**与蓝本的差异**：蓝本“家长字段挂在学员表”，此处规范化，避免同一家长多孩重复与改号遗漏。
- 必填：学员姓名、年级、家长姓名、家长手机号、来源渠道、分组。
- 年级：`g1` / `g2` / `g3` / `other`；分组：`class` / `one_on_one` / `lead` / `visitor`；状态：`active` / `paused` / `archived` / `churned`。
- 来源渠道：`douyin` / `xiaohongshu` / `referral` / `offline` / `other`。
- 手机号唯一（家长维度）；同一手机号重复建学员时复用家长记录并提示。
- 状态流转：`lead → one_on_one|class`、`visitor → lead`、任意在读 → `paused`、`paused → active`、`active|paused → archived|churned`。**禁止物理删除**；归档必须填原因，进入归档后不参与批量派发，但历史任务/课时/预约仍可查。
- 任何学员状态变更写审计日志。

#### 3.1.2 标签

- 标签分类固定四类：`grade` / `paper` / `learning` / `operation`；标签本身可增删（老师维护），但分类不可增。
- `student_tags` 多对多；标签只用于筛选与批量操作，**绝不参与权限判定**。

#### 3.1.3 跟进记录

- 一条沟通一条记录：`occurred_at`、`channel`（`wechat` / `phone` / `meeting` / `other`）、`content`、`concern`、`next_action`、`next_follow_up_at`。
- 新建跟进时若填了 `next_follow_up_at`，同步更新学员的 `next_follow_up_at`（取最近一条）。
- 回访提醒：每日 09:00 扫描 `next_follow_up_at <= 今日 23:59` 且学员状态非归档，生成老师站内待办（AF-14）。同一学员同日只提醒一次（`dedupe_key`）。
- 学员详情时间线聚合：建档、分组变更、跟进、任务、预约、课时、通知回执（只读聚合，分页）。

#### 3.1.4 沟通留痕（M7 已砍）

**不建设聊天系统，也不提供任何 IM 能力**：没有会话、消息发送、客服消息、群聊接口；前端消息中心只读系统通知。老师在微信与家长沟通后，用“跟进记录”留痕；涉及任务、时间、费用、成绩的结论必须落到对应业务表（蓝本第 9 章要求）。后端只提供跟进记录与待办。

#### 3.1.5 视图

提供筛选参数即可，不单独建视图表：全部、班课在读、一对一在读、意向池（`lead` + `visitor`）、待回访（`next_follow_up_at <= now`）。列表默认排除 `archived`。

#### 3.1.6 Excel 导入（Q6）

老师从飞书导出/整理 `.xlsx` 后一次性导入，模板列与飞书截图对齐：

| 列 | 必填 | 映射 | 说明 |
| --- | --- | --- | --- |
| 学员姓名 | ✅ | `students.name` | |
| 年级 | ✅ | `students.grade` | 高一/高二/高三/其他 → `g1/g2/g3/other` |
| 学校 |  | `students.school` | |
| 家长姓名 | ✅ | `guardians.name` | 同手机号复用家长 |
| 家长手机号 | ✅ | `guardians.phone` | 唯一键；同手机号+学员姓名判重 |
| 家长飞书账号 |  | — | **忽略**（平台迁移后无对应字段） |
| 来源渠道 | ✅ | `students.source` | 转介绍/抖音/小红书/线下/其他 |
| 客户分组 | ✅ | `students.group_code` | 班课在读/一对一在读/待转化意向/咨询访客 |
| 自定义标签 |  | `student_tags` | 逗号/顿号分隔，自动建标签（分类 `learning`） |
| 当前状态 | ✅ | `students.status` | 在读/暂停/结课归档/流失 |
| 下次回访时间 |  | `students.next_follow_up_at` | `YYYY-MM-DD`，按 Asia/Shanghai 当日 09:00 |
| 备注 |  | `students.remark` | |

- 接口：`GET /api/v1/imports/students/template`（返回 `{filename, content_base64}`）、`POST /api/v1/imports/students/preview`（JSON `{content_base64, mode}`，≤5MB、≤5000 行，返回逐行错误与批次 id）、`POST /api/v1/imports/{id}/commit`、`GET /api/v1/imports/{id}`、`POST /api/v1/imports/{id}/rollback`。
- **上传通道限制**：小程序 `callContainer` 请求体上限 100 KiB，base64 还会膨胀约 4/3，因此当前实际约 60KB xlsx；前端已加 80KB base64 守卫。大文件后续改用 `wx.cloud.uploadFile`→COS fileID→服务端拉取（M3 接入），接口保持 batch 语义不变。
- 校验：表头必须匹配；手机号按中国大陆 11 位校验，姓名/手机号为空的行整行报错；枚举值中文/英文均可。
- 判重：`guardians.phone` + `students.name` 已存在 → 默认跳过并在报告标注；`mode=update` 预留，当前按 create 处理。
- 写入：一个批次一个事务（原子）；任一行失败整批回滚并记录 `import_batches.error_report`，不产生半成品数据。
- 回滚：只允许回滚“本批次新建、且无任务/预约/流水引用”的学员与家长；否则拒绝并列出引用。
- 权限：仅 `owner`；导入与回滚都写审计日志。

### 3.2 M2 任务下发与作业提交批阅

#### 3.2.1 任务

- 字段：标题、说明、类型（`homework` / `exam_correction` / `trial` / `checkin`）、截止时间、状态、附件（≤10 个）、创建人、发布时间。
- 状态机：`draft → published → closed`。`draft` 可编辑；`published` 可关闭；`closed` 只读。当前实现不含 `archived`。
- 发布校验：至少一个下发对象；`due_at` 必须晚于当前时间。
- 下发对象：当前实现只支持显式 `student_ids`；按分组/标签解析为后续增强。
- **一人一条** `task_assignments`，快照 `student_id`、`due_at`、`status=pending`。单次发布上限 500 人。
- 发布与生成记录在同一事务。

#### 3.2.2 提交

- 入口：家长端。当前实现支持文字（≤5000 字）+ 图片（≤9 张，先 `wx.cloud.uploadFile` 得 fileID）。独立文件附件、批改图片为后续增强。
- 截止后仍可提交，标记 `is_late=true`，不阻断。
- 每次提交生成一条 `submissions`（`attempt_no` 递增），作业记录状态置 `submitted`；保留历史 attempt，老师默认看最新，可展开历史。
- 已批阅（`reviewed`）后不允许再提交；`returned` 可重交。
- 幂等：客户端带 `Idempotency-Key`，24h 内重复请求返回同一结果。

#### 3.2.3 批阅

- 状态机：`pending → submitted → reviewed | returned`；`returned → submitted`（重交）。
- 批阅动作（单条或批量）：评分（0~100，可空）、评语（≤2000 字）、批改图片（≤9 张）、打回理由（**打回必填**）。
- 打回后家长收到通知（AF-04）；批阅完成通知家长（AF-03）。
- 并发：`version` 乐观锁；重复批阅返回 `TASK_ALREADY_REVIEWED`。
- 批阅工作台：按 `due_at` 升序，支持按分组/学员/任务筛选，默认只看 `submitted`。

#### 3.2.4 统计口径（必须与看板一致）

| 指标 | 定义 |
| --- | --- |
| 下发人数 `assigned_count` | 该任务 `task_assignments` 行数 |
| 已提交人数 `submitted_count` | 至少有一次提交的 assignment 数（按人去重） |
| 提交次数 `submission_count` | `submissions` 行数（蓝本“按提交记录计数”的口径） |
| 提交率 | `submitted_count / assigned_count`，分母为 0 时返回 0 |
| 逾期未提交名单 | `status=pending AND due_at < now` 的学员 |
| 待批阅数 | `status=submitted` |

#### 3.2.5 模板与档案

- 模板保存标题、说明、类型、默认截止偏移（1~720 小时），复用时只选对象与截止时间；当前实现为 `task_templates` 表 + 教师端模板选择/存为模板。
- 学员作业档案：家长端 `/api/v1/portal/archive` 已实现（按任务发布时间倒序）；教师端按学员聚合为后续增强。
- 导出：逾期名单、提交名单为后续增强（xlsx job）。

### 3.3 M3 一对一预约排课

#### 3.3.1 时段

- 由老师创建：日期、开始/结束（`timestamptz`）、时长 30~120 分钟、备注。
- 同一老师时段不可重叠（EXCLUDE 约束或事务校验）。
- 状态：`open` / `locked` / `completed` / `cancelled`。`locked` 表示已被预约占用（含待审核）。
- 批量开放：按周模板一次生成多个时段；过去未处理的 `open` 时段由 job 每小时标记 `cancelled`。
- **与蓝本的差异（关键）**：蓝本“审核通过才锁定时段”，会出现两人同时提交、审核时才冲突。本方案在**提交预约时即锁定**（`open → locked`），用数据库唯一约束兜底，保证 TC-08 必然通过；驳回/取消时释放。

#### 3.3.2 预约

- 字段：学员、时段、诉求备注、状态、审核意见、提交时间、审核人/时间、完成时间、消课流水 ID。
- 状态机：`pending → approved | rejected`；`approved → completed | cancelled | no_show`；`pending → cancelled`（家长撤回）。
- 提交校验：
  1. 学员归属当前家长且状态 `active`；
  2. 学员课时余额 > 0（余额 ≤0 时禁止提交，返回 `LESSON_HOURS_EXHAUSTED`）；
  3. 时段 `open` 且未开始；
  4. 该学员无重叠预约（同一天同一时间段）。
- 并发控制：数据库级唯一约束保证“一个时段只有一条有效预约”——PG 用部分唯一索引 `bookings(slot_id) WHERE status IN ('pending','approved')`，MySQL 用生成列 `active_flag` + `UNIQUE(slot_id, active_flag)`（见 [data-model.md](data-model.md) §1.1）；冲突返回 `409 BOOKING_SLOT_TAKEN`；提交成功后时段置 `locked`。
- 审核：通过 → `approved`，通知家长（AF-08）；驳回 → 必填理由 → `rejected`，时段回 `open`，通知家长（AF-09）。
- 改期：不原地修改；先取消旧预约（释放时段），再由家长重新提交，保留两条记录。
- 完成：老师标记 `completed` 并选择出勤（`attended` / `no_show`）。
  - `attended`：同一事务写入一条 `consume` 流水（默认扣时段时长对应课时，可在设置中配置 1 次 = 1 课时或按小时），回写 `lesson_ledger_id`，预约置 `completed`，时段置 `completed`；**幂等**，重复调用返回已有结果。
  - `no_show`：预约置 `no_show`，**不自动扣课时**；老师可手工调整并填写原因。
- 提醒：AF-07（提交提醒老师）、AF-08/09（结果通知）、AF-10（课前 24h）、AF-11（课后未补录，次日 09:00 提醒老师）。
- 家长端只返回 `open` 且未开始的时段（含自己已占用的）；预约记录含状态与审核意见。

#### 3.3.3 日历

- 老师端按周/日查询预约：返回学员、年级、诉求、状态、时段；同一接口供“本周课表”使用。

### 3.4 M4 课时台账

- `lesson_accounts` 每学员一条：`purchased_hours`、`used_hours`、`balance_hours`（缓存值）、`valid_until`、`status`（`active` / `exhausted` / `expired`）。
- `lesson_ledger` 只追加：`type`（`purchase` / `consume` / `adjust` / `refund`）、`delta_hours`（有符号）、`balance_after`、`ref_type` / `ref_id`、`operator_id`、`reason`、`occurred_at`。
- **不改流水**：纠错走 `adjust`，原因必填；`adjust` 需老师权限。
- 余额一致性：
  - 写流水与更新账户余额在同一事务，并对账户行 `SELECT ... FOR UPDATE`；
  - 余额不得为负（默认策略，见 Q8）；扣到 0 时账户置 `exhausted`；
  - 每日 03:00 对账：`balance_hours` 必须等于 `SUM(delta_hours)`，不一致告警并记录（AF 对账任务）。
- 预警：余额 ≤2 提醒老师（AF-12，阈值可配）；≤0 触发门户/预约禁用（AF-13）。
- 有效期：过期由 job 标记 `expired`，预约时校验。

### 3.5 班课

- `classes`：名称、学科、起止日期、上课节奏（自由文本/枚举）、状态。
- `class_members`：加入/退出时间；只允许 `active` 学员加入。
- `class_sessions`：日期、内容要点、出勤记录（`present` / `absent` / `leave`，逐人）。
- 班课**不扣课时**、不产生课时流水；计费在课包维度，系统仅记录出勤与内容。

### 3.6 M5 资料库与权限

- 资料分类（`material_categories`）+ 分组可见矩阵（`material_visibility`：category × group → bool），初始值按蓝本表 7。
- 资料：标题、分类、年级、文件（可多个）、状态（`draft` / `published` / `archived`）、发布时间。
- 家长列表 = `published` 且 `visibility[current_group] = true`；**服务端过滤**，不返回无权资料的任何元数据。
- 下载：后端校验权限后，返回短期有效的云存储下载信息（fileID + 临时链接，5 分钟）或服务端流式转发；记录 `material_download_logs`（谁、何时、哪份）。小程序用 `wx.cloud.downloadFile({fileID})` 获取临时文件后打开。
- 老师可覆盖单个资料的分组可见性（在分类矩阵基础上），但不得越过“保密”语义（二期）。
- 聊天窗口不直接发原文件（蓝本要求），统一走小程序资料页。

### 3.7 M6 通知与回执

- 通知：标题、正文（富文本/纯文本）、附件、发送人群（分组/标签/学员）、是否需要回执、发送时间、状态（`draft` / `sending` / `sent`）。
- 发送时快照受众，按**家长**生成回执（一个家长一条，含其名下相关学员），去重。
- 家长确认 → `confirmed` + 时间；老师可筛选未确认并一键催办；48h 未确认自动再推一次（AF-15），只推一次。
- 回执可导出（姓名、手机号、状态、确认时间）。
- 通知同时进入小程序消息中心；如家长授权了订阅消息则叠加推送。

### 3.8 M9 看板

四组指标（口径固定，写入接口文档与前端文案）：

1. **客户结构**：四类分组人数、本月新增学员数、本月转化率 =（本月由 `lead/visitor` 变为 `class/one_on_one` 的学员数）/（本月新增 `lead/visitor` 数）。
2. **交付质量**：区间内任务数、下发人次、提交率、逾期未提交数、待批阅数。
3. **一对一运营**：待审核预约数、本周排课数、本月课时消耗（`SUM(-delta)` where type=consume）。
4. **待办**：待回访数、已通过但时间已过未完成预约数、未确认回执数、余额 ≤2 学员数。

实现：单条聚合 SQL / 物化视图按需；结果进程内缓存 60 秒（有 Redis 时用 Redis）；`timezone` 固定上海。响应必须小于 100 KB（云托管响应上限 1000 KiB），指标只返回聚合值，名单另走分页/导出。

### 3.9 导出

- 四类：客户名单、作业提交名单（按任务）、回执确认名单、课时流水。
- **全部走异步**：`POST /api/v1/exports` 建 `jobs` + `exports` 记录并立即返回 id；调度器分页查询、生成 xlsx 到云存储，前端轮询状态；完成后返回 `fileID`，小程序 `wx.cloud.downloadFile` → `wx.openDocument`。不占用 15 s 请求窗口。
- 小批量（≤1000 行）可在请求内生成，但响应仍只返回 `fileID`，不直接回传文件字节。
- 格式：`.xlsx`（主）；`.csv`（UTF-8 BOM）仅后台/运维下载，不作为小程序主路径。
- 导出行为写审计（谁、什么条件、多少行）；文件保留 7 天，job 清理。

## 4. 权限模型与安全

### 4.1 角色与主体

- `staff`（老师）：JWT `role=staff`，`sub=staff_id`。角色从 `staff.role` 读取（缓存 60s，权限变更即时失效）：
  - `owner`：全部权限，含成员管理、课时调整、导出、导入、通知发送；
  - `assistant`（结构预留，一期不开放创建入口）：可批阅、下发任务、查看预约；**禁止**访问跟进记录、收费/课时成本、导出、导入、成员管理、设置。
- 一期只创建 `owner`，但 `staff` 表、角色校验、审计必须按多老师实现；新增成员接口 `POST /api/v1/staff` 仅 `owner` 可用。
- `guardian`（家长）：JWT `role=guardian`，`sub=guardian_id`。必须 `status=active`。
- 未绑定/待审批家长只能访问 `/auth/*` 与 `/me`，其余接口返回 `403 GUARDIAN_NOT_APPROVED`；越权资源仍统一 404。

### 4.2 数据隔离（红线）

- 所有家长端查询必须：`JOIN student_guardians sg ON sg.student_id = ... AND sg.guardian_id = :sub`。
- 资源 ID 均为 UUIDv7，不接受客户端传入的 `guardian_id`。
- 越权访问统一返回 `404 NOT_FOUND`（避免枚举），并记录安全审计。
- 代码评审强制检查：新增 `/portal/*` 或 `guardian` 可见接口必须附带隔离测试。

### 4.3 认证流程

**老师**：手机号 + 密码 → Argon2id 校验 → access token（2h）+ refresh token（30d，轮换、哈希入库）。连续失败 5 次锁定 15 分钟（优先用 Redis；未启用时用 `login_attempts` 表）。成员由 `owner` 创建，初始密码首次登录强制修改。

**家长**（Q2：个人主体不能用 `getPhoneNumber`，**禁止**调用手机号快速验证）：

1. 身份来源（云托管）：
   - 主路径：服务端直接读取网关注入的 `X-WX-OPENID` / `X-WX-APPID` / `X-WX-SOURCE`；**不调用** `code2session`。
   - 兜底：H5/本地调试或网关未注入时，走 `wx.login()` 拿 `code` → 后端 `code2session` 换 `openid`。
   - 无论哪条路径，都只信服务端解析出的 openid；禁止客户端自报。
2. 已绑定且 `active` → 直接发 token（无需再走绑定页）；
3. 未绑定/待审批 → 返回 `bind_ticket`（10 分钟，签名），进入绑定页；
4. 家长**手动输入手机号**（可选输入老师给的 6 位绑定码）；
5. 按手机号匹配 `guardians`：
   - **命中且未绑定**：绑定 `openid`，状态 `pending`，等老师审批（主路径：导入 Excel 时已建档）；
   - **命中但已绑定其他 openid**：返回 `409 BIND_CONFLICT`，需老师重置，禁止顶号；
   - **未命中 + 有效绑定码**：用绑定码定位学员，创建/复用家长（需填家长姓名），直接 `active`（绑定码即老师授权），并写审计；
   - **未命中且无码**：提示“请联系老师建档”，**不自动创建学员/家长**。
6. 老师审批通过 → `active`，家长可拉取数据；驳回 → `rejected` 并记录原因。
7. 更换微信：老师端“重置绑定”清除 `wx_openid`，家长重新走 2~6。

**绑定码**：6 位数字，7 天有效，一次性；`POST /api/v1/students/{id}/binding-codes` 生成（owner），绑定成功后 `used_at` 落库；限流 5 次/小时/家长，失败 10 次锁定 1 小时。

**会话**：access token 带 `jti`，登出写数据库 `token_denylist`（或 Redis，若启用）至过期；refresh 轮换，旧 token 立即失效。无 Redis 时只用数据库，不要用进程内黑名单（多实例不一致）。

### 4.4 其它安全要求

- 云托管终止 TLS；生产**关闭公网访问**，只允许小程序/公众号调用；gRPC 与 metrics 不暴露公网（若需，仅内网/VPC）。
- 只信网关注入的 `X-WX-*`；启动时打印实际 header 名（框架可能转小写/驼峰）；校验 `X-WX-SOURCE` 存在。WS 首帧必须应用层 token 认证（iOS「高性能+」可能无 openid header）。
- 密码/密钥只从环境变量注入，`.env` 不入库；日志过滤 `password`、`token`、`phone`（中间四位打码）。
- 数据库账号最小权限：应用账号首次启动需建表权限（或由 DBA 预建表），上线后收回 DDL；数据库不暴露公网。
- 上传：扩展名/MIME/文件头/大小校验；云存储私有读；下载先校验业务权限再发 fileID/临时链接。
- 限流：无 Redis 时用 MySQL 计数表 + 进程内令牌桶（登录 5 次/分/IP、绑定 5 次/小时/家长、通用 60 次/分/主体、上传 20 次/分）；扩多实例前必须换 Redis。
- 审计：登录、审批、批阅、打回、消课、课时调整、导出、资料下载、权限变更。
- 依赖安全：CI 跑 `govulncheck`；镜像每周重建并扫描。
- 隐私：收集手机号、作业图片、成绩均属个人信息；提供隐私政策与最小化收集；家长可申请导出/删除（删除走归档 + 匿名化）。

## 5. 文件与对象存储

存储层抽象为 `storage.Store`；**生产实现为云托管对象存储（COS，fileID）**，本地开发用磁盘/MinIO 适配器。不再以 SeaweedFS 预签名为主路径。

| 项 | 规则 |
| --- | --- |
| 上传方式（唯一主路径） | 小程序 `wx.cloud.uploadFile({cloudPath, filePath})` 直传云托管对象存储，返回 `fileID`；业务接口只提交 `fileID`，服务端用 COS SDK 校验存在性、大小、类型与归属 |
| 服务端落库 | `files` 表存 `storage_key`/`file_id`、原始文件名、MIME、大小、hash、上传者、业务引用；**不存临时路径** |
| 下载方式 | 服务端校验业务权限后，用 `wx.cloud.downloadFile({fileID})` 或短期临时链接；必要时服务端流式转发（小文件），仍先校验权限 |
| 对象键 | `{env}/{yyyy}/{mm}/{uuid}.{ext}`，不使用原始文件名；`cloudPath` 由服务端下发或按规则生成 |
| 原始文件名 | 仅存 DB，下载时通过 `Content-Disposition` 或前端展示名还原 |
| 图片 | jpg / png / webp；单张 ≤10MB；客户端压缩到长边 ≤2000px；服务端解码校验 |
| 文档 | pdf / doc / docx / xls / xlsx / ppt / pptx / zip；单个 ≤30MB |
| 权限 | 桶私有；fileID 不等于授权，所有读取前校验资料分类 × 分组、作业归属、guardian scope |
| 请求体 | 文件绝不走 `callContainer` 正文（100 KiB 上限）；仅传 fileID 与元数据 |
| 清理 | 未被业务引用的 `files` 由 job 每日清理（>24h 且无引用）；导出文件保留 7 天 |
| 配额 | 单用户每日上传 ≤500MB；超限 `429` |
| 备份 | COS 版本控制/跨地域复制（可选）；对象键与 DB `files.storage_key` 必须可对账 |
| 本地开发 | `storage.Store` 的 local/MinIO 实现；生产代码不得写本地持久盘 |

## 6. 通知、自动化与定时任务

### 6.1 Outbox 模型

`notifications` 表即 Outbox：

- 字段：`recipient_type`（`staff` / `guardian`）、`recipient_id`、`channel`（`wx_subscribe` / `inapp` / `ws`）、`template_key`、`payload`（MySQL `JSON` / PG `jsonb`）、`status`（`queued` / `sending` / `sent` / `failed` / `skipped`）、`scheduled_at`、`attempts`、`last_error`、`dedupe_key`（唯一）。
- 生产：业务事务内插入（与业务同提交）。
- 消费：内嵌调度器每 30s 取 `status=queued AND scheduled_at<=now`，按行锁/状态机抢占（`FOR UPDATE SKIP LOCKED`），批量 100；指数退避 1m/5m/30m/2h，最多 5 次，超限 `failed` 并告警。
- 站内信 = `channel=inapp` 且 `status=sent`，家长/老师消息中心直接查该表。

### 6.2 微信订阅消息

- 一次性订阅：在家长主动操作（提交作业、提交预约、确认回执）时调用 `wx.requestSubscribeMessage`，把授权次数记录到 `wx_subscriptions`。
- 发送时若无可用次数 → 降级 `inapp` + WebSocket，标记 `skipped`，不重试。
- 模板 ID 通过配置注入（`WX_TEMPLATE_*`），禁止硬编码。
- 长期订阅仅对特定类目开放，不能假设可无限推送（风险见 §11）。

### 6.3 自动化清单（对应蓝本 AF-01 ~ AF-15）

| 编号 | 触发 | 实现 | 收件人 | 模板键 |
| --- | --- | --- | --- | --- |
| AF-01 | 任务发布 | 同步事件 → outbox | 家长 | `task_assigned` |
| AF-02 | 家长提交作业 | 同步事件 | 老师 | `task_submitted_staff` |
| AF-03 | 批阅完成 | 同步事件 | 家长 | `task_reviewed` |
| AF-04 | 打回 | 同步事件 | 家长 | `task_returned` |
| AF-05 | 截止前 12h 仍待提交 | cron 每 10 分钟 | 家长 | `task_due_reminder` |
| AF-06 | 截止后次日 09:00 | cron 每日 | 家长逾期单条 + 老师汇总 | `task_overdue` / `task_overdue_digest_staff` |
| AF-07 | 家长提交预约 | 同步事件 | 老师 | `booking_submitted_staff` |
| AF-08 | 审核通过 | 同步事件 | 家长 | `booking_approved` |
| AF-09 | 审核驳回 | 同步事件 | 家长 | `booking_rejected` |
| AF-10 | 课前 24h | cron 每 10 分钟 | 家长 | `booking_reminder` |
| AF-11 | 预约时间已过且未完成 | cron 每日 09:00 | 老师 | `booking_complete_reminder_staff` |
| AF-12 | 余额 ≤2 | 流水后检查（dedupe 每日一次） | 老师 | `hours_low_staff` |
| AF-13 | 余额 ≤0 | 流水后检查 | 家长 + 老师 | `hours_empty` |
| AF-14 | 回访日到达 | cron 每日 09:00 | 老师 | `follow_up_due_staff` |
| AF-15 | 回执 48h 未确认 | cron 每小时 | 家长一次 + 老师汇总 | `receipt_reminder` / `receipt_digest_staff` |

补充内部任务：时段过期（每小时）、账户过期（每日）、余额对账（每日 03:00）、孤儿文件清理（每日 04:00）、导出任务（事件驱动）、Outbox 重试（30s）。

### 6.4 定时任务选主

- MVP 云托管 `min=max=1`，天然单实例；但仍用数据库锁防止发布重叠：
  - MySQL：`SELECT GET_LOCK('hm:job', 30)`；PG：`pg_try_advisory_lock(hashtext('hm:job'))`。
- 每个任务再用 `jobs` 表的行锁/`locked_until` 抢占，保证即使多实例也不重复执行。
- 定时触发可用云托管控制台 cron；若用进程内 ticker，必须确保最小实例 ≥1 且不会被缩容到 0。
- 锁连接断开即释放；任务执行时间必须小于锁租期，长任务分片并续租。

## 7. 协议与 API 设计

### 7.1 Proto 与代码生成

- 目录：`api/proto/hm/v1/{common,auth,crm,task,booking,lesson,class,material,notice,file,dashboard,notify,settings}.proto`。
- `buf.yaml` + `buf.gen.yaml`：
  - `protoc-gen-go`、`protoc-gen-go-grpc`
  - `protoc-gen-go-http`（Kratos HTTP 路由）
  - `protoc-gen-openapi`（生成 swagger，供前端生成 TS 类型）
- `option go_package = "github.com/bishop9910/hm-lesson/backend/gen/hm/v1;hmv1";`（Q4 已定：`github.com/bishop9910/hm-lesson`）。
- 兼容性：CI 跑 `buf breaking --against '.git#branch=main'`；字段只增不删、不改号；枚举新增值放末尾。
- v3 注意：HTTP JSON 编解码显式导入 `_ "github.com/go-kratos/kratos/v3/encoding/protojson"`；日志用 `log/slog`。

### 7.2 传输

| 协议 | 入口 | 用途 | 鉴权 |
| --- | --- | --- | --- |
| HTTP/JSON | 云托管服务 `hm-api`，容器 80，`/api/v1/*` | 小程序（家长端 / 教师模式）经 `wx.cloud.callContainer` | Bearer JWT + 网关注入 `X-WX-OPENID` |
| WebSocket | 同一服务，`/api/v1/ws` | 消息中心实时推送，经 `wx.cloud.connectContainer` | 首帧应用层 token；网关 openid 仅作辅助 |
| gRPC | 仅进程内/内网工具 | 内部调用、运维 CLI | 不暴露公网 |
| Health | `/healthz`（不查库）、`/readyz`（查库/存储） | 云托管探针 | 无 |
| Metrics | `/metrics` | 指标 | 仅内网/关闭公网后不可达 |

> 不再有 Caddy 443、8000/9000 端口映射；云托管容器只监听 80。小程序侧**不需要配置服务器域名**。

### 7.3 通用约定

- JSON：`snake_case`；时间 RFC3339（UTC，`Z`）；ID 为 UUID 字符串；空值用 `null`。
- 分页：`page`（默认 1）、`page_size`（默认 20，最大 100）；响应 `{items, total, page, page_size}`。
- 排序：`sort` 白名单（如 `created_at`、`due_at`），`order=asc|desc`。
- 幂等：`Idempotency-Key` 头用于提交作业、创建预约、消课、导出、导入提交；存 `idempotency_keys` 表（key + actor + route + request_hash + response，24h），唯一约束 `(actor_id, key)`；重复请求返回首次结果。无 Redis 时也成立。
- 请求体：JSON ≤100 KiB（云托管硬限）；列表页 `page_size` 默认 20；响应 ≤1000 KiB，超限说明必须分页或改文件。
- 超时：单请求 p95 <1s，硬上限 15s；导出/批量导入/群发通知一律异步。
- 时间：服务端只接受 UTC；前端显示按 `Asia/Shanghai`。金额 `int64` 分。
- 错误：Kratos 标准 `{code, reason, message, metadata}`；HTTP 状态与 `code` 对应；`reason` 为稳定英文标识。

| HTTP | reason 示例 | 场景 |
| --- | --- | --- |
| 400 | `INVALID_ARGUMENT` | 参数/状态非法 |
| 401 | `UNAUTHENTICATED` / `TOKEN_EXPIRED` | 未登录/过期 |
| 403 | `PERMISSION_DENIED` / `GUARDIAN_NOT_APPROVED` | 越权/待审批 |
| 404 | `NOT_FOUND` | 资源不存在或无权可见 |
| 409 | `BOOKING_SLOT_TAKEN` / `STATE_CONFLICT` / `TASK_ALREADY_REVIEWED` | 并发/状态冲突 |
| 422 | `LESSON_HOURS_EXHAUSTED` / `VALIDATION_FAILED` | 业务校验失败 |
| 429 | `RATE_LIMITED` | 限流 |
| 500 | `INTERNAL` | 未预期错误（不泄露细节） |

### 7.4 接口清单

> 角色：`S`=老师，`G`=家长，`-`=公开。所有 `/portal/*` 仅 `G`；除注明外均为 `S`。

#### 认证与账号

| 方法 | 路径 | 角色 | 说明 |
| --- | --- | --- | --- |
| POST | `/api/v1/auth/staff/login` | - | 手机号+密码 |
| POST | `/api/v1/auth/wx/login` | - | code 换 token 或 bind_ticket |
| POST | `/api/v1/auth/wx/bind` | - | bind_ticket + **手动手机号** + 可选绑定码（不调用 getPhoneNumber） |
| POST | `/api/v1/auth/refresh` | - | 轮换 refresh |
| POST | `/api/v1/auth/logout` | S/G | 吊销 |
| GET | `/api/v1/me` | S/G | 当前主体、绑定状态、孩子列表 |
| GET | `/api/v1/ws/ticket` | S/G | 换取 WS 短时 ticket |
| GET/POST | `/api/v1/staff` | S(owner) | 成员列表 / 新增（Q5 预留） |
| PATCH | `/api/v1/staff/{id}` | S(owner) | 改角色/禁用（不可禁用最后一个 owner） |
| POST | `/api/v1/auth/bind-request` | - | 家长绑定申请落库（手机号，待老师审批；M1 已实现） |
| GET | `/api/v1/dashboard/summary` | S | 首页/工作台真实汇总（M1 已实现） |

> 当前代码已实现：`POST /api/v1/auth/staff/login`（兼容 `/auth/login`）、`GET /api/v1/auth/me`、`GET/POST /api/v1/staff`、`PATCH /api/v1/staff/{id}`、`POST /api/v1/auth/bind-request`、`GET /api/v1/dashboard/summary`；M2 已实现 `GET/POST /api/v1/students`、`GET/PATCH /api/v1/students/{id}`、`GET/POST /api/v1/students/{id}/follow-ups`、`POST /api/v1/students/{id}/binding-codes`、`GET /api/v1/guardians/pending`、`POST /api/v1/guardians/{id}/approve|reject`、`POST /api/v1/auth/guardian/login`、`GET /api/v1/portal/home`、`GET /api/v1/portal/students/{id}`；M3 已实现 `GET/POST /api/v1/tasks`、`GET /api/v1/tasks/{id}`、`POST /api/v1/tasks/{id}/publish|close`、`GET /api/v1/reviews`、`POST /api/v1/assignments/{id}/review`、`GET/POST /api/v1/task-templates`、`PATCH/DELETE /api/v1/task-templates/{id}`、`GET /api/v1/portal/tasks`、`GET /api/v1/portal/tasks/{id}`、`POST /api/v1/portal/tasks/{id}/submit`、`GET /api/v1/portal/archive`。表中其余接口为规划，尚未实现，前端不得假设可用。

#### 学员 / 家长 / 标签 / 跟进（S）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET/POST | `/api/v1/students` | 列表（筛选 group/status/tag/q）、建档 |
| GET/PATCH | `/api/v1/students/{id}` | 详情、编辑 |
| POST | `/api/v1/students/{id}/archive` | 归档（必填原因） |
| GET | `/api/v1/students/{id}/timeline` | 聚合时间线 |
| GET/POST | `/api/v1/students/{id}/follow-ups` | 跟进记录 |
| PUT | `/api/v1/students/{id}/tags` | 覆盖标签 |
| GET | `/api/v1/guardians` | 家长列表（含绑定状态） |
| POST | `/api/v1/guardians/{id}/approve` | 审批绑定 |
| POST | `/api/v1/guardians/{id}/reject` | 拒绝绑定 |
| POST | `/api/v1/guardians/{id}/reset-binding` | 重置微信绑定 |
| POST | `/api/v1/students/{id}/binding-codes` | 生成 6 位绑定码（7 天、一次性） |
| GET/POST | `/api/v1/tags` | 标签维护 |
| GET | `/api/v1/follow-ups/upcoming` | 待回访 |
| GET | `/api/v1/imports/students/template` | 下载导入模板（base64 xlsx） |
| POST | `/api/v1/imports/students/preview` | 上传 base64 xlsx，返回逐行校验 |
| POST | `/api/v1/imports/{id}/commit` | 提交预览批次 |
| GET | `/api/v1/imports/{id}` | 批次详情 + 错误报告 |
| POST | `/api/v1/imports/{id}/rollback` | 回滚本批次新建数据 |

#### 任务（S）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET/POST | `/api/v1/tasks` | 列表 / 建草稿 |
| GET | `/api/v1/tasks/{id}` | 详情 + 统计 + 逐份作业 |
| POST | `/api/v1/tasks/{id}/publish` | 发布并生成 assignment |
| POST | `/api/v1/tasks/{id}/close` | 截止 |
| GET | `/api/v1/reviews` | 待批阅队列（跨任务） |
| POST | `/api/v1/assignments/{id}/review` | 批阅 / 打回（`return_reason` 非空即打回） |
| GET/POST | `/api/v1/task-templates` | 模板列表 / 新建 |
| PATCH/DELETE | `/api/v1/task-templates/{id}` | 模板维护 |
| GET | `/api/v1/portal/tasks` | 家长任务列表 |
| GET | `/api/v1/portal/tasks/{id}` | 家长任务详情 + 历史提交 |
| POST | `/api/v1/portal/tasks/{id}/submit` | 家长提交/重交 |
| GET | `/api/v1/portal/archive` | 家长作业档案 |

#### 预约 / 时段

| 方法 | 路径 | 角色 | 说明 |
| --- | --- | --- | --- |
| GET/POST | `/api/v1/slots` | S | 列表 / 新建 |
| POST | `/api/v1/slots/batch` | S | 批量开放 |
| PATCH/DELETE | `/api/v1/slots/{id}` | S | 修改 / 取消（未被占用） |
| GET | `/api/v1/bookings` | S | 待审核/全部 |
| POST | `/api/v1/bookings/{id}/approve` | S | 通过 |
| POST | `/api/v1/bookings/{id}/reject` | S | 驳回（必填理由） |
| POST | `/api/v1/bookings/{id}/complete` | S | 完成 + 出勤 + 消课 |
| POST | `/api/v1/bookings/{id}/no-show` | S | 未到课 |
| GET | `/api/v1/calendar/bookings` | S | 周/日视图 |
| GET | `/api/v1/portal/slots` | G | 可预约时段 |
| POST | `/api/v1/portal/bookings` | G | 提交预约 |
| GET | `/api/v1/portal/bookings` | G | 我的预约 |
| POST | `/api/v1/portal/bookings/{id}/cancel` | G | 撤回（限 pending） |

#### 课时 / 班课（S）

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/v1/lesson-accounts` | 列表（含预警） |
| GET | `/api/v1/lesson-accounts/{student_id}` | 账户详情 |
| POST | `/api/v1/lesson-accounts/{student_id}/purchase` | 充值（写流水） |
| POST | `/api/v1/lesson-accounts/{student_id}/adjust` | 调整（必填原因） |
| GET | `/api/v1/lesson-ledger` | 流水（按学员/月份） |
| GET/POST | `/api/v1/classes` | 班级 |
| GET/PATCH | `/api/v1/classes/{id}` | 班级详情 |
| POST | `/api/v1/classes/{id}/members` | 加/移除学员 |
| GET/POST | `/api/v1/classes/{id}/sessions` | 上课记录 |
| PATCH | `/api/v1/class-sessions/{id}/attendance` | 出勤 |

#### 资料 / 通知 / 文件

| 方法 | 路径 | 角色 | 说明 |
| --- | --- | --- | --- |
| GET/POST | `/api/v1/material-categories` | S | 分类与可见矩阵 |
| PUT | `/api/v1/material-categories/{id}/visibility` | S | 更新矩阵 |
| GET/POST | `/api/v1/materials` | S | 资料 |
| PATCH/DELETE | `/api/v1/materials/{id}` | S | 上下架/归档 |
| GET | `/api/v1/portal/materials` | G | 有权资料 |
| GET | `/api/v1/portal/materials/{id}/download` | G | 校验后返回 fileID/临时链接 |
| POST | `/api/v1/files/register` | S/G | 云存储直传后登记 fileID、大小、MIME、归属 |
| GET | `/api/v1/files/{id}/download` | S/G | 校验权限后返回 fileID/短期临时链接 |
| POST | `/api/v1/files/upload` | S/G | 仅 H5/本地调试的服务端流式上传（≤30MB，生产小程序不走） |
| GET/POST | `/api/v1/notices` | S | 通知 |
| POST | `/api/v1/notices/{id}/send` | S | 发送 |
| GET | `/api/v1/notices/{id}/receipts` | S | 回执名单 |
| POST | `/api/v1/notices/{id}/remind` | S | 催办 |
| GET | `/api/v1/portal/notices` | G | 我的通知 |
| POST | `/api/v1/portal/notices/{id}/confirm` | G | 确认回执 |

#### 家长端任务 / 消息 / 看板

| 方法 | 路径 | 角色 | 说明 |
| --- | --- | --- | --- |
| GET | `/api/v1/portal/home` | G | 家长首页聚合：孩子、待办数、5 卡片摘要 |
| GET | `/api/v1/portal/students/{id}` | G | 家长端学员详情：剩余课时、作业状态、课时流水（M2 已实现） |
| GET | `/api/v1/portal/tasks` | G | 待办/已批阅 |
| GET | `/api/v1/portal/tasks/{assignment_id}` | G | 详情 |
| POST | `/api/v1/portal/tasks/{assignment_id}/submissions` | G | 提交/重交 |
| GET | `/api/v1/portal/archive` | G | 作业档案 |
| GET | `/api/v1/portal/lesson-account` | G | 课时与近期课表 |
| GET | `/api/v1/notifications` | S/G | 消息中心 |
| POST | `/api/v1/notifications/read` | S/G | 已读 |
| GET | `/api/v1/dashboard/summary` | S | 四组指标 |
| GET | `/api/v1/dashboard/todos` | S | 待办 |
| POST | `/api/v1/exports` | S | 创建导出 |
| GET | `/api/v1/exports/{id}` | S | 导出状态/下载 |
| GET/PUT | `/api/v1/settings` | S | 业务参数 |
| GET | `/api/v1/audit-logs` | S | 审计（只读） |

> 完整字段与请求/响应结构以 proto + OpenAPI 为准；上表只定义边界，避免前后端各自发明接口。

### 7.5 WebSocket

- 连接（小程序）：`wx.cloud.connectContainer({config:{env}, service:'hm-api', path:'/api/v1/ws'})`，不经过域名，也不需要配置 socket 合法域名。
- 认证：连接建立后客户端**首帧**发 `{"type":"auth","token":"<access_token>"}`；服务端校验通过前不下发任何业务帧，5 秒未认证断开。网关 `X-WX-OPENID` 存在时可作二次校验，但**不能作为唯一凭据**（iOS「高性能+」读不到）。
- H5/调试：可用 `GET /api/v1/ws/ticket` 换 60 秒一次性 ticket，再以 `?ticket=` 连接；小程序路径不需要 ticket。
- 服务端帧：`{"type":"notification.created","data":{...},"ts":"..."}`、`ping`。
- 客户端帧：`{"type":"pong"}`。
- 心跳 30s，2 次无响应断开；断线重连指数退避（1s→30s），重连后拉 `/notifications?since=` 补齐。
- 单实例用内存连接表；多实例需 Redis Pub/Sub（预留接口）；云托管必须 `min≥1` 且连接数受实例规格限制。

## 8. 配置与环境变量

`configs/config.yaml`（非敏感默认值）+ 环境变量覆盖（`HM_` 前缀）；云托管用控制台环境变量注入，**不提交真实值**。

```yaml
server:
  http_addr: 0.0.0.0:80        # 云托管只监听 80
  ws_path: /api/v1/ws
  read_timeout: 10s
  write_timeout: 15s           # 不得超过 callContainer 15s
  shutdown_timeout: 15s
data:
  database:
    driver: mysql              # 固定 MySQL 5.7.18（Q22 已确认）
    source: ${HM_DB_DSN}
    max_open: 20
    max_idle: 10
    conn_max_lifetime: 30m
  redis:                       # 可选；不配则走 DB + 进程内
    addr: ${HM_REDIS_ADDR}
    password: ${HM_REDIS_PASSWORD}
storage:
  driver: cloud                # cloud | local（dev）
  bucket: ${HM_STORAGE_BUCKET}
  region: ${HM_STORAGE_REGION}
  local_dir: ./tmp/uploads     # 仅 dev
auth:
  jwt_secret: ${HM_JWT_SECRET}
  access_ttl: 2h
  refresh_ttl: 720h
wx:
  app_id: ${HM_WX_APP_ID}
  app_secret: ${HM_WX_APP_SECRET}   # 仅订阅消息/code2session 兜底需要
  template_ids:
    task_assigned: ${HM_WX_TPL_TASK_ASSIGNED}
rules:
  task_due_reminder_hours: 12
  booking_reminder_hours: 24
  receipt_reminder_hours: 48
  follow_up_hour: 9
  low_balance_threshold: 2
  booking_hours_per_session: 1.0
```

| 变量 | 必填 | 说明 |
| --- | --- | --- |
| `HM_ENV` | ✅ | `dev` / `staging` / `prod` |
| `HM_DB_DSN` | ✅ | MySQL：`user:pass@tcp(host:3306)/hm?parseTime=true&charset=utf8mb4&loc=UTC`；PG：`postgres://...` |
| `HM_DB_DRIVER` | ✅ | 固定 `mysql`（Q22 已确认） |
| `HM_JWT_SECRET` | ✅ | ≥32 字节随机；生产不得有默认值 |
| `HM_STORAGE_DRIVER` | ✅ | `cloud`（生产）或 `local`（dev） |
| `HM_STORAGE_BUCKET` / `HM_STORAGE_REGION` | ✅（cloud） | 云托管对象存储桶与地域（上海） |
| `HM_WX_APP_ID` | ✅ | 小程序 AppID |
| `HM_WX_APP_SECRET` | ❌ | 仅订阅消息/兜底登录需要；callContainer 主路径不需要 |
| `HM_WX_TPL_*` | ❌ | 订阅消息模板；未配置则只发站内信 |
| `HM_REDIS_ADDR` / `HM_REDIS_PASSWORD` | ❌ | 不配则禁用 Redis，限流/缓存走 DB + 进程内 |
| `HM_HTTP_ADDR` | ❌ | 默认 `0.0.0.0:80` |
| `HM_LOG_LEVEL` | ❌ | 默认 `info` |
| `HM_TIMEZONE` | ❌ | 默认 `Asia/Shanghai`；存库仍用 UTC |

- 微信身份 header（`X-WX-OPENID` 等）由网关注入，**不需要配置**；启动时打印一次实际 header 名用于确认。
- 启动时校验必填项，缺失直接退出并打印变量名（不打印值）。
- 生产关闭“公网访问”；`HM_ENV=prod` 时若检测到公网访问开启，打印高优先级告警。

## 9. 非功能需求

| 类别 | 目标 |
| --- | --- |
| 性能 | 读接口 P95 < 200ms、写接口 P95 < 400ms（不含上传/导出）；首包 < 1s；单请求硬上限 15s |
| 容量 | 500 学员 / 2000 任务 / 3000 预约/年；50 并发；单表 < 500 万行；响应 ≤1000 KiB |
| 可用性 | 99.5%/月；云托管 `min=max=1` 避免冷启动；优雅停机 15s；发布滚动/灰度 |
| 冷启动 | MySQL Serverless 关闭自动暂停（或最小 CCU）；Go 进程启动 < 3s；`data.Migrate` 幂等，DDL 只在新表时执行 |
| 备份 | 数据库每日全量（腾讯云侧，保留 7 天 + 每周 4 份）；COS 版本控制；每月恢复演练 |
| 恢复 | RPO ≤ 24h，RTO ≤ 4h（单服务）；建表失败拒绝启动，可回滚上一版本镜像 |
| 可观测 | JSON 日志（`request_id`、`trace_id`、`actor`、`route`、`latency_ms`、`status`）；云托管日志检索；`/healthz`、`/readyz`；关键业务指标写 DB |
| 可维护 | 配置外置（环境变量）；建表可重复执行；模块边界清晰；关键路径有注释与测试 |
| 合规 | 隐私政策、最小收集、日志脱敏、数据导出/删除流程；小程序隐私协议声明 |
| 平台限制 | 请求 100 KiB / 响应 1000 KiB / 超时 15 s；无持久盘；请求后不可依赖后台线程；WS 连接数受实例规格限制 |

## 10. 验收用例映射

| 用例 | 场景 | 自动化层 | 关键断言 |
| --- | --- | --- | --- |
| TC-01 | 新增咨询客 | API 集成 | 学员/家长落库，分组 `visitor`，意向池可见 |
| TC-02 | 分组流转 | API 集成 | 变更留痕（审计/时间线），历史跟进仍在 |
| TC-03 | 任务下发 | API 集成 | 按人生成 assignment，Outbox 生成家长通知 |
| TC-04 | 作业提交 | API 集成 | 状态 `submitted`，批阅工作台可见 |
| TC-05 | 批阅与打回 | API 集成 | 打回必填理由；状态回 `pending`；通知含理由 |
| TC-06 | 逾期统计 | 集成 + 时间注入 | 逾期名单准确、可导出 |
| TC-07 | 时段开放 | API 集成 | 家长端只返回 open 时段 |
| TC-08 | 预约冲突 | **并发集成测试** | 两请求仅一成功，另一 409；数据库级约束生效（MySQL 生成列唯一索引） |
| TC-09 | 预约驳回 | API 集成 | 时段回 open，理由入库并通知 |
| TC-10 | 课前提醒 | job 测试 | 24h 前生成通知，收件人正确 |
| TC-11 | 课时扣减 | 集成 + 幂等 | 流水 +1 条，余额减少，重复调用不重复扣 |
| TC-12 | 余额闸门 | API 集成 | 余额 0 时提交预约 422，门户显示提示 |
| TC-13 | 资料隔离 | 权限矩阵测试 | 意向组只见试听/招生资料 |
| TC-14 | 行权限 | **安全测试** | A 家长访问 B 学员资源全 404 |
| TC-15 | 回执统计 | API 集成 | 未确认名单准确、可催办、48h 只再推一次 |
| TC-16 | 看板对账 | 集成 | 指标与明细 SQL 完全一致 |
| TC-17 | Excel 导入 | API 集成 | 10 行样例全部落库；重复行跳过；回滚后无残留（**新增，非蓝本**） |

## 11. 测试策略与质量门禁

- **单元测试**：`biz` 层覆盖率 ≥70%；状态机、余额计算、权限判定必须表驱动。
- **集成测试**：testcontainers-go 起 MySQL 5.7.18 + MinIO 本地适配器；存储用本地适配器，云存储只做契约测试；每个测试独立 schema 或事务回滚；空库 up/down/up 必须通过。**本机无 Docker**：集成测试在 Debian/CI 上跑，Windows 只跑单元/契约测试。
- **并发测试**：同槽位 20 并发提交、消课 10 并发、重复幂等键。
- **契约测试**：`buf lint` + `buf breaking`；OpenAPI diff 需人工确认；`callContainer` 请求/响应大小与 15 s 超时边界。
- **云托管测试**：网关注入 header 解析、WS 首帧认证、fileID 上传/下载、关闭公网后不可直连、冷启动健康检查。
- **安全测试**：越权矩阵（家长 × 资源 × 动作）、JWT 篡改/过期、上传伪造 MIME、限流、客户端伪造 `X-WX-OPENID` 被忽略。
- **性能测试**：k6 50 VU 混合场景 10 分钟，P95 达标；单响应 ≤1000 KiB。
- **CI 门禁**：`gofmt`、`go vet`、`golangci-lint`、`go test -race`、集成测试、镜像构建、`govulncheck`。

## 12. 风险与扩展路线

### 12.1 风险

| 风险 | 影响 | 缓解 |
| --- | --- | --- |
| 微信订阅消息一次性、类目限制 | 关键提醒触达不足 | 站内信 + 小程序红点 + 教师模式待办；二期短信 |
| 个人主体未认证（Q2） | `getPhoneNumber` 不可用；工具类目提审可能被要求企业主体/补充资质 | 绑定改“手机号+审批/绑定码”，不依赖该组件；定位与文案按工具类（无培训/招生/售课/IM）；先按个人主体+工具类目提审，被拒再变更（Q19） |
| 手机号仅凭手动输入被冒绑 | 家长数据泄露 | 绑定先 `pending`，老师按手机号后四位/姓名审批；一 openid 一 guardian；失败限速 + 审计；绑定码一次性 |
| 微信云托管锁定 | 迁移成本、计费变化、地域仅上海 | 保持容器化与标准 MySQL、S3 抽象；DB/存储都可外接；不写平台专有 API（除微信身份 header/订阅消息） |
| MySQL 方言 | 部分唯一索引/JSONB/RETURNING 需改写 | 已在 `migrations/*.sql` 落地生成列唯一索引；SQL 集中在 `data` 层，禁用 PG 专有语法 |
| AppID/主体变更 | openid 变化，家长绑定全部失效 | 先确认 AppID 是否变更再建生产环境（Q26）；若不变更则直接建；变更前只做体验版；绑定数据可重置但需公告 |
| 云托管无持久盘/会缩容 | 本地文件丢失、后台任务中断 | 文件全走 COS；长任务落 `jobs` + 定时触发；`min=max=1`；禁止请求后 goroutine |
| 时段并发 | 撞车 | 数据库级唯一约束 + 事务 + 409（TC-08 覆盖） |
| 余额与流水不一致 | 对账争议 | 同事务 + 每日对账 + 只能追加流水 |
| 云托管 MySQL 自动暂停 | 冷启动报错 `CynosDB serverless instance is resuming...` | 生产关闭自动暂停/设最小 CCU；启动重试 + 友好错误页 |

### 12.2 扩展路线（不改表结构优先）

1. **拆分服务**：一期单进程内嵌 `job`；规模上来后把 `notify`、`export`、`report` 拆为独立服务/容器，proto 与 biz 直接复用；拆出前先用 `jobs` 表解耦。
2. **消息队列**：Outbox 消费者可切换到 NATS/Kafka/腾讯云 CKafka（生产者不变）。
3. **读扩展**：数据库加只读实例；看板走只读/汇总表。
4. **多租户**：新增 `org_id` 列 + 复合索引 + 行级过滤；当前不使用，但避免硬编码单老师 ID。
5. **存储**：云托管 COS → 独立 COS 桶/跨地域复制，仅改 `storage.Store` 配置。
6. **实时**：WS 多实例时接 Redis Pub/Sub；当前单实例内存表。
7. **教师端**：若手机批阅成为瓶颈，把 `pkg-teacher` 页面复用为独立 Web（同一套 API 与 TS 类型）；一期不做。
8. **数据库**：若先选 MySQL、后迁 PG，保持 SQL 在 `data` 层、建表脚本独立，按“双写/导出导入”迁移；不要跨层写方言 SQL。



