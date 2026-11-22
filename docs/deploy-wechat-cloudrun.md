# 微信云托管部署手册 · HM-Lesson

> 适用：本项目从「Debian + Docker Compose + Caddy」迁移到**微信云托管**。
> 最后更新：2026-11-22。控制台菜单与 CLI 参数会随平台更新，**以控制台实际生成的命令为准**；本文不写死未验证的 flag。

---

## 1. 先回答三个问题

### 1.1 还需要服务器域名吗？

**小程序侧不需要。**

- 小程序调后端：`wx.cloud.callContainer`，不走 `request` 合法域名，**不需要在小程序后台配置服务器域名**。
- 小程序实时通信：`wx.cloud.connectContainer`，**不需要 socket 合法域名**。
- 图片/附件：`wx.cloud.uploadFile` / `wx.cloud.downloadFile`，走云存储，**不需要 uploadFile/downloadFile 合法域名**。

只有以下情况才需要自定义域名：

| 场景 | 是否需要域名 |
| --- | --- |
| 微信小程序（家长端 + 教师模式） | ❌ 不需要 |
| 公众号/H5 页面调同一后端 | ✅ 需要，且域名要 ICP 备案 + 上传 HTTPS 证书 |
| 未来做独立 Web 管理端 | ✅ 需要 |
| 给第三方/运维调公开 API | ✅ 需要 |
| 云托管默认公网域名 | ⚠️ 仅联调，不适合生产 |

### 1.2 备案怎么算？

**小程序已备案，域名没有 ICP 备案，都不影响上线。**

- 小程序通过 `callContainer` 访问云托管，不校验域名 ICP 备案。
- **小程序备案**已在 2026-11-22 确认完成（微信公众平台 → 设置 → 基本设置 → 小程序备案）；它与域名 ICP 备案是两件事，完成即可提交审核。
- **域名 ICP 备案当前未办，也不需要**；将来若绑自定义域名（H5/Web/对外 API），那个域名必须已 ICP 备案，云托管不提供备案功能。
- 云托管默认公网域名只用于联调，不作为生产依赖。

### 1.3 推荐上线形态

```text
微信小程序（家长端主包 + pkg-teacher 分包）
        │
        │ wx.cloud.callContainer  (HTTP/JSON)
        │ wx.cloud.connectContainer (WebSocket)
        ▼
微信云托管 · 上海 · 单服务 hm-api（容器端口 80）
        │
        ├── MySQL 8.0（云托管托管库，Q22 已确认）
        ├── 云托管对象存储（COS，fileID）
        └── 可选：腾讯云 Redis（上海，同 VPC）
```

- **一个服务**同时提供 HTTP 与 WebSocket；不拆 `api` / `job` 两个容器。
- MVP `min=max=1`：避免定时任务多实例重复、避免冷启动；后续要扩再引 Redis 锁与独立调度。
- **生产关闭“公网访问”**，只允许小程序/公众号调用。
- CI 构建镜像 → 推送 TCR → 云托管拉取运行；容器启动只做“迁移 + 起服务”。

---

## 2. 前置条件

| 项 | 要求 | 当前状态 |
| --- | --- | --- |
| 微信小程序 AppID | 已注册；确认是否会在 Q19 过程中变更 | 个人主体，未认证 |
| 微信云托管 | 已开通，绑定上述 AppID | 未开通 |
| 腾讯云账号 | 与云托管同主体，已实名 | 待确认 |
| 镜像仓库 TCR | 腾讯云容器镜像服务，建议上海地域 | 待开通 |
| 数据库 | **MySQL 8.0**：本地容器 / 生产云托管 | Q22 已确认 |
| 对象存储 | 生产云托管 COS；本地 MinIO 容器 | Q23 已确认 |
| Redis | **一期不用**（Q24） | 已确认 |
| 域名 | 小程序上线**不需要** | 未做 ICP 备案，也不需要；未来自定义域名再办 |
| 小程序备案 | 在微信公众平台单独提交，**与域名 ICP 备案是两件事** | ✅ 已完成（2026-11-22 确认） |
| Go / Node | 本地已装 Go 1.25.7、Node 22、pnpm | ✅ |

