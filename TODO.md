# HM-Lesson 待办与路线图

> 最后更新：2026-11-25 14:25
> 当前代码基线：`main` @ `8cc6f4b`；本轮新增 `TODO.md`、`HANDOVER.md`、微信订阅消息（迁移 `000006` + `pkg/wx` + 投递 worker + 家长端开关）、M4 预约与课时（迁移 `000007` + biz/data/service + 4 个页面）与 README 索引，尚未提交
> 生产服务：微信云托管 `hm-lesson`，环境 `prod-d8gq8vzftc39caa5b`，MySQL 5.7.18 + COS
> 规则：完成一项就勾选；不要在生产库造测试数据；代码/部署/权限约定见 [`AGENTS.md`](AGENTS.md)。

---

## P0：下一次对话优先做

### 1. 微信订阅消息（最高优先级）

**为什么现在只有站内通知**

家长端目前只有站内 `notifications` 表 + 进入门户时的弹窗：

- 发布任务 → 写 `task_published`
- 批阅 → 写 `task_reviewed`
- 打回 → 写 `task_returned`

它不会在微信聊天列表里出现“服务通知”。要做的是**微信订阅消息**（`wx.requestSubscribeMessage` + `subscribeMessage.send`）。

**2026-11-25 进展**：代码已落地（`000006_subscription_messages`、`pkg/wx`、`biz/data/service` 订阅用例与 worker、`GET/POST /api/v1/portal/subscriptions`、家长门户「开启提醒」按钮、云端环境变量文档与 ADR-029）。缺模板 ID 时整条链路 no-op（只发站内信），因此**不需要等配置就能保持服务正常**；剩下的是运营方填配置 + 真机验证。

**开始前必须让用户提供/确认**

- [ ] 小程序后台「订阅消息」里可用的模板 ID：建议至少 3 个
  - [x] 作业发布通知：`qV_nVYIcFpaTP71yWyu-6Ln1haEHfb5UEk93UkPKebQ`（已配进 `HM_WX_SUBSCRIBE_TEMPLATES`）
  - [ ] 作业批阅完成通知：用户自建模板（作业名称 + 批改分数），**等审核**
  - [ ] 作业被打回通知：建议新建（作业名称 + 学员姓名 + 打回原因），**等审核**
- [x] 云托管环境变量：`HM_WX_APP_ID`、`HM_WX_APP_SECRET`（2026-11-25 用户已在云端配好）
  - AppSecret 只放云托管环境变量，**禁止入库/写文档**
  - 启动时 `probeWXToken` 会异步换一次 access_token：日志出现「微信 access_token 获取成功」即证明 AppID/AppSecret 可用；出现「获取失败」按 `wx.APIError` 的 code 排查（40013 invalid appid / 40125 invalid appsecret / 40164 IP 白名单）
- [ ] 确认工具类目下这些模板可选（个人主体 + 未认证，模板库可能受限）
- [x] 确认家长端是否已有 `openid`（`guardians.openid` 列已存在；登录时从 `X-WX-OPENID` 采集，需 `X-WX-SOURCE` 非空）

**实现清单**

- [x] 后端新增 `guardians.openid` 采集
  - 家长登录接口从网关注入的 `X-WX-OPENID` 取 openid（要求 `X-WX-SOURCE` 非空，拒绝客户端自报）；失败不阻断登录
- [x] 新增 `subscription_grants` 表（迁移 `000006_*.up/down.sql`）
  - `id / guardian_id / template_key / remaining / last_requested_at / created_at / updated_at`，`UNIQUE(guardian_id, template_key)`
  - 一次性订阅每 accept 一次只能发一条，发送成功后 `remaining - 1`；额度上限 3
- [x] 新增 `pkg/wx`（`client.go` + `subscribe.go`）
  - `access_token` 获取与缓存（单实例内存 + 过期前 5 分钟刷新；多实例前必须改共享缓存）
  - `SendSubscribe(ctx, msg)`；错误分永久/可重试，失败只落 `last_error`，不影响业务事务
- [x] 通知落 `notification_deliveries`（新表）
  - 发布/批阅在**同一事务内**入队，进程内 worker 每 30 秒发送；请求路径不调微信
- [x] 家长端「开启作业提醒」按钮（`pages/portal/index.vue`）
  - 由用户点击触发 `wx.requestSubscribeMessage({ tmplIds })`，逐个回传 `accept/reject` 到 `POST /api/v1/portal/subscriptions`
  - 未配置模板时按钮不展示（`configured=false`）
- [x] 深链
  - `page` 用 `pages/portal/tasks/detail?id=<assignment_id>`
  - 注意：家长任务接口里的 `id` 是 **assignment id**，不是 task id
- [ ] 订阅消息模板字段映射（按实际模板字段名填 `HM_WX_SUBSCRIBE_TEMPLATES`，见 `docs/deploy-wechat-cloudrun.md` §7.1）

**验收**

- [ ] 家长点“允许通知”后，教师发布任务，家长微信收到服务通知
- [ ] 教师批阅/打回后，家长收到对应通知，点开进入正确作业
- [ ] 家长拒绝/未授权时，业务不报错、不重复弹
- [x] 模板 ID 未配置时后端 no-op，不影响发布/批阅（单测覆盖 `DeliverOnce`/worker 禁用分支）
- [ ] 真实真机验证（开发者工具不能完整验证服务通知）

---

### 2. 上线安全收尾

