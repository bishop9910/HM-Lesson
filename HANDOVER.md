# HANDOVER · 给下一个接手 HM-Lesson 的 Agent

> 先读 [`AGENTS.md`](AGENTS.md)、[`TODO.md`](TODO.md)，再读本文件。
> 当前基线：`main` @ `8cc6f4b`；工作区未提交改动含 `TODO.md`、`HANDOVER.md`、README 索引与微信订阅消息整套代码，尚未提交。
> 最后更新：2026-11-25 13:40

---

## 1. 三十秒了解项目

HM-Lesson（鹤鸣作业本）是给独立学科老师用的**工具类微信小程序**：

- 家长：看孩子作业、提交作业、看批阅结果、看课时、看资料；
- 老师：学员 CRM、任务/作业/批阅、预约/课时（待做）、资料（待做）、看板（待做）；
- 一个 uni-app 小程序：家长端在主包，教师模式在 `pkg-teacher` 分包；
- 后端：Go Kratos v3 单服务 `hm-api`（实际部署名 `hm-lesson`），HTTP/JSON；
- 生产：微信云托管（上海），MySQL 5.7.18，COS；
- 不做：即时通讯/聊天、支付、录播、短信。

当前状态：**M1 骨架 + 多老师 + M2 CRM/Excel 导入 + M3 任务/作业/批阅/通知已上线并生产验证**；微信订阅消息已实现（配置模板 ID 后才真正发送，未配置 no-op），待真机验收。M4 之后未做。

---

## 2. 生产与仓库事实

| 项 | 值 |
| --- | --- |
| 仓库 | `https://github.com/bishop9910/HM-Lesson`，分支 `main` |
| 云托管服务 | `hm-lesson` |
| 环境 ID | `prod-d8gq8vzftc39caa5b` |
| 临时公网 URL | `https://hm-lesson-353410-10-1540142477.sh.run.tcloudbase.com`（仅联调，上线前关闭） |
| 数据库 | 腾讯云 MySQL `5.7.18-cynos-2.1.14-log`，库 `hm_lesson` |
| 存储 | 云托管 COS，fileID |
| 缓存 | 一期不用 Redis |
| owner | 已创建（手机号见云托管/私聊，不在仓库） |
| 测试数据 | `test` 学员、`13000000000` 家长、已关闭旧任务（清理前问用户） |

**不要把任何密钥写进仓库**：`HM_JWT_SECRET`、`HM_WX_APP_SECRET`、MySQL 密码、云托管私钥都只在控制台。

---

## 3. 本地启动与验证

### 后端

```powershell
cd D:\codes\HM-Lesson\backend
gofmt -l .
go build ./...
go vet ./...
go test ./... -count=1
```

如果沙箱拒绝用户目录缓存：

```powershell
$env:GOCACHE = "$env:TEMP\hm-gocache"
$env:GOMODCACHE = "$env:TEMP\hm-gopath\pkg\mod"
```

本机 **没有 Docker/MySQL**，所以只能跑单元测试；集成/迁移实测在 CI 或 Debian。

### 前端

```powershell
cd D:\codes\HM-Lesson\frontend
pnpm install
pnpm --filter @hm/mp typecheck
pnpm --filter @hm/mp build:mp-weixin
```

产物：

```text
frontend/apps/mp/dist/build/mp-weixin
```

微信开发者工具导入这个 **dist 目录**，不要导入 `apps/mp` 源码目录。

### 环境注意

- Windows 下直接往 `backend/`、`frontend/` 子目录用 shell 重定向写文件可能被 DSH 沙箱拒绝；用 `write` 工具，或单次 `danger-full-access` 执行。
- `pnpm --filter` 有时触发 pnpm 自检锁冲突；可直接用 `apps/mp/node_modules/.bin/vue-tsc.CMD` 与 `uni.CMD`。
- 云托管公网 URL 用 Node `fetch` 测；PowerShell schannel 曾报 `SEC_E_NO_CREDENTIALS`。

---

## 4. 代码地图

