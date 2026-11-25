# HANDOVER · 给下一个接手 HM-Lesson 的 Agent

> 先读 [`AGENTS.md`](AGENTS.md)、[`TODO.md`](TODO.md)，再读本文件。
> 当前版本 **v1.3.6**（第一版第 36 次发布）；代码基线 `main` @ `e5ba70e`，M6/M7/M8/M9 已推送并在测试环境跑通，本轮后续修补（资料发布校验、按钮/输入框样式）尚未提交。
> 最后更新：2026-11-25 22:50

---

## 0. 本轮交接（v1.3.6）——先看这一节

### 0.1 这一轮做完了什么

| 模块 | 内容 | 迁移 | 验证程度 |
| --- | --- | --- | --- |
| M4 | 一对一预约 + 课时台账（时段/审批/消课幂等/未到课不扣） | `000007` `000008` | 线上探针全通过 |
| M5-a | 学员删除（两次确认，有无流水区别对待）+ 内部系统账号 `system` | `000010` | 线上探针通过 |
| M5-b | 资料库：六分类 + 分类×分组可见矩阵 + 多附件 + 下载日志 | `000011` | 线上探针通过（含分组不可见负向用例），真机下载待验 |
| M6 | 通知：发布→回执→催办→48h 再推→回执 xlsx | `000012` | ✅ 线上全链路（家长 `13000000000`）：建→发送→家长确认→教师看到 confirmed→xlsx 6518B |
| M7 | 班课：班级/成员/课次与出勤（不扣课时） | `000012` | ✅ 线上写链路：建班→加成员→记课→详情→删课次→归档 |
| M8 | 家长门户五卡（待交作业/待确认回执/剩余课时/待上预约/可看资料） | — | 线上 `pending_receipts` 正确；真机核对待做 |
| M9 | 看板四组（客户结构/交付质量/一对一运营/待办，60 秒缓存） | — | ✅ 线上 `groups=4` |
| 其他 | 家长账号体系（密码登录/审批设密/账号清理）、批量删除、owner 保护、system 审计与控制台 | `000009` `000010` | 已上线验证 |

迁移总数 **12**。本地验证全绿：`gofmt -l .` 无输出、`go build/vet/test` 0、`vue-tsc --noEmit` 0、`uni build -p mp-weixin` DONE。
线上探针：`scripts/dev/probe-m6.mjs` **24/24**；`scripts/dev/probe-notice.mjs` **22/23**（唯一未过项 `publish without files rejected` 是待发布的新校验）。

### 0.2 下一轮的第一件事（按顺序）

1. **重新发布一次**：带「资料发布前必须有附件」校验（`400 MATERIAL_NO_FILES`，`biz/material.go` 的 `ErrMaterialNoFiles`），发布后 `probe-notice.mjs` 应 23/23。
2. **小程序重新导入** `frontend/apps/mp/dist/build/mp-weixin`：包含输入框 88rpx 修复与资料库按钮改 view 磁贴。
3. **真机验收**（详见 `TODO.md` P0-4）：相机上传、资料下载、通知回执、班课记课、门户五卡与看板数字。
4. **订阅消息收尾**：等「作业已批改」「作业被打回」两个模板审核通过后，把 ID + 关键词字段名填进 `HM_WX_SUBSCRIBE_TEMPLATES`（`docs/deploy-wechat-cloudrun.md` §7.1）。云调用通道已打通（`HM_WX_CALL_MODE=cloud`）。
5. **上线安全收尾**：关闭云托管公网访问、删 `HM_BOOTSTRAP_PASSWORD`、换掉 root 数据库账号、上传文件补服务端校验。
6. **测试数据清理**：旧任务 `test`、4 条「回执冒烟」通知、2 个已归档空班级、时区脏数据，清理前先问用户（或走 system 控制台）。

### 0.3 这一轮踩过、下一轮别再踩的坑