> **AppID 顺序**：云托管环境绑定小程序 AppID。Q19 改为“工具类目、个人主体先试”后，**不变更主体/AppID 就可以直接建生产环境**；只有预计提审被拒、需要变更主体时才先变更（AppID 可能变化，openid、绑定关系、云环境都要迁移）。见 Q26。
>
> **开通资格**：个人主体/未认证小程序能否开通云托管，官方文档未给出明确结论；以控制台能否看到入口为准。若被拒，先确认 Q19 主体路径，不要先建生产环境。

---

## 3. 开通与创建环境

1. 微信公众平台 → 小程序 → 「云开发/云托管」入口，开通云托管。
2. 选择地域：**上海**（云托管当前地域；VPC 不支持跨地域）。
3. 创建环境：`prod`（生产）、`dev`（联调，可选）。环境 ID 记入部署变量。
4. 创建服务：名称 `hm-api`，端口 `80`，规格按最小可用起步（如 0.5 CPU / 1 GiB，按控制台可选规格）。
5. 实例策略：MVP **最小实例 1、最大实例 1**；关闭“无请求缩容到 0”，避免冷启动。
6. 健康检查：路径 `/healthz`，端口 80。
7. 公网访问：联调期可临时开启；**上线前关闭**。

> 控制台若提供“模板部署/从代码仓库持续集成”，可先用模板跑通，再换成自建 Dockerfile；不要在生产环境首次部署时同时改构建与改代码。

---

## 4. 数据库（已定：MySQL 8.0）

### 4.1 本地开发：Docker Compose MySQL 8.0

```yaml
# deploy/local/docker-compose.yml（示意）
services:
  mysql:
    image: mysql:8.0
    command: --character-set-server=utf8mb4 --collation-server=utf8mb4_0900_ai_ci
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: hm_lesson
      MYSQL_USER: hm
      MYSQL_PASSWORD: hm_dev_password
    ports: ["3306:3306"]
    volumes: ["mysql_data:/var/lib/mysql"]
    healthcheck:
      test: ["CMD", "mysqladmin", "ping", "-h", "127.0.0.1", "-uroot", "-proot"]
      interval: 5s
      retries: 20
volumes:
  mysql_data:
```

- 本机没有 Docker 时，这一步在 Debian/CI 上跑；不要在 Windows 本地装 Docker（Q16）。
- 本地连接串：`hm:hm_dev_password@tcp(127.0.0.1:3306)/hm_lesson?charset=utf8mb4&parseTime=true&loc=UTC`。
- 迁移与生产共用同一套 `migrations/`，不维护两套 SQL。

### 4.2 生产：云托管 MySQL 8.0

- 控制台创建 MySQL 8.0 实例；字符集 **utf8mb4**；库名 `hm_lesson`。
- **`lower_case_table_names` 在创建时固定，之后不能改**：建库前决定；本项目表名全小写，两种取值都能跑。
- 记录连接信息：内网地址、端口、用户名、密码、库名。
- 生产**关闭“自动暂停”**（或设最小 CCU），避免冷启动报 `CynosDB serverless instance is resuming, please try connecting again.`。
- 规格上限约 20 Mbps / 1000 QPS，对本项目足够；大报表查询要加索引与分页。
- **自带备份能力有限**：必须在腾讯云侧配置备份策略，并每月做一次恢复演练。

环境变量（模板可能已注入，按实际命名）：

```text
HM_DB_DRIVER=mysql
HM_DB_DSN=<user>:<password>@tcp(<host>:3306)/hm_lesson?charset=utf8mb4&parseTime=true&loc=UTC
```

### 4.3 方言约定（MySQL 8.0）

- 表名/库名全小写；字符集 `utf8mb4`，排序 `utf8mb4_0900_ai_ci`。
- `uuid` → `char(36)`；`timestamptz` → `datetime(3)`（UTC 存）；`jsonb` → `json`；`boolean` → `tinyint(1)`。
- 无部分唯一索引：预约冲突用生成列 `active_flag` + `UNIQUE(slot_id, active_flag)`（见 data-model §1.1）。
- 无 `RETURNING`/`ON CONFLICT`/`FILTER`/`ILIKE`；用 `LAST_INSERT_ID()`、`ON DUPLICATE KEY`、应用层过滤。
- `CHECK` 需 8.0.16+；`TEXT` 不能有默认值，需默认值时用 `VARCHAR`。
- 不再提供 PostgreSQL 路线；如未来迁移，另开 ADR。