```text
backend/
├── cmd/hm/main.go            # 唯一入口：Kratos 路由、DB 初始化、迁移、bootstrap
├── migrations/               # 000001~000005，embed，启动自动 up
├── internal/
│   ├── biz/                  # auth/crm/task/import/dashboard
│   ├── data/                 # mysql repo、迁移 runner、staff、crm、task、import
│   ├── service/              # HTTP handler / DTO / 权限
│   └── conf/                 # HM_* + 云托管 MYSQL_*/COS_* 映射
├── pkg/{password,token,xlsximport}
└── go.mod                    # Kratos v3、GORM、excelize

frontend/apps/mp/src/
├── pages/home/index.vue      # 当前首屏（不是 pages/index）
├── pages/bind/               # 家长绑定申请
├── pages/guardian-login/     # 家长登录（手机号+可选绑定码）
├── pages/portal/             # 家长门户、学员详情、任务、档案
├── pkg-teacher/pages/        # 登录、工作台、成员、学员、任务、批阅、导入
├── utils/request.ts          # callContainer / H5 回退，staff/guardian 双 token
├── utils/cloud.ts            # wx.cloud.uploadFile 图片
├── stores/session.ts         # 本地 token/role + refresh()
└── styles/tokens.scss        # 纸面主题令牌
```

### 路由与 token

- 教师 token：`hm_token`，角色 `owner|assistant`
- 家长 token：`hm_guardian_token`，角色 `guardian`
- 请求只走 `utils/request.ts`；微信端 `wx.cloud.callContainer`，`X-WX-SERVICE: hm-lesson`

### 已实现接口（HTTP/JSON）

```text
POST /api/v1/auth/login
GET  /api/v1/auth/me
GET/POST /api/v1/staff
PATCH /api/v1/staff/{id}

GET/POST /api/v1/students
GET/PATCH /api/v1/students/{id}
GET/POST /api/v1/students/{id}/follow-ups
POST /api/v1/students/{id}/binding-codes
GET  /api/v1/guardians/pending
GET  /api/v1/guardians                 # 家长账号总览（owner；含学员名、has_password、last_login_at、student_count）
POST /api/v1/guardians/{id}/approve    # 可带 {student_id, password}；留空返回 initial_password
POST /api/v1/guardians/{id}/reject
POST /api/v1/guardians/{id}/password   # owner 重置家长密码
DELETE /api/v1/guardians/{id}          # 仅无在读学员；409 GUARDIAN_HAS_STUDENTS
POST /api/v1/auth/guardian/login       # {phone,password} 或 {phone,code,password}
POST /api/v1/auth/guardian/password    # 家长自助改密
POST /api/v1/auth/password             # 老师自助改密（owner/assistant）

GET  /api/v1/dashboard/summary
POST /api/v1/auth/bind-request

GET  /api/v1/portal/home
GET  /api/v1/portal/students/{id}
GET  /api/v1/portal/tasks
GET  /api/v1/portal/tasks/{assignmentId}
POST /api/v1/portal/tasks/{assignmentId}/submit
GET  /api/v1/portal/archive
GET  /api/v1/portal/notifications
POST /api/v1/portal/notifications/{id}/read
POST /api/v1/portal/notifications/read-all

GET/POST /api/v1/tasks
GET  /api/v1/tasks/{id}
POST /api/v1/tasks/{id}/publish
POST /api/v1/tasks/{id}/close
DELETE /api/v1/tasks/{id}          # draft 或 closed
GET  /api/v1/reviews
POST /api/v1/assignments/{id}/review

GET/POST /api/v1/task-templates
PATCH/DELETE /api/v1/task-templates/{id}

GET  /api/v1/imports/students/template
POST /api/v1/imports/students/preview
GET  /api/v1/imports/{id}
POST /api/v1/imports/{id}/commit
POST /api/v1/imports/{id}/rollback

GET/POST /api/v1/slots                 # 教师开放时段（POST 201）
POST /api/v1/slots/batch               # {start_at,end_at,weekdays[],weeks} 按周批量
PATCH/DELETE /api/v1/slots/{id}        # 仅 open 可改/可取消
GET  /api/v1/bookings                  # ?status=&from=&to=
POST /api/v1/bookings/{id}/approve
POST /api/v1/bookings/{id}/reject      # {reason} 必填
POST /api/v1/bookings/{id}/complete    # 标记已上课并消课，幂等，返回 {ok,lesson_ledger_id,hours}
POST /api/v1/bookings/{id}/no-show     # 不扣课时
GET  /api/v1/calendar/bookings         # 目前与 bookings 同源

GET  /api/v1/portal/slots
GET/POST /api/v1/portal/bookings       # POST {student_id,slot_id,request_note}
POST /api/v1/portal/bookings/{id}/cancel
GET  /api/v1/portal/lesson-account     # 家长可见的课时余额与流水

GET  /api/v1/lesson-accounts                       # items + threshold(=2)
GET  /api/v1/lesson-accounts/{studentId}           # account + ledger
POST /api/v1/lesson-accounts/{studentId}/purchase  # {hours,reason}（owner）
POST /api/v1/lesson-accounts/{studentId}/adjust    # {delta_hours,reason}（owner，原因必填）
GET  /api/v1/lesson-ledger                         # ?student_id=&from=&to=
```