- **`CREATE TABLE IF NOT EXISTS` 会静默骗过迁移**：`000007` 因为 `000002` 早建过同名表而被整段跳过，导致线上 5 个接口 1054 报错；修复方式是 `000008` 先 `DROP` 再建。新建表前一定先 `grep` 一遍 `backend/migrations`。
- **`000012` 只做增量 ALTER**：班课/通知五张表 `000002` 已建，M6/M7 不再建表。
- **MySQL 5.7 的 JSON 列不吃 binary，也不吃空串**：写 `file_ids`/`attendance`/`payload` 必须 `string(payloadJSON)`（否则 3144）；空列表要写 `"[]"`、空对象要写 `"{}"`（空串报 **3140**，症状是「列表 200、新建 500、日志里没有 1054」）。
- **时区**：DSN 已自动带 `time_zone='+00:00'`（`data.withUTCSession`），自拼 `HM_DB_DSN` 时要手动带，否则 SQL `NOW(3)` 会写本地墙上时间。
- **含中文的源码只能用 `write`/`edit` 工具改**，用 PowerShell `Set-Content` 会按 ANSI 读坏文件（会报 `string literal not terminated`）。
- **小程序 `scroll-view` 横向滚动**要给直接子节点绑确定宽度（控制台结果表就是这样修的）。
- **原生 `<input>` 只给 padding 会裁字**：必须 `height: 88rpx; line-height: 88rpx; padding: 0 22rpx; box-sizing: border-box`；**flex 里放原生 `<button>` 会被压变形**，工具条统一用 `<view class="btn">` 磁贴（`flex:1; height:80rpx; line-height:80rpx; text-align:center; white-space:nowrap`）。

---

## 1. 三十秒了解项目

HM-Lesson（鹤鸣作业本）是给独立学科老师用的**工具类微信小程序**：

- 家长：看孩子作业、提交作业、看批阅结果、看课时、看资料、看通知并确认回执、自己预约上课；
- 老师：学员 CRM、任务/作业/批阅、预约/课时、资料库、班课、通知回执、看板、家长账号管理；
- 一个 uni-app 小程序：家长端在主包，教师模式在 `pkg-teacher` 分包；
- 后端：Go Kratos v3 单服务 `hm-api`（实际部署名 `hm-lesson`），HTTP/JSON；
- 生产：微信云托管（上海），MySQL 5.7.18，COS；
- 不做：即时通讯/聊天、支付、录播、短信。

当前状态：**M1–M9 全部落盘（v1.3.6）**。M1/M2/M3/M4/M5 已上线并验证；M6/M7/M8/M9 待部署与真机验证；订阅消息待两个模板审核。

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
DELETE /api/v1/students/{id}           # 仅 owner；前端两次确认；有课时流水时 409 STUDENT_HAS_LESSON_LEDGER，需 ?force=1
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

M5/M6/M7/M9 与 system 后台（2026-11-25 新增）：