---

## 5. 对象存储（云托管内置 COS）

- 控制台开通对象存储；记录 bucket、地域、访问域名。
- 小程序端：

```ts
const res = await wx.cloud.uploadFile({
  cloudPath: `hm/${bizType}/${uuid}.${ext}`,
  filePath: tempFilePath,
})
// res.fileID 入库；不要存临时路径
```

- 展示：`<image src="{{fileID}}" />` 可直接用 `cloud://` 地址。
- 下载：`wx.cloud.downloadFile({ fileID })` → `tempFilePath` → `wx.openDocument`。
- 服务端：用腾讯云 COS SDK 读写同一 bucket；对象键与 fileID 可互转。若 SDK 需要密钥，使用**最小权限子账号**，放环境变量，禁止入库/入镜像。
- 权限：私有读。客户端拿到 fileID 不等于有权限，仍要经云托管鉴权；服务端签发短期下载链接或流式转发。
- 本地开发：无法直接调云存储，抽象 `storage.Store`；`deploy/local/docker-compose.yml` 起 MinIO 容器（`minio/minio:latest`，`server /data --console-address :9001`），仅用于 H5/本地调试。**生产实现不得依赖本地磁盘或自建 MinIO**：小程序没有域名/ICP 时，文件上传只能走 `wx.cloud.uploadFile` → COS；`wx.uploadFile` 到自建 MinIO 需要合法域名，`callContainer` 又只有 100 KiB 请求体，代理图片不可行。
- 本地 MinIO 环境变量：`HM_STORAGE_DRIVER=minio`、`HM_S3_ENDPOINT=http://127.0.0.1:9000`、`HM_S3_PUBLIC_ENDPOINT=http://127.0.0.1:9000`、`HM_S3_BUCKET=hm-dev`；生产改为 `HM_STORAGE_DRIVER=cos`。

---

## 6. Redis（可选）

- 云托管不内置 Redis；需要时在**腾讯云上海**购买 Redis，通过云托管「私有网络设置」打通 VPC，内网访问。
- 一期可不用：
  - 限流：MySQL 计数表 + 进程内令牌桶；MVP 单实例可用。
  - 缓存：进程内 LRU（短 TTL），不做跨实例一致性。
  - 分布式锁：MVP 不需要；扩到多实例前必须补 Redis 或数据库锁。
- 一旦 `max instances > 1`，**必须**把限流/锁换成 Redis 或数据库实现，不能靠进程内状态。

---

## 7. 环境变量

在云托管服务「环境变量」中配置；敏感值只放控制台，不写 Dockerfile。

| 变量 | 示例 | 说明 |
| --- | --- | --- |
| `HM_ENV` | `prod` | 环境标识 |
| `HM_HTTP_ADDR` | `0.0.0.0:80` | 云托管要求容器监听 80 |
| `HM_DB_DRIVER` | `mysql` / `postgres` | 与 Q22 一致 |
| `HM_DB_DSN` | 见 §4 | 含密码，敏感 |
| `HM_JWT_SECRET` | 随机 32+ 字节 | 家长/教师 token 签名 |
| `HM_WX_APP_ID` | `wx...` | 小程序 AppID |
| `HM_WX_APP_SECRET` | `...` | 仅订阅消息、`code2session` 兜底；callContainer 主路径不需要，能不用就不配 |
| `HM_WX_TPL_*` | `...` | 订阅消息模板 ID；未配置则只发站内信 |
| `HM_STORAGE_DRIVER` | `cloud` / `local` | 生产 `cloud` |
| `HM_STORAGE_BUCKET` | `...` | COS bucket |
| `HM_STORAGE_REGION` | `ap-shanghai` | 与云托管同地域 |
| `HM_REDIS_ADDR` | 空或内网地址 | 不用 Redis 时留空 |
| `HM_LOG_LEVEL` | `info` | `debug` 仅联调 |
| `HM_TIMEZONE` | `Asia/Shanghai` | 展示时区；存库仍 UTC |

> 云托管模板可能自动注入 `CBR_ENV_ID`、`MYSQL_USERNAME`、`MYSQL_PASSWORD`、`MYSQL_ADDRESS`。可以复用，但业务配置要显式映射，禁止在代码里散落读取。

---

## 8. 数据库迁移

### 8.1 推荐：启动时执行（单实例）