---

## 5. 关键约定与踩过的坑

1. **家长任务 id 是 assignment id**
   `/portal/tasks/{id}` 里的 `id` 是 `task_assignments.id`，不是 `tasks.id`。前端任务列表拿到的就是这个 id；用 task id 会 404。

2. **`student_guardians` 没有 status 列**
   家长状态在 `guardians.status`。历史 SQL 曾写 `sg.status='active'`，已修；新增查询不要重犯。

3. **家长 DTO 不能复用教师 DTO**
   教师 `studentDTO` 含 `note/tags/next_follow_up_at`。家长必须走 `toPortalStudentDTO`，否则泄露老师备注（D-13）。

4. **Kratos `fail()` 只能写一次响应**
   `service/auth.go` 的 `fail()` 返回 `errHandled` 并设置 `X-HM-Handled: 1`；`cmd/hm/main.go` 的 ErrorEncoder 检查该 header。否则会双份 JSON。

5. **任务删除是级联的**
   `draft|closed` 可删；事务清理 `submissions → task_assignments → tasks`，并清理通知。`published` 必须先结束。

6. **通知是站内通知 + 微信订阅消息**
   `notifications` 表 + 家长门户弹窗是主路径；微信订阅消息已接（见 §7），未配置模板 ID 时自动退化。

7. **小程序首屏是 `pages/home/index`**
   `pages/index/index.vue` 已不注册；改首页不要改错文件。所有 `reLaunch` 指向 `pages/home/index`。

8. **底部弹层**
   四个弹层 `.sheet` 都有 `max-height:88vh; overflow-y:auto` + `@click.stop`；否则微信里点输入框会被遮罩当点击关闭。

9. **微信 WXSS 不要依赖 flex `gap`**
   工作台曾因 `gap` + 50% 宽度导致每行只剩一个；用 `space-between` + 48.5% 或 margin。

10. **云托管环境变量修改 ≠ 重新构建**
    改环境变量生成的是“配置版本”，复用旧镜像。代码改动必须让云托管从新 commit 构建；构建后确认镜像 tag 变化。

11. **迁移**
    启动时 `data.Migrate` 用同一连接 `GET_LOCK('hm_migrate',30)`；当前 9 个版本。新增迁移必须成对 up/down，`migrate_test.go` 的期望数量要同步。

12. **`CREATE TABLE IF NOT EXISTS` 会静默骗过迁移（2026-11-25 线上事故）**
    `000002_teaching` 早就用旧列集建过 `slots/bookings/lesson_accounts/lesson_ledger`，`000007` 用 `IF NOT EXISTS` 建同名的四张表 → 一句都没执行、迁移版本却照样记为已执行，线上 5 个接口全部 `Error 1054 Unknown column`。教训：**新建表前先 `grep` 一遍 migrations 里有没有同名表**；改已存在的表必须走 `ALTER`，不能用 `CREATE TABLE IF NOT EXISTS` 蒙过去。修复见 `000008_fix_booking_lesson_tables`（DROP 空表后按新结构重建）。

13. **MySQL 5.7**
    没有部分唯一索引、没有 RETURNING/ON CONFLICT/JSONB；`CHECK` 会被解析但忽略，业务层必须校验。字符集 `utf8mb4_unicode_ci`。

