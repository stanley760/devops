# devops-frontend React 19 + shadcn/ui + Tailwind 4 + solid-glass 迁移设计规格

| 元数据 | 值 |
| --- | --- |
| 文档日期 | 2026-11-20 |
| 状态 | **决策已确认（spec-ready）** |
| 适用范围 | `devops-frontend`（保留目录名、`dist` 产物、双入口、网关代理配置） |
| 目标架构 | React 19 + TypeScript + Vite + Tailwind CSS 4 + shadcn/ui（base-nova）+ solid-glass |
| 文档类型 | 设计规格（Design Spec），不含实现计划 |

---

## 1. 背景与目标

### 1.1 背景

当前 `devops-frontend` 基于 **Vue 3 + Vite + TypeScript + GSAP + Three.js**，采用双入口结构（后台管理 `index.html` 与 C 端门户 `portal.html`），经 Higress 网关代理接入，源码约 80 个文件，具备完整的 API、composable、自研 UI 组件与视图层级。

用户决定**彻底废弃现有前端架构**，以 React 19 技术栈重写，视觉风格升级为 **solid-glass（实心玻璃拟态）**。本规格固化已确认的技术选型、架构边界、数据契约与验收标准，作为后续实现的唯一依据。

### 1.2 目标

1. 用 **React 19 + TypeScript + Vite** 原位替换 Vue 技术栈，保留 `devops-frontend` 目录名、`dist` 构建产物、双入口（`index.html` / `portal.html`）与 `vite.config.ts` 网关代理配置。
2. 引入 **Tailwind CSS 4**（经 `@tailwindcss/vite`）与 **shadcn/ui（base-nova / Base UI 底座）**，并以 **solid-glass** 扩展组件库作为统一视觉语言；**深色为主、浅色可用**，主题可切换并跟随系统。
3. 实现 **核心功能对等（core-parity，非完全对等）**：真实 API、动态菜单、验证码、CRUD 骨架、权限控制、401 会话失效、主题切换、网关代理。
4. 框架无关的契约（HTTP 信封、会话、外观、权限、列表）以 React hooks / context / store 重新实现，**不从 Vue 代码复制**。

### 1.3 非目标（明确排除）

- **不做完全功能对等**：不追求与旧实现逐项一致，只保证上文列出的核心能力对等。
- **不保留 Vue 运行时**：迁移完成后 `vue`、`vue-router`、`vue-tsc`、`go-captcha-vue`、`gsap`、`three`、`@types/three`、`@vitejs/plugin-vue` 相关代码与依赖全部移除。
- **不重写后端**：所有接口契约（信封、字段命名、分页、权限码、验证码题型）与后端既有实现严格对齐，前端不得要求后端改动。
- **不引入新的构建/部署体系**：产物仍为 `devops-frontend/dist`，仍由 Higress 网关 + nginx 边缘托管；不新增 CI 流水线定义。
- **不做 Three.js hero 的等价重写**：C 端首屏 Three.js 玻璃立方体**简化为静态玻璃视觉**，不保留 WebGL 渲染管线、GLB 模型加载与屏幕空间折射。
- **不实现此范围外的页面/功能**：`monitor` 之外的扩展监控面板、移动端专属交互、PWA、国际化（i18n）均不在本次范围。

---

## 2. 确认的范围与约束

### 2.1 范围

**双端均纳入：** 后台管理（admin）与 C 端门户（portal）。

**核心能力对等项：** 真实 API 调用、动态菜单、行为式验证码、CRUD 骨架、权限控制、401 会话失效处理、主题切换、网关代理。

**后台管理页面范围：**
- 登录页（含行为式验证码）；
- 工作台（Dashboard）；
- 8 个系统模块的列表页与基础 CRUD：用户、角色、菜单、部门、岗位、租户、字典、参数配置；
- 日志列表：登录日志、操作日志；
- 异常页：403、404。

**C 端门户页面范围：**
- 营销首页（静态玻璃视觉）；
- 登录、注册、个人资料三个核心页面。

**必须保留的行为：** 动态菜单、401 处理、按钮级权限、主题切换、网关代理。

### 2.2 约束（不可违背）

1. **目录与产物**：目录名保持 `devops-frontend`；构建输出仍为 `devops-frontend/dist`；双入口 `index.html`（admin）与 `portal.html`（portal）保留。
2. **网关代理**：`vite.config.ts` 的代理能力必须保留，且语义一致——
   - `/system/api` 代理至 Higress 网关（默认 `http://127.0.0.1:8080`，`Host: api.devops.local`）；
   - `/user/api` 代理至 Higress 网关（`Host: user-api.devops.local`）；
   - 环境变量覆盖项保留：`VITE_GATEWAY_TARGET`、`VITE_GATEWAY_SYSTEM_HOST`、`VITE_GATEWAY_USER_HOST`、`VITE_DIRECT_SERVICE`、`VITE_API_PROXY_TARGET`、`VITE_USER_PROXY_TARGET`；
   - **只代理 `/system/api` 与 `/user/api` API 前缀**，不代理整个 `/system`，否则深链接刷新会被网关守卫拒绝。
