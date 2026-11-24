# 前端需求规格 · HM-Lesson

> 业务蓝本：[`鹤鸣老师课堂-飞书家校系统搭建需求与实施清单.docx`](<鹤鸣老师课堂-飞书家校系统搭建需求与实施清单.docx>)
> 技术基线：uni-app（Vue3 + Vite + TypeScript）+ pnpm + wot-design-uni；**一个微信小程序，家长端主包 + 教师模式分包**（Q1=B）
> 读者：前端开发、UI、测试、后续接手的 AI Agent
> 关联：[README](../README.md) · [backend-requirements.md](backend-requirements.md) · [open-questions.md](open-questions.md)

---

## 1. 客户端范围

| 客户端 | 用户 | 优先级 | 形态 | 说明 |
| --- | --- | --- | --- | --- |
| **家长端（主包）** | 家长 | P0 | uni-app → 微信小程序 | 唯一家长入口，替代飞书业务小程序 |
| **教师模式（`pkg-teacher` 分包）** | 老师（owner）；助教二期 | P0 | 同一小程序，登录后按角色分流 | 工作台、学员、跟进、任务/批阅、排课、台账、资料、通知、导入导出、设置；按 375px 手机宽度设计 |
| H5 | 内部调试 | 辅助 | uni-app H5 构建 | 仅开发联调，不对外发布 |
| 学生端 | — | 不做 | — | 一期不设独立学生账号 |

> **为什么不做独立 Web**：Q1=B。少一套构建/部署/登录；老师日常操作以“看一条、点两下”为主；教师页面放分包，家长端主包体积不受影响。代价是批阅与看板必须为手机重新设计（见 §6）。

## 2. 技术栈与工程结构