14. **MySQL 会话时区必须钉成 UTC（2026-11-25 线上发现）**
    云托管 MySQL 的 `time_zone` 是 `+08:00`，`NOW(3)` / `CURRENT_TIMESTAMP` 写进 `datetime` 的是**本地墙上时间**，而 Go 侧写入的是 UTC → 同一列里差 8 小时（实测 `bookings.reviewed_at` 比 `lesson_ledger.occurred_at` 快 8 小时）。`data.Open` 现在会通过 `withUTCSession` 给 DSN 追加 `time_zone='+00:00'`（go-sql-driver 在握手时执行 `SET time_zone`），已显式配过就不覆盖。**自己拼 `HM_DB_DSN` 时要手动带上这个参数**，否则时间又会混。

15. **家长登录已要密码，旧账号必须先设密（2026-11-25，ADR-030）**
    `guardians.password_hash` 默认空串表示“还没设密码”。这类账号**不能**密码登录（返回 `PASSWORD_REQUIRED`），必须先用绑定码登录并当场设密，或由 owner 在「家长账号 → 重置密码」补设。审批通过时一定要把 `initial_password` 转告家长（只返回一次）。
    删除家长账号是**物理删除**，会连同 `notifications`/`notification_deliveries`/`subscription_grants`/`student_guardians` 一起清掉；`lesson_ledger` 等业务流水保留。仍绑定在读学员时返回 409。

16. **家长账号随学员同步**
    学员 `status` 改成 `graduated`/`lost` 时，`data/crm.go` 的 `syncGuardianStatus` 会把“已无在读学员”的家长置为 `disabled`。因此「家长账号」页的“可清理”提示依赖学员状态正确维护；只在 `archived_at` 层面归档而没有改 `status` 的学员不会触发同步。

---

## 6. 最近完成（不要重复做）

- 多老师 owner/assistant、成员管理、Argon2id、JWT；
- 家长绑定申请、审批、6 位绑定码、guardian 登录、家长门户；
- CRM 学员列表/详情/跟进/标签/回访、Excel 导入（预览/提交/回滚）；
- M3 任务：创建/发布/关闭/删除、一人一条 assignment、提交（文字+图片）、打回重交、批阅评分/评语/批改图、统计、模板、作业档案；
- 站内通知：发布/批阅/打回写 `notifications`，家长门户弹窗 + 已读；
- 微信订阅消息：`000006` 两张表 + `pkg/wx` + 额度/队列 worker + 家长门户「开启提醒」（2026-11-25，见 §7）；
- 家长账号体系：手机号+密码登录（`000009`）、审批设初始密码、家长/老师自助改密、owner 账号总览/重置密码/物理删除、学员状态同步停用（2026-11-25，见 §5.15–5.16）；
- 首页/工作台/页面纸面主题、长按批量操作、弹层修复、拍照/相册。

生产已验证：M2/M3 主链路、逾期提交、删除、通知、批改图字段、家长隐私 DTO（D-13 随后提交）。
**未真机验证**：相机拍照、`wx.cloud.uploadFile`、家长通知弹窗点按、红色块真机。

---

## 7. 已完成：微信订阅消息（2026-11-25，待模板 ID + 真机验证）

代码已全部落地，**未配置模板时整条链路 no-op**（只发站内通知），所以没有模板 ID 也不影响发布/批阅。

**接入点（改这些地方之前先读 ADR-029）**