3. **契约对齐**：后端 DTO 读接口为 camelCase、写接口为 snake_case 的命名约定必须严格遵守；ID 为字符串（雪花 ID），**任何情况下不得强制转换为数字**。
4. **不做代码复制**：所有框架无关逻辑在 React 侧重新实现，禁止将 Vue 组件/composable 逻辑直接迁移改写。
5. **无占位实现**：所有页面、组件、契约均有明确行为；不允许 TODO / TBD / 占位骨架进入交付。

---

## 3. 总体架构

### 3.1 技术栈

| 层级 | 选型 | 说明 |
| --- | --- | --- |
| 框架 | **React 19** | 函数组件 + Hooks；注意 React 19 与 Base UI 的已知兼容性陷阱，组件用法以 Base UI 文档为准 |
| 语言 | **TypeScript** | 全量类型；`typecheck` 为独立校验步骤 |
| 构建 | **Vite** | 保留双入口 `rollupOptions.input`、`chunkSizeWarningLimit: 1600`、`server.host: '127.0.0.1'` |
| 路由（admin） | **React Router** | 静态路由 + 会话后动态注册的动态路由 |
| 路由（portal） | **Hash 路由** | `#/`、`#/login`、`#/register`、`#/profile`、`#/docs` |
| 样式 | **Tailwind CSS 4** | 经 `@tailwindcss/vite` 插件接入 |
| 组件库 | **shadcn/ui（base-nova / Base UI）** | `components.json` 的 `style` 取 `base-nova`，`tailwind.config` 为空字符串，`css` 指向 `src/index.css`，`iconLibrary` 为 `lucide` |
| 视觉扩展 | **solid-glass** | 经 `npx shadcn@latest add deeplethe/solid-glass/solid-glass` 引入 |
| 动效 | **tw-animate-css** | 必装，配合 Tailwind 4 动画体系 |

### 3.2 目录结构（建议）

迁移后 `devops-frontend/src` 目标结构（以实际实现为准，以下为规格约束）：

```
devops-frontend/
├── index.html                 # admin 入口（保留）
├── portal.html                # portal 入口（保留）
├── vite.config.ts             # 保留网关代理 / 双入口 / alias @ → src
├── components.json            # style: base-nova, css: src/index.css, iconLibrary: lucide
├── src/
│   ├── index.css              # Tailwind → tw-animate-css → shadcn/tailwind.css → solid-glass.css
│   ├── main.tsx               # admin 挂载入口
│   ├── App.tsx                # admin 路由与布局编排
│   ├── portal/
│   │   ├── main.tsx           # portal 独立挂载根
│   │   ├── App.tsx            # portal hash 路由与页面编排
│   │   └── pages/             # Home / Login / Register / Profile / Docs
│   ├── lib/                   # 框架无关共享层
│   │   ├── http/              # 信封解包、请求、refresh 单飞
│   │   ├── session/           # admin 会话存储与版本
│   │   ├── appearance/        # 主题 / 密度 / 折叠
│   │   ├── permission/        # 权限集合与 PermissionButton
│   │   ├── toast/             # 全局提示
│   │   ├── confirm/           # 全局确认
│   │   ├── list/              # useListPage / 树列表
│   │   ├── options/           # 下拉选项 / 字典选项
│   │   ├── datetime/          # 时间格式化
│   │   └── portal-session/    # portal_tokens 存储与刷新
│   ├── api/
│   │   ├── admin/
│   │   │   ├── auth.ts        # captcha / login / refresh / logout / info / routers / profile
│   │   │   └── system.ts      # user/role/menu/dept/post/tenant/dict/config/日志
│   │   └── portal/
│   │       └── auth.ts        # captcha / login / register / refresh / logout / profile
│   ├── router/
│   │   ├── routes.ts          # 静态路由 + COMPONENT_MAP 注册表
│   │   └── guards.ts          # 登录守卫、动态路由注册、401 跳转
│   ├── store/
│   │   └── session.ts         # user / roles / permissions / routes 的 context/store
│   ├── components/
│   │   ├── layout/            # AppShell / Sidebar / Topbar / Content / CommandPalette
│   │   ├── business/          # ListPage / FormDialog / PermissionButton / SearchForm
│   │   └── ui/                # solid-glass 组件映射与封装
│   └── views/                 # admin 页面（Login/Dashboard/system/monitor/异常页）
└── dist/                      # 构建产物（保留）
```

### 3.3 双入口与网关

- `index.html` → 加载 `src/main.tsx`，挂载 admin 应用（React Router，Web History）。
- `portal.html` → 加载 `src/portal/main.tsx`，挂载 portal 应用（Hash 路由，独立根）。
- `vite.config.ts` 的 `resolve.alias['@'] = src`、`server.port: 5173`、`server.host: '127.0.0.1'`、`build.chunkSizeWarningLimit: 1600` 及双入口 `input` 全部保留。
- 两个入口共享同一套 `src/lib` 契约层与 `src/api` 接口层，但拥有独立的挂载根与路由实例。

### 3.4 主题与外观

**策略：深色为主（dark 为默认），浅色可用，主题可切换并跟随系统。**