容器启动命令：

```bash
./hm-migrate up && ./hm
```

- 迁移前先取数据库锁：MySQL `SELECT GET_LOCK('hm_migrate', 60)`（已写入 `cmd/migrate` 的实现约定）。
- 拿不到锁就退出并让云托管重试；不要并发执行。
- 迁移失败**不要起服务**（fail fast），避免旧代码配新表/新代码配旧表。
- 发布前备份；破坏性 DDL 必须单独评审。

### 8.2 备选：CI 一次性任务

- 在部署流水线里先跑迁移镜像/任务，成功后再发布新版本。
- 适合多人协作与多环境；需要把数据库网络对 CI 开放（不推荐公网），或跑在云托管内的一次性任务里。

### 8.3 版本校验

- 服务启动读取 `schema_migrations`，低于代码要求则拒绝启动并打印待执行版本。
- 健康检查 `/healthz` 不查库；`/readyz` 查库与存储，用于发布后验证。

---

## 9. 首次部署（控制台）

1. 本地构建镜像，确认 `docker build` 通过（本机无 Docker 时在 Debian/CI 构建）。
2. 推送镜像到 TCR（上海）。
3. 云托管 → 服务 `hm-api` → 新建版本：选镜像、端口 80、实例 1/1、环境变量、健康检查 `/healthz`。
4. 部署后看日志：应打印监听端口、DB 连接、迁移版本、实际微信 header 名。
5. 用云托管“调试/公网访问”临时验证：
   - `GET /healthz` → 200
   - `GET /readyz` → 200
   - `GET /api/v1/meta`（若有）→ 版本信息
6. 关闭公网访问，改用小程序 `callContainer` 联调。
7. 首次管理员账号：通过一次性 CLI/种子创建，**不要**在公网暴露初始化接口；创建后立即改密。

---

## 10. CI/CD（GitHub Actions）

### 10.1 流程

```text
push main
  → go build / test / buf lint
  → pnpm build:mp（可选，前端单独流水线）
  → docker build
  → push TCR
  → wxcloud 部署新版本
  → 健康检查 /readyz
  → 失败自动回滚上一版本
```

### 10.2 Secrets（仓库级）

| Secret | 用途 |
| --- | --- |
| `WX_CLOUD_ENV_ID` | 云托管环境 ID |
| `WX_CLOUD_SERVICE` | `hm-api` |
| `TCR_REGISTRY` / `TCR_NAMESPACE` / `TCR_REPO` | 镜像地址 |
| `TCR_USERNAME` / `TCR_PASSWORD` | 推送镜像 |
| `WX_CLOUD_PRIVATE_KEY` | 云托管 CLI 私钥（如使用） |
| `TENCENT_SECRET_ID` / `TENCENT_SECRET_KEY` | 仅当 CLI 需要；优先用私钥/最小权限 |

- 私钥只放 GitHub Secrets，禁止打印；日志里出现即视为泄露，立即轮换。
- 部署 job 只允许 `main` 分支与手动触发；PR 只跑测试，不部署。
- 具体 CLI 命令以云托管控制台「持续集成/CLI」生成的为准；本文不写死参数。

### 10.3 回滚

- 云托管保留历史版本：控制台回滚到上一版本即可。
- 数据库迁移**不自动回滚**：破坏性变更必须向前兼容（先加列、后删列，分两次发布）。
- 回滚前确认新版本没有写入旧代码不认识的数据。

---

## 11. 小程序端接入

### 11.1 初始化

```ts
// app.ts / 启动页
wx.cloud.init({ env: 'prod-xxxx', traceUser: true })
```

基础库最低：`callContainer` **2.23.0**，`connectContainer` **2.21.1**；`app.json` 里设置最低基础库，低版本给出升级提示。

### 11.2 请求封装

```ts
// utils/request.ts（微信端）
const res = await wx.cloud.callContainer({
  config: { env: ENV_ID },
  path: '/api/v1/tasks?page=1&page_size=20',
  method: 'GET',
  header: { 'X-WX-SERVICE': 'hm-api', 'X-HM-Token': token },
  data: undefined,
})
```