| 位置 | 作用 |
| --- | --- |
| `backend/migrations/000006_subscription_messages.{up,down}.sql` | `subscription_grants`（额度）+ `notification_deliveries`（队列） |
| `backend/internal/conf/conf.go` | `WXSubscribeTemplates / WXTemplateLang / WXMiniprogramState`；`ParseSubscribeTemplates()` 解析 JSON |
| `backend/pkg/wx/client.go` | `access_token` 进程内缓存（提前 5 分钟刷新）、`IsPermanent()` 错误分类 |
| `backend/pkg/wx/subscribe.go` | `SendSubscribe(ctx, SubscribeMessage)`，40001/42001 失效重试一次 |
| `backend/internal/biz/subscribe.go` | 用例：`Status` / `Grant` / `Enabled` / `DeliverOnce` / `RunWorker`；模板渲染与截断 |
| `backend/internal/data/subscribe.go` | 额度 upsert、`ClaimDeliveries`（FOR UPDATE + status='sending'）、发送结果回写 |
| `backend/internal/service/subscribe.go` | `PortalSubscriptions`(GET) / `GrantSubscriptions`(POST)，reason `INVALID_TEMPLATE_KEY` |
| `backend/internal/service/crm.go` `GuardianLogin` | 登录成功后调 `BindGuardianOpenID`（需要 `X-WX-SOURCE` 非空） |
| `backend/internal/service/task.go` `PublishTask`/`ReviewAssignment` | 业务成功后再落投递（best-effort，日志 warn，不回滚业务） |
| `backend/cmd/hm/main.go` | `buildSubscriptions()` 组装；`kratos.AfterStart` 起 `subs.RunWorker(ctx)` |
| `frontend/apps/mp/src/pages/portal/index.vue` | 提醒卡片 + `wx.requestSubscribeMessage` + 回传 `grant_subscriptions` |

**配置（云托管环境变量，改完必须重新构建/发布）**

```json
HM_WX_SUBSCRIBE_TEMPLATES={"task_published":{"id":"","fields":{"thing1":"title","time2":"due_at"}},"task_reviewed":{"id":"","fields":{"thing1":"title","number2":"score"}},"task_returned":{"id":"","fields":{"thing1":"title","thing2":"student"}}}
```

可选变量：`HM_WX_TMPL_LANG`（默认 `zh_CN`）、`HM_WX_MP_STATE`（默认 `formal`，联调可 `trial`/`developer`）。
可用变量名：`title` / `student` / `date` / `due_at` / `score`；关键词前缀决定截断（thing≤20、phrase≤5、name≤10）。

**额度与重试语义（别改错）**

- 一次性订阅：家长 accept 一次 → `remaining +1`（上限 3）；发送成功 `-1`；失败**不退还**。
- `remaining = 0` 时不入队；站内通知照旧写。
- worker 每 30 秒取 `pending` 且 `next_attempt_at <= now` 的行；失败按 1/2/4 分钟退避（上限 10 分钟），`attempts >= 3` 置 `failed`。
- 永久错误（`43101` 用户拒收、`47003` 参数/关键词错、`40003` openid 无效、`40037` 模板 ID 错、`41030` page 无效）直接 `failed`，不重试。
- 发送成功/失败只更新投递行，**不影响** `tasks / task_assignments` 状态。

**仍需做的验收**

- [x] `HM_WX_APP_ID` / `HM_WX_APP_SECRET` 已在云托管配置（2026-11-25）；启动日志 `微信 access_token 获取成功，订阅消息可发送` 即证明可用，失败会打 `Warn` 并带 `wx.APIError` 的 code；
- [ ] 运营方给出剩余 2 个模板 ID（「已批改」「打回」）并补进 `HM_WX_SUBSCRIBE_TEMPLATES`；「发布作业」已配 `qV_nVYIcFpaTP71yWyu-6Ln1haEHfb5UEk93UkPKebQ`；
- [ ] 家长真机点“开启提醒”→ 教师发布 → 微信服务通知到达；
- [ ] 点通知进入正确作业（`pages/portal/tasks/detail?id=<assignment_id>`）；
- [ ] 拒绝/未配置时业务成功、不重复弹。

---

## 7.5 已完成：M4 一对一预约与课时（2026-11-25）

**迁移**：`backend/migrations/000007_booking_lesson.up.sql` / `.down.sql`（`slots`、`bookings`、`lesson_accounts`、`lesson_ledger`）+ **`000008_fix_booking_lesson_tables.up.sql` / `.down.sql`（必须一起跑，否则线上全是 1054）**；`internal/data/migrate_test.go` 期望 8 个迁移。表结构细节见 `docs/data-model.md` §3.4；事故原因见 §5 第 12 条。

**代码接入点**