- DOM 策略由旧实现的根元素 `data-theme` 属性**切换为 `html.dark` class**，以兼容 solid-glass 的暗色机制。
- 外观偏好存储在 `devops.appearance`（JSON），字段：`theme`（`'light' | 'dark' | 'auto'`）、`density`（`'comfy' | 'regular' | 'compact'`）、`railCollapsed`（boolean）。
- `auto` 模式下读取 `window.matchMedia('(prefers-color-scheme: dark)')`；系统主题变化时若为 `auto` 则重新应用。
- **主题必须在应用挂载前应用**（于 `main.tsx` 执行 `applyAppearance()`），避免首屏闪烁（FOUC）。
- 存储键：`devops.access_token`、`devops.refresh_token`、`devops.tenant_id`、`devops.tenant_code`、`devops.session_ver`、`devops.last_tenant`、`devops.appearance`、`devops.last_user`；portal 侧仅 `portal_tokens`。**保持既有存储键与存储结构不变**，以复用用户既有登录态/偏好。

---

## 4. 数据层与契约

本层为框架无关契约，React 侧以 hooks / context / store 重新实现。以下契约为**规格事实，实现必须严格一致**。

### 4.1 HTTP 信封与请求

**统一信封**：所有响应为 `{ code, msg, data }`。

- `code === 200` → 解包返回 `data`；
- `code !== 200` → 抛出携带业务 `code` 与 `msg` 的 `ApiError`；
- 响应解析失败（非 JSON）→ 抛出 `ApiError(res.status, '响应解析失败（HTTP {status}）')`。

**Query 参数规则**：构建 URL 时**跳过**值为 `undefined`、`null`、`''` 的字段（空值不参与序列化）。

**请求头**：

- `Accept: application/json`；
- 携带 body 时 `Content-Type: application/json`；
- 认证请求（`auth !== false`）追加：
  - `Authorization: Bearer {access_token}`（存在时）；
  - `X-Tenant-ID: {tenant_id}`（admin 侧存在时）。

**刷新机制（admin）**：

- 刷新接口：`POST /system/api/v1/auth/refresh`，body `{ refresh_token }`，匿名请求；
- **单飞（single-flight）**：并发 401 只触发一次刷新，其余等待同一 Promise；
- **只重试一次**：刷新成功后原请求重试一次；刷新失败或版本不符则清理会话并触发未授权跳转；
- **会话版本守卫**：请求发起时记录会话版本 `ver`；若响应为 401 但 `ver !== sessionVersion()`（会话已被切换），判定为「会话已切换」，不触发刷新/跳转，直接抛出。

**错误分类约定（登录/校验场景）**：`400` = 参数/验证码错误；`401` = 凭据错误。

### 4.2 会话存储（admin）

| localStorage 键 | 内容 |
| --- | --- |
| `devops.access_token` | 访问令牌 |
| `devops.refresh_token` | 刷新令牌 |
| `devops.tenant_id` | 当前租户 ID |
| `devops.tenant_code` | 当前租户编码 |
| `devops.session_ver` | 会话版本号（登录/登出递增，用于旧请求判定过期） |
| `devops.last_tenant` | 上次登录的租户编码（兜底链用） |
| `devops.appearance` | 外观偏好 JSON |
| `devops.last_user` | 上次登录用户信息 |

- `session.save(pair, tenant)`：写入 token 与租户，`loggedIn` 置真，版本号递增并返回新版本。
- `session.saveTokens(pair)`：仅轮换 token，版本号递增并返回新版本。
- `session.clear()`：移除 token/租户键，`loggedIn` 置假，版本号递增并返回新版本。
- **页面与组件不得直接读写上述键**，仅经 `lib/session` 与 `lib/portal-session` 的入口操作。

### 4.3 管理端认证接口（`/system/api/v1/auth`）

| 方法 | 路径 | 说明 | 认证 |
| --- | --- | --- | --- |
| GET | `/captcha` | 获取行为式验证码挑战（可带 `tenant_code`） | 否 |
| POST | `/login` | 登录，返回 TokenPair + 租户信息 | 否 |
| POST | `/refresh` | 刷新双 token | 否 |
| POST | `/logout` | 登出 | 是 |
| GET | `/info` | 会话信息（用户/租户/角色/权限） | 是 |
| GET | `/routers` | 动态菜单路由树 | 是 |
| PUT | `/profile` | 更新个人资料 | 是 |
| PUT | `/profile/password` | 修改密码 | 是 |

**关键类型：**

- `TokenPair { access_token, refresh_token, expires_in }`
- `LoginResult extends TokenPair { tenant: TenantBrief }`，`TenantBrief { id, name, code }`
- `AuthUser { id, username, nickname?, email?, mobile?, gender?, avatar?, deptId?, postIds?, remark?, loginIp?, loginDate? }`（id 为字符串）
- `AuthInfo { user: AuthUser, tenant: TenantBrief, roles: string[], permissions: string[] }`

### 4.4 动态菜单（`RouterNode` 与 COMPONENT_MAP）

`RouterNode` 结构：

```
{ id, name, path, component, componentName?, icon?, sort, visible, keepAlive?, alwaysShow?, children? }
```

**COMPONENT_MAP**（菜单 `component` 字符串 → 页面懒加载映射）必须覆盖以下键：

| component 键 | 页面 |
| --- | --- |
| `dashboard/index` | 工作台（静态提供，后端 `/auth/routers` 不下发，置于菜单首位） |
| `system/user/index` | 用户管理 |
| `system/role/index` | 角色管理 |
| `system/menu/index` | 菜单管理 |
| `system/dept/index` | 部门管理 |
| `system/post/index` | 岗位管理 |
| `system/dict/index` | 字典管理 |
| `system/config/index` | 参数配置 |
| `system/loginlog/index` | 登录日志 |
| `system/operationlog/index` | 操作日志 |
| `platform/tenant/index` | 租户管理 |

