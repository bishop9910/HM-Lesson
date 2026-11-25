# 架构决策记录（ADR）· HM-Lesson

> 规则：任何**架构级**选择（框架、存储、协议、部署形态、安全模型）必须在此登记，格式统一为「背景 / 决定 / 理由 / 影响 / 回滚」。
> 产品规则（状态机、权限矩阵、课时与预约规则）不放这里，放需求文档。
> 状态：`已采纳` / `待确认` / `已废弃`

| 编号 | 决策 | 状态 |
| --- | --- | --- |
| ADR-001 | 后端采用 Go Kratos v3（而非 v2 / Gin） | 已采纳 |
| ADR-002 | 数据库：PostgreSQL 17 + GORM v2/pgx | **已废弃**（Q22 确认 MySQL 5.7.18，见 ADR-023） |
| ADR-003 | 版本化 SQL 迁移（格式与 golang-migrate 兼容，执行器见 ADR-028） | 已采纳 |
| ADR-004 | 主键 UUIDv7（应用侧生成） | 已采纳 |
| ADR-005 | 枚举用 `text/varchar + CHECK`，不用数据库原生 enum | 已采纳 |
| ADR-006 | 对象存储 S3 抽象（初版 MinIO） | **已废弃**（生产改云托管 COS，见 ADR-022） |
| ADR-007 | 反向代理 Caddy 2（自动 HTTPS） | **已废弃**（云托管终止 TLS，见 ADR-020） |
| ADR-008 | 教师密码 Argon2id | 已采纳 |
| ADR-009 | 一期单体多模块，不拆微服务 | 已采纳 |
| ADR-010 | 权限在服务端强制隔离（guardian scope） | 已采纳 |
| ADR-011 | 提交预约即锁定时间段（数据库级唯一约束） | 已采纳（PG 部分唯一索引 / MySQL 生成列，随 Q22） |
| ADR-012 | 课时流水只追加，余额为派生值 | 已采纳 |
| ADR-013 | 家长端 UI 用 wot-design-uni | 已采纳（Q10） |
| ADR-014 | 教师管理端为独立 Web 应用 | **已废弃**（Q1=B，见 ADR-016） |
| ADR-015 | 不引入消息队列，用 Outbox 表 | 已采纳 |
| ADR-016 | 教师模式并入同一个小程序（分包） | 已采纳（Q1） |
| ADR-017 | 家长绑定：手动手机号 + 审批 / 绑定码 | 已采纳（Q2） |
| ADR-018 | 对象存储默认 SeaweedFS，通用 S3 + 代理回退 | **已废弃**（生产改云托管 COS，见 ADR-022；本地 dev 仍可 MinIO） |
| ADR-019 | Excel 导入采用预览/提交两阶段 + 批次回滚 | 已采纳（Q6） |
| ADR-020 | 生产部署改用微信云托管（上海，单服务 `hm-api`） | 已采纳（Q15 变更） |
| ADR-021 | 小程序接入不依赖服务器域名/ICP 备案（callContainer / connectContainer） | 已采纳 |
| ADR-022 | 生产对象存储用云托管 COS + fileID，S3 仅本地 dev | 已采纳（Q11 变更） |
| ADR-023 | 数据库：MySQL 5.7.18（本地容器 / 生产云托管） | 已采纳（Q22 确认） |
| ADR-024 | 单服务内嵌调度，`min=max=1` + 数据库锁 | 已采纳 |
| ADR-025 | 迁移在启动时加锁执行（CI 一次性任务备选） | 已采纳（执行器见 ADR-028） |
| ADR-026 | 产品定位为工具类（作业收集/家校协作），砍掉一切 IM | 已采纳（2026-11-22 用户指令） |
| ADR-027 | 一期不引入 Redis（MySQL 计数 + 进程内缓存） | 已采纳（Q24） |
| ADR-028 | 迁移执行器：项目内置 runner，恢复版本化迁移 | 已采纳（2026-11-23 用户要求恢复迁移） |

---

## ADR-001 后端框架：Kratos v3

**背景**：需求要求“全 protocol”（同一 IDL 生成 gRPC 与 HTTP），并在 Debian/Docker 长期自托管；团队规模小，需要成熟的分层与代码生成。

**决定**：`github.com/go-kratos/kratos/v3` v3.0.x，Go 1.25。

**理由**：v3 于 2026-06-26 正式发布；原生 protobuf 双栈、内置 middleware、配置与日志标准化（`log/slog`）；比 Gin 更适合“proto 为唯一事实来源”；v2 已进入维护末期。

**影响**：需要 Go 1.25；JWT 中间件改用 `contrib/middleware/jwt/v3`；HTTP JSON 需显式导入 `encoding/protojson`；`transport/http/binding` 已移除，参数绑定用 `Context.Bind*`。生成代码需按 v3 重新生成。

**回滚**：若 v3 生态阻塞（如插件缺 v3 支持），退到 v2.9.x 需要改 import 路径、JWT 中间件与 codec 注册，工作量约 1~2 天，故在 M1 先做最小验证。

---

## ADR-002 数据库：PostgreSQL 17 + GORM v2/pgx

**背景**：业务包含状态机、并发抢时段、课时流水、报表统计；飞书多维表格无法提供事务与约束。

**决定**：PostgreSQL 17；ORM 用 GORM v2（`gorm.io/gorm`）+ pgx v5 驱动；复杂报表与批量操作写原生 SQL。

**理由**：部分唯一索引可直接解决预约冲突；`numeric` 精确计算课时；JSONB 可存标签快照；运维成熟、备份简单。

**影响**：需维护迁移；GORM 的零值/软删除需统一约定（本项目用显式 `archived_at`，不依赖 GORM 软删除）；报表 SQL 集中在 `data/report`。