| 文件 | 职责 |
| --- | --- |
| `backend/internal/biz/booking.go` | 时段校验（30~120 分钟、不能过去）、批量开放（weekdays 1..7 / weeks 1..26）、预约状态机用例 |
| `backend/internal/biz/lesson.go` | 课时充值/调整校验、`LowBalanceThreshold = 2`、账户与流水聚合 |
| `backend/internal/data/booking.go` | `FOR UPDATE` 锁时段 + 锁行状态机；`CreateBooking` 校验学员归属/余额/重叠；`CompleteBooking` 幂等消课 |
| `backend/internal/data/lesson.go` | `ApplyLedger` 事务：行锁 → 算余额 → 插流水 → 更新账户（不变量 `balance = purchased - used`） |
| `backend/internal/service/booking.go` | 时段/预约 DTO 与错误码映射（409 `BOOKING_SLOT_TAKEN`/`STATE_CONFLICT`、422 `LESSON_HOURS_EXHAUSTED`） |
| `backend/internal/service/lesson.go` | 课时查询/充值/调整（owner）与家长视图 `PortalLessonAccount` |
| `backend/cmd/hm/main.go` | 21 条路由 + `HM_LESSON_HOURS_PER_SESSION`（默认 1） |
| `frontend/apps/mp/src/pages/portal/booking.vue` | 家长：选学员 → 挑时段 → 提交（可写诉求）→ 我的预约/撤回 |
| `frontend/apps/mp/src/pkg-teacher/pages/slots/index.vue` | 教师：开放单条 / 按周批量 / 取消空时段 |
| `frontend/apps/mp/src/pkg-teacher/pages/bookings/index.vue` | 教师：待审核/已通过/已完成/全部，通过、驳回（必填理由）、消课、未到课 |
| `frontend/apps/mp/src/pkg-teacher/pages/lessons/index.vue` | 教师：账户列表（≤2 标红）、充值/调整（原因必填）、最近流水 |

**语义（别改错）**

- 提交即锁时段：`UPDATE slots SET status='locked' WHERE status='open'` + `bookings.active_flag` 生成列上的 `uq_bookings_active_slot`，冲突 409。
- 驳回/家长撤回 → 时段回 `open`；完成/未到课 → 时段 `completed`。
- `complete` 是幂等入口：已消过课直接返回既有 `lesson_ledger_id`；`no_show` **不扣课时**（Q8）。
- 余额 ≤0（`minBalance = 0.01`）时提交预约返回 422；`valid_until` 过期或 `status='expired'` 同样拒绝。
- 家长 token 只能操作自己绑定学员的预约，越权返回 404（不泄露存在性）。

**未做/待跟进**

- [x] 真实 MySQL 5.7 上验证：2026-11-25 部署后发现 `000007` 被 `IF NOT EXISTS` 静默跳过（`000002` 已建过同名旧表），已用 `000008` 重建四张表，待重新发布后复测接口；
- [ ] 预约通知 AF-07/08/09：`notifications.guardian_id` 目前 NOT NULL，无法给老师发通知，需迁移支持 staff 维度；
- [ ] 班课（班级名单、上课记录、不扣课时）；
- [ ] 课前 24h 提醒（AF-10，需定时任务 + 订阅消息模板）。

---

## 8. 部署与排错

- 推送后到云托管控制台确认**新版本/新镜像 tag**；只改环境变量不会重建。
- 健康检查：`/healthz`、`/readyz`；`/readyz` 查 DB。
- 迁移在启动时自动执行；失败看容器日志里的 `执行数据库迁移失败`。
- 临时公网测试用 Node：

```powershell
node -e "fetch('https://.../healthz').then(r=>r.text()).then(console.log)"
```

- 云托管日志里 `已创建初始负责人` 只在首次出现；`HM_BOOTSTRAP_PASSWORD` 用完应删除。

---

## 9. 交接检查清单

- [ ] `git status` 干净；HEAD 与远端一致
- [ ] `go build/vet/test` 通过
- [ ] `pnpm typecheck`、`build:mp-weixin` 通过
- [ ] 云托管部署的是最新 commit（看镜像 tag）
- [ ] `/healthz`、`/readyz`、owner 登录、家长登录通过
- [ ] 确认没有把 `HM_*` 密钥写进仓库
- [ ] 读 `TODO.md` 的 P0：先做**上线安全收尾**（关公网、删 bootstrap 密码、换 DB 账号）与真机验收；订阅消息只差模板 ID 与真机