- 动态路由全部来自 `GET /auth/routers`，**前端不虚构菜单**；未注册的 `component` 键仅 `console.warn` 并跳过。
- 动态路由经注册表（registry）映射为 React Router 路由，挂载于 AppShell 的子路由位；菜单数据与路由注册状态需分别跟踪，避免「数据就绪但路由未注册」导致业务路径落 404。

### 4.5 权限

- 权限码为**精确字符串**，来自 `GET /auth/info` 的 `permissions: string[]`；**精确匹配，不支持通配符**。
- **`PermissionButton`**：以权限码为输入，**无权限时隐藏对应操作**（按钮不渲染），而非仅置灰。后端仍强制校验，前端隐藏仅为体验层。
- 已确认的权限码清单（实现必须沿用）：

| 模块 | 权限码 |
| --- | --- |
| 用户 | `system:user:create`、`system:user:update`、`system:user:update-password`、`system:user:delete` |
| 角色 | `system:role:create`、`system:role:update`、`system:role:assign`、`system:role:delete` |
| 菜单 | `system:menu:create`、`system:menu:update`、`system:menu:delete` |
| 部门 | `system:dept:create`、`system:dept:update`、`system:dept:delete` |
| 岗位 | `system:post:create`、`system:post:update`、`system:post:delete` |
| 字典 | `system:dict:create`、`system:dict:update`、`system:dict:delete` |
| 参数 | `system:config:create`、`system:config:update`、`system:config:delete` |
| 登录日志 | `system:loginlog:clean`、`system:loginlog:remove` |
| 操作日志 | `system:operationlog:clean`、`system:operationlog:remove` |
| 租户 | `platform:tenant:create`、`platform:tenant:update`、`platform:tenant:delete` |

### 4.6 列表分页

- 请求参数：`page_num`、`page_size`（**`page_size` 上限 1000**）；
- 响应结构 `PageResult<T>`：`{ list: T[], total: number, page_num: number, page_size: number, pages: number }`；
- **所有 ID 为字符串（雪花 ID），不得强制转换为数字**；
- `useListPage` 提供：查询参数、行数据、总数、当前页、页大小、加载态、导出态、选中集合、`load`/`search`/`reset`/`setPage`/`setPageSize`/`exportData`/`afterMutate`；
- 列表请求需**竞态防护**（以递增请求序号作废过期响应）；删除末页末行后自动回退一页。

### 4.7 行为式验证码

**题型**：`click`、`slide`、`dragdrop`、`rotate`。

**挑战字段**（`GET /auth/captcha`）：

```
{ id, type, image, thumb?, width, height, text?,
  thumb_x?, thumb_y?, thumb_width?, thumb_height?, thumb_size?, enabled? }
```

- `text`：click 型目标形状提示（如 `"circle,star"`）；
- `thumb_x/thumb_y`：slide/dragdrop 拼图块在主图内的初始左上角坐标（px）；
- `thumb_width/thumb_height`：slide/dragdrop 拼图块尺寸（px）；
- `thumb_size`：rotate 待旋转缩略图边长（px）；
- **`enabled=false` 时跳过验证码校验**（登录直接提交）；字段缺失时按开启处理。

**作答映射（按题型）：**

| 题型 | 作答字段 |
| --- | --- |
| click | `captcha_points: Array<{ x, y }>`（主图像素坐标） |
| slide | `captcha_x`、`captcha_y` |
| dragdrop | `captcha_x`、`captcha_y` |
| rotate | `captcha_angle`（**仅在无拖拽时角度为 0**） |

- 所有题型均携带 `captcha_id`；
- 挑战为**一次性消费**（服务端 GetDel），**校验失败必须重新拉取新题**；
- **登录页验证码不常驻**：用户点击「登录」时才拉题并弹框，提交时回传答案；验证失败在弹框内提示并自动换题；其他失败（凭据错误、限流等）关闭弹框，错误显示在登录卡上。

### 4.8 外观

- 存储键 `devops.appearance`，JSON 结构：`{ theme, density, railCollapsed }`，默认 `{ theme: 'auto', density: 'regular', railCollapsed: false }`。
- DOM 应用：在 `<html>` 上增删 `dark` class（替代旧 `data-theme` 属性），同时可保留密度/折叠的 class/属性映射。
- 系统主题变化监听：`matchMedia('(prefers-color-scheme: dark)')` 的 `change` 事件，`auto` 模式下重新应用。
- **挂载前应用**，避免首屏闪烁。

### 4.9 租户解析（登录入口）

解析顺序（依次短路，不可反转）：

1. 查询参数 `?tenantCode=`（默认入口，未登录访问任意页被守卫带回 `/login?tenantCode=xxx`）；
2. 路径参数 `/t/{code}/login`（显式入口）；
3. 子域首段（`acme.admin.example.com` → `acme`，仅认至少三级域名，保留段 `www/admin/app/api/console/portal/static/cdn/dev/test/staging` 除外，`localhost` 与 IP 直连不取）；
4. localStorage `devops.last_tenant`；
5. 兜底 `'default'`。