| 项 | 选型 | 约束 |
| --- | --- | --- |
| 框架 | uni-app（Vue3 + Vite）官方 `vite-ts` 模板 | 只用 Vue3 组合式 API；不用 Vue2 语法 |
| 语言 | TypeScript `strict` | `noImplicitAny`、`strictNullChecks` 全开；禁止 `any`（万不得已用 `unknown` + 收窄） |
| 包管理 | **pnpm**（workspace） | 禁止 npm/yarn；提交 `pnpm-lock.yaml`；Node 22 LTS |
| UI | [wot-design-uni](https://github.com/Moonofweisheng/wot-design-uni) 1.14.x | TS 友好、easycom 自动引入、主题变量可覆盖 |
| 状态 | Pinia | 按域拆 store；禁止把所有状态塞一个 store |
| 请求 | 微信端 `wx.cloud.callContainer` + H5 调试 `uni.request`，统一自封装 + OpenAPI 生成类型 | 不引入 axios；小程序生产路径不走 `uni.request` |
| 时间 | dayjs（按需引入） | 统一 `Asia/Shanghai` 展示 |
| 校验 | wot 表单规则 + 轻量共享校验函数 | 不引 zod（控制包体） |
| 样式 | SCSS + 设计令牌（CSS 变量 / SCSS 变量） | 禁用 Tailwind/UnoCSS（小程序兼容与产物体积考虑） |
| 图表 | **不引入** | 教师看板用“卡片 + 进度条 + 名单下钻”，小程序端禁止 ECharts |
| 代码质量 | ESLint flat config + Prettier + Stylelint + vue-tsc | 提交前 lint-staged |
| 测试 | Vitest + Vue Test Utils；miniprogram-automator（可选） | 关键工具函数与 store 必须有测试 |

### 2.1 目录

```text
frontend/
├── pnpm-workspace.yaml
├── package.json                 # 根脚本：lint / typecheck / test / build
├── apps/
│   └── mp/                      # 单小程序：家长端主包 + 教师模式分包
│       ├── src/
│       │   ├── pages/           # 主包：启动/绑定/首页/任务/我的
│       │   ├── pages-sub/       # 家长端分包：预约、作业档案、资料、通知
│       │   ├── pkg-teacher/     # 教师模式分包（页面 + 组件 + 教师专有 api）
│       │   ├── components/      # 家长端业务组件
│       │   ├── composables/     # useAuth / usePagination / useUpload / useRole
│       │   ├── stores/          # auth / app / notification / teacher
│       │   ├── api/             # 模块化请求函数（类型来自 packages/api-types）
│       │   ├── utils/           # request / format / upload / guard
│       │   ├── styles/          # tokens.scss / mixins.scss
│       │   ├── static/          # 仅放小图与图标，禁止大图
│       │   ├── App.vue
│       │   ├── main.ts
│       │   ├── manifest.json
│       │   ├── pages.json       # 含 subPackages：pages-sub 与 pkg-teacher
│       │   └── uni.scss
│       ├── project.config.json  # 微信开发者工具配置（appid 分环境）
│       └── vite.config.ts
└── packages/
    ├── api-types/               # OpenAPI → TS 类型（生成物，不手改）
    └── shared/                  # 枚举文案、格式化、校验、常量、设计令牌导出
```

> 教师页面只能放 `pkg-teacher/`，家长端页面不得 import 教师模块；公共组件下沉 `packages/shared` 或 `components/`。

### 2.2 环境与构建

- 环境：`dev` / `staging` / `prod`，通过 `.env.[mode]` 注入 `VITE_WX_CLOUD_ENV_ID`（小程序必填）与 `VITE_API_BASE_URL`（仅 H5 调试）。**小程序生产不配置接口域名**。
- 小程序 `appid` 不写死在源码：`manifest.json` 用构建变量或 `project.config.json` 分环境覆盖；`appid` 不提交敏感 secret（小程序无 secret）。
- 构建命令：`pnpm --filter @hm/mp dev:mp-weixin`、`pnpm --filter @hm/mp build:mp-weixin`。
- 产物：`apps/mp/dist/dev/mp-weixin` 导入微信开发者工具；CI 只做构建与类型检查，不做真机。

## 3. 微信平台约束（必须提前确认，见 Q2）

### 3.1 主体与类目（Q2：个人主体、未认证；Q19：按工具类目）

- 定位是**工具类小程序**：作业收集、任务批阅、预约与课时记录、资料与通知；**不做教育培训/招生平台**，不出现课程售卖、培训报名、直播授课、付费内容。
- **即时通讯整体砍掉**：没有家长-老师聊天、群聊、客服会话、私信；站内“消息中心”只承载系统通知（批阅结果、预约结果、提醒），不提供用户间发送消息的入口。
- 当前个人主体不能使用手机号快速验证（`getPhoneNumber`），类目以微信后台实际可选的**工具/效率**子类目为准；先按个人主体提审，不把主体变更当硬前置。
- 代码层面不依赖任何认证主体能力：绑定走手动手机号 + 审批/绑定码；不做微信支付。
- **小程序备案已完成**（2026-11-22 确认，个人主体可办）。
- 若审核要求企业主体/额外资质，再按 Q19 变更；AppID、主体名称、隐私协议主体全部走配置，变更时只换配置。

### 3.2 登录与绑定（与后端 §4.3 对应）

```text
打开小程序
  └─ wx.login → code
       └─ POST /api/v1/auth/wx/login
            ├─ 已绑定且 active → 发 token → 按角色分流（staff → 教师工作台；guardian → 首页）
            ├─ 未绑定 → bind_ticket → 绑定页
            │     └─ 家长手动输入手机号（+ 可选 6 位绑定码）
            │           ├─ 命中未绑定档案 → 提交申请 → pending 待审批页
            │           ├─ 命中已被他人绑定 → 409 → “请联系老师重置绑定”
            │           ├─ 未命中 + 有效绑定码 → 直接 active → 首页
            │           └─ 未命中 + 无码 → “请联系老师建档/索取绑定码”
            └─ 已绑定 pending → 待审批页（可刷新）
```

- **先绑定再可见**，不得做“先浏览后绑定”。
- 绑定页由家长**手动输入手机号**，不调用 `getPhoneNumber`（个人主体不可用，且避免主体变更后返工）。
- 待审批页展示老师联系方式与“重新检查”按钮；不展示任何学员数据。
- 绑定码：6 位、7 天有效、一次性；错误提示不泄露“手机号是否存在”，统一“信息不匹配，请联系老师”。
- token 过期自动 refresh；refresh 失败回登录页并清缓存。

### 3.3 订阅消息

- 一次性订阅，必须在**用户点击动作**后调用 `wx.requestSubscribeMessage`（提交作业、提交预约、确认回执时顺带请求）。
- 用户拒绝不阻断主流程，仅站内信触达；文案要说明“开启后可收到批阅/上课提醒”。
- 模板 ID 由后端配置下发（`GET /api/v1/me` 或配置接口），前端不硬编码。
- 不能假设能无限推送（长期订阅受类目限制）。

### 3.4 网络接入（微信云托管）

- **不需要配置服务器域名，也不需要域名 ICP 备案**：小程序用 `wx.cloud.callContainer`（HTTP）和 `wx.cloud.connectContainer`（WebSocket），请求直接进云托管网关。小程序备案已完成。
- 基础库要求：`callContainer` ≥ **2.23.0**，`connectContainer` ≥ **2.21.1**；`app.json` 里无需 `request`/`socket` 合法域名。
- 启动时 `wx.cloud.init({ env: '<云托管环境ID>' })`；环境 ID 从构建变量注入，不硬编码在页面。
- 请求体 ≤ **100 KiB**、响应 ≤ **1000 KiB**、超时 ≤ **15 s**：图片/文件必须先 `wx.cloud.uploadFile` 拿 `fileID` 再提交；列表必须分页；导出走异步文件。
- 图片/附件显示用 `<image src="cloud://...">`（支持），下载用 `wx.cloud.downloadFile` 后 `wx.openDocument`。
- 生产关闭云托管“公网访问”；关闭后 H5/浏览器无法直连，H5 只用本地 mock 或开发环境，不作为上线端。
- 只有未来做独立 Web/对外 API 时才需要自定义域名：必须 ICP 备案 + 手动上传证书（无自动 HTTPS），见 [部署手册](deploy-wechat-cloudrun.md)。
- 本地/开发者工具：`wx.cloud` 需真实 AppID 且已开通云托管；无环境时用 H5 调试适配层（mock 或 `uni.request` 指向本地），不要把本地地址写进小程序生产代码。

### 3.5 包体与性能

- 主包 ≤2MB，整包 ≤20MB（以微信最新限制为准）。
- 主包只放：启动/绑定、首页、任务列表/详情/提交、我的；预约、资料、档案、通知进 `pages-sub`。
- 教师模式整体放 `pkg-teacher` 分包（目标 ≤4MB），首次进入才下载；家长端主包不引用其任何文件。
- 图片：本地只放 icon 与 ≤50KB 的插画；列表图用网络图 + `lazy-load`；上传前压缩（长边 ≤2000px，质量 0.8）。
- 首屏：骨架屏 + 并行请求（`/api/v1/me` + `/api/v1/portal/home`），目标 4G 下 <2s 可交互。
- 禁止在小程序里引入 ECharts、富文本编辑器、大型 UI 全量包。

### 3.6 隐私与合规

- 配置《用户隐私保护指引》，首次调用隐私接口前处理 `wx.requirePrivacyAuthorize`。
- 收集范围：微信头像昵称（可选）、手机号（绑定必需）、作业图片/文件、成绩。逐项说明用途。
- 家长可查看隐私政策；提供“注销/删除数据”入口（走老师确认，后端归档+匿名化）。
- 不在前端日志、埋点中输出手机号、成绩、作业内容。

## 4. 信息架构

### 4.1 家长端 tabBar（4 个）

| Tab | 路径 | 内容 |
| --- | --- | --- |
| 首页 | `pages/home/index` | 问候、孩子切换、5 张功能卡、待办提醒、最近动态 |
| 任务 | `pages/tasks/index` | 待提交 / 已提交待批阅 / 已批阅（含打回） |
| 预约 | `pages-sub/booking/index` | 我的预约 + 发起预约 |
| 我的 | `pages/profile/index` | 孩子与家长信息、课时、资料、通知、隐私、联系老师 |

> 作业档案、资料、通知、课时详情均从首页/我的进入分包页面，不占 tab。

### 4.2 页面清单

| # | 页面 | 路径 | 分包 | 优先级 |
| --- | --- | --- | --- | --- |
| P01 | 启动/登录 | `pages/launch/index` | 主 | P0 |
| P02 | 绑定手机号 | `pages/bind/index` | 主 | P0 |
| P03 | 待审批 | `pages/bind/pending` | 主 | P0 |
| P04 | 首页 | `pages/home/index` | 主 | P0 |
| P05 | 任务列表 | `pages/tasks/index` | 主 | P0 |
| P06 | 任务详情 | `pages/tasks/detail` | 主 | P0 |
| P07 | 提交作业 | `pages/tasks/submit` | 主 | P0 |
| P08 | 提交成功/回执 | `pages/tasks/result` | 主 | P1 |
| P09 | 作业档案 | `pages-sub/archive/index` | 子 | P1 |
| P10 | 作业档案详情 | `pages-sub/archive/detail` | 子 | P1 |
| P11 | 预约首页/我的预约 | `pages-sub/booking/index` | 子 | P0 |
| P12 | 选择时段 | `pages-sub/booking/slots` | 子 | P0 |
| P13 | 预约确认（填诉求） | `pages-sub/booking/create` | 子 | P0 |
| P14 | 预约详情 | `pages-sub/booking/detail` | 子 | P0 |
| P15 | 资料列表 | `pages-sub/materials/index` | 子 | P1 |
| P16 | 资料详情/预览 | `pages-sub/materials/detail` | 子 | P1 |
| P17 | 课时与课表 | `pages-sub/hours/index` | 子 | P1 |
| P18 | 通知列表 | `pages-sub/notices/index` | 子 | P1 |
| P19 | 通知详情/回执 | `pages-sub/notices/detail` | 子 | P1 |
| P20 | 消息中心 | `pages-sub/notifications/index` | 子 | P1 |
| P21 | 我的 | `pages/profile/index` | 主 | P0 |
| P22 | 隐私政策/关于 | `pages-sub/about/index` | 子 | P1 |

### 4.3 教师模式底部导航（分包，仅 staff 可见）

| Tab | 路径 | 内容 |
| --- | --- | --- |
| 工作台 | `pkg-teacher/pages/home/index` | 今日待办（待审批预约/待批阅/待确认消课/待审批绑定）、课时预警、快捷入口 |
| 学员 | `pkg-teacher/pages/students/index` | 搜索、分组筛选、学员卡片；详情含跟进/任务/课时/绑定 |
| 任务 | `pkg-teacher/pages/tasks/index` | 任务列表、下发、批阅队列、统计 |
| 我的 | `pkg-teacher/pages/profile/index` | 账号、成员（owner）、导入、导出、审计、设置、切回家长视图（若同时是家长） |

教师页面清单（全部 `pkg-teacher/pages/` 下，P0 优先）：

| # | 页面 | 路径 | 优先级 |
| --- | --- | --- | --- |
| T01 | 教师工作台 | `home/index` | P0 |
| T02 | 学员列表 | `students/index` | P0 |
| T03 | 学员详情 | `students/detail` | P0 |
| T04 | 新建/编辑学员 | `students/edit` | P0 |
| T05 | 跟进记录 | `students/follow-ups` | P0 |
| T06 | 绑定审批 | `bindings/index` | P0 |
| T07 | 绑定码生成 | `bindings/code` | P1 |
| T08 | 任务列表 | `tasks/index` | P0 |
| T09 | 下发任务 | `tasks/create` | P0 |
| T10 | 批阅队列 | `tasks/review-queue` | P0 |
| T11 | 批阅详情 | `tasks/review` | P0 |
| T12 | 任务统计 | `tasks/stats` | P1 |
| T13 | 时段管理 | `slots/index` | P0 |
| T14 | 预约审核 | `bookings/index` | P0 |
| T15 | 预约详情/消课 | `bookings/detail` | P0 |
| T16 | 课时账户 | `lessons/accounts` | P0 |
| T17 | 消课流水 | `lessons/ledger` | P0 |
| T18 | 调整课时 | `lessons/adjust` | P0 |
| T19 | 班课列表 | `classes/index` | P1 |
| T20 | 班课详情/上课记录 | `classes/detail` | P1 |
| T21 | 资料管理 | `materials/index` | P1 |
| T22 | 通知/回执 | `notices/index` | P1 |
| T23 | 回执名单 | `notices/receipts` | P1 |
| T24 | Excel 导入 | `imports/index` | P0 |
| T25 | 看板 | `dashboard/index` | P1 |
| T26 | 导出中心 | `exports/index` | P1 |
| T27 | 成员管理（owner） | `staff/index` | P1 |
| T28 | 审计日志 | `audit/index` | P2 |
| T29 | 设置（课时规则/通知模板） | `settings/index` | P2 |

> 教师页面不得被家长端主包引用；`pages.json` 中 `pkg-teacher` 为 `independent: false` 的普通分包（需与主包共享登录态）。
>
> **tabBar 约束**：微信原生 tabBar 页面必须在主包，因此教师模式**不使用原生 tabBar**，而是在 `pkg-teacher` 内用共享组件 `<TeacherTabBar>` 固定在底部，切换用 `redirectTo`（避免页面栈膨胀）。家长端仍用原生 4 tab；教师登录后 `reLaunch` 到 `pkg-teacher/pages/home/index`，切回家长视图用 `reLaunch` 到家长首页。`TeacherTabBar` 必须处理 `safe-area-inset-bottom` 与分包异步加载时的占位，避免闪烁。

## 5. 页面规格（家长端）

> 通用要求：每个页面必须有 **加载态（骨架）/ 空态 / 错误态（可重试）**；下拉刷新；列表分页到 20 条触底加载；错误文案说人话，不显示错误码。

### P01 启动/登录

- 启动即静默 `wx.login` → `/api/v1/auth/wx/login`；成功且 `role=staff` → `reLaunch` 到教师工作台（`pkg-teacher/pages/home/index`）；`role=guardian` 且已绑定 → 家长首页；未绑定 → 绑定页；待审批 → 待审批页。
- 角色以服务端 `/api/v1/me` 为准，前端不缓存角色做权限判断；家长端与教师端共用登录页。
- 失败：展示“网络开小差了”+ 重试；不无限转圈。
- 首次进入展示品牌页（≤800ms，可跳过），不做强制授权弹窗。

### P02 绑定手机号

- 说明文案：“绑定后老师确认，才能看到孩子的作业与课时”。
- 手机号**手动输入**（`type="number"`，11 位校验）；可选“绑定码”输入框（6 位，老师提供）。
- 隐私勾选（同意《隐私政策》与《用户协议》）后才能点。
- 提交 → 待审批页；已绑他人 → 提示“该手机号已被绑定，请联系老师重置”。
- **禁止**使用 `<button open-type="getPhoneNumber">`。

### P03 待审批

- 展示“已提交，等待老师确认”，老师微信/电话（可复制）。
- “重新检查”按钮；下拉刷新。
- 不展示任何学员姓名/任务。

### P04 首页

- 顶部：孩子姓名 + 年级 + 分组（只读）+ 切换孩子（多孩）。
- 5 张卡片（蓝本第 10 章）：
  1. **我的任务**：待提交数、待批阅数、最近截止；点击进任务 tab；
  2. **作业记录**：最近一次批阅结果与评语摘要；
  3. **一对一预约**：可预约时段数 / 待审核 / 下次上课时间；
  4. **我的资料**：按分组可见的最新 3 份；
  5. **课时与上课记录**：剩余课时、最近课表；余额 ≤2 高亮提醒，≤0 显示“课时已用完，请联系老师”。
- 待办区：未确认回执、被打回作业。
- 空态：新学员无任务时显示引导文案，不显示假数据。
- 请求聚合：`GET /api/v1/portal/home`；失败时降级为并行 3 个接口，不阻塞整页。

### P05 任务列表

- Tab：待提交（含被打回）、已提交、已批阅；默认“待提交”。
- 卡片：标题、任务类型标签、截止时间、状态标签（待提交/已提交/已批阅/已打回/逾期）、附件数。
- 逾期：红色“已逾期”标签，但仍可提交（文案说明“仍可补交”）。
- 筛选：全部/作业/订正/打卡；按截止时间排序。
- 空态：统一 `EmptyState`（插画 + 一句引导 + 可选按钮），文案“暂时没有任务”。

### P06 任务详情

- 标题、说明（可折叠）、附件（预览/下载）、截止时间、状态、批阅结果。
- 已批阅：评分、评语、批改图片、批阅时间。
- 已打回：打回理由置顶显示 + “重新提交”按钮。
- 已提交：展示每次提交时间与内容（最新在前），可查看历史 attempt。
- 操作：待提交/已打回 → 去提交；已批阅 → 只读。

### P07 提交作业

- 三种输入可并存：文字（≤5000 字）、图片（≤9，拍照/相册）、文件（≤3，微信文件）。
- 图片：压缩后上传，显示进度与失败重试；支持预览、删除、排序。
- 文件：展示文件名/大小；超限提示。
- 截止后提交：顶部黄色提示“已过截止时间，提交将标记为逾期”。
- 提交前二次确认（尤其多图）；成功后 → P08。
- 防重复：提交按钮 loading + 禁用；网络失败保留草稿（本地 storage，按 assignment 维度）。

### P09/P10 作业档案

- 按时间倒序的作业时间线：任务、提交、评分、评语、打回理由。
- 支持按学期/类型筛选；详情只读；可查看批改图片大图。

### P11 预约首页

- 顶部“可预约”入口 + 我的预约列表（待审核/已通过/已完成/已取消/已驳回）。
- 状态说明：待审核=已占位，老师确认后才成功；已驳回显示理由。
- 余额 ≤0：入口禁用 + “课时已用完”提示（与后端 422 一致）。
- 空态：引导“去看看老师开放的时段”。

### P12 选择时段

- 按日期分组（今天/明天/本周），只显示 `open` 且未开始；显示起止时间、时长、备注。
- 多选禁止；点击选中 → 下一步。
- 无可用时段：空态 + “老师每周日放出下周时段”说明（文案可配）。
- 时区/日期格式统一：`MM月DD日 周X HH:mm`。

### P13 预约确认

- 展示学员、时段、时长；诉求备注（≤500 字，必填？建议选填但提示“写清薄弱点便于备课”）。
- 提示：“提交后时段将为你保留，需老师确认后才算预约成功”。
- 提交成功 → 详情页；409 冲突 → 提示“该时段刚被约走”，刷新时段列表。

### P14 预约详情

- 状态进度条：提交 → 审核 → 上课 → 完成；驳回/取消显示原因。
- 课前 24h 提醒说明；取消规则（是否可取消、截止时间）。
- 已完成：显示消课结果与剩余课时。

### P15/P16 资料

- 按分类分组（班课讲义/一对一资料/月考真题/试听/招生），只显示有权资料。
- 资料卡：标题、分类、年级、更新时间、文件类型/大小。
- 详情：文件列表，点击预览（pdf/图片）或下载；失败提示。
- 禁止展示无权分类的占位（不泄露存在性）。

### P17 课时与课表

- 剩余课时大字展示、累计购买/已消耗、有效期。
- 流水列表：日期、类型、增减、余额、备注；分页。
- 近期课表：未来 7 天已通过预约；过去未完成标记。
- 余额 ≤2 提示续费沟通；≤0 红字提示。

### P18/P19 通知与回执

- 列表：标题、时间、是否需要回执、回执状态。
- 详情：正文、附件；“我已确认”按钮（幂等），确认后显示时间。
- 需回执未确认：首页待办 + 红点 + 订阅消息提醒。

### P20 消息中心（仅系统通知）

- 站内信列表（批阅、打回、预约结果、提醒），未读红点；点击跳转对应业务页。
- **不是聊天**：不提供输入框、会话列表、用户间私信或客服会话；家长与老师的沟通在线下微信完成，结论由老师录入跟进记录。
- 全部已读；分页；WS 新消息插入顶部并 toast。

### P21 我的

- 孩子信息（姓名/年级/分组/学校）、家长信息（姓名/手机号脱敏）、切换孩子。
- 入口：课时、资料、作业档案、通知、隐私政策、联系老师、版本号。
- 退出登录（清 token，回登录页）。
- 注销/删除数据：说明流程，提交申请给老师（P1）。

## 6. 教师模式（P0，`pkg-teacher` 分包）

> Q1=B 已定：教师模式与家长端同一个小程序，不做独立 Web。所有页面按 375px 宽度设计；信息密度靠“分组卡片 + 筛选抽屉 + 底部操作条”实现，不搬桌面表格。

### 6.1 设计取舍

| 桌面习惯 | 手机替代 |
| --- | --- |
| 宽表格 | 卡片列表（每卡 2~3 行关键信息）+ 详情页 |
| 列排序/多条件筛选 | 顶部筛选胶囊 + 底部抽屉（`wd-action-sheet` / 自定义） |
| 批量勾选 | 长按/右上“选择”进入多选模式，底部批量操作条 |
| 右键菜单 | 卡片右侧“更多” → `wd-action-sheet` |
| 图表看板 | 指标卡 + 进度条 + Top 名单，点击下钻列表 |
| 文件下载 | `wx.downloadFile` → `wx.openDocument`；或“转发给微信好友/自己” |
| 多图批阅 | 全屏图片查看器（左右滑、双指缩放）+ 快捷评语 + 上一条/下一条 |

### 6.2 工作台（T01）

- 顶部：今日日期、问候、角色标识（owner/assistant）。
- 待办卡（可点击直达）：
  1. **待审批预约**（数量）→ T14；
  2. **待批阅作业**（数量，含逾期）→ T10；
  3. **待确认消课**（已完成未录流水）→ T15；
  4. **待审批绑定**（数量）→ T06；
  5. **课时预警**（余额 ≤2）→ T16；
  6. **今日回访**（M1）→ T02 筛选“今日回访”。
- 快捷入口：下发任务、开放时段、导入学员、发通知、看板。
- 下拉刷新；无待办时显示“今天没有待办”+ 插画，不显示假数据。

### 6.3 学员（T02–T05）

- 列表：搜索（姓名/学校/家长手机号后四位）、分组 tab（全部/班课在读/一对一在读/待转化/咨询访客/已归档/流失）、排序（下次回访/最近更新）。
- 卡片：姓名、年级、学校、分组标签、状态、下次回访（逾期红色）、课时余额（≤2 橙色）。
- 详情：基本信息、家长与绑定状态（已绑/待审批/未绑）、标签、跟进时间线、任务记录、预约记录、课时账户、资料可见范围预览。
- 操作：编辑、加跟进、生成绑定码、流转分组（需二次确认 + 原因）、归档、发起预约、下发任务。
- 新建/编辑：字段与 Excel 模板一致；手机号唯一性校验；家长可多个（父母）。
- 跟进：时间、方式（电话/微信/面谈/其他）、内容、顾虑、下一步、下次回访；保存后可“再建一条”。

### 6.4 绑定审批（T06–T07）

- 待审批列表：家长手机号（脱敏）、学员姓名、申请时间、openid 后 4 位。
- 操作：通过 / 驳回（必填理由）；通过后家长端即时可见。
- 生成绑定码：选择学员 → 生成 6 位码 → 展示有效期与复制/转发；同一学员同时只保留一个有效码。

### 6.5 任务与批阅（T08–T12）

- 下发（T09）：
  - 对象选择：学员多选 / 分组 / 标签；实时显示“将下发给 N 人”，超过 50 人二次确认。
  - 字段：标题、说明、类型、截止时间、附件；支持“存为模板”。
  - 提交后展示“已下发 N 人”，失败名单可重试。
- 批阅队列（T10）：按截止/提交时间排序；筛选“待批阅/已逾期/已打回”；卡片显示学员、任务、提交时间、图片数。
- 批阅详情（T11）：
  - 顶部学员与任务信息；内容区文字 + 图片九宫格（点击全屏）+ 文件。
  - 底部固定操作条：评分（快捷 1~5 星或输入）、评语（快捷短语）、批改图上传、**通过/打回**。
  - 打回必填理由；通过后可“下一条”，保持滚动位置。
  - 草稿自动保存（本地），防止误退。
- 统计（T12）：下发/已提交/提交率/逾期名单；点击名单进学员详情。

### 6.6 时段与预约（T13–T15）

- 时段：按周视图/日期列表；批量开放（选日期范围 + 时间段模板 + 时长）、单条编辑、取消（已被预约需先处理预约）。
- 预约审核：待审核列表 + 日历；通过锁时段、驳回填理由；冲突时提示“该时段已被占用”并刷新。
- 详情/消课：学员、诉求、时段、状态；**“标记已完成”** 才写课时流水（二次确认，显示扣减后余额）；“未到课”需选择是否扣课时（默认不扣，需填原因）。
- 改期：引导“取消后重新预约”，不直接改时段。

### 6.7 课时与班课（T16–T20）

- 账户列表：学员、总购买、已消耗、剩余（流水汇总）、有效期、预警标识。
- 调整：充值/扣减必须填原因；显示调整前后余额；写审计。
- 流水：按时间倒序，类型（购买/消课/调整/退回）、关联预约/班课、操作人。
- 班课：班级卡（名称/学科/节奏/人数）、成员管理、上课记录（日期/出勤/内容）；班课不扣课时。

### 6.8 资料、通知、导入导出（T21–T26）

- 资料：分类树、上传（`wx.cloud.uploadFile` → fileID → 登记）、可见分组矩阵、上下架、下载日志。
- 通知：标题/正文/附件、人群选择、是否回执、发送预览；回执名单（已确认/未确认/催办）、导出。
- 导入（T24）：下载模板 → 选择文件（`wx.chooseMessageFile`）→ 上传预览 → 错误行列表（可下载错误报告）→ 确认导入 → 结果统计 → 可回滚。
- 看板（T25）：客户结构、交付质量、一对一运营、待办四组卡片。
- 导出（T26）：四类 xlsx；生成后 `wx.downloadFile` + `wx.openDocument`，或“发送到微信”。

### 6.9 成员、设置、审计（T27–T29）

- 成员（仅 owner）：邀请/创建 assistant、停用、重置密码；一期只创建 owner，但页面与接口按多老师实现。
- 设置：提醒时间、余额阈值、课时规则开关、通知模板、隐私协议版本。
- 审计：按时间/操作人/对象筛选，只读。

### 6.10 移动端约束

- 所有列表必须有加载/空/错误三态；分页 20 条。
- 底部固定操作条不得遮挡内容（`padding-bottom: env(safe-area-inset-bottom) + 120rpx`）。
- 图片上传并发 ≤3；批阅页图片先加载缩略图，点击再取原图。
- 长文本（评语/说明）用 `wd-textarea` 自适应高度，最多 6 行后内部滚动。
- 危险操作（归档、扣课时、删除资料、停用成员）必须二次确认 + 填原因。
- 教师模式不做复杂动画；页面切换保持系统默认。

## 7. 设计系统

### 7.1 方向

家长端关键词：**温和、可信、清晰、不幼稚**。教师模式关键词：**高效、克制、信息优先**。避免教培行业常见的“大红大金”和 AI 默认紫渐变。用低饱和青绿主色 + 琥珀强调，圆角卡片、轻阴影、留白充足；家长端字号偏大，教师端密度更高但保持同一套色板。

- 模式：仅浅色（一期）；不做暗色。
- 气质：Soft UI，克制动效（200–250ms），不用复杂视差/3D。
- 可访问性：正文对比度 ≥4.5:1；可点击区域 ≥44×44px；不只靠颜色表达状态（配文字/图标）。

### 7.2 令牌（家长端）

```scss
// 颜色
$color-primary: #0F766E;        // 主色（深青）
$color-primary-strong: #0D9488; // 渐变/高亮
$color-primary-soft: #CCFBF1;   // 浅底
$color-accent: #D97706;         // CTA/提醒（琥珀）
$color-bg: #F6FBF9;             // 页面底色
$color-surface: #FFFFFF;        // 卡片
$color-text: #123B36;           // 主文字
$color-text-secondary: #5B7A74; // 次要
$color-border: #D9E8E4;
$color-danger: #DC2626;
$color-warning: #D97706;
$color-success: #16A34A;

// 圆角 / 间距（rpx）
$radius-sm: 8rpx;  $radius-md: 16rpx;  $radius-lg: 24rpx;  $radius-xl: 32rpx;
$space-xs: 8rpx; $space-sm: 16rpx; $space-md: 24rpx; $space-lg: 32rpx; $space-xl: 48rpx;

// 字号
$font-caption: 24rpx; $font-body: 28rpx; $font-title: 32rpx; $font-h2: 36rpx; $font-h1: 44rpx;
```

- 阴影：`0 8rpx 24rpx rgba(15, 118, 110, 0.08)`，禁止重投影。
- 渐变仅用于首页头部/主按钮：`linear-gradient(135deg, #0F766E, #0D9488)`。
- 状态色：待提交=灰蓝、待批阅=琥珀、已批阅=青绿、已打回/逾期=红；同时配文字标签。

### 7.3 字体与图标

- 中文：`PingFang SC, HarmonyOS Sans SC, Source Han Sans SC, system-ui, sans-serif`；数字用 `DIN Alternate / system` 可选。
- 行高 1.5~1.6；标题字重 600，正文 400。
- 图标：wot-design-uni 内置图标优先；不足处用统一线性图标（24×24，2px 描边），不混用多套风格。
- 插画：仅空态/引导使用，控制在 3 张以内，本地 ≤50KB/张。

### 7.4 组件使用

- 基础：`wd-button`、`wd-input`、`wd-textarea`、`wd-cell`、`wd-tag`、`wd-tabs`、`wd-popup`、`wd-toast`、`wd-message-box`、`wd-loadmore`、`wd-skeleton`、`wd-upload`、`wd-calendar`、`wd-datetime-picker`。
- 业务组件（自研，放 `components/`）：`ChildSwitcher`、`TaskCard`、`BookingStatusSteps`、`LessonBalanceCard`、`EmptyState`、`ErrorState`、`SectionHeader`、`FileItem`、`MaterialCard`、`NoticeCard`。
- easycom 自动引入；禁止整库全量注册。
- 主题覆盖用 wot 的 CSS 变量，改 `--wot-color-primary` 等，不直接改 node_modules。

### 7.5 交互规范

- 所有提交类按钮：loading + 防抖 + 结果反馈（toast/结果页）。
- 危险操作（取消预约、退出、删除草稿/已结束任务）：二次确认，说明后果；删除已结束任务需明确提示提交记录、评分、评语一并删除。
- 列表：下拉刷新 + 触底加载 + “没有更多了”；空态与错误态分开。
- 表单：失焦校验 + 提交校验；错误定位到字段；键盘不遮挡输入（`adjust-position`）。
- 页面转场：默认系统动画；不做自定义复杂转场（小程序性能与一致性）。
- 触觉：关键成功可 `wx.vibrateShort`（可选，勿滥用）。

### 7.6 教师模式密度

- 同一套颜色/圆角/阴影；字号整体降一档：正文 26rpx、卡片标题 30rpx、页面标题 36rpx、辅助 22rpx。
- 间距：卡片内 20rpx、卡片间 16rpx、页面左右 24rpx；列表行高 ≥88rpx（保证点按）。
- 教师卡片允许两行信息 + 右侧状态标签；不使用表格组件。
- 状态标签统一：待提交（灰蓝）、待批阅（琥珀）、已批阅（青绿）、已打回/逾期（红）、已完成（青绿描边）。
- 触控目标 ≥80rpx 高；底部操作条按钮 ≥88rpx。

## 8. 状态、请求与鉴权

### 8.1 Store

| store | 内容 |
| --- | --- |
| `useAuthStore` | access/refresh token、角色（guardian/staff）、`me`、绑定状态、孩子列表、`ensureLogin()`、`logout()`、`routeAfterLogin()` |
| `useAppStore` | 系统配置（提醒文案、客服微信、隐私版本）、网络状态、全局 loading |
| `useNotificationStore` | 未读数、WS 连接状态、最近消息、`markRead()` |
| `useTeacherStore` | 教师待办计数、批阅草稿、最近筛选条件（仅 staff 初始化） |

- token 存 `uni.setStorageSync`（access 2h / refresh 30d）；启动时先 refresh 再拉 `me`。
- 不持久化作业草稿以外的业务数据；退出登录清空所有 store 与缓存（保留隐私同意记录）。
- 教师模式进入前必须 `me.role === 'staff'`，否则 `reLaunch` 回家长首页；不在前端做细粒度权限判断（assistant 限制由服务端返回 403 并提示）。

### 8.2 请求封装

```ts
// utils/request.ts（示意）
interface ApiError { code: number; reason: string; message: string; metadata?: Record<string, string> }

async function request<T>(options: RequestOptions): Promise<T>
// 能力：按平台选择传输层（小程序 callContainer / H5 uni.request）、Bearer 注入、
//      401 自动 refresh 一次并重放、错误归一化、15s 超时、GET 去重、分页参数、request_id 透传
```

- **平台分流**：`#ifdef MP-WEIXIN` 用 `wx.cloud.callContainer`；`#ifdef H5` 用 `uni.request`（仅调试）。业务代码只调 `request()`，不直接碰传输 API。
- `callContainer` 参数：`{ config: { env: VITE_WX_CLOUD_ENV_ID }, path: '/api/v1/...', method, header: { 'X-WX-SERVICE': 'hm-api', ... }, data, timeout: 15000 }`；没有 baseURL、没有自定义域名。
- 大小守卫：请求体 `JSON.stringify(data)` > **100 KiB** 直接报错（提示改用文件上传）；响应对 `JSON` 长度 > **1000 KiB** 记录告警并提示分页。
- 错误映射：`GUARDIAN_NOT_APPROVED` → 待审批页；`LESSON_HOURS_EXHAUSTED` → 课时提示；`BOOKING_SLOT_TAKEN` → 刷新时段；`RATE_LIMITED` → “操作太频繁”。
- 不把后端 `message` 直接抛给家长（可能含内部信息）；用前端文案表。
- **上传**：`wx.cloud.uploadFile({ cloudPath, filePath })` → 得 `fileID` → `POST /api/v1/files/register` 登记；并发 3；单文件失败重试 2 次；进度条；图片先压缩（长边 ≤2000px、质量 0.8）。**禁止**把 base64 塞进 JSON。
- **下载/预览**：`GET /api/v1/files/{id}/download` 校验权限 → 返回 `fileID` → `wx.cloud.downloadFile({ fileID })` → `wx.openDocument`；图片直接用 `<image src="cloud://...">`。
- H5 调试上传可回退 `POST /api/v1/files/upload`（multipart），生产小程序不走该接口。

### 8.3 路由守卫

- `pages/launch` 统一判定：无 token → 登录；`role=staff` → 教师工作台（`reLaunch` 到教师 tab）；`role=guardian` 未绑定 → 绑定；待审批 → 待审批；正常 → 家长首页。
- 教师分包页面 `onLoad` 校验 `me.role === 'staff'`；家长分包页面校验已绑定且 active。
- 业务页 `onShow` 校验 `me.status`，异常回启动页，避免半登录态。
- 分享进入（如资料页）：先走登录/绑定，再回跳原页面（记录 `redirect`）；教师页面不配置分享。

### 8.4 WebSocket

- 用 `wx.cloud.connectContainer({ config: { env }, service: 'hm-api', path: '/api/v1/ws' })`，不经过域名，也没有 socket 合法域名。
- 连接成功后**首帧**发 `{"type":"auth","token":"<access_token>"}`；未收到 `{"type":"auth.ok"}` 前不展示“已连接”。
- iOS「高性能+」可能拿不到网关注入的 openid，因此**必须**走应用层 token，不能只依赖 header。
- 心跳 30s；断线指数退避 1s→30s；重连成功后 `GET /api/v1/notifications?since=` 补齐，避免漏消息。
- 只用于站内信/待办提醒；任务状态以 REST 拉取为准，WS 消息只触发刷新。

## 9. 性能与体验指标

| 指标 | 目标 |
| --- | --- |
| 首屏可交互 | 4G 下 <2s（P75） |
| 页面切换 | <300ms |
| 列表滚动 | 稳定 60fps；图片懒加载 |
| 主包体积 | <1.5MB（留余量） |
| 教师分包体积 | <4MB；首次进入 <800ms（4G P75） |
| 接口失败率 | <1%（前端可重试） |
| 上传成功率 | >98%（3 次重试内） |

- 首页聚合请求，避免瀑布；任务详情与列表共享缓存（30s）。
- 图片：压缩 + 缩略图（如后端支持）+ 懒加载；禁止 base64 大图。
- 长列表用分页，不用一次性渲染；单次响应控制在 **200 KB** 以内（硬限 1000 KiB）。
- 分包：`pkg-teacher` 不预下载；家长端不使用 `preloadRule` 加载教师分包。
- 冷启动：首页并行请求不超过 2 个；云托管冷启动时展示骨架屏，失败给“重试”，不要白屏。

## 10. 测试与验收

| 类型 | 工具 | 覆盖 |
| --- | --- | --- |
| 类型 | `vue-tsc --noEmit` | 全量 |
| 单元 | Vitest | request 封装（callContainer/uni.request 双分支）、token 刷新、大小守卫、金额/课时格式化、状态文案映射、上传流程（mock） |
| 组件 | Vue Test Utils | TaskCard、BookingStatusSteps、EmptyState |
| 云托管契约 | 开发者工具/真机 | callContainer 头、100 KiB 请求拒绝、15 s 超时、fileID 上传下载、WS 首帧认证 |
| 构建 | `uni build -p mp-weixin` | 产物与分包检查（主包体积脚本） |
| 端到端（可选） | miniprogram-automator | 绑定→审批→看任务→提交→批阅→家长可见；预约→审核→完成→扣课时（需 Windows + 开发者工具） |
| 手工 | 验收清单 | 16 条 TC 的前端侧断言 + 教师模式 375px 走查 |

前端验收要点（对应 TC）：

- TC-01：家长手动手机号 + 审批通过后可见；未审批看不到任何数据。
- TC-04/05：提交多图成功、打回理由展示、重交成功。
- TC-07/08：只显示可约时段；并发被抢时提示并刷新。
- TC-11：教师标记完成后余额更新；未到课默认不扣。
- TC-12：余额 0 时预约入口禁用且提示。
- TC-13：意向账号只看到试听/招生资料。
- TC-14：A 家长无法通过任何入口看到 B 的数据（含分享链接）。
- TC-15：回执确认后状态即时更新，首页待办消失。
- TC-17：Excel 预览错误行可见、导入后可回滚。

## 11. 工程规范

- 组件：`<script setup lang="ts">`；`defineProps<Props>()` / `defineEmits<...>()`；props 与 emits 必须有类型。
- 命名：页面目录 kebab-case；组件 PascalCase；composable `useXxx`；store `useXxxStore`。
- 教师代码只在 `pkg-teacher/`；家长端页面禁止 import 教师模块；公共组件下沉。
- 禁止：`any`、`@ts-ignore`（确需时写原因并集中管理）、在模板里写复杂表达式、绕过 `utils/request.ts` 直接调 `uni.request`/`wx.cloud.callContainer`。
- 样式：`scoped` + BEM 轻量命名；颜色/间距只用令牌；禁止 magic number（除 1px 边框）。
- 提交：Conventional Commits；一个 PR 一个主题；UI 变更附截图。
- 文档：新增页面更新本文件页面清单；新增接口先改 proto 再生成类型。
- 依赖：新增依赖需说明体积与必要性，经确认后安装（遵守“不擅自安装/下载”规则）。

## 12. 待确认（前端相关）

**已定**：Q1=B 单小程序教师模式；Q10=wot-design-uni；Q12=青绿 + 琥珀、浅色、无品牌素材；Q18=一期不做暗色；绑定不用 `getPhoneNumber`；Q3 小程序侧不需要域名/ICP 备案（小程序备案已完成）；Q15=微信云托管；Q11=云托管对象存储（fileID）。

仍需确认（详见 [open-questions.md](open-questions.md)）：

1. **Q19 类目与主体**：已确认 A（个人主体 + 工具类目先提审）；隐私协议主体按个人主体填写。
2. **Q22 数据库路线**：MySQL 5.7.18（云托管）——仅后端，不影响前端请求层。
3. **Q23 对象存储**：云托管内置 COS 或独立 COS 桶；前端都用 `wx.cloud.uploadFile`/`fileID`。
4. **Q24 Redis**：一期是否启用；不影响前端。
5. **Q25 自定义域名**：小程序不需要；只有做 H5/独立 Web 时才需要，届时才配 `VITE_API_BASE_URL`。
6. **Q26 主体/AppID 变更时点**：已确认不变更，直接建生产环境；若未来变更，需重置 openid 与家长绑定。
7. **Q21 教师页面范围**：是否接受手机批阅/看板卡片化；有无必须桌面的操作。
8. **订阅消息模板 ID**：由后台配置下发，上线前在微信后台申请并回填。
9. **审核素材**：小程序头像、简介、截图、隐私政策链接（主体变更后一起提交）。
10. **云托管环境 ID**：构建变量 `VITE_WX_CLOUD_ENV_ID`，由用户创建环境后提供。

完整清单与回复模板见 [open-questions.md](open-questions.md)。