> **2026-11-22 更新**：部署改为微信云托管后，托管数据库只有 MySQL 5.7/8.0；本文档保留 PG 17 作为备选（腾讯云实例 + VPC）。推荐改为 MySQL 5.7.18，见 ADR-023，**最终以 Q22 为准**。MySQL 下 `uuid`→`char(36)`、`jsonb`→`json`、`timestamptz`→`datetime(3)`、部分唯一索引→生成列唯一索引。

**回滚**：换库成本高，故 M1 即锁定；如必须换，优先考虑兼容 PG 协议的 Cockroach/云 PG，而非 MySQL。

---

## ADR-003 迁移：版本化 SQL（原 golang-migrate）

> **2026-11-23 更新**：仍采用版本化迁移语义（up/down、版本表、只前进），但执行器改为项目内置 runner（见 ADR-028），不实际引入 `golang-migrate` 依赖；文件命名与行为保持兼容。

**背景**：用户明确要求“数据库使用 migration 进行迁移”。

**决定**：`golang-migrate/migrate` v4；SQL 文件放 `backend/migrations/`，用 `embed` 打进 `cmd/migrate`；启动时校验版本。

**理由**：SQL 透明、可 review、可 down；不引入 ORM 自动迁移的隐式行为；与 Docker 部署配合简单（一次性 Job）。

**影响**：每次改表必须写成对迁移；生产只前进，修复用新迁移；CI 在空库与种子库各跑 up/down/up。

**回滚**：本地可 `down`；生产回滚用“新迁移补偿”，不执行破坏性 down。

---

## ADR-004 主键：UUIDv7

**背景**：多端写入、导出、外部引用；自增 ID 会泄露量级且合并数据困难。

**决定**：应用侧生成 UUIDv7（时间有序），存 `uuid` 类型。

**理由**：索引局部性好于 UUIDv4；便于分库/导入；不依赖数据库扩展。

**影响**：ID 在 JSON 中为字符串；日志与前端需按字符串处理；禁止暴露连续序号。

**回滚**：成本极高（全表主键），视为不可逆决策。

---

## ADR-005 枚举：text + CHECK

**背景**：状态机多，PG enum 增删值需要 DDL 且事务限制多。

**决定**：所有枚举列用 `text` + 命名 CHECK 约束；Go 侧用常量 + 校验函数。

**理由**：加值只需改 CHECK（新迁移）；跨库兼容；ORM 友好。

**影响**：数据库不阻止拼写错误以外的语义错误，需在 biz 层校验状态迁移。

**回滚**：可平滑改为 enum，但无必要。

---

## ADR-006 对象存储：S3 抽象（初版 MinIO）