- 前三项命中即**锁定**：登录页不渲染租户输入，仅账号 + 密码；换租户通过修改地址栏 `tenantCode` 实现。
- 登录成功后写入 `devops.last_tenant`。

### 4.10 登录错误文案映射

后端返回英文，前端做展示映射（正则匹配，未命中退回原文）：

| 后端错误 | 展示文案 |
| --- | --- |
| `incorrect username or password` | 账号或密码错误 |
| `user does not exist` | 账号不存在或已被删除 |
| `tenant not found` | 租户不存在，请重新选择 |
| `tenant is disabled` | 租户已被禁用，请联系管理员 |
| `too many` / `rate limit` | 操作过于频繁，请稍后再试 |

### 4.11 C 端门户契约（`/user/api/v1`）

**接口：**

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/auth/captcha` | 行为式验证码挑战 |
| POST | `/auth/login` | 登录，返回 TokenPair |
| POST | `/auth/register` | 注册，返回 TokenPair |
| POST | `/auth/refresh` | 刷新 token |
| POST | `/auth/logout` | 登出 |
| GET | `/profile` | 获取个人资料 |
| PUT | `/profile` | 更新个人资料 |
| PUT | `/profile/password` | 修改密码 |

**类型：**

- `TokenPair { access_token, refresh_token, expires_in, issued_at }`（`issued_at` 为 unix 毫秒）
- `PublicProfile { id: number, username, nickname, email, mobile, gender, avatar, created_at }`（**`id` 为数字**，`created_at` 为 unix 毫秒时间戳）
- 验证码挑战/作答字段与 admin 侧一致（见 4.7）。

**会话存储：** 唯一键 `portal_tokens`（JSON）。

**刷新机制：** 401 时单飞刷新、成功后重试一次；刷新失败清理 `portal_tokens` 并抛 `PortalApiError(401, '登录已过期，请重新登录')`。

**Hash 路由与保留文案：**

- 路由：`#/`（首页）、`#/login`、`#/register`、`#/profile`、`#/docs`；
- 品牌与文案保留：`Design World`、`Explore New Ideas`、`Explore Now`（点击跳 `#/register`）、`Sign In`、`Sign Up`、`立即注册`；
- 个人资料页字段与校验消息保留原实现文案（昵称/邮箱/手机号/性别/注册时间，编辑资料与修改密码两个表单）。

---

## 5. 页面与组件设计

### 5.1 Admin 布局（AppShell）

**AppShell** 组合：`Sidebar` + `Topbar` + `Content`，全部使用 shadcn/ui + solid-glass 组件。

**Sidebar（侧边栏）：**

- 渲染动态菜单（来自 store 的 routes），支持分组与单页组；工作台（`dashboard/index`）置于首位；
- 折叠状态由 `devops.appearance.railCollapsed` 驱动（折叠为 rail 模式）；
- 使用 solid-glass 玻璃质感；图标经 lucide。

**Topbar（顶栏）：**

- **Ctrl/⌘ + K 命令面板**（CommandPalette）：快速跳转、搜索菜单项；
- **面包屑（Breadcrumb）**：依据当前路由展示层级路径；
- **用户菜单（UserMenu）**：修改密码、外观切换、退出登录；
- **移动端抽屉（MobileDrawer）**：小屏下侧边栏转为抽屉。

**Content：** 承载业务页面与异常页（403 / 404），动态路由挂载于此。

**其他顶栏信息：** 租户信息展示（当前租户名/编码）。

### 5.2 Admin 页面

**登录页（Login）：**

- 按 4.9 解析租户入口；已登录访问登录页跳工作台；
- 表单：用户名、密码；租户锁定时无租户输入；
- 验证码按 4.7 交互（点击登录时拉题弹框，失败自动换题）；
- 错误按 4.10 文案映射，落于登录卡自身（非 toast）；
- 登录成功保存会话、写入 `devops.last_tenant`、注册动态路由后跳工作台或 `redirect`。

**工作台（Dashboard）：** 只读真实接口（账号/菜单/审计日志），不含虚构监控指标。

**系统模块 CRUD（用户/角色/菜单/部门/岗位/租户/字典/参数）：**

- 统一以 **ListPage + useListPage** 骨架构建：查询区（SearchForm）+ 表格 + 分页 + 批量操作条（Portal 风格的批量操作容器）；
- 新增/编辑统一用 **shadcn Dialog** + Field/Input/Select/Switch/AlertDialog 组合（表单对话框）；
- 删除使用 **AlertDialog** 二次确认；
- **权限控制**：所有动作按钮经 `PermissionButton` 包裹，无权限隐藏；
- **核心字段与动作约束**：