```text
GET/POST /api/v1/material-categories            # 六分类 + 分组可见性
PATCH    /api/v1/material-categories/{id}
PUT      /api/v1/material-categories/{id}/visibility
GET/POST /api/v1/materials
GET/PATCH/DELETE /api/v1/materials/{id}
POST     /api/v1/materials/{id}/publish | /archive
GET      /api/v1/material-downloads             # owner，下载日志
GET      /api/v1/portal/materials
POST     /api/v1/portal/materials/{id}/download # 先记日志再返 fileID

GET/POST /api/v1/classes
GET/PATCH /api/v1/classes/{id}                  # 详情含 members/sessions
POST     /api/v1/classes/{id}/archive
POST     /api/v1/classes/{id}/members
DELETE   /api/v1/classes/{id}/members/{studentId}
GET/POST /api/v1/classes/{id}/sessions
DELETE   /api/v1/classes/{id}/sessions/{sessionId}

GET/POST /api/v1/notices
GET/PATCH/DELETE /api/v1/notices/{id}           # 仅草稿可改/删
POST     /api/v1/notices/{id}/send
GET      /api/v1/notices/{id}/receipts
POST     /api/v1/notices/{id}/remind
GET      /api/v1/notices/{id}/receipts.xlsx     # owner
GET      /api/v1/portal/notices
POST     /api/v1/portal/notices/{id}/confirm

GET      /api/v1/dashboard/board                # ?from=&to=，60 秒进程内缓存

GET      /api/v1/system/audit                   # 仅 system（staff.hidden）
GET      /api/v1/system/permissions
POST     /api/v1/system/query                   # 只读 SQL，单语句 ≤4000 字符、≤200 行、8s

POST     /api/v1/students/batch-delete          # {ids,force}，单次 ≤50
POST     /api/v1/guardians/batch-delete
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
    启动时 `data.Migrate` 用同一连接 `GET_LOCK('hm_migrate',30)`；当前 12 个版本（`000001`~`000012`）。新增迁移必须成对 up/down，`migrate_test.go` 的期望数量要同步。

12. **`CREATE TABLE IF NOT EXISTS` 会静默骗过迁移（2026-11-25 线上事故）**
    `000002_teaching` 早就用旧列集建过 `slots/bookings/lesson_accounts/lesson_ledger`，`000007` 用 `IF NOT EXISTS` 建同名的四张表 → 一句都没执行、迁移版本却照样记为已执行，线上 5 个接口全部 `Error 1054 Unknown column`。教训：**新建表前先 `grep` 一遍 migrations 里有没有同名表**；改已存在的表必须走 `ALTER`，不能用 `CREATE TABLE IF NOT EXISTS` 蒙过去。修复见 `000008_fix_booking_lesson_tables`（DROP 空表后按新结构重建）。`000011_materials` 里同样先 `DROP TABLE IF EXISTS materials`（000002 留了旧形态单文件表），否则资料库会整体被跳过。

13. **MySQL 5.7**
    没有部分唯一索引、没有 RETURNING/ON CONFLICT/JSONB；`CHECK` 会被解析但忽略，业务层必须校验。字符集 `utf8mb4_unicode_ci`。JSON 列传参必须用 `string(payload)`，传 `[]byte` 会命中 `3144`（binary 字符集不被接受）。

14. **MySQL 会话时区必须钉成 UTC（2026-11-25 线上发现）**
    云托管 MySQL 的 `time_zone` 是 `+08:00`，`NOW(3)` / `CURRENT_TIMESTAMP` 写进 `datetime` 的是**本地墙上时间**，而 Go 侧写入的是 UTC → 同一列里差 8 小时（实测 `bookings.reviewed_at` 比 `lesson_ledger.occurred_at` 快 8 小时）。`data.Open` 现在会通过 `withUTCSession` 给 DSN 追加 `time_zone='+00:00'`（go-sql-driver 在握手时执行 `SET time_zone`），已显式配过就不覆盖。**自己拼 `HM_DB_DSN` 时要手动带上这个参数**，否则时间又会混。

15. **家长登录已要密码，旧账号必须先设密（2026-11-25，ADR-030）**
    `guardians.password_hash` 默认空串表示“还没设密码”。这类账号**不能**密码登录（返回 `PASSWORD_REQUIRED`），必须先用绑定码登录并当场设密，或由 owner 在「家长账号 → 重置密码」补设。审批通过时一定要把 `initial_password` 转告家长（只返回一次）。
    删除家长账号是**物理删除**，会连同 `notifications`/`notification_deliveries`/`subscription_grants`/`student_guardians` 一起清掉；`lesson_ledger` 等业务流水保留。仍绑定在读学员时返回 409。

16. **家长账号随学员同步**
    学员 `status` 改成 `graduated`/`lost` 时，`data/crm.go` 的 `syncGuardianStatus` 会把“已无在读学员”的家长置为 `disabled`。因此「家长账号」页的“可清理”提示依赖学员状态正确维护；只在 `archived_at` 层面归档而没有改 `status` 的学员不会触发同步。

17. **删除学员是不可撤销的物理删除（2026-11-25，ADR-031）**
    `DELETE /api/v1/students/{id}` 仅 owner 可调用，前端必须两次确认。后端按外键顺序删 `submissions`→`notifications`→`task_assignments`→`bookings`→`lesson_ledger`→`lesson_accounts`→`binding_codes`→`follow_ups`→`class_members`→`student_guardians`→`students`，再把相关家长置为未绑定。学员存在课时流水时返回 409 `STUDENT_HAS_LESSON_LEDGER`，必须带 `?force=1` 才会删流水——**默认路径不允许静默丢账**。新增引用 `students(id)` 的表时，务必同步把删除顺序写进 `data/crm.go` 的 `DeleteStudent`，否则删除会因外键失败。

18. **系统账号是隐藏的 owner（2026-11-25，ADR-031）**
    `staff.hidden=1` 的账号（用户名固定 `system`）只在登录时以 owner 身份出现，`GET /api/v1/staff` 与工作台 `staff_total` 都不包含它，`PATCH /api/v1/staff/{id}` 对它返回 403 `SYSTEM_ACCOUNT_LOCKED`（改密走 `POST /api/v1/auth/password`）。初始密码取 `HM_SYSTEM_PASSWORD`：只在账号**首次创建**或 `password_hash` 为空时写入，之后自助改的密码不会被重启覆盖。

---

## 6. 最近完成（不要重复做）

- 多老师 owner/assistant、成员管理、Argon2id、JWT；
- 家长绑定申请、审批、6 位绑定码、guardian 登录、家长门户；
- CRM 学员列表/详情/跟进/标签/回访、Excel 导入（预览/提交/回滚）；
- M3 任务：创建/发布/关闭/删除、一人一条 assignment、提交（文字+图片）、打回重交、批阅评分/评语/批改图、统计、模板、作业档案；
- 站内通知：发布/批阅/打回写 `notifications`，家长门户弹窗 + 已读；
- 微信订阅消息：`000006` 两张表 + `pkg/wx` + 额度/队列 worker + 家长门户「开启提醒」（2026-11-25，见 §7）；
- 家长账号体系：手机号+密码登录（`000009`）、审批设初始密码、家长/老师自助改密、owner 账号总览/重置密码/物理删除、学员状态同步停用（2026-11-25，见 §5.15–5.16）；
- 学员删除（`000010` + `DELETE /api/v1/students/{id}`）：owner 两次确认，连带清理作业/预约/绑定，家长回到未绑定；有课时流水需 `?force=1`（ADR-031）；
- 内部系统账号（`000010` `staff.hidden`）：用户名固定 `system`，仅内部测试与调试用，不在成员列表与统计中出现；初始密码取 `HM_SYSTEM_PASSWORD`，可自助改密；
- M5 资料库（`000011`）：六分类 + 分组可见矩阵、`file_ids` 多附件、家长下载写 `material_download_logs`（见 §7.6）；
- M7 班课、M6 通知回执 + 导出、M9 看板、M8 门户五卡（`000012`，见 §7.7）；
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

**迁移**：`backend/migrations/000007_booking_lesson.up.sql` / `.down.sql`（`slots`、`bookings`、`lesson_accounts`、`lesson_ledger`）+ **`000008_fix_booking_lesson_tables.up.sql` / `.down.sql`（必须一起跑，否则线上全是 1054）**；`internal/data/migrate_test.go` 期望迁移数量（当前 12 个）。表结构细节见 `docs/data-model.md` §3.4；事故原因见 §5 第 12 条。

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
- [ ] 班课（班级名单、上课记录、不扣课时）——已完成，见 §7.7；
- [ ] 课前 24h 提醒（AF-10，需定时任务 + 订阅消息模板）。

---

## 7.6 已完成：M5 学习资料库（2026-11-25）

**迁移**：`backend/migrations/000011_materials.up.sql` / `.down.sql`（`material_categories`、`material_visibility`、`materials`、`material_download_logs`）。**注意 up 里先 `DROP TABLE IF EXISTS materials`**：`000002_teaching` 留过一张旧形态的单文件 `materials` 表，不先删就会被 `CREATE TABLE IF NOT EXISTS` 静默跳过（同 §5 第 12 条的坑）。`internal/data/migrate_test.go` 期望 11 个迁移。

**代码接入点**

| 文件 | 职责 |
| --- | --- |
| `backend/internal/biz/material.go` | 六分类蓝本矩阵 `DefaultCategories()`、资料输入校验（标题 ≤200、说明 ≤1000、年级白名单、文件 ≤9）、状态机 draft/published/archived、owner 才可删/改分类 |
| `backend/internal/data/material.go` | `EnsureCategories`（`INSERT IGNORE` 幂等补齐）、分类矩阵读写、`portalMaterialSelect` 三重过滤、下载日志 |
| `backend/internal/service/material.go` | 14 个 handler + 稳定 reason（`MATERIAL_NOT_FOUND`/`MATERIAL_CATEGORY_NOT_FOUND`/`MATERIAL_STATE`/`FORBIDDEN`） |
| `backend/cmd/hm/main.go` | 14 条路由 + 启动时 `EnsureDefaults` |
| `frontend/apps/mp/src/pkg-teacher/pages/materials/index.vue` | 教师：分类切换、上传（`chooseMessageFile` → `wx.cloud.uploadFile`）、发布/归档/删除 |
| `frontend/apps/mp/src/pages/portal/materials.vue` | 家长：按分类分组看已发布资料，下载前先 `POST /api/v1/portal/materials/{id}/download` 记日志，再 `wx.cloud.downloadFile` + `wx.openDocument` |

**语义（别改错）**

- 家长端可见性只在服务端判定：`status='published'` **且** 该学员分组的 `material_visibility.visible=1` **且** 家长/学员都是 active；不可见与不存在统一返回 `MATERIAL_NOT_FOUND`（404），不泄露存在性。
- 分类与分组矩阵的写操作只给 owner；资料 CRUD 助教可用（批阅、发资料是助教日常）；删除资料只给 owner。
- 文件走 `file_ids` JSON 数组（`{name,fileID,size}`），入库时用 `string(payload)`（MySQL 5.7 JSON 列不接受 `[]byte`）。
- 下载日志逐文件一条，记录 `guardian_id`、可解析出的 `student_id`、`ip`。

**未做/待跟进**

- [ ] 真机验证上传与下载打开（`wx.openDocument` 需要真机）；
- [ ] 单份资料在分类矩阵之上的额外分组覆盖（二期）；
- [ ] 资料与消息中心的联动（M6）。

---

## 7.7 已完成：M7 班课 + M6 通知回执/导出 + M9 看板 + M8 门户五卡（2026-11-25）

**迁移**：`backend/migrations/000012_notices_delivery.up.sql` / `.down.sql`。这五张表（`classes`/`class_members`/`class_sessions`/`notices`/`notice_receipts`）**由 `000002_teaching` 早已建好**，`000012` 只做增量 `ALTER`：`notices.audience_type`、`notices.file_ids`、`notice_receipts.renotified_at` 与 `idx_notice_receipts_pending`。`internal/data/migrate_test.go` 期望 12 个迁移。

**代码接入点**

| 文件 | 职责 |
| --- | --- |
| `backend/internal/biz/class.go` + `data/class.go` + `service/class.go` | 班级 CRUD/归档、成员加入退出（只允许 active 学员）、课次与出勤（出勤存 `class_sessions.attendance` JSON）；9 个接口 |
| `backend/internal/biz/notice.go` + `data/notice.go` + `service/notice.go` | 通知 CRUD/发送（事务内快照受众 + 每人一条回执 + 站内信）、催办、家长确认（幂等）、48h 再推、回执 xlsx 导出 |
| `backend/internal/biz/dashboard.go` + `data/dashboard.go` + `service/dashboard.go` | `Board` 四组聚合 + 60 秒进程内缓存（互斥锁 + 区间命中） |
| `backend/internal/data/crm.go` `PortalHome` | M8 五卡字段：`pending_receipts`/`upcoming_bookings`/`next_booking_at`/`materials_count`/`latest_material_name` |
| `backend/cmd/hm/main.go` | 22 条新路由；`buildServices` 现在返回 `(*service.Server, *biz.SubscriptionUsecase, *biz.NoticeUsecase, *gorm.DB)`；`AfterStart` 起 `runNoticeRenotify`（10 分钟一轮） |
| `frontend/apps/mp/src/pkg-teacher/pages/{classes,notices,board}/index.vue` | 班课、通知、看板 |
| `frontend/apps/mp/src/pages/portal/{notices,index}.vue` | 家长消息中心；门户首页五卡 + 下次上课 |

**语义（别改错）**

- 班课**不扣课时**、不写 `lesson_ledger`；班级状态只有 `active`/`archived`（归档代替 planned/ongoing/finished），见 ADR-032。
- 通知发送：`draft` → `sent` 在一次事务内完成；受众为空报 400 `NOTICE_NO_AUDIENCE`；`INSERT IGNORE notice_receipts` 保证一人一条。
- 48h 再推：worker 扫 `renotified_at IS NULL AND created_at <= now-48h`，推完写 `renotified_at`，**只推一次**。
- 看板转化率是**近似口径**（用区间新增学员的当前分组，无分组历史）；响应只给聚合值，名单走分页/导出。
- 回执 xlsx 在请求内生成（≤2000 行）并直接流式返回，不落盘（容器无持久盘）。

**未做/待跟进**

- [ ] 真机验证：发布通知 → 家长确认 → 教师看到已确认数与催办；班课记一次课；看板数字与列表一致；
- [ ] 客户名单/作业提交名单/课时流水导出（待 P2 异步 `jobs`）；
- [ ] 课前 24h 提醒（AF-10）与预约通知（AF-07/08/09，需要 `notifications` 支持教师维度）。

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