- `path` 必须以 `/` 开头；服务名通过 `header['X-WX-SERVICE']` 指定（示例里的 `hm-api`），不在 path 里写服务名。若环境只有一个服务，省略该 header 通常也能路由；显式写更稳。
- 请求体 ≤100 KiB、响应 ≤1000 KiB、超时 ≤15 s；大文件走 `wx.cloud.uploadFile`。
- H5 调试才用 `uni.request`；通过条件编译隔离，禁止运行时判断平台后混用。

### 11.3 WebSocket

```ts
const { socketTask } = await wx.cloud.connectContainer({
  config: { env: ENV_ID },
  service: 'hm-api',
  path: '/api/v1/ws',
})
socketTask.onOpen(() => socketTask.send({ data: JSON.stringify({ type: 'auth', token }) }))
```

- 首帧必须发应用层 token；不要只依赖 `x-wx-openid`。
- iOS「高性能+」模式可能读不到微信身份 header，token 是兜底路径。
- 断线重连：指数退避，最多 5 次；前台恢复时重连；收到 401 停止重连并跳登录。

### 11.4 文件

```ts
const { fileID } = await wx.cloud.uploadFile({ cloudPath, filePath })
await api.submitHomework({ taskId, text, fileIDs: [fileID] })
```

- 禁止把图片转 base64 塞 JSON。
- 下载：`wx.cloud.downloadFile` → `wx.openDocument`；大文件提示进度。

---

## 12. 域名与证书（仅在需要时）

- 小程序上线：**跳过本节**。
- 需要 H5/Web/对外 API：
  1. 域名 ICP 备案（当前没有，需先办；主体要与小程序/云托管一致）；
  2. 云托管 → 自定义域名 → 添加域名，CNAME 到云托管提供的目标；
  3. 上传证书（crt/key，nginx 格式）；云托管**不自动签发**，到期前手动续；
  4. 1 个域名对应 1 个服务；1 个服务可绑多个域名；
  5. 小程序后台仍需把该域名加入合法域名（如果走 `request` 而非 `callContainer`）。

---

## 13. 定时任务与异步任务

- 云托管支持定时触发，但要求**最小实例 ≥1**；MVP 设 1/1。
- 禁止在请求返回后靠 goroutine 继续跑；实例可能被回收。
- 长任务统一落 `jobs` 表：

| 任务 | 触发 | 执行 |
| --- | --- | --- |
| Outbox 投递 | 每 1 分钟 | 取未发送，调订阅消息/写站内信，失败退避 |
| 回访提醒 | 每日 08:30 | 查 `next_follow_up_at`，给老师发站内信 |
| 课前 24h 提醒 | 每小时 | 查次日预约，发订阅消息 |
| 课后未补录 | 次日 09:00 | 查昨日已完成未记录，提醒老师 |
| 课时对账 | 每日 03:00 | 流水汇总 vs 余额缓存，不一致告警 |
| 导出 | 请求触发 | 建 job，分片生成 xlsx 到 COS，前端轮询 fileID |

- 每个 job 用数据库行锁/状态机抢占（`status=running` + `locked_until`），保证单实例也不重复。
- 多实例前必须换 Redis 锁或数据库 advisory lock。
- 定时触发配置在控制台；容器内不依赖 cron daemon。

---

## 14. 可观测与告警

- 日志：stdout JSON；云托管日志检索按 `trace_id`/`request_id`。
- 指标：请求量、5xx、p95 延迟、DB 连接数、job 失败数；云托管监控 + 腾讯云监控。
- 告警：5xx 比例 >1% 持续 5 分钟、`/readyz` 连续失败、job 积压 >100、磁盘/连接数接近上限。
- 审计：`audit_logs` 表，不依赖日志留存。
- 禁止日志打印手机号、openid 全量、作业正文。

---

## 15. 备份与恢复

| 对象 | 方式 | 频率 | 恢复演练 |
| --- | --- | --- | --- |
| MySQL | 腾讯云数据库备份/手动导出（`mysqldump`） | 每日 | 每月 |
| COS | 版本控制/跨地域复制（可选） | 实时/每日 | 每季度 |
| 配置/密钥 | 密码管理器/控制台 | 变更时 | — |

- 备份文件加密，访问权限最小化。
- 恢复顺序：数据库 → 对象存储 → 服务；恢复后跑 `/readyz` 与关键链路冒烟。
- 删除类操作先确认有备份；生产禁止 `DROP DATABASE` 类脚本。

---

## 16. 安全检查清单（上线前）