| 模块 | 核心字段 | 核心动作 | 特殊约束 |
| --- | --- | --- | --- |
| 用户 | username, nickname, deptId, postIds, roleIds, email, mobile, gender, avatar, remark, status | 新增/编辑/重置密码/删除 | `deptId` 必填（写接口空串 400）；重置密码独立动作 |
| 角色 | name, code, sort, dataScope, dataScopeDeptIds, status, remark, menuIds | 新增/编辑/分配菜单/删除 | `data_scope_dept_ids` 为 **JSON 数组字符串**（非逗号分隔），需 `parseDeptIDsJson`/`encodeDeptIDsJson` 转换；**内置角色不可删除** |
| 菜单 | name, permission, type(1目录/2菜单/3按钮), sort, parentId, path, icon, component, componentName, status, visible, keepAlive, alwaysShow | 新增/新增子项/编辑/删除 | 树形结构，父子级联；按钮类型必填权限标识 |
| 部门 | name, parentId, sort, leaderUserId, phone, email, status | 新增/新增子项/编辑/删除 | 树形结构，父子级联 |
| 岗位 | code, name, sort, status, remark | 新增/编辑/删除 | —— |
| 租户 | name, code, contactName, contactMobile, expireTime, remark, adminUsername, adminPassword | 新增/编辑/删除 | —— |
| 字典 | 类型（name/type/status/remark）+ 数据项（dictType/label/value/sort） | 类型新增/编辑/删除 + 数据项新增/编辑/删除 | 双层结构：类型列表 + 数据项列表 |
| 参数 | configKey, configName, configValue, type(1内置/2自定义), visible, remark | 新增/编辑/删除 | **内置（type=1）不可删除**；内置键控件映射：`sys.account.registerUser`、`sys.captcha.enabled` → Switch；`sys.captcha.type` → 字典 `sys_captcha_type` 下拉 |

- **写接口字段命名**：写接口统一 snake_case（`config_key`、`dict_type`、`css_class`、`keep_alive`、`always_show`、`data_scope`、`leader_user_id`、`contact_name`、`expire_time` 等）；读接口 DTO 为 camelCase。

**日志列表：**

- **登录日志 / 操作日志**：列表 + 查询 + 清空（`system:loginlog:clean` / `system:operationlog:clean`）+ 批量删除（`system:loginlog:remove` / `system:operationlog:remove`），经 PermissionButton 控制。

**异常页：**

- **403（NoAccess）**：无权限访问页；
- **404（NotFound）**：未匹配路由的兜底页。

### 5.3 Portal 页面

- **首页（Home）**：营销首屏，静态玻璃视觉（替代 Three.js），保留品牌 `Design World`、`Explore New Ideas`、`Explore Now`（→ `#/register`）等文案与导航结构。
- **登录（Login）**：`Sign In`，表单用户名/密码 + 验证码，失败换题；已登录跳 `#/profile`。
- **注册（Register）**：`Sign Up`，用户名/昵称/邮箱/密码/确认密码 + 验证码；两次密码不一致提示 `两次输入的密码不一致`；注册即登录。
- **个人资料（Profile）**：展示与编辑 `PublicProfile` 字段（昵称/邮箱/手机号/性别/注册时间），含编辑资料与修改密码两个表单，保留原校验消息。
- **产品文档（Docs）**：静态文档占位页（保留 `#/docs` 路由）。
- **退出**：导航栏退出入口，登出后清理 `portal_tokens` 回 `#/`。

### 5.4 solid-glass 组件映射

shadcn/ui 提供基础原语，solid-glass 提供玻璃质感变体；映射关系如下（实现以 solid-glass 注册表为准）：

| 业务需求 | shadcn/ui / solid-glass 组件 |
| --- | --- |
| 按钮 | solid-glass Button（含 primary/ghost/link/danger 变体） |
| 输入框 | shadcn Input |
| 下拉选择 | shadcn Select |
| 开关 | shadcn Switch |
| 对话框 | shadcn Dialog（**portal 侧对话框需带主题**） |
| 确认框 | shadcn AlertDialog |
| 表单字段 | shadcn Field + Label |
| 提示 | shadcn Toast（toast 系统） |
| 侧边栏 / 顶栏 / 内容容器 | solid-glass 容器/侧边栏变体 + Tailwind 布局 |
| 命令面板 | solid-glass Command/Dialog 变体 |
| 图标 | lucide-react |

**组件化约束：**

- **不得重命名 `data-slot`**，保持 solid-glass/shadcn 的插槽约定；
- portal 侧对话框需继承主题（与 admin 一致的 dark/light）；
- solid registry 的额外组件为可选增强，非必需项。

---

## 6. 迁移策略与清理清单

### 6.1 实现方法

- 框架无关契约在 React 侧**重新实现**为 hooks / context / store，**不从 Vue 代码复制**；
- 按模块自底向上构建：先契约与基础设施，再布局与页面，最后清理。

### 6.2 迁移顺序（固定）

1. **基础层（base）**：工程初始化（React 19 + Vite + TS）、Tailwind 4 + `@tailwindcss/vite`、shadcn/ui（base-nova）、solid-glass 接入、CSS 导入顺序、`main.tsx` 挂载前主题应用、`lib/` 契约层（http/session/appearance/permission/toast/confirm/list/options/datetime/portal-session）、`vite.config.ts` 代理与双入口保留。
2. **Admin 登录 + 布局 + 工作台**：登录页（验证码、租户解析、错误映射）、AppShell（Sidebar/Topbar/Content/命令面板/面包屑/用户菜单/移动抽屉/租户信息）、动态路由注册、401 处理、Dashboard。
3. **CRUD 模块**：ListPage + useListPage + 表单对话框 + PermissionButton 骨架，按用户 → 角色 → 菜单 → 部门 → 岗位 → 租户 → 字典 → 参数 顺序实现，再补日志列表。
4. **Portal**：独立根 + hash 路由、静态玻璃首页、登录/注册/个人资料/文档、portal_tokens 与刷新、验证码。
5. **清理**：移除 Vue/GSAP/Three.js/自研样式架构（见 6.3）。
6. **验证**：执行第 7 节验证标准。