- [ ] 关闭云托管「公网访问」（当前临时公网 URL 仍开放）
- [ ] 删除云托管环境变量 `HM_BOOTSTRAP_PASSWORD`
- [ ] 轮换 MySQL root 密码；新建只对 `hm_lesson.*` 有权限的应用账号
- [ ] 确认云托管构建的是最新 commit（改环境变量只生成配置版本，不会重新构建镜像）
- [ ] 备份演练：云托管 MySQL 备份 + COS 版本/生命周期
- [ ] 图片上传补服务端校验：MIME、大小、图片解码、对象键

### 3. 真机验收（M3 遗留）

- [ ] 家长提交作业：真实相机拍照 → `wx.cloud.uploadFile` → fileID → 提交
- [ ] 教师批阅：拍照/相册上传批改图 → 家长端可看
- [ ] 家长端通知弹窗实际点按（站内通知）
- [ ] 逾期任务红色数字（不是整块红底）
- [ ] 任务模板 PATCH 的零值元数据（D-07）复测

---

## P1：业务功能（按顺序）

### M4 一对一预约 / 课时 / 班课

> 2026-11-25 进展：预约与课时的**后端 + 小程序页面**已完成并本地验证（`go build/vet/test` 全绿、`vue-tsc` 0、`uni build -p mp-weixin` DONE）。迁移 `000007_booking_lesson` 与真实 MySQL 5.7 尚需部署时验证。

- [x] 可预约时段：教师建时段（单条 + 按周批量）、家长只看可预约（`/api/v1/slots`、`/api/v1/portal/slots`；教师页 `pkg-teacher/pages/slots`）
- [x] 预约：提交即锁时段（`bookings.active_flag` 生成列 + `uq_bookings_active_slot`）；审核通过/驳回；冲突先审先得（409 `BOOKING_SLOT_TAKEN`）
- [x] 消课：教师标记完成才写 `lesson_ledger`（`consume_ref_id` 生成列保证只扣一次，幂等）；未到课不扣课时（Q8）
- [x] 课时账户：总购买/已消耗/剩余/有效期；≤2 标记 `low_balance`，余额 0 禁止预约（422 `LESSON_HOURS_EXHAUSTED`）；充值/调整（调整原因必填）
- [ ] 班课：班级、名单、每次上课记录（班课不扣课时）
- [x] 家长门户补：我的预约（提交/撤回）、课时余额与流水（`pages/portal/booking`、`/api/v1/portal/lesson-account`）
- [ ] 预约通知：AF-07/08/09 需要 `notifications` 支持教师维度（`guardian_id` 目前 NOT NULL），随 M6 一起改

### M5 资料库

- [ ] 目录：高一/高二/高三一轮/高三二轮/月考真题/试听招生
- [ ] 分类 × 分组权限矩阵（服务端判定）
- [ ] 上传到 COS → `files` → 资料记录
- [ ] 下载：短期临时链接或服务端流式；记 `download_logs`
- [ ] 家长端「我的资料」

### M6 通知回执 + 导出

- [ ] 通知表 + 按人回执
- [ ] 未确认催办；48h 自动再推
- [ ] 回执导出 xlsx
- [ ] 客户/作业提交/课时流水导出 xlsx（异步任务 + COS）

### M8/M9

- [ ] 家长门户首页 5 卡片完整（任务/作业/预约/资料/课时）
- [ ] 看板四组：客户结构/交付质量/一对一运营/待办
- [ ] 图表不做 ECharts，用卡片/进度条/名单下钻

---

## P2：系统能力补课

- [ ] WebSocket：`wx.cloud.connectContainer` + 首帧 token 认证
- [ ] Outbox / jobs：定时触发 + 行锁；不要依赖请求返回后的 goroutine
- [ ] `audit_logs` 业务写入（登录、权限、删除、导出、审批）
- [ ] 登录限流（IP + 账号；无 Redis 用 MySQL 计数表）
- [ ] proto 真正生成并用于 HTTP 层（当前是手写 JSON handler；`buf lint` 尚未本地跑）
- [ ] MySQL 集成测试 / E2E（testcontainers 或 Debian 环境）
- [ ] CI：`buf lint`、`govulncheck`、构建产物、部署步骤

---

## P3：小修与体验

- [ ] 输入框/弹层在真机上的键盘遮挡复验
- [ ] 家长端任务深链、空态、错误态统一
- [ ] 教师端任务模板管理完整（当前 PATCH 已做，列表/删除可用性复验）
- [ ] 老页面 `pages/index/index.vue` 已不注册，确认没有遗留 `reLaunch` 指向
- [ ] 生产测试数据清理：`test` 学员、`13000000000` 家长、已关闭旧任务（需用户确认）

---

## 已知缺陷 / 风险

| 编号 | 内容 | 状态 |
| --- | --- | --- |
| D-13 | 家长登录返回教师字段 `note/tags/next_follow_up_at` | 代码已修，随最新 commit 验证生产响应 |
| D-07 | 任务模板 PATCH 曾返回零值元数据 | 代码已修，未复测 |
| D-12 | 拍照/批改图仅 API 层验证 | 待真机 |
| — | 云托管公网访问仍开启 | 待关闭 |
| — | MySQL root 仍在使用 | 待换应用账号 |
| — | 图片只存 fileID，无 MIME/大小/解码校验 | 待补 |
| — | WebSocket、Outbox 调度、登录限流、审计业务写入未实现 | 见 P2 |

---

## 明确不做

即时通讯/聊天/客服会话/群聊/私信、付费课程/在线支付、录播、短信兜底、自动最优排课。

产品定位是**工具类**（作业收集与家校协作），不是教育培训/招生平台；名称、简介、页面文案不要出现培训/招生/售课。