> 2026-11-18 更新：具体选型由 [ADR-018](#adr-018-对象存储默认-seaweedfs通用-s3--代理回退) 取代；本 ADR 的“私有桶 + 短时签名 + `storage.Store` 抽象”原则继续有效。

**背景**：讲义、作业图片、批改图需要私有存储与短时下载；数据需自主可控。

**决定**：S3 兼容存储，私有桶；上传用预签名 PUT，下载用 5 分钟预签名 GET；接口抽象为 `storage.Store`。

**理由**：可 Docker 自托管；S3 API 使未来切云 OSS 只改配置；避免文件进数据库。

**影响**：需要备份对象存储数据卷；小程序需把文件域名加入白名单；大文件上传要分片（二期）。

**回滚**：切云 OSS 实现同一接口，迁移历史对象用 `mc mirror` 或 `weed` 工具。

---

## ADR-007 反向代理：Caddy 2

**背景**：单机 Docker 部署，需要 HTTPS、HTTP/2、WebSocket 转发与低维护。

**决定**：Caddy 2 作为唯一入口，自动申请/续期证书；Go 只监听内网端口。

**理由**：配置量远小于 Nginx + certbot；自动 HTTPS 对小程序“必须 HTTPS”友好；WebSocket 反代开箱可用。

**影响**：Caddyfile 需纳入仓库与备份；国内服务器 80/443 备案与放行。

**回滚**：Caddyfile 可等价翻译为 Nginx，域名与证书不受影响。

> **2026-11-22 废弃**：生产改微信云托管，TLS 由云托管终止，服务只监听容器 80；不再需要 Caddy、证书与 443。保留本 ADR 仅作历史；自建 Debian 回退时仍适用。见 ADR-020。

---

## ADR-008 教师密码：Argon2id

**背景**：教师账号少但权限极高（可见全部学员、成绩、财务）。

**决定**：Argon2id（`golang.org/x/crypto/argon2`），每用户随机 salt，参数记录在哈希串中；连续失败 5 次锁 15 分钟。

**理由**：抗 GPU/ASIC 优于 bcrypt；OWASP 推荐；单用户校验成本可接受。

**影响**：登录接口耗时约 50~100ms，需限流；参数升级时用“登录时重哈希”。

**回滚**：可兼容 bcrypt 校验并渐进迁移，不强制重置密码。

---

## ADR-009 部署形态：单体多模块

**背景**：中小型项目（初期 <500 学员），但要求“留优化空间”。

**决定**：一个 `backend` 仓库、一个 `api` 进程（+ 独立 `job`、`migrate`），内部按 biz 模块分目录；不拆微服务、不引服务网格。

**理由**：事务简单、部署简单、调试成本低；目录边界为未来拆分留接口；当前量级微服务只会增加运维负担。

**影响**：模块间禁止直接查对方表；跨模块调用走 biz 接口；`job` 与 `api` 共享代码但独立进程。

**回滚**：拆分时按 biz 模块抽服务，先拆 `notify`/`export` 等低耦合模块。

---

## ADR-010 权限：服务端 guardian scope

**背景**：飞书方案用“行权限 + 账号字段”，迁移到自研后若只在客户端过滤会泄露全部数据（TC-14 高危）。

**决定**：家长 token 只携带 `guardian_id`；每次请求由 `authz` 解析其 `approved` 的学员集合；所有按学员的 repo 查询强制带该集合；越权返回 404。

**理由**：单点收口，避免每个 handler 各自判断；404 不泄露资源存在性；可测试。

**影响**：repo 接口签名需要 `Scope` 参数；缓存 60s，绑定变更后主动失效；教师账号不受此限。

**回滚**：不可回滚到“客户端过滤”方案。

---

## ADR-011 预约并发：提交即锁时段

**背景**：蓝本要求“一个时段只能一人”，且冲突时保留先审核通过的（TC-08）。

**决定**：家长提交预约时在事务内把 `slots.status` 置 `locked`；`bookings` 上建部分唯一索引 `(slot_id) WHERE status IN ('pending','approved')`；老师驳回/取消时释放。

**理由**：提交即占位可避免“审核时才发现冲突”；唯一索引是最后防线，防并发穿透；比“仅审核通过才锁”体验更可预期。

**影响**：待审核预约会占用时段；需设置“待审核超时自动释放”（建议 24h，Q9）；改期=取消+重提。

> **2026-11-22 方言修正**：MySQL 5.7.18 不支持部分唯一索引。实现改为 `active_flag` 生成列（`CASE WHEN status IN ('pending','approved') THEN 1 ELSE NULL END`）+ `UNIQUE(slot_id, active_flag)`；或事务内 `SELECT ... FOR UPDATE` 作为补充。语义不变，TC-08 仍要求数据库级约束。

> **2026-11-25 落地补充**：`backend/migrations/000007_bookings_lessons.up.sql` 已按上述方式建 `uq_bookings_active_slot(slot_id, active_flag)`；提交预约在事务内先 `SELECT ... FOR UPDATE` 锁时段再 `UPDATE slots SET status='locked' WHERE status='open'`，唯一索引兜底并发穿透（1062 映射 409 `BOOKING_SLOT_TAKEN`）。**时段之间“同一老师不重叠”在 5.7 无 `EXCLUDE` 可用，只能由应用层在 `CreateSlot` / `BatchCreateSlots` 的同一事务里做区间相交校验**（`ensureNoOverlap`），这是与原始“数据库级约束”唯一的降级点，写在这里以免后人误以为有库级保证。消课的“只扣一次”同理用 `lesson_ledger.consume_ref_id` 生成列 + `uq_lesson_ledger_consume` 实现。

**回滚**：改为“审核通过才锁”需迁移索引并处理已占位数据，成本中等，故先在 Q9 确认。

---

## ADR-012 课时：只追加流水

**背景**：蓝本要求“剩余课时必须由流水汇总，不许手改”，避免对账纠纷。

**决定**：`lesson_ledger` 只追加（`purchase` / `consume` / `adjust` / `refund`；赠送是否单列类型待 Q8）；`lesson_accounts.balance_hours` 为缓存列，由事务内更新，并每日对账修正；任何“修改余额”的操作都转成一条带原因的流水。

**理由**：可审计、可重算；并发消课用行锁 + 幂等键；对账能发现漂移。

**影响**：UI 不提供“直接改余额”；调整需填原因；导出以流水为准。

**回滚**：不可回滚为“直接改余额”。

---

## ADR-013 家长端 UI：wot-design-uni（Q10 已确认）

**背景**：uni-app Vue3 + TS，需要 TS 友好、可按需引入、适配微信小程序。

**决定**：wot-design-uni 1.14.x；主题用 CSS 变量覆盖；缺失组件自研。

**理由**：类型完整、组件覆盖家长端场景（日历、上传、表单、弹层）、easycom 减少样板。

**影响**：需锁定版本；升级前看 breaking change；自定义主题变量集中在 `styles/tokens.scss`。

**回滚**：若遇阻塞，备选 nutui-uniapp；组件层已隔离，替换成本主要在表单与日历。

---

## ADR-014 教师管理端：独立 Web（**已废弃**）

> 2026-11-18：Q1=B，委托方选择“教师模式并入小程序”。本决策废弃，见 [ADR-016](#adr-016-教师模式并入同一个小程序分包)。保留原文仅作决策历史。

**背景**：批阅（多图/长文）、排课日历、台账表格、看板在手机小程序内体验差。

**决定**：`frontend/apps/admin`，Vue3 + Vite + TS + Naive UI；与小程序共享 `packages/api-types`、`packages/shared`。

**理由**：桌面端信息密度与操作效率；可独立发布，不影响家长端审核；Naive UI 的表格/表单/图表生态成熟。

**影响**：多一套构建与部署（静态站点）；需管理端登录与权限；CORS 白名单。

**回滚**：若 Q1 选“小程序教师模式”，管理端页面降级为移动布局，功能不减但体验下降。

---

## ADR-015 异步：Outbox 而非 MQ

**背景**：通知、导出、对账、定时提醒需要可靠异步；初期量级小。

**决定**：业务事务内写 `outbox_events`；`job` 进程用 `FOR UPDATE SKIP LOCKED` 拉取并投递；失败重试 + 死信标记；不引入 Kafka/RabbitMQ。

**理由**：与业务同事务，避免“消息发了但数据没落”；运维零新增组件；量级足够。

**影响**：需要清理历史事件；投递延迟秒级；多实例 job 用 DB 选主。

**回滚**：若吞吐成为瓶颈，可把 outbox 消费者替换为 MQ 生产者，表结构不变。

---

## ADR-016 教师模式并入同一个小程序（分包）

**背景**：Q1=B。项目只有一位老师、需快速上线、维护两套客户端成本高；老师本人也在微信里工作。2C8G 单机部署也不适合再托管一套 Web 静态站。

**决定**：一个 uni-app 应用、一次审核发布；主包是家长端，教师页面全部放 `pkg-teacher` 分包；登录入口统一，`/me` 返回角色后分流到教师工作台或家长首页；两端共用同一套 API、`packages/api-types`、设计令牌与请求层。

**理由**：少一套构建、部署、登录与权限体系；教师日常动作（审批、批阅、回执）以“看一条、点两下”为主，手机可完成；分包保证家长端主包体积；未来若需要桌面端，可复用教师页面与 API 快速抽 Web。

**影响**：批阅多图、台账、看板在 375px 宽度下必须重新设计（表格转卡片、分步操作、图片全屏滑动）；xlsx 导出需 `wx.downloadFile` + `wx.openDocument` 或转发文件；教师分包不能引用家长端私有组件，公共组件下沉 `packages/shared`；主包 ≤1.5MB 的约束更紧。

**回滚**：后端与 proto 不变；新建 `apps/admin` 复用 `packages/api-types` 与教师页面逻辑，成本主要在布局与路由，预计 3~5 天。

---

## ADR-017 家长绑定：手动手机号 + 审批 / 绑定码

**背景**：Q2 确认小程序为个人主体、未认证，`getPhoneNumber` 不可用；Excel 导入时已建立家长手机号档案；家长首次进入必须“先绑定再可见”。

**决定**：
1. 主路径：`wx.login` 拿 openid → 家长手动输入手机号 → 匹配 `guardians` → 绑定 openid、状态 `pending` → 老师在教师模式一键通过 → `active`。
2. 辅助路径：老师为学员生成 6 位绑定码（7 天、一次性）；家长输入手机号 + 绑定码 → 直接 `active`（码即授权），并写审计。
3. 无匹配且无码：提示联系老师，不自动创建。
4. 一个 openid 只能绑一个 guardian；已绑他人时返回 409，必须老师重置。
5. 不接短信、不做手机号 OTP；靠老师审批 + 限流 + 审计控制冒绑风险。

**理由**：不依赖认证主体能力，当前即可开发上线；导入数据可直接复用；审批是最后一道人工校验；绑定码覆盖“家长已到店、老师当面给码”的场景。

**影响**：家长多一步输入与等待；老师每天需处理少量审批；绑定码泄露风险用 7 天 + 一次性 + 限流缓解；后续主体认证后可把 `getPhoneNumber` 作为“自动填手机号”的可选增强，不改变数据模型与状态机。

**回滚**：若改为手机号快速验证，只替换绑定页第 4 步的取号方式；`binding_codes` 可保留或停用。

---

## ADR-018 对象存储：默认 SeaweedFS，通用 S3 + 代理回退

**背景**：Q11 选择 SeaweedFS 或通用 S3；2C8G 单机希望少一个重组件；小程序上传下载需要稳定可达的 HTTPS 域名。

**决定**：`storage.Store` 只依赖 S3 API；Compose 默认部署 SeaweedFS 单容器（master + volume + filer + S3 网关），数据落独立卷；配置 `HM_S3_ENDPOINT`、`HM_S3_PUBLIC_ENDPOINT`、`storage.mode`。预签名 PUT/GET 优先；Q20 在 Debian 实测若不支持，则 `mode=proxy` 走服务端流式上传/下载，接口与落库字段不变。

**理由**：SeaweedFS 单二进制、资源占用低、S3 兼容；同时保留 MinIO/云 OSS 的替换能力；预签名可减轻 2C8G 的 API 带宽压力；代理回退保证“任何 S3 实现都能跑”。

**影响**：Compose 增加一个服务与数据卷；备份脚本要覆盖 `weed` 数据目录；预签名 URL 的主机名必须是外网可达的 `public_endpoint`，不能是容器内地址；代理模式会占 API 带宽，需限制并发与单文件 30MB。

**回滚**：切 MinIO 或云 OSS 只改环境变量与 compose 服务；历史对象用 `weed backup` / `mc mirror` 迁移。

> **2026-11-22 废弃（生产）**：云托管自带 COS，小程序 `wx.cloud.uploadFile` 直传拿 `fileID`，不需要 S3 网关、预签名与 `public_endpoint`，也不再依赖域名。见 ADR-022；本 ADR 仅保留本地 dev 的 MinIO 适配器与自建回退。

---

## ADR-019 Excel 导入：预览/提交两阶段 + 批次回滚

**背景**：Q6 确认有飞书导出的 Excel；手工录 10+ 学员易错，后续学员增多需要批量建档；导入必须可校验、可撤销。

**决定**：提供与飞书列对齐的 xlsx 模板；`preview` 上传后逐行校验并返回错误，`commit` 按 `batch_id` 写入；每 200 行一个事务；`import_batches` 记录统计与错误报告；支持回滚本批次新建且无业务引用的数据；忽略“家长飞书账号”列。

**理由**：两阶段避免“导到一半失败留下脏数据”；批次记录让老师能核对与撤销；忽略飞书列避免把平台耦合带进新系统。

**影响**：需要 `excelize` 解析（纯 Go，无 CGO）；导入限 owner；错误报告需要能下载/查看；判重规则（手机号 + 学员姓名）必须与手工建档一致。

**回滚**：保留手工建档入口；导入功能可整体下线而不影响其他模块。

---

## ADR-020 部署：改用微信云托管

**背景**：原方案是 Debian + Docker Compose + Caddy 自托管；用户 2026-11-22 决定改用微信云托管。小程序备案已完成；域名没有 ICP 备案，也不需要。目标是减少运维、让小程序直连、避免域名与证书维护。

**决定**：生产部署到**微信云托管（上海）**，单服务 `hm-api`，容器端口 80，MVP `min=max=1`；TLS、网关、扩缩容由平台负责；不再使用 Caddy、生产 Compose、自建 SeaweedFS。小程序经 `callContainer`/`connectContainer` 接入；生产关闭“公网访问”。

**理由**：与微信生态同源，免域名/ICP 备案/证书（小程序备案已完成）；小程序身份由网关注入，减少 `code2session` 与 AppSecret 暴露；运维面小，适合单人维护；后续可平滑扩实例。

**影响**：平台锁定（地域仅上海、无托管 PG、无持久盘、请求/响应/超时受限）；冷启动与缩容会影响后台任务；日志只走 stdout；CI 需接入 TCR/wxcloud。需要保留可回退的 Dockerfile 与标准 MySQL/PG、S3 抽象。

**回滚**：应用保持普通容器与 12-factor 配置，可迁回 Debian + Compose + Caddy；数据库用逻辑备份导入；对象存储用 COS 导出/镜像。回滚成本主要在数据迁移，不在代码。

---

## ADR-021 小程序接入：不依赖服务器域名与 ICP 备案

**背景**：用户问“换成云托管后域名是不是也不需要了”。小程序传统 `request`/`socket` 需要配置合法域名且域名必须 ICP 备案；云托管提供免域名的容器调用。

**决定**：小程序生产**只用** `wx.cloud.callContainer`（HTTP）与 `wx.cloud.connectContainer`（WebSocket）；不配置 `request`/`uploadFile`/`downloadFile`/`socket` 合法域名；`wx.cloud.init` 注入环境 ID。自定义域名仅用于未来 H5/独立 Web/对外 API。

**理由**：官方支持且不依赖域名 ICP 备案（小程序备案已完成）；请求直接进云托管网关，注入 `X-WX-*` 身份；少一层 DNS/证书/备案维护。域名 ICP 备案不保留、也不作为小程序上线前置。

**影响**：基础库要求 `callContainer ≥2.23.0`、`connectContainer ≥2.21.1`；请求 100 KiB、响应 1000 KiB、超时 15 s；H5 不能作为生产端直连（关闭公网后不可达）；WS 在 iOS「高性能+」可能无 openid，必须应用层 token。

**回滚**：绑定自定义域名（ICP + 手动证书）后，把请求层切回 `uni.request`/`wx.connectSocket`；`request.ts` 已做平台分流，改动集中在一个文件。

---

## ADR-022 对象存储：生产用云托管 COS + fileID

**背景**：原方案 SeaweedFS/S3 + 预签名；云托管自带对象存储，小程序可直接上传，且 100 KiB 请求体不允许文件走 API。

**决定**：生产 `storage.Store` 实现为云托管 COS：客户端 `wx.cloud.uploadFile` 得 `fileID`，业务接口只提交 fileID 与元数据；服务端用 COS SDK 校验并落 `files` 表；下载先鉴权再返回 fileID/短期临时链接。本地 dev 用 MinIO 容器，S3 适配器仅用于本地/未来自建回退。**2026-11-22 已确认 Q23**：小程序无域名/ICP 时上传只能走 COS，因此生产不采用自建 MinIO。

**理由**：官方推荐路径，绕开请求体限制；fileID 与小程序 `downloadFile`/`<image>` 原生兼容；桶私有，权限仍在业务层；无需预签名主机与备案域名。

**影响**：fileID 不等于授权，必须每次校验；对象键与 DB 需对账；本地无法直连云存储，需适配器；COS 备份/版本控制要单独配置；历史 SeaweedFS 数据需迁移。

**回滚**：`storage.Store` 接口不变，可切回 S3/MinIO；fileID 与对象键做一次映射导入；客户端上传路径改回预签名/代理。

---

## ADR-023 数据库：统一 MySQL 5.7.18（本地容器 / 生产云托管）

**背景**：云托管只提供 MySQL 5.7/8.0（Serverless），没有托管 PostgreSQL。用户确认 Q22 用 MySQL 容器起步，但云托管服务容器无持久盘，不能把 MySQL 跑在生产服务容器内。原数据模型按 PG 17 设计，但尚未写建表代码。

**决定（2026-11-22 已确认）**：统一 MySQL 5.7.18。本地开发用 Docker Compose `mysql:5.7`；生产用云托管 MySQL 5.7.18；两套同一方言、同一套 `backend/migrations/*.sql`（版本化迁移，执行器见 ADR-028），仅 `HM_DB_DSN` 不同。`data` 层 GORM + `mysql` 驱动，复杂 SQL 写原生；不用 PostgreSQL。

**理由**：满足“先起容器跑着”的开发诉求；生产数据不放在无持久盘的服务容器；托管库同账号/同网络、成本低；500 学员规模足够；MySQL 5.7.18 支持 JSON、生成列；`CHECK` 会被解析但忽略，枚举/范围校验由应用层兜底。

**影响**：MySQL 无部分唯一索引，预约冲突用生成列 + 唯一索引；无 `RETURNING`/`ON CONFLICT`/`JSONB`/`FILTER`；`lower_case_table_names` 建库前固定；Serverless 会自动暂停，需关闭或设最小 CCU；备份/慢日志需腾讯云侧配置；`utf8mb4` 必须显式设置。

**回滚**：若选 MySQL 后发现限制，迁 PG 需改建表脚本与少量 SQL；保持 SQL 集中在 `data`、不在 biz 写方言，迁移成本可控。

---

## ADR-024 单服务内嵌调度：`min=max=1` + 数据库锁

**背景**：云托管实例会缩容，官方明确不能依赖请求返回后的后台 goroutine；拆独立 worker 容器会增加成本与部署复杂度。

**决定**：一个 `hm-api` 进程同时提供 HTTP/WS 与内嵌调度器；MVP `min=max=1`；定时任务用控制台 cron 或进程内 ticker 触发；每个任务用数据库锁（MySQL `GET_LOCK` / PG advisory lock）+ `jobs` 表行锁抢占，保证发布重叠/未来多实例不重复。

**理由**：规模小，单实例足够；少一个服务与一套部署；数据库锁是最后防线；任务落表可重试、可观测。

**影响**：发布时会有秒级不可用；实例故障时任务暂停，恢复后补跑；长任务必须分片并续租，不能阻塞请求；扩到多实例前要把调度拆出或改 Redis 锁。

**回滚**：`jobs` 表与调度器接口已解耦，可把调度器拆为独立 `cmd/job` 容器；云托管可再加一个服务。

---

## ADR-025 迁移：启动时加锁执行，CI 一次性任务备选

**背景**：云托管没有“发布前钩子”，但支持容器启动命令；多实例/滚动发布可能并发执行迁移。

**决定**：`hm` 启动时自动执行迁移（`data.Migrate`），也可在启动命令里显式跑 `./migrate up` 成功后再启动 `hm`；迁移用 MySQL `GET_LOCK('hm_migrate', 30)`，拿不到锁则等待/退出；失败 fail fast，不启动半迁移状态。CI 可在部署前跑一次性迁移任务作为备选。

**理由**：简单、无需额外平台能力；锁避免并发 DDL；失败可见，便于回滚镜像。

**影响**：启动时间增加（迁移通常 <5s）；迁移失败会导致发布失败，必须先备份；破坏性 DDL 仍需人工确认与维护窗口；不要每次冷启动都跑重迁移（有版本表，幂等）。

**回滚**：迁移文件成对提供 `down`；发布回滚到上一镜像；数据库用发布前备份恢复。生产禁止无备份的破坏性迁移。

---

## ADR-026 产品定位为工具类，砍掉即时通讯

**背景**：小程序当前是个人主体、未认证（Q2）。原方案按“教育培训”叙事，容易触发教育类目/学科培训审核；同时原飞书方案包含 M7 一对一沟通，若在小程序内实现聊天/客服会话，会落入社交/即时通讯类目，增加资质与审核风险。用户明确（2026-11-22）：**Q19 不是教育，是工具，只包含作业收集等功能，不符合条例的即时通讯功能直接砍**。

**选项**
- A. 保留 IM：体验接近飞书，但需额外类目/资质，审核风险高，且与“交付系统”定位无关。
- B. **砍掉 IM，定位工具类**：只做作业收集、任务批阅、预约与课时、资料与通知；家长-老师沟通仍在线下微信完成，老师把结论录入跟进记录。
- C. 改为公众号/企业微信承载 IM：引入第二套身份与消息体系，一期成本过高。

**决定**：选 B。产品对外表述、小程序名称/简介/截图、隐私协议统一按“作业收集/班级管理工具”；**不实现**聊天、会话、客服消息、群聊、用户间私信；站内“消息中心”只读系统通知。类目优先选工具/效率；个人主体先提审，审核要求企业主体再变更（Q19），不把主体变更当前置。

**理由**：工具类目更匹配实际功能，审核叙事一致；砍掉 IM 同时去掉消息存储、内容审核、会话权限、未读数一致性等一整块复杂度；家长本来就在微信里，聊天不需要再建一套。

**影响**：M7 从“P1 改为微信沟通+跟进留痕”进一步降为“**不做**”；后端无会话/消息表与发送接口；前端无聊天入口，P20 仅系统通知；需求文档、README、AGENTS、前端页面清单、测试用例同步删除 IM 相关描述。订阅消息/站内信仍保留，用于系统提醒。

**回滚**：若未来确需 IM，必须新增独立 ADR、评估类目与内容审核方案，并引入会话/消息表与权限模型；当前数据模型不预留 IM 表，避免“先建后废”。

---

## ADR-027 一期不引入 Redis

**背景**：云托管不内置 Redis；要用需在腾讯云上海购买实例并打通 VPC。一期 `min=max=1`，规模约 500 学员，限流/缓存/WS 单实例内存即可满足。用户确认 Q24：暂时不做。

**决定**：一期不引入 Redis。限流用 MySQL 计数表 + 进程内令牌桶；缓存用进程内 LRU；会话黑名单落 `token_denylist` 表；WebSocket 连接表放进程内。代码保留 `HM_REDIS_*` 配置为空时的降级路径，业务层不直接依赖 Redis 客户端。

**理由**：少一个付费实例、一套 VPC 与连接配置；单实例下语义足够；规模增长前不引入运维复杂度。

**影响**：多实例部署前必须补 Redis 或改为数据库共享状态；进程重启会丢失本地缓存与 WS 连接（客户端重连补拉）；限流精度低于 Redis。

**回滚**：按 `cache.Store`/`ratelimit.Store` 接口切换 Redis 实现，配置打开即可，不改业务代码。

---

## ADR-028 迁移执行器：项目内置 runner（恢复版本化迁移）

**背景**：项目曾短暂改为“预上线直接建表”（ADR-028 旧版）；2026-11-23 用户明确“不对不对，还是要迁移”，要求恢复版本化迁移。同时不想为一个执行器再引入 golang-migrate 依赖（下载、方言、多语句 SQL 解析都要额外验证）。

**决定**：采用项目内置的轻量迁移执行器：
- `backend/migrations/NNNN_<snake_case>.up.sql` / `.down.sql` 成对提供，`//go:embed` 打进二进制；
- `data.Migrate` 在**同一 MySQL 连接**上 `GET_LOCK('hm_migrate', 30)`，读 `schema_migrations` 当前版本，按版本顺序执行未应用的 up，完成后释放锁；
- `data.MigrateDown` 从高到低执行 down 并删除版本行；`cmd/migrate` 提供手工 `up` / `down -n` / `version`；
- `cmd/hm` 启动时自动执行 up；失败 fail fast，不启动半迁移状态。

**理由**：迁移文件本来就是多语句 SQL，复用已有 `splitSQL` 即可；不新增第三方依赖与下载；行为对使用者仍是标准版本化迁移（版本表、up/down、只前进）；实现小、可测（`loadMigrations`/`splitSQL` 单测）。

**影响**：执行器需自己保证解析/顺序/锁/失败回滚语义；迁移文件命名必须严格成对；生产只前进，破坏性 down 需备份与人工确认；CI 必须在空库跑 `up → down → up`。将来若需要更复杂能力（out-of-order、强制版本、多数据库驱动），可替换为 golang-migrate，迁移文件格式保持兼容。

**回滚**：保留迁移文件格式不变，直接换成 golang-migrate 的 embed 源即可；版本表结构兼容（version/name/applied_at）。

---

## ADR-029 微信订阅消息：一次额度表 + 进程内投递 worker + 模板全配置

**背景**：Q7 已定「订阅消息 + 站内信」，M3 只落了站内弹窗。个人主体未认证，无法用手机号快速验证，也没有短信兜底；作业发布/批改要让家长在微信里看到，只能走一次性订阅消息（`wx.requestSubscribeMessage`，每次 accept 只能发一条）。云托管实例随时可能被回收，不能在请求里同步调微信接口，也不引入 Redis/独立 worker 容器（Q24、ADR-024）。

**决定**：
- 新增 `subscription_grants(guardian_id, template_key, remaining, last_requested_at)`，`UNIQUE(guardian_id, template_key)`；用户 accept 一次 `remaining +1`（上限 3），发送成功 `-1`，无额度时只写站内通知，不弹微信。
- 新增 `notification_deliveries` 作为投递队列；**入队与业务写操作在同一事务内**（发布、批阅），文案以变量快照（`title/student/date/due_at/score`）落 `payload`，worker 发送时按模板 `fields` 渲染并截断。
- 模板配置走单个环境变量 `HM_WX_SUBSCRIBE_TEMPLATES`（JSON：`template_key -> {id, fields{关键词:变量名}}`），另加 `HM_WX_TMPL_LANG`、`HM_WX_MP_STATE`；缺 AppID/AppSecret/模板时 sender 为 nil，整条链路 no-op（只留站内信），业务接口不报错。
- worker 由 kratos `AfterStart` 起 goroutine、`Stop()` 取消 ctx 退出，30 秒一轮、启动先跑一次；取件用 `SELECT ... FOR UPDATE` + `UPDATE status='sending'` 两段式（MySQL 5.7 无 `SKIP LOCKED`），10 分钟仍 `sending` 的由 `ResetStaleDeliveries` 放回 `pending`；失败 1/2/4 分钟退避（上限 10 分钟），最多 3 次，永久错误直接 `failed`。
- openid 只在家长登录时从网关注入头采集，且必须 `X-WX-SOURCE` 非空才可信；同一 openid 已被其它 `guardians` 行占用时忽略，不阻断登录。
- 新增接口 `GET /api/v1/portal/subscriptions`、`POST /api/v1/portal/subscriptions`（只支持家长 token）。

**理由**：订阅消息是微信允许的、唯一不依赖域名/短信的触达手段；模板 ID 只能在公众平台后台看到，硬编码或猜关键词必然 47003，所以全部外置为配置；额度与队列落库而不是进程内，实例回收不丢任务，也天然满足幂等重试。

**影响**：新增两张表与一个后台 goroutine，多实例部署前必须换成带 `SKIP LOCKED` 的领取方式或共享锁（当前 MVP `min=max=1`）；不退还已消耗额度（微信侧已扣），家长需重新点“开启提醒”；模板关键词固定要求运营方在后台按 `fields` 中的变量名填写，否则发送报 47003 并置 `failed`。

**回滚**：删掉 `HM_WX_SUBSCRIBE_TEMPLATES`（或 AppSecret）即整体退回“只发站内信”；`000006` 的 down 会删除两张新表，站内通知不受影响。

**补充（2026-11-25）：云托管必须走「云调用」通道。** 实测容器内 `api.weixin.qq.com` 被平台 DNS 改写到 `169.254.10.1`，未在控制台「云调用 → 微信令牌权限」放行的接口一律 `connection refused`；同期 `www.baidu.com` 的 443 可通，证明公网出口本身正常。因此 `pkg/wx` 增加第二种调用方式：`HM_WX_CALL_MODE=cloud` 时用 `NewCloudClient`，请求 `http://api.weixin.qq.com`、不带 `access_token`（由平台网关鉴权）、也不需要 `HM_WX_APP_SECRET`；默认值仍是 `direct`，本地与非云托管环境行为不变。代价是生产依赖控制台配置（开放接口服务 + `subscribeMessage.send` 权限），配置缺失时表现为发送全部 `failed`，但业务写操作不受影响。

## ADR-030 家长账号改为「手机号 + 密码」，并由 owner 管理账号生命周期

**背景**：此前家长只要知道手机号就能登录门户（`GuardianByPhone` 直接签发 token），等于没有凭据；同时测试中出现「家长提交绑定申请后直接 active 但没有学员关联」的脏账号，审批页看不到、也没人能清理，长期累积成垃圾数据。用户要求：家长账号要有密码；所有账号（含 owner）能自己改密码；owner 能删除长期未绑定家长的绑定申请与账号；家长账号要随学员状态同步清理。

**决定**：
- `guardians` 增加 `password_hash`（Argon2id，`NOT NULL DEFAULT ''`）与 `last_login_at`，新增 `idx_guardians_status_created`（`000009_guardian_passwords`）。
- 登录：`POST /api/v1/auth/guardian/login` 改为 `{phone, password}`；也接受 `{phone, code, password}`（绑定码首次绑定 + 当场设密码，缺密码返回 `PASSWORD_REQUIRED`）。未设密码的旧账号不能密码登录，会明确提示用绑定码进入并设密。
- 审批：`POST /api/v1/guardians/{id}/approve` 支持 `password`；不传则服务端生成一次性 8 位密码，**只在响应里返回一次**（`initial_password`），老师转告家长。
- 自助改密：`POST /api/v1/auth/password`（staff，owner/assistant 均可）、`POST /api/v1/auth/guardian/password`（家长，未设过密码时允许不带旧密码）。
- owner 管理：`GET /api/v1/guardians`（含学员姓名、是否设密、最近登录、`student_count`）、`DELETE /api/v1/guardians/{id}`（仍绑定在读学员返回 409 `GUARDIAN_HAS_STUDENTS`；删除时连带清理 notifications / notification_deliveries / subscription_grants / student_guardians）、`POST /api/v1/guardians/{id}/password`（重置密码）。
- 与学员同步：学员 `status` 改为非 `active` 后，若家长已无任何在读学员，账号自动置 `disabled`，进入「可清理」列表；家长端因此无法继续登录。
- 密码规则：家长 6~32 位（手机端输入），老师 8~64 位；全部只存 Argon2id 哈希，任何接口都不回传哈希。

**理由**：手机号即凭据等于零鉴权，泄露一次手机号就能看别人孩子的成绩；而家长群体不可能记住复杂规则，所以阈值取 6 位。账号清理必须由 owner 显式执行（物理删除不可撤销），但「何时可删」由学员状态自动推导，避免老师手工判断遗漏。

**影响**：老家长账号（`password_hash=''`）在 owner 补设/重置密码前无法密码登录；家长登录页与教师端新增页面；删除账号会连带删除历史通知（但 `lesson_ledger` 等业务流水保留，`operator_id` 为老师）。

**回滚**：`000009` 的 down 删除两列与索引；服务端回滚到旧版本后家长仍可按手机号登录（旧逻辑），但已设置的密码失效。

## ADR-031 学员删除：物理删除 + 财务流水二次确认；内部系统账号用 `staff.hidden`

**背景**：AGENTS §3.9 原本规定「不得物理删除学员」，退课只归档。但 2026-11-25 用户明确要求学员档案「也要能支持删除」，用于清理测试与垃圾数据（测试学员、误建档案），并且删除后要解除家长绑定，让家长账号可被删除或重新绑定新学员。同时用户要求一个「锁死的开发者账号」用于内部调试，用户名固定 `system`。

**决定**：
- 学员删除 `DELETE /api/v1/students/{id}` 仅 owner 可调用；前端**两次弹窗确认**。
- 删除为物理删除：按外键依赖顺序清理 `submissions`、`notifications`、`task_assignments`、`bookings`、`lesson_ledger`、`lesson_accounts`、`binding_codes`、`follow_ups`、`class_members`、`student_guardians`，最后删 `students`；随后把相关家长置为未绑定（无在读学员则 `disabled`）。
- **财务纪律保留**：若该学员存在 `lesson_ledger` 流水，接口返回 409 `STUDENT_HAS_LESSON_LEDGER`，必须显式带 `?force=1`（前端第三次确认）才连流水一起删除。默认路径不会静默丢账。
- 系统账号：`staff` 增加 `hidden tinyint(1) NOT NULL DEFAULT 0`（`000010_system_account`）；启动时 `EnsureSystem` 保证 `phone='system'` 的账号存在、`hidden=1`、`role=owner`、`active`；密码只在首次创建或哈希为空时取 `HM_SYSTEM_PASSWORD` 写入，之后本人可在小程序自助改密；`ListStaff` 与服务台 `staff_total` 都过滤 `hidden`，成员管理对 `hidden` 账号返回 403 `SYSTEM_ACCOUNT_LOCKED`。

**理由**：物理删除是用户对「清理垃圾」的明确诉求，且这些数据（测试学员）没有保留价值；但课时流水是钱账，误删无法复原，所以用 `force` 把「连流水一起删」变成一个显式、可审计的选择。系统账号用隐藏列而不是新角色，避免污染 `role ∈ {owner, assistant}` 的权限矩阵与前端角色分支。

**影响**：删除不可撤销，任务/预约/作业历史随学员消失；`lesson_ledger.operator_id` 的历史仍指向老师，但被删学员的流水行不存在（force 情况下）。`000010` 只加一列，可安全回滚。

**回滚**：`000010` 的 down 删除 `idx_staff_hidden` 与 `hidden` 列；服务端回滚后学员删除接口消失（前端入口需同步删除），系统账号变为普通 owner（不再隐藏）。

## ADR-032 M6/M7 复用 `000002` 已建的班课与通知表，只做增量迁移

**背景**：`docs/data-model.md` 的 `classes`/`class_members`/`class_sessions`/`notices`/`notice_receipts` 在 `000002_teaching.up.sql` 就已被创建（生产环境已存在），但列集与理想模型有差异：`classes.status` 只有 `active`/`archived`、节奏字段叫 `schedule_note`、没有独立 `class_attendance` 表（出勤以 JSON 存在 `class_sessions.attendance`）、`notices.status` 只有 `draft`/`sent`、回执用 `reminded_at`/`renotified_at` 而非 `reminder_count`。此前 `000007` 的 `CREATE TABLE IF NOT EXISTS` 曾因同类情况被静默跳过（线上 1054 事故）。

**决定**：
- **不重建、不改列名**：M6/M7 在既有表上实现接口与页面，新增能力只通过 `000012_notices_delivery` 的 `ALTER TABLE` 补三处——`notices.audience_type varchar(16) DEFAULT 'all'`、`notices.file_ids json NULL`、`notice_receipts.renotified_at datetime(3) NULL` 与 `idx_notice_receipts_pending`。
- 出勤继续用 `class_sessions.attendance` JSON（键 `student_id`、值 `present|absent|leave`），解析时按 `class_members` 补姓名；不引入 `class_attendance` 表。
- 班课**不扣课时**：`class_sessions` 增删不写 `lesson_ledger`。
- 48 小时未确认自动再推由**进程内 worker**（`kratos.AfterStart` 里 `runNoticeRenotify`，10 分钟一轮）完成，用 `renotified_at IS NULL` 保证只推一次，不引入独立 worker 容器（延续 ADR-027/029）。

**理由**：生产已存在这些表，重建意味着破坏性 DDL 与数据风险；增量 `ALTER` 可回滚、可空列安全。保持与既有 `lesson_ledger`、`task_assignments` 等表的实现风格一致（出勤 JSON、无独立从表），也让 M7 能在不改表的前提下先跑起来。

**影响**：`docs/data-model.md` §3.5/§3.7 与理想模型的差异已在该处「实现现状」引用块中记录；后续若要四态班级或独立出勤表，需要新迁移做数据搬迁。

**回滚**：`000012` 的 down 删除 `audience_type`/`file_ids`/`renotified_at` 与索引；服务端回滚后通知发送仍可用（只剩 `draft`/`sent` 与受众 JSON），48h 再推能力消失。