### 6.3 清理清单（迁移完成后必须移除）

- Vue 运行时依赖：`vue`、`vue-router`、`vue-tsc`、`@vitejs/plugin-vue`。
- 动画与 3D：`gsap`、`three`、`@types/three`，及 `src/portal/hero.ts` 的 Three.js 渲染管线（替换为静态玻璃视觉）。
- 验证码组件库：`go-captcha-vue`（替换为 React 自实现的验证码组件，对接 4.7 契约）。
- 自研样式架构：`src/styles/` 下全部自研样式（`base.css`、`components.css`、`instruments.css`、`layout.css`、`materials.css`、`pages.css`、`tokens.css`）由 Tailwind 4 + shadcn/tailwind.css + solid-glass.css 取代；`src/portal/style.css` 由统一 `src/index.css` 体系取代。
- 全部 `.vue` 文件、`components/ui/*.vue` 自研组件、`components/business/*`、`components/layout/*` 旧实现。
- 旧 `data-theme` 属性应用逻辑（由 `html.dark` class 策略取代），但**保留 `devops.appearance` 存储结构**。
- `package.json` 中相应依赖与 `vue-tsc` 脚本（`typecheck` 改为基于 `tsc`）。

### 6.4 CSS 导入顺序（固定，不可调整）

`src/index.css` 的导入顺序必须为：

```
@import "tailwindcss";              # Tailwind 4
@import "tw-animate-css";            # 动画
@import "shadcn/tailwind.css";       # shadcn 基础层（来自 shadcn 包）
@import "./solid-glass.css";         # solid-glass 玻璃质感
```

并在其中声明 `@custom-variant dark (&:is(.dark *))`，使 `dark:` 变体匹配 `html.dark` class。

### 6.5 shadcn/ui 初始化约束

- `components.json`：`style` = `base-nova`、`tailwind.config` = `""`（空字符串）、`css` = `src/index.css`、`iconLibrary` = `lucide`；
- **Base UI（base-nova）为必需底座**；
- solid-glass 通过 `npx shadcn@latest add deeplethe/solid-glass/solid-glass` 添加；
- **tw-animate-css 为必需项**，不可省略。

---

## 7. 验证标准

### 7.1 静态校验

- **`npm run typecheck`**：全量 TypeScript 校验，零错误。
- **`npm run build`**：双入口构建成功，产物输出至 `devops-frontend/dist`，`index.html` 与 `portal.html` 均生成且可独立加载。

### 7.2 开发服务器浏览器冒烟

启动 dev server（端口 5173，`host: 127.0.0.1`），浏览器实测：

- **Admin 登录**：验证码拉取与提交、登录成功跳转工作台、各登录错误文案映射正确、租户入口解析（`?tenantCode` / `/t/{code}/login`）正确；
- **工作台**：正常渲染，真实接口数据加载；
- **CRUD**：各模块列表加载、分页、新增/编辑/删除/重置密码、表单校验、内置项不可删除、树形级联、权限按钮隐藏；
- **Portal**：首页静态玻璃视觉渲染、登录/注册/个人资料/文档路由跳转、验证码与校验消息、退出。

### 7.3 契约测试（针对数据层，框架无关）

以下契约行为需有对应测试覆盖，确保 React 侧实现与规格一致：

1. **信封解包**：`code === 200` 返回 `data`；非 200 抛 `ApiError`（携带 code/msg）；非 JSON 响应抛解析失败错误。
2. **Query 空值剔除**：`undefined`/`null`/`''` 字段不出现在 URL。
3. **刷新单飞**：并发 401 只触发一次 refresh；成功后原请求重试一次；失败清理会话。
4. **会话版本守卫**：响应 401 但版本不匹配时判定「会话已切换」，不触发刷新/跳转。
5. **菜单映射**：`RouterNode.component` 经 COMPONENT_MAP 正确映射为页面组件，未注册键跳过。
6. **验证码映射**：四种题型（click/slide/dragdrop/rotate）作答字段映射正确；`enabled=false` 跳过；失败换题。
7. **租户解析**：查询参数 → 路径 → 子域 → 存储 → `default` 的短路顺序与锁定行为。
8. **登录错误映射**：五类后端错误文案映射正确。
9. **portal 契约**：TokenPair/PublicProfile 字段、portal_tokens 存储、刷新与重试。

### 7.4 行为验证

- **401**：会话失效后清理会话、提示、跳登录页，携带正确 `tenantCode` 与 `redirect`；
- **权限**：无权限的按钮不渲染（DOM 中不存在）；
- **主题**：深/浅/跟随系统切换生效，`auto` 随系统变化，挂载前应用无闪烁；
- **动态菜单**：登录后动态注册路由，深链接刷新后能正确回到对应页面（不落 404）；
- **代理**：`/system/api` 与 `/user/api` 请求经网关代理成功，非 API 前缀（如 `/system/user`）走 SPA fallback 不触发 401。

---

## 8. 风险与取舍

