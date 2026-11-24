# devops-frontend

工业级运维管理平台前端 —— React 19 + TypeScript + Vite（双入口：后台 `/` 与 C 端门户 `/portal.html`），
**solid-glass 玻璃质感主题**（玻璃质感 × 工业仪表），对接 devops-system（用户权限系统）。

页面集合与菜单完全对齐 `GET /system/api/v1/auth/routers` 返回的真实菜单
（系统管理：用户 / 角色 / 菜单 / 部门 / 岗位 / 租户 / 字典 / 参数；日志：登录 / 操作）
与工作台首页；**不新增后端不存在的监控功能菜单**。

## 开发

```bash
npm install
npm run dev        # http://localhost:5173
npm run build      # npm run typecheck && vite build
npm run typecheck
npm run test:run
```

前置（见根目录 README）：

```bash
cd devops-id && go run . server        # gRPC :9990
cd devops-system && go run . migrate   # 建表 + 种子
cd devops-system && go run . server    # HTTP :9999
```

开发代理：**默认经网关**——`/system/api` → `http://127.0.0.1:8080`（Host `api.devops.local`）、
`/user/api` → `http://127.0.0.1:8080`（Host `user-api.devops.local`）。
JWT 只在网关验一次，业务服务只认网关注入的身份头，所以开发流量也必须经网关
（可用 `VITE_GATEWAY_TARGET` / `VITE_GATEWAY_SYSTEM_HOST` / `VITE_GATEWAY_USER_HOST` 覆盖）；
`VITE_DIRECT_SERVICE=1` 直连仅在业务服务以 jwt 模式运行时可用。

种子账号：租户 `default` · `admin / Admin@123`。验证码以登录页显示为准。

## 目录

| 路径 | 职责 |
| --- | --- |
| `src/api` | 统一 HTTP（信封解包 / 双 token 刷新 / X-Tenant-ID）、认证与系统管理接口（`admin/`）、C 端接口（`portal/`）、会话存储 |
| `src/router` | 静态入口、`routes.tsx` 后台动态菜单路由、登录守卫、401 统一跳转 |
| `src/lib` | 外观、认证编排（COMPONENT_MAP）、列表状态、HTTP/session/tenant、提示 / 确认（React 版） |
| `src/components/ui` | shadcn/ui（base-nova / Base UI 底座）+ solid-glass 玻璃质感控件（React `.tsx`） |
| `src/components/business` | ListPage、SearchForm、FormDialog、PermissionButton、CaptchaDialog（React `.tsx`） |
| `src/components/layout` | AppShell、Sidebar、Topbar、CommandPalette、MobileDrawer、外观 / 密码弹窗（React `.tsx`） |
| `src/views` | 登录、工作台、系统管理页（用户/角色/菜单/部门/岗位/字典/参数/租户）、日志页、异常页（React `.tsx`） |
| `src/portal` | C 端门户（Hash 路由）：首页、文档、登录 / 注册 / 个人中心与验证码弹窗（React `.tsx`） |
| `src/store` | React Context 会话状态 |
| `src/test` | Vitest + React Testing Library 契约与页面测试 |
| `src/index.css`、`src/solid-glass.css` | Tailwind CSS 4 + tw-animate-css + solid-glass 玻璃样式 |

## 技术栈

- React 19 + TypeScript + Vite（双入口：`index.html` 后台管理 / `portal.html` C 端门户）
- Tailwind CSS 4（`@tailwindcss/vite`）+ tw-animate-css
- shadcn/ui（base-nova / Base UI 底座）+ solid-glass 玻璃质感组件
- React Router（admin 动态菜单路由）/ Hash 路由（portal）
- Vitest + React Testing Library（契约与页面测试）

质量命令：`npm run typecheck` / `npm run test:run` / `npm run build`

## 关键约定

- **接口契约**：响应信封 `{code, msg, data}`；分页 `{list, total, page_num, page_size, pages}`；
  认证头 `Authorization: Bearer <access_token>` + `X-Tenant-ID`；登录需 `captcha`（按挑战类型作答）。
- **权限**：按钮级用 `PermissionButton` 组件（`code` 属性与 Casbin 权限标识一致）；
  前端隐藏不能替代后端校验。
- **菜单**：`src/router/routes.tsx` 的 `COMPONENT_MAP` 注册 `system/user/index` 等真实组件键；
  页面不改会话存储与路由就绪标记，退出 / 401 统一走 `clearSession()`。
- **新增列表页**：复用 React `ListPage` + `useListPage`；新增表单：复用 `FormDialog`；
  接口复用 `src/api/admin/` 的类型与方法。