- [ ] 关闭云托管公网访问
- [ ] `HM_JWT_SECRET` 为随机强密钥，未使用默认值
- [ ] DB 密码、COS 密钥只在控制台/Secrets
- [ ] 家长越权返回 404（抽测）
- [ ] 教师模式服务端校验 role（改前端参数也进不去）
- [ ] 上传扩展名/MIME/大小校验；fileID 不能越权读他人文件
- [ ] WS 首帧 token 校验；无 token 断开
- [ ] 登录/绑定/提交/预约限流生效
- [ ] 日志无敏感信息
- [ ] 备份已配置且做过一次恢复
- [ ] 隐私协议、用户授权、订阅消息模板已配置
- [ ] 体验版真机跑完 TC-01~TC-17

---

## 17. 成本估算（量级，非报价）

以“1 服务 + 1 MySQL + 少量 COS + 可选 Redis”计：

| 项 | 计费方式 | 备注 |
| --- | --- | --- |
| 云托管 | 实例规格 × 时长 + 流量 | 1/1 常驻，低配即可 |
| MySQL | Serverless CCU/存储 | 关闭自动暂停会增加常驻成本 |
| COS | 存储 + 请求 + 流量 | 作业图片是主要增量 |
| Redis | 实例规格 | 可选，一期可不买 |
| TCR | 存储/流量 | 镜像小，费用低 |

> 具体金额以腾讯云账单为准；上线前用控制台“费用预估”核对，设置预算告警。

---

## 18. 故障排查

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| 部署后 502 | 容器未监听 80 / 启动崩溃 | 看启动日志；确认 `HM_HTTP_ADDR=0.0.0.0:80` |
| 健康检查失败 | `/healthz` 未实现或路径错 | 实现不查库的健康检查 |
| 小程序调不通 | 未 `wx.cloud.init` / env 错 / 服务名错 | 核对 env 与服务名；看云托管调用日志 |
| 401/身份为空 | header 名大小写不符 / 未走网关 | 启动打印 header；只信 `X-WX-*` |
| DB 连接超时 | MySQL 暂停 / VPC 未通 / 安全组 | 关闭自动暂停；检查 VPC 与安全组 |
| `CynosDB ... resuming` | Serverless 冷启动 | 重试 + 关闭自动暂停 |
| 上传 403 | fileID 不属于当前用户 / 桶权限 | 服务端校验归属；私有读配置 |
| WS 连不上 | 基础库低 / 公网关闭后仍用公网地址 | 升基础库；改用 `connectContainer` |
| 迁移锁等待 | 多实例同时启动 | 确认 `min=max=1`；检查锁超时 |
| 定时任务不跑 | 实例缩容到 0 / cron 配错 | 最小实例 1；核对时区 |

---

## 19. 上线顺序（建议）

1. 回复 Q19/Q22/Q26：主体、数据库、AppID 时点。
2. 创建云托管环境与服务；打通数据库；开通 COS。
3. 部署最小版本：`/healthz`、`/readyz`、迁移、日志。
4. 小程序接 `callContainer` 跑通登录/绑定。
5. 按 M1→M6 交付业务；每个里程碑在云托管 dev 环境验收。
6. 关闭公网访问；配置告警、备份、预算。
7. 体验版真机回归；提审；发布。
8. 发布后 7 天观察 5xx、p95、job 失败、存储增长。

---

## 20. 已确认结论（2026-11-22）

| 编号 | 事项 | 结论 / 上线前动作 |
| --- | --- | --- |
| Q19 | 类目与主体 | **A：个人主体 + 工具类目先提审**；先去控制台确认云托管能否开通，被拒再变更主体 |
| Q22 | 数据库 | **MySQL 8.0**：本地 Docker 容器，生产云托管 MySQL；关闭自动暂停 |
| Q23 | 对象存储 | **生产 COS/fileID**；本地 MinIO 容器，仅 dev |
| Q24 | Redis | **一期不用**；扩多实例前再补 |
| Q25 | 自定义域名 | **不绑**；未来 H5/Web 再办 |
| Q26 | AppID | **不变更**；直接建生产环境 |
| Q27 | CI | **控制台持续集成**起步；Actions 后续加门禁 |
| Q28 | 功能范围 | 作业 + 预约 + 课时 + 资料 + 通知 + 看板全保留；IM 不做 |