| 风险/取舍 | 说明 | 应对 |
| --- | --- | --- |
| React 19 与 Base UI 兼容性 | solid-glass 底座为 Base UI（base-nova），存在已知的 React 19 兼容陷阱 | 组件用法严格遵循 Base UI 文档，不自行封装底层渲染逻辑；不做超出 Base UI 能力的自定义 |
| `data-slot` 重命名 | 重命名会破坏 solid-glass/shadcn 插槽约定，导致样式/变体失效 | 明确禁止重命名 `data-slot` |
| 主题 DOM 策略切换 | 旧 `data-theme` 属性 → `html.dark` class，需与 solid-glass 暗色机制一致 | 统一 `@custom-variant dark (&:is(.dark *))`；挂载前应用；保留 `devops.appearance` 存储结构 |
| 双入口共享契约 | 两入口共享 `lib/` 契约层，但路由与挂载根独立 | 契约层框架无关、纯函数化；路由实例各自独立 |
| ID 类型混淆 | admin ID 为字符串（雪花），portal `PublicProfile.id` 为数字 | 严格按各端契约类型使用，禁止强制转换 |
| 写接口命名 | 写接口 snake_case、读接口 camelCase，易混 | 在 API 层集中做字段映射，页面层只接触规范类型 |
| 验证码一次性消费 | 挑战一次性，失败必须换题 | 验证码组件内置失败重载逻辑，契约测试覆盖 |
| 内置项误删 | 内置配置/角色被删影响后端默认行为 | 前端明确禁止删除（type=1 / 内置角色），仅允许改值 |
| 网关代理范围 | 误将整个 `/system` 前缀代理会导致深链接刷新 401 | 严格仅代理 `/system/api`、`/user/api`；契约测试覆盖 |
| 不做代码复制 | 从 Vue 复制易引入旧架构包袱与隐性 bug | 以契约为准重新实现，契约测试兜底 |

---

## 9. 验收清单

以下各项全部满足时，本迁移视为验收通过：

- [ ] `devops-frontend` 目录名、`dist` 产物、`index.html`/`portal.html` 双入口、`vite.config.ts` 网关代理（`/system/api`、`/user/api`，含环境变量覆盖）全部保留且工作正常。
- [ ] 技术栈为 React 19 + TypeScript + Vite + Tailwind CSS 4（`@tailwindcss/vite`）+ shadcn/ui（base-nova）+ solid-glass；`components.json` 的 `style=base-nova`、`tailwind.config=""`、`css=src/index.css`、`iconLibrary=lucide`。
- [ ] `src/index.css` 导入顺序为 Tailwind → tw-animate-css → shadcn/tailwind.css → solid-glass.css，并声明 `@custom-variant dark (&:is(.dark *))`。
- [ ] 深色为默认、浅色可用，主题切换（light/dark/auto）与系统跟随生效；`devops.appearance` 存储结构保留；主题在挂载前应用无闪烁。
- [ ] `lib/` 契约层（http/session/appearance/permission/toast/confirm/list/options/datetime/portal-session）按第 4 节契约实现，框架无关。
- [ ] Admin 认证接口（captcha/login/refresh/logout/info/routers/profile/profile-password）全部对接，信封解包、空值剔除、单飞刷新、会话版本守卫行为正确。
- [ ] 动态菜单经 `/auth/routers` + COMPONENT_MAP 正确映射，前端不虚构菜单，深链接刷新不落 404。
- [ ] 权限码精确匹配，`PermissionButton` 无权限时隐藏按钮，第 4.5 节权限码清单全部覆盖。
- [ ] 分页契约（`page_num`/`page_size`、上限 1000、`PageResult` 结构）正确；ID 全程按字符串处理。
- [ ] 行为式验证码四种题型（click/slide/dragdrop/rotate）作答映射、`enabled=false` 跳过、失败换题行为正确。
- [ ] 租户解析（查询 → 路径 → 子域 → 存储 → `default`）与锁定行为正确；登录错误文案映射正确。
- [ ] Admin 页面齐全：登录（验证码）、工作台、8 个系统模块列表与基础 CRUD、登录/操作日志列表、403/404；核心字段/动作/权限/树形级联/内置不可删除/配置控件映射符合第 5.2 节。
- [ ] Portal 页面齐全：静态玻璃首页、登录/注册/个人资料/文档，hash 路由（`#/`、`#/login`、`#/register`、`#/profile`、`#/docs`）与保留文案（Design World、Explore New Ideas、Explore Now、Sign In、Sign Up、立即注册）正确。
- [ ] portal 契约（`/user/api/v1`、TokenPair、PublicProfile、`portal_tokens`、单飞刷新与重试）正确。
- [ ] AppShell（Sidebar/Topbar/Content）、命令面板（Ctrl/⌘+K）、面包屑、用户菜单（密码/外观/登出）、移动抽屉、租户信息全部实现。
- [ ] solid-glass 组件映射与「不重命名 `data-slot`」约束落实；portal 对话框带主题。
- [ ] Vue/GSAP/Three.js/go-captcha-vue/自研样式（`src/styles/`、`src/portal/style.css`）及全部 `.vue` 文件已移除，`package.json` 依赖与脚本已更新。
- [ ] `npm run typecheck` 零错误；`npm run build` 成功且双入口产物可加载。
- [ ] 浏览器冒烟（登录/工作台/CRUD/portal）与第 7.3 节契约测试全部通过。
- [ ] 401 行为（清理会话、提示、跳登录、带 tenantCode/redirect）验证通过。
