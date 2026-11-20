# devops-frontend React 19 + solid-glass 迁移实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 `devops-frontend` 从 Vue 3 + GSAP + Three.js 原位重写为 React 19 + TypeScript + Vite + Tailwind 4 + shadcn/ui（base-nova）+ solid-glass，实现核心功能对等（core-parity），保留双入口、`dist` 产物与网关代理。

**Architecture:** 两个独立挂载根（admin `index.html` → `src/main.tsx`，portal `portal.html` → `src/portal/main.tsx`）共享一套框架无关契约层 `src/lib`（http/session/appearance/permission/toast/confirm/list/options/datetime/portal-session/tenant/captcha/errors/types）与接口层 `src/api`。admin 用 React Router（静态路由 + 登录后经 COMPONENT_MAP 动态注册），portal 用自实现 hash 路由。主题为 `html.dark` class 策略，挂载前应用。

**Tech Stack:** React 19、TypeScript、Vite、Tailwind CSS 4（`@tailwindcss/vite`）、shadcn/ui base-nova（Base UI 底座）、solid-glass、tw-animate-css、lucide-react、sonner、React Router、Vitest + React Testing Library + jsdom。

**Spec:** `docs/superpowers/specs/2026-11-20-frontend-solid-glass-migration-design.md`

## Global Constraints

- React **19**（`react@^19`、`react-dom@^19`）；函数组件 + Hooks；组件用法以 Base UI 文档为准，不自行封装底层渲染逻辑。
- Tailwind CSS **4**，经 `@tailwindcss/vite` 插件接入；`tw-animate-css` 必装。
- shadcn/ui 使用 **base-nova（Base UI 底座）**；`components.json`：`style: base-nova`、`tailwind.config: ""`、`css: src/index.css`、`iconLibrary: lucide`。
- solid-glass 经 `npx shadcn@latest add deeplethe/solid-glass/solid-glass` 引入；**不得重命名 `data-slot`**；portal 对话框需带主题。
- `src/index.css` 导入顺序固定：`@import "tailwindcss";` → `@import "tw-animate-css";` → `@import "shadcn/tailwind.css";` → `@import "./solid-glass.css";`，并声明 `@custom-variant dark (&:is(.dark *))`。
- 双入口保留：`index.html`（admin）+ `portal.html`（portal）；`vite.config.ts` 保留 `alias @ → src`、`server.port: 5173`、`server.host: '127.0.0.1'`、`build.chunkSizeWarningLimit: 1600`、双入口 `rollupOptions.input`。
- 网关代理只代理 **`/system/api`** 与 **`/user/api`**（不代理整个 `/system`），环境变量覆盖保留：`VITE_GATEWAY_TARGET`、`VITE_GATEWAY_SYSTEM_HOST`、`VITE_GATEWAY_USER_HOST`、`VITE_DIRECT_SERVICE`、`VITE_API_PROXY_TARGET`、`VITE_USER_PROXY_TARGET`。
- 后端契约：读接口 camelCase、写接口 snake_case；**admin 全部 ID 为字符串（雪花 ID），禁止强制转数字**；portal `PublicProfile.id` 为数字、`created_at` 为 unix 毫秒。
- 分页：请求 `page_num`/`page_size`（`page_size` 上限 1000）；响应 `PageResult<T> = { list, total, page_num, page_size, pages }`。
- 权限码精确匹配、无通配符；`PermissionButton` 无权限时**不渲染**。
- 不从 Vue 代码复制逻辑；框架无关契约以 hooks/context/store 重新实现。
- 存储键保持不变：`devops.access_token`、`devops.refresh_token`、`devops.tenant_id`、`devops.tenant_code`、`devops.session_ver`、`devops.last_tenant`、`devops.appearance`、`devops.last_user`；portal 仅 `portal_tokens`。
- 主题策略：`html.dark` class（替代旧 `data-theme` 属性），深色为主；`devops.appearance` JSON 结构 `{ theme, density, railCollapsed }` 保留；主题在 `main.tsx` 挂载前应用。
- 禁止引入 Vue/GSAP/Three.js/go-captcha-vue（迁移结束后全部移除，含依赖）。
- 包管理器 **npm**（项目有 `package-lock.json`）；质量命令：`npm run typecheck`（基于 `tsc --noEmit`）、`npm run build`、`npm run test:run`。
- 中间过程允许旧 Vue 源码暂存（`vue` 依赖在 Task 15 前保留以维持旧文件可编译）；构建入口在 Task 1 即切换为 React，旧 `.vue` 不再被任何入口引用。
- 不做代码复制、无占位实现（不允许 TODO/TBD/骨架进入交付）。

---

## File Structure

以下为迁移完成后 `devops-frontend` 的目标文件清单（按区域分组，标注 Create/Modify/Delete 与职责）。

### 入口与工程配置

| 操作 | 文件 | 职责 |
| --- | --- | --- |
| Modify | `devops-frontend/index.html` | admin 入口，`<script type="module" src="/src/main.tsx">`，挂载点 `<div id="root">` |
| Modify | `devops-frontend/portal.html` | portal 入口，`<script type="module" src="/src/portal/main.tsx">`，挂载点 `<div id="root">` |
| Modify | `devops-frontend/vite.config.ts` | React + Tailwind 插件、双入口、网关代理（`/system/api`、`/user/api`）、vitest test 配置 |
| Create | `devops-frontend/components.json` | shadcn 配置：base-nova / 空 tailwind.config / `src/index.css` / lucide |
| Modify | `devops-frontend/tsconfig.json` | React JSX 配置（`jsx: react-jsx`），含 vitest 类型 |
| Create | `devops-frontend/src/vite-env.d.ts` | `/// <reference types="vite/client" />` |
| Delete | `devops-frontend/env.d.ts` | 旧 Vue 类型垫片（Task 15 清理） |
| Modify | `devops-frontend/package.json` | 依赖与脚本（dev/build/preview/typecheck/test/test:run） |
| Modify | `devops-frontend/README.md` | 更新技术栈描述（Task 15，若含前端栈描述） |

### 共享契约层 `src/lib`（框架无关，两入口共用）

| 操作 | 文件 | 职责 |
| --- | --- | --- |
| Create | `src/index.css` | 固定导入顺序 + `@custom-variant dark` + 密度/折叠工具类 |
| Create | `src/lib/types.ts` | 全部契约类型：`TokenPair`、`TenantBrief`、`LoginResult`、`AuthUser`、`AuthInfo`、`RouterNode`、`PageResult<T>`、`CaptchaChallenge`、`CaptchaAnswer`、`PortalTokenPair`、`PublicProfile`、`Appearance` |
| Create | `src/lib/http.ts` | `ApiError`、`buildQuery`（空值剔除）、`request<T>()`（信封解包、认证头、401 单飞刷新、会话版本守卫） |
| Create | `src/lib/session.ts` | admin 会话存储（8 个 `devops.*` 键）、`sessionVersion`/`getSession`/`saveSession`/`saveTokens`/`clearSession` |
| Create | `src/lib/appearance.ts` | `Appearance` 读写、`applyAppearance`（`html.dark` + 密度 + rail）、`watchSystemTheme`、`useAppearance` |
| Create | `src/lib/permission.tsx` | `PermissionsProvider`/`usePermissions`/`hasPermission`（精确匹配） |
| Create | `src/lib/toast.ts` | sonner 封装：`toast`/`toastSuccess`/`toastError` |
| Create | `src/lib/confirm.tsx` | `ConfirmProvider`/`useConfirm`（AlertDialog 承载，返回 Promise<boolean>） |
| Create | `src/lib/list.ts` | `useListPage<T>()`：查询/分页/竞态防护/选中集/load/search/reset/setPage/setPageSize/exportData/afterMutate |
| Create | `src/lib/options.ts` | 选项缓存 `loadOptions`/`invalidateOptions`/`useOptions` |
| Create | `src/lib/datetime.ts` | `formatDateTime`/`formatDate`（unix 毫秒与字符串均支持） |
| Create | `src/lib/portal-session.ts` | `PortalApiError`、`portalRequest<T>()`（401 单飞刷新、失败清 `portal_tokens`）、tokens 存取 |
| Create | `src/lib/tenant.ts` | `resolveTenantEntry`：query → path `/t/{code}/login` → 子域 → `devops.last_tenant` → `'default'`，锁定语义 |
| Create | `src/lib/captcha.ts` | `captchaEnabled`（`enabled=false` 跳过）、`buildCaptchaAnswer`（四题型作答映射） |
| Create | `src/lib/errors.ts` | `mapLoginError`：五类后端错误 → 中文文案 |
| Create | `src/test/setup.ts` | Vitest setup：`@testing-library/jest-dom`、localStorage 清理 |

### 接口层 `src/api`

| 操作 | 文件 | 职责 |
| --- | --- | --- |
| Create | `src/api/admin/auth.ts` | captcha/login/refresh/logout/info/routers/profile/profile-password（camelCase 读 + snake_case 写） |
| Create | `src/api/admin/system.ts` | user/role/menu/dept/post/tenant/dict/config/loginlog/operationlog 全部接口与类型，写接口统一 snake_case 映射 |
| Create | `src/api/portal/auth.ts` | portal captcha/login/register/refresh/logout/profile/profile-password |

### 状态与路由（admin）

| 操作 | 文件 | 职责 |
| --- | --- | --- |
| Create | `src/store/session.tsx` | `SessionProvider`/`useSession`：`authInfo`、`routes`、`bootstrap()`、`reload()`、`logout()` |
| Create | `src/router/routes.tsx` | 静态路由 + `COMPONENT_MAP`（11 键）+ `buildDynamicRoutes`（未注册键 `console.warn` 跳过）+ `flattenMenus` |
| Create | `src/router/guards.tsx` | `RequireAuth`/`RequireGuest`、`installUnauthorizedHandler`（401 清会话跳登录，带 tenantCode/redirect） |
| Create | `src/App.tsx` | admin 路由编排（BrowserRouter + SessionProvider + useRoutes） |
| Create | `src/main.tsx` | admin 挂载：先 `applyAppearance()` 再 `createRoot`，挂 ConfirmProvider/Toaster |
| Delete | `src/main.ts` | 旧 Vue admin 入口（Task 15） |
| Delete | `src/App.vue` | 旧 Vue 根组件（Task 15） |
| Delete | `src/router/index.ts` | 旧 Vue Router（Task 15） |

### 业务组件 `src/components`

| 操作 | 文件 | 职责 |
| --- | --- | --- |
| Create | `src/components/business/CaptchaDialog.tsx` | 行为式验证码弹框：click/slide/dragdrop/rotate 四题型交互、失败换题、作答映射 |
| Create | `src/components/business/PermissionButton.tsx` | 权限按钮（无权限不渲染） |
| Create | `src/components/business/SearchForm.tsx` | 查询区（input/select 字段 + 查询/重置） |
| Create | `src/components/business/ListPage.tsx` | 列表骨架：搜索区 + 表格 + 分页 + 批量操作条 + 工具栏 |
| Create | `src/components/business/FormDialog.tsx` | 表单对话框（Dialog + Field 组合） |
| Create | `src/components/layout/AppShell.tsx` | Sidebar + Topbar + Content 编排 |
| Create | `src/components/layout/Sidebar.tsx` | 动态菜单渲染、rail 折叠、玻璃质感 |
| Create | `src/components/layout/Topbar.tsx` | 面包屑、租户信息、用户菜单、Ctrl/⌘+K |
| Create | `src/components/layout/CommandPalette.tsx` | 命令面板（CommandDialog，菜单快速跳转） |
| Create | `src/components/layout/AppearanceDialog.tsx` | 外观设置（主题/密度/折叠） |
| Create | `src/components/layout/PasswordDialog.tsx` | 修改密码对话框 |
| Create | `src/components/layout/MobileDrawer.tsx` | 小屏侧边栏抽屉（Sheet） |

### Admin 页面 `src/views`

| 操作 | 文件 | 职责 |
| --- | --- | --- |
| Create | `src/views/Login.tsx` | 登录页（租户解析/锁定、验证码、错误映射、redirect） |
| Create | `src/views/Dashboard.tsx` | 工作台（账号/菜单/审计日志真实数据） |
| Create | `src/views/NoAccess.tsx` | 403 |
| Create | `src/views/NotFound.tsx` | 404 |
| Create | `src/views/system/Users.tsx` | 用户管理 |
| Create | `src/views/system/Roles.tsx` | 角色管理（含分配菜单树） |
| Create | `src/views/system/Menus.tsx` | 菜单管理（树形） |
| Create | `src/views/system/Depts.tsx` | 部门管理（树形） |
| Create | `src/views/system/Posts.tsx` | 岗位管理 |
| Create | `src/views/system/Tenants.tsx` | 租户管理 |
| Create | `src/views/system/Dict.tsx` | 字典管理（类型 + 数据项双面板） |
| Create | `src/views/system/Configs.tsx` | 参数配置（内置键控件映射） |
| Create | `src/views/monitor/LoginLogs.tsx` | 登录日志 |
| Create | `src/views/monitor/OperationLogs.tsx` | 操作日志（含详情抽屉） |

### Portal `src/portal`

| 操作 | 文件 | 职责 |
| --- | --- | --- |
| Create | `src/portal/main.tsx` | portal 挂载（先 applyAppearance） |
| Create | `src/portal/App.tsx` | hash 路由 + 导航栏 + 页面编排 |
| Create | `src/portal/useHashRoute.ts` | hash 路由解析与导航 |
| Create | `src/portal/components/PortalDialog.tsx` | 带主题的 portal 对话框封装 |
| Create | `src/portal/components/PortalCaptchaDialog.tsx` | portal 验证码弹框 |
| Create | `src/portal/pages/Home.tsx` | 静态玻璃营销首页（Design World / Explore New Ideas / Explore Now） |
| Create | `src/portal/pages/Login.tsx` | Sign In |
| Create | `src/portal/pages/Register.tsx` | Sign Up |
| Create | `src/portal/pages/Profile.tsx` | 个人资料（编辑资料 + 修改密码） |
| Create | `src/portal/pages/Docs.tsx` | 文档页 |
| Delete | `src/portal/main.ts`、`src/portal/api.ts`、`src/portal/captcha.ts`、`src/portal/pages.ts`、`src/portal/hero.ts`、`src/portal/style.css` | 旧 portal 实现（Task 15） |

### 清理目标（Task 15 全部 Delete）

- `src/api/` 旧文件（`auth.ts`、`http.ts`、`index.ts`、`session.ts`、`system.ts` —— React 版位于 `src/api/admin/`、`src/api/portal/` 子目录，不冲突）
- `src/components/` 旧文件（`Icon.vue`、`TopoNet.vue`、`business/*.vue`、`business/index.ts`、`layout/*.vue`、`ui/*.vue`、`ui/index.ts`、`ui/types.ts`）
- `src/composables/`（全部 `.ts`）
- `src/styles/`（全部 7 个 css）
- `src/views/` 旧 `.vue` 文件与 `system/parts/MenuCheckTree.vue`
- `src/portal/` 旧 `.ts`/`.css`
- `env.d.ts`
- `package.json` 移除依赖：`vue`、`vue-router`、`vue-tsc`、`@vitejs/plugin-vue`、`go-captcha-vue`、`gsap`、`three`、`@types/three`

---

### Task 1: 工程脚手架（React 19 + Vite + Tailwind 4 + shadcn base-nova + solid-glass + 测试工具链）

**Files:**
- Modify: `devops-frontend/package.json`
- Modify: `devops-frontend/index.html`
- Modify: `devops-frontend/portal.html`
- Modify: `devops-frontend/vite.config.ts`
- Modify: `devops-frontend/tsconfig.json`
- Create: `devops-frontend/components.json`
- Create: `devops-frontend/src/index.css`
- Create: `devops-frontend/src/vite-env.d.ts`
- Create: `devops-frontend/src/main.tsx`
- Create: `devops-frontend/src/portal/main.tsx`
- Create: `devops-frontend/src/test/setup.ts`
- Create: `devops-frontend/src/lib/appearance.ts`（最小实现，供挂载前应用主题；hook 扩展在 Task 2）

**Interfaces:**
- Consumes: 无（首个任务）。
- Produces: `src/main.tsx`（admin 挂载）、`src/portal/main.tsx`（portal 挂载）、`src/index.css`（固定导入顺序）、`applyAppearance(a?: Appearance): void` / `loadAppearance(): Appearance` / `DEFAULT_APPEARANCE`（Task 2 扩展 hook）、`src/test/setup.ts`；scripts：`dev`/`build`/`preview`/`typecheck`/`test`/`test:run`。

- [ ] **Step 1: 安装 React 技术栈依赖（保留 Vue 依赖到 Task 15）**

```bash
cd devops-frontend
npm install react@^19 react-dom@^19 react-router-dom@^7 lucide-react sonner tw-animate-css
npm install -D @vitejs/plugin-react @types/react @types/react-dom tailwindcss@^4 @tailwindcss/vite vitest jsdom @testing-library/react @testing-library/jest-dom @testing-library/user-event
```

- [ ] **Step 2: 更新 `package.json` 脚本**

把 `scripts` 段替换为：

```json
{
  "scripts": {
    "dev": "vite",
    "build": "npm run typecheck && vite build",
    "preview": "vite preview",
    "typecheck": "tsc --noEmit",
    "test": "vitest",
    "test:run": "vitest run"
  }
}
```

- [ ] **Step 3: 重写 `vite.config.ts`（React + Tailwind 插件，代理与双入口原样保留）**

```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { fileURLToPath, URL } from 'node:url'

// 只代理 /system/api 与 /user/api（不代理整个 /system，否则深链接刷新会被
// 网关守卫 401）。B 端 Host=api.devops.local，C 端 Host=user-api.devops.local。
const gatewayTarget = process.env.VITE_GATEWAY_TARGET || 'http://127.0.0.1:8080'
const systemHost = process.env.VITE_GATEWAY_SYSTEM_HOST || 'api.devops.local'
const userHost = process.env.VITE_GATEWAY_USER_HOST || 'user-api.devops.local'
const direct = process.env.VITE_DIRECT_SERVICE === '1'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    host: '127.0.0.1',
    proxy: {
      '/system/api': direct
        ? {
            target: process.env.VITE_API_PROXY_TARGET || 'http://127.0.0.1:9999',
            changeOrigin: true,
          }
        : {
            target: gatewayTarget,
            changeOrigin: true,
            headers: { Host: systemHost },
          },
      '/user/api': direct
        ? {
            target: process.env.VITE_USER_PROXY_TARGET || 'http://127.0.0.1:10000',
            changeOrigin: true,
          }
        : {
            target: gatewayTarget,
            changeOrigin: true,
            headers: { Host: userHost },
          },
    },
  },
  build: {
    chunkSizeWarningLimit: 1600,
    rollupOptions: {
      input: {
        main: fileURLToPath(new URL('./index.html', import.meta.url)),
        portal: fileURLToPath(new URL('./portal.html', import.meta.url)),
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    css: false,
  },
})
```

- [ ] **Step 4: 重写 `index.html` 与 `portal.html`（双入口指向 React 挂载根）**

`index.html` body：

```html
<div id="root"></div>
<script type="module" src="/src/main.tsx"></script>
```

`portal.html` body：

```html
<div id="root"></div>
<script type="module" src="/src/portal/main.tsx"></script>
```

两文件保留/补齐 `<html lang="zh-CN">`、charset/viewport meta；`<title>`：admin `DevOps 管理平台`，portal `Design World`。

- [ ] **Step 5: 重写 `tsconfig.json`（React JSX）**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "moduleResolution": "bundler",
    "resolveJsonModule": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": false,
    "noUnusedParameters": false,
    "noFallthroughCasesInSwitch": true,
    "skipLibCheck": true,
    "types": ["vitest/globals", "@testing-library/jest-dom"],
    "baseUrl": ".",
    "paths": { "@/*": ["src/*"] }
  },
  "include": ["src"]
}
```

（旧 `src/env.d.ts` 若含 `*.vue` 垫片可暂留，Task 15 删除。）

- [ ] **Step 6: 创建 `src/vite-env.d.ts` 与 `src/test/setup.ts`**

`src/vite-env.d.ts`：

```ts
/// <reference types="vite/client" />
```

`src/test/setup.ts`：

```ts
import '@testing-library/jest-dom/vitest'

beforeEach(() => {
  localStorage.clear()
})
```

- [ ] **Step 7: shadcn init（base-nova）并添加基础组件**

```bash
npx shadcn@latest init -t vite -b base
npx shadcn@latest add button input textarea label table dialog alert-dialog dropdown-menu sheet select switch checkbox tabs card badge tooltip popover separator skeleton scroll-area alert avatar progress breadcrumb command sonner field pagination
```

init 生成/改写 `components.json`、`src/index.css`、`src/lib/utils.ts`。核对 `components.json` 关键字段必须为：`"style": "base-nova"`、`"tailwind": { "config": "", "css": "src/index.css" }`、`"iconLibrary": "lucide"`。若 init 生成的 `baseColor` 等其他字段为默认值可保留。

- [ ] **Step 8: 添加 solid-glass**

```bash
npx shadcn@latest add deeplethe/solid-glass/solid-glass
```

可选增强（仅当后续任务实际用到，非必需）：`npx shadcn@latest add deeplethe/solid-glass/panel deeplethe/solid-glass/tag deeplethe/solid-glass/dot-loader deeplethe/solid-glass/status-glyph`。

- [ ] **Step 9: 固化 `src/index.css` 顶部导入顺序**

init 生成后，把 `src/index.css` 的**顶部导入区**改写为以下内容（`:root`/`.dark` CSS 变量块、`@theme inline` 块保留在其后，不改任何 `data-slot` 样式）：

```css
@import "tailwindcss";
@import "tw-animate-css";
@import "shadcn/tailwind.css";
@import "./solid-glass.css";

@custom-variant dark (&:is(.dark *));
```

并在文件末尾追加密度/折叠工具类：

```css
html[data-density='compact'] { --spacing-scale: 0.85; }
html[data-density='comfy'] { --spacing-scale: 1.1; }
html.rail-collapsed .rail-wide { display: none; }
```

- [ ] **Step 10: 创建 `src/lib/appearance.ts`（挂载前主题应用）**

```ts
export type Theme = 'light' | 'dark' | 'auto'
export type Density = 'comfy' | 'regular' | 'compact'

export interface Appearance {
  theme: Theme
  density: Density
  railCollapsed: boolean
}

export const DEFAULT_APPEARANCE: Appearance = {
  theme: 'auto',
  density: 'regular',
  railCollapsed: false,
}

const KEY = 'devops.appearance'

export function loadAppearance(): Appearance {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return { ...DEFAULT_APPEARANCE }
    const parsed = JSON.parse(raw) as Partial<Appearance>
    return {
      theme: parsed.theme ?? DEFAULT_APPEARANCE.theme,
      density: parsed.density ?? DEFAULT_APPEARANCE.density,
      railCollapsed: parsed.railCollapsed ?? DEFAULT_APPEARANCE.railCollapsed,
    }
  } catch {
    return { ...DEFAULT_APPEARANCE }
  }
}

export function saveAppearance(a: Appearance): void {
  localStorage.setItem(KEY, JSON.stringify(a))
}

export function systemPrefersDark(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches
}

export function resolveDark(a: Appearance): boolean {
  return a.theme === 'dark' || (a.theme === 'auto' && systemPrefersDark())
}

export function applyAppearance(a: Appearance = loadAppearance()): void {
  const root = document.documentElement
  root.classList.toggle('dark', resolveDark(a))
  root.dataset.density = a.density
  root.classList.toggle('rail-collapsed', a.railCollapsed)
}

export function watchSystemTheme(cb: (a: Appearance) => void): () => void {
  const mql = window.matchMedia('(prefers-color-scheme: dark)')
  const onChange = () => cb(loadAppearance())
  mql.addEventListener('change', onChange)
  return () => mql.removeEventListener('change', onChange)
}
```

- [ ] **Step 11: 创建两个挂载入口与最小根组件**

`src/main.tsx`：

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { applyAppearance } from '@/lib/appearance'
import App from './App'

applyAppearance()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

`src/portal/main.tsx`：

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { applyAppearance } from '@/lib/appearance'
import PortalApp from './App'

applyAppearance()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <PortalApp />
  </StrictMode>,
)
```

`src/App.tsx`：

```tsx
export default function App() {
  return <div className="min-h-screen">devops admin</div>
}
```

`src/portal/App.tsx`：

```tsx
export default function PortalApp() {
  return <div className="min-h-screen">Design World</div>
}
```

（这两个最小根在 Task 3 / Task 13 立即替换。）

- [ ] **Step 12: 写工具链验证测试（红→绿）**

`src/test/toolchain.test.tsx`：

```tsx
import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { applyAppearance, resolveDark, DEFAULT_APPEARANCE } from '@/lib/appearance'

describe('test toolchain + appearance bootstrap', () => {
  it('renders a React 19 component', () => {
    render(<div>hello</div>)
    expect(screen.getByText('hello')).toBeInTheDocument()
  })

  it('applyAppearance toggles html.dark for dark theme', () => {
    applyAppearance({ ...DEFAULT_APPEARANCE, theme: 'dark' })
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('resolveDark follows system in auto mode', () => {
    const spy = vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    } as unknown as MediaQueryList)
    expect(resolveDark({ theme: 'auto', density: 'regular', railCollapsed: false })).toBe(true)
    spy.mockRestore()
  })
})
```

- [ ] **Step 13: 运行测试与静态校验**

```bash
npm run test:run
npm run typecheck
npm run build
```

Expected: 测试 3 个 PASS；typecheck 零错误；build 成功且 `dist/index.html`、`dist/portal.html` 均生成。（若 typecheck 报旧 `.vue` 文件错误，把 tsconfig `include` 临时改为 `["src/**/*.ts", "src/**/*.tsx"]`，Task 15 移除 Vue 文件后无需排除。）

- [ ] **Step 14: Commit**

```bash
git add -A devops-frontend
git commit -m "feat(frontend): scaffold React 19 + Vite + Tailwind 4 + shadcn base-nova + solid-glass dual-entry base"
```

### Task 2: 共享契约层（http / session / tenant / captcha / errors / options / datetime / list / permission / toast / confirm / portal-session）

**Files:**
- Create: `devops-frontend/src/lib/types.ts`
- Create: `devops-frontend/src/lib/http.ts`
- Create: `devops-frontend/src/lib/session.ts`
- Create: `devops-frontend/src/lib/tenant.ts`
- Create: `devops-frontend/src/lib/captcha.ts`
- Create: `devops-frontend/src/lib/errors.ts`
- Create: `devops-frontend/src/lib/datetime.ts`
- Create: `devops-frontend/src/lib/options.ts`
- Create: `devops-frontend/src/lib/list.ts`
- Create: `devops-frontend/src/lib/portal-session.ts`
- Create: `devops-frontend/src/lib/toast.ts`
- Create: `devops-frontend/src/lib/confirm.tsx`
- Create: `devops-frontend/src/lib/permission.tsx`
- Modify: `devops-frontend/src/lib/appearance.ts`（追加 `useAppearance` hook）
- Test: `devops-frontend/src/test/http.test.ts`、`src/test/session.test.ts`、`src/test/tenant.test.ts`、`src/test/captcha.test.ts`、`src/test/errors.test.ts`、`src/test/datetime.test.ts`、`src/test/list.test.tsx`、`src/test/portal-session.test.ts`、`src/test/permission.test.tsx`、`src/test/confirm.test.tsx`、`src/test/appearance.test.tsx`

**Interfaces:**
- Consumes: `loadAppearance`/`applyAppearance`（Task 1）；sonner Toaster（Task 1 组件）；shadcn AlertDialog。
- Produces（后续所有任务依赖的精确契约）:
  - `ApiError extends Error { status: number; code: number; msg: string }`；`new ApiError(status: number, code: number, msg: string)`
  - `request<T>(path: string, opts?: RequestInit & { auth?: boolean; query?: Record<string, unknown> }): Promise<T>` — 信封解包，`auth !== false` 时带 Bearer + X-Tenant-ID；401 走单飞刷新并重试一次；刷新失败清会话并触发未授权跳转；响应 401 且 `ver !== sessionVersion()` 时直接抛「会话已切换」。
  - `session`：`sessionVersion(): number`、`isLoggedIn(): boolean`、`getAccessToken(): string | null`、`getRefreshToken(): string | null`、`getTenantId(): string | null`、`getTenantCode(): string | null`、`getSession(): Session`、`saveSession(pair: { access_token; refresh_token }, tenant: { id; code } | null): number`、`saveTokens(pair: { access_token; refresh_token }): number`、`clearSession(): number`、`setLastUser(username: string): void`、`getLastTenant(): string | null`、`onUnauthorized(cb: () => void): () => void`、`fireUnauthorized(): void`。
  - `interface Session { access_token: string | null; refresh_token: string | null; tenant_id: string | null; tenant_code: string | null; loggedIn: boolean; ver: number }`
  - `resolveTenantEntry(search: string, pathname: string, hostname: string): { code: string; locked: boolean }`
  - `captchaEnabled(c: CaptchaChallenge | null | undefined): boolean`；`buildCaptchaAnswer(c: CaptchaChallenge, a: CaptchaAnswer): Record<string, unknown>`
  - `mapLoginError(msg: string): string`
  - `formatDateTime(v: string | number | null | undefined): string`、`formatDate(v: string | number | null | undefined): string`
  - `useListPage<T extends { id: string }>(cfg: ListPageConfig<T>): ListPageState<T>`（签名见实现）
  - `portalSession`：`getTokens(): PortalTokenPair | null`、`saveTokens(t: PortalTokenPair): void`、`clearTokens(): void`、`isLoggedIn(): boolean`
  - `portalRequest<T>(path: string, opts?): Promise<T>`；`PortalApiError extends Error { status: number; msg: string }`
  - `PermissionsProvider({ permissions, children })`；`usePermissions(): { permissions: string[]; has(p: string): boolean }`
  - `useAppearance(): { appearance: Appearance; update(patch: Partial<Appearance>): void }`
  - `toast(msg)`、`toastSuccess(msg)`、`toastError(msg)`
  - `ConfirmProvider`；`useConfirm(): (opts: { title: string; description?: string; confirmText?: string; danger?: boolean }) => Promise<boolean>`
  - `loadOptions<T>(key, loader): Promise<T[]>`、`invalidateOptions(key?: string): void`、`useOptions<T>(key, loader): { data: T[]; loading: boolean; reload(): void }`

- [ ] **Step 1: 创建 `src/lib/types.ts`（全部契约类型）**

```ts
export type { Appearance, Density, Theme } from '@/lib/appearance'

// ── 通用 ──
export interface PageResult<T> {
  list: T[]
  total: number
  page_num: number
  page_size: number
  pages: number
}

// ── Admin 认证 ──
export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_in: number
}

export interface TenantBrief {
  id: string
  name: string
  code: string
}

export interface LoginResult extends TokenPair {
  tenant: TenantBrief
}

export interface AuthUser {
  id: string
  username: string
  nickname?: string
  email?: string
  mobile?: string
  gender?: string
  avatar?: string
  deptId?: string
  postIds?: string[]
  remark?: string
  loginIp?: string
  loginDate?: string
}

export interface AuthInfo {
  user: AuthUser
  tenant: TenantBrief
  roles: string[]
  permissions: string[]
}

export interface RouterNode {
  id: string
  name: string
  path: string
  component: string
  componentName?: string
  icon?: string
  sort: number
  visible: boolean
  keepAlive?: boolean
  alwaysShow?: boolean
  children?: RouterNode[]
}

// ── 行为式验证码 ──
export type CaptchaType = 'click' | 'slide' | 'dragdrop' | 'rotate'

export interface CaptchaChallenge {
  id: string
  type: CaptchaType
  image: string
  thumb?: string
  width: number
  height: number
  text?: string
  thumb_x?: number
  thumb_y?: number
  thumb_width?: number
  thumb_height?: number
  thumb_size?: number
  enabled?: boolean
}

export interface CaptchaAnswer {
  points?: Array<{ x: number; y: number }>
  x?: number
  y?: number
  angle?: number
}

// ── Portal ──
export interface PortalTokenPair {
  access_token: string
  refresh_token: string
  expires_in: number
  issued_at: number
}

export interface PublicProfile {
  id: number
  username: string
  nickname: string
  email: string
  mobile: string
  gender: string
  avatar: string
  created_at: number
}

// ── Admin system 模块（读 camelCase；写 snake_case 由 api 层映射） ──
export interface SystemUser {
  id: string
  username: string
  nickname: string
  deptId: string
  postIds: string[]
  roleIds: string[]
  email?: string
  mobile?: string
  gender?: string
  avatar?: string
  remark?: string
  status: number
  createdAt?: string
}

export interface SystemRole {
  id: string
  name: string
  code: string
  sort: number
  dataScope: number
  dataScopeDeptIds: string[]
  builtin?: boolean
  status: number
  remark?: string
}

export interface SystemMenu extends RouterNode {
  type: number // 1目录 2菜单 3按钮
  parentId: string
  permission?: string
  status: number
}

export interface SystemDept {
  id: string
  name: string
  parentId: string
  sort: number
  leaderUserId?: string
  phone?: string
  email?: string
  status: number
}

export interface SystemPost {
  id: string
  code: string
  name: string
  sort: number
  status: number
  remark?: string
}

export interface SystemTenant {
  id: string
  name: string
  code: string
  contactName: string
  contactMobile: string
  expireTime?: string
  remark?: string
}

export interface DictType {
  id: string
  name: string
  type: string
  status: number
  remark?: string
}

export interface DictItem {
  id: string
  dictType: string
  label: string
  value: string
  sort: number
}

export interface SystemConfig {
  id: string
  configKey: string
  configName: string
  configValue: string
  type: number // 1内置 2自定义
  visible: boolean
  remark?: string
}

export interface LoginLog {
  id: string
  username: string
  ip: string
  status: number
  msg: string
  loginAt: string
}

export interface OperationLog {
  id: string
  username: string
  module: string
  action: string
  method: string
  uri: string
  ip: string
  statusCode?: number
  request?: string
  response?: string
  createdAt: string
}
```

- [ ] **Step 2: 创建 `src/lib/session.ts`（admin 会话存储）**

```ts
export interface Session {
  access_token: string | null
  refresh_token: string | null
  tenant_id: string | null
  tenant_code: string | null
  loggedIn: boolean
  ver: number
}

const ACCESS = 'devops.access_token'
const REFRESH = 'devops.refresh_token'
const TENANT_ID = 'devops.tenant_id'
const TENANT_CODE = 'devops.tenant_code'
const VER = 'devops.session_ver'
const LAST_TENANT = 'devops.last_tenant'
const LAST_USER = 'devops.last_user'

function currentVer(): number {
  const raw = Number(localStorage.getItem(VER) ?? '0')
  return Number.isFinite(raw) ? raw : 0
}

function bumpVer(): number {
  const v = currentVer() + 1
  localStorage.setItem(VER, String(v))
  return v
}

let unauthorizedCb: (() => void) | null = null

export const session = {
  sessionVersion: (): number => currentVer(),
  isLoggedIn: (): boolean => localStorage.getItem(ACCESS) != null,
  getAccessToken: (): string | null => localStorage.getItem(ACCESS),
  getRefreshToken: (): string | null => localStorage.getItem(REFRESH),
  getTenantId: (): string | null => localStorage.getItem(TENANT_ID),
  getTenantCode: (): string | null => localStorage.getItem(TENANT_CODE),
  getSession: (): Session => ({
    access_token: localStorage.getItem(ACCESS),
    refresh_token: localStorage.getItem(REFRESH),
    tenant_id: localStorage.getItem(TENANT_ID),
    tenant_code: localStorage.getItem(TENANT_CODE),
    loggedIn: localStorage.getItem(ACCESS) != null,
    ver: currentVer(),
  }),
  saveSession(
    pair: { access_token: string; refresh_token: string },
    tenant: { id: string; code: string } | null,
  ): number {
    localStorage.setItem(ACCESS, pair.access_token)
    localStorage.setItem(REFRESH, pair.refresh_token)
    if (tenant) {
      localStorage.setItem(TENANT_ID, tenant.id)
      localStorage.setItem(TENANT_CODE, tenant.code)
      localStorage.setItem(LAST_TENANT, tenant.code)
    }
    return bumpVer()
  },
  saveTokens(pair: { access_token: string; refresh_token: string }): number {
    localStorage.setItem(ACCESS, pair.access_token)
    localStorage.setItem(REFRESH, pair.refresh_token)
    return bumpVer()
  },
  clearSession(): number {
    localStorage.removeItem(ACCESS)
    localStorage.removeItem(REFRESH)
    localStorage.removeItem(TENANT_ID)
    localStorage.removeItem(TENANT_CODE)
    return bumpVer()
  },
  setLastUser(username: string): void {
    localStorage.setItem(LAST_USER, JSON.stringify({ username }))
  },
  getLastTenant(): string | null {
    return localStorage.getItem(LAST_TENANT)
  },
  onUnauthorized(cb: () => void): () => void {
    unauthorizedCb = cb
    return () => {
      if (unauthorizedCb === cb) unauthorizedCb = null
    }
  },
  fireUnauthorized(): void {
    unauthorizedCb?.()
  },
}
```

- [ ] **Step 3: 写会话测试并验证通过**

`src/test/session.test.ts`：

```ts
import { describe, it, expect, beforeEach } from 'vitest'
import { session } from '@/lib/session'

beforeEach(() => localStorage.clear())

describe('admin session store', () => {
  it('saveSession writes tokens/tenant and bumps version', () => {
    const v0 = session.sessionVersion()
    const v1 = session.saveSession(
      { access_token: 'a', refresh_token: 'r' },
      { id: '1', code: 'acme' },
    )
    expect(v1).toBe(v0 + 1)
    expect(session.isLoggedIn()).toBe(true)
    expect(session.getTenantCode()).toBe('acme')
    expect(localStorage.getItem('devops.last_tenant')).toBe('acme')
  })

  it('clearSession removes token/tenant keys and bumps version', () => {
    session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
    const v = session.clearSession()
    expect(session.isLoggedIn()).toBe(false)
    expect(session.getTenantId()).toBeNull()
    expect(session.sessionVersion()).toBe(v)
  })

  it('saveTokens rotates tokens only', () => {
    session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
    const before = session.getTenantCode()
    session.saveTokens({ access_token: 'a2', refresh_token: 'r2' })
    expect(session.getAccessToken()).toBe('a2')
    expect(session.getTenantCode()).toBe(before)
  })
})
```

Run: `npm run test:run -- src/test/session.test.ts` → Expected: PASS。

- [ ] **Step 4: 创建 `src/lib/http.ts`（信封解包 + 单飞刷新 + 会话版本守卫）**

```ts
import { session } from '@/lib/session'
import type { TokenPair } from '@/lib/types'

export class ApiError extends Error {
  status: number
  code: number
  msg: string
  constructor(status: number, code: number, msg: string) {
    super(msg)
    this.status = status
    this.code = code
    this.msg = msg
  }
}

export function buildQuery(query: Record<string, unknown> | undefined): string {
  if (!query) return ''
  const parts: string[] = []
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null || v === '') continue
    parts.push(`${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
  }
  return parts.length ? `?${parts.join('&')}` : ''
}

// 单飞：并发 401 共享同一个刷新 Promise
let refreshing: Promise<boolean> | null = null

function doRefresh(): Promise<boolean> {
  if (!refreshing) {
    refreshing = (async () => {
      const rt = session.getRefreshToken()
      if (!rt) return false
      const res = await fetch('/system/api/v1/auth/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({ refresh_token: rt }),
      })
      if (!res.ok) return false
      const envelope = (await res.json()) as { code: number; data?: TokenPair }
      if (envelope.code !== 200 || !envelope.data) return false
      session.saveTokens(envelope.data)
      return true
    })().finally(() => {
      refreshing = null
    })
  }
  return refreshing
}

interface RequestOptions extends RequestInit {
  auth?: boolean
  query?: Record<string, unknown>
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const auth = opts.auth !== false
  const verAtStart = session.sessionVersion()
  const headers = new Headers(opts.headers)
  headers.set('Accept', 'application/json')
  if (opts.body) headers.set('Content-Type', 'application/json')
  if (auth) {
    const token = session.getAccessToken()
    if (token) headers.set('Authorization', `Bearer ${token}`)
    const tenantId = session.getTenantId()
    if (tenantId) headers.set('X-Tenant-ID', tenantId)
  }

  const send = async (): Promise<Response> =>
    fetch(`${path}${buildQuery(opts.query)}`, { ...opts, headers })

  let res = await send()
  if (res.status === 401 && auth) {
    // 会话版本守卫：请求发起后版本已变（用户已切换/登出会话），不刷新不跳转
    if (verAtStart !== session.sessionVersion()) {
      throw new ApiError(401, 401, '会话已切换，请重新操作')
    }
    const ok = await doRefresh()
    if (!ok) {
      session.clearSession()
      session.fireUnauthorized()
      throw new ApiError(401, 401, '登录已过期，请重新登录')
    }
    // 只重试一次
    headers.set('Authorization', `Bearer ${session.getAccessToken()}`)
    res = await send()
    if (res.status === 401) {
      session.clearSession()
      session.fireUnauthorized()
      throw new ApiError(401, 401, '登录已过期，请重新登录')
    }
  }

  let envelope: { code: number; msg?: string; data?: unknown }
  try {
    envelope = (await res.json()) as { code: number; msg?: string; data?: unknown }
  } catch {
    throw new ApiError(res.status, res.status, `响应解析失败（HTTP ${res.status}）`)
  }
  if (envelope.code !== 200) {
    throw new ApiError(res.status, envelope.code, envelope.msg || `请求失败（HTTP ${res.status}）`)
  }
  return envelope.data as T
}
```

- [ ] **Step 5: 写 http 契约测试并验证**

`src/test/http.test.ts`：

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { request, ApiError, buildQuery } from '@/lib/http'
import { session } from '@/lib/session'

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('buildQuery', () => {
  it('skips undefined/null/empty values', () => {
    expect(buildQuery({ a: '1', b: undefined, c: null, d: '', e: 'ok' })).toBe('?a=1&e=ok')
  })
  it('returns empty string for empty object', () => {
    expect(buildQuery({})).toBe('')
  })
})

function envelopeOk(data: unknown, status = 200) {
  return new Response(JSON.stringify({ code: 200, msg: 'ok', data }), { status })
}

describe('request envelope', () => {
  it('unwraps data when code === 200', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(envelopeOk({ hello: 1 }))
    await expect(request<{ hello: number }>('/x')).resolves.toEqual({ hello: 1 })
  })

  it('throws ApiError with code/msg when code !== 200', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ code: 400, msg: '参数错误' }), { status: 200 }),
    )
    const err = await request('/x').catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe(400)
    expect((err as ApiError).msg).toBe('参数错误')
  })

  it('throws parse-failure error for non-JSON response', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(new Response('not json', { status: 502 }))
    const err = await request('/x').catch((e) => e)
    expect((err as ApiError).msg).toBe('响应解析失败（HTTP 502）')
  })

  it('sends Bearer + X-Tenant-ID when logged in', async () => {
    session.saveSession({ access_token: 'tok', refresh_token: 'r' }, { id: '42', code: 'acme' })
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(envelopeOk(null))
    await request('/x')
    const headers = new Headers((spy.mock.calls[0]![1] as RequestInit).headers)
    expect(headers.get('Authorization')).toBe('Bearer tok')
    expect(headers.get('X-Tenant-ID')).toBe('42')
  })

  it('single-flights concurrent 401 refresh and retries once', async () => {
    session.saveSession({ access_token: 'old', refresh_token: 'r1' }, { id: '42', code: 'acme' })
    let refreshCalls = 0
    vi.spyOn(global, 'fetch').mockImplementation(async (_url, init) => {
      const url = _url as string
      if (url.includes('/auth/refresh')) {
        refreshCalls++
        return envelopeOk({ access_token: 'new', refresh_token: 'r2', expires_in: 3600 })
      }
      const auth = new Headers((init as RequestInit).headers).get('Authorization')
      if (auth === 'Bearer old') return new Response('', { status: 401 })
      return envelopeOk('done')
    })
    const [r1, r2] = await Promise.all([request<string>('/x'), request<string>('/y')])
    expect(r1).toBe('done')
    expect(r2).toBe('done')
    expect(refreshCalls).toBe(1)
    expect(session.getAccessToken()).toBe('new')
  })

  it('clears session and fires unauthorized when refresh fails', async () => {
    session.saveSession({ access_token: 'old', refresh_token: 'bad' }, { id: '42', code: 'acme' })
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/auth/refresh')) {
        return new Response(JSON.stringify({ code: 401, msg: 'expired' }), { status: 200 })
      }
      return new Response('', { status: 401 })
    })
    let fired = false
    const off = session.onUnauthorized(() => {
      fired = true
    })
    const err = await request('/x').catch((e) => e)
    off()
    expect(fired).toBe(true)
    expect(session.isLoggedIn()).toBe(false)
    expect((err as ApiError).status).toBe(401)
  })

  it('treats stale-version 401 as session switched (no refresh)', async () => {
    session.saveSession({ access_token: 'old', refresh_token: 'r' }, { id: '42', code: 'acme' })
    let refreshCalls = 0
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/auth/refresh')) {
        refreshCalls++
        return envelopeOk({ access_token: 'n', refresh_token: 'r2', expires_in: 1 })
      }
      return new Response('', { status: 401 })
    })
    const p = request('/x')
    // 模拟在请求飞行中会话被切换
    session.saveTokens({ access_token: 'other', refresh_token: 'other' })
    const err = await p.catch((e) => e)
    expect((err as ApiError).msg).toContain('会话已切换')
    expect(refreshCalls).toBe(0)
  })
})
```

Run: `npm run test:run -- src/test/http.test.ts` → Expected: PASS。

- [ ] **Step 6: 创建 `src/lib/tenant.ts`（租户入口解析）**

```ts
import { session } from '@/lib/session'

const RESERVED_SUBDOMAINS = new Set([
  'www', 'admin', 'app', 'api', 'console', 'portal', 'static', 'cdn', 'dev', 'test', 'staging',
])

export interface TenantEntry {
  code: string
  locked: boolean
}

export function resolveTenantEntry(search: string, pathname: string, hostname: string): TenantEntry {
  // 1. 查询参数 ?tenantCode=
  const q = new URLSearchParams(search.startsWith('?') ? search.slice(1) : search)
  const fromQuery = q.get('tenantCode')
  if (fromQuery) return { code: fromQuery, locked: true }

  // 2. 路径参数 /t/{code}/login
  const m = pathname.match(/^\/t\/([^/]+)\/login\/?$/)
  if (m) return { code: decodeURIComponent(m[1]!), locked: true }

  // 3. 子域首段（至少三级域名；保留段/localhost/IP 不取）
  const parts = hostname.split('.')
  const isIp = /^\d{1,3}(\.\d{1,3}){3}$/.test(hostname)
  if (!isIp && hostname !== 'localhost' && parts.length >= 3) {
    const first = parts[0]!
    if (!RESERVED_SUBDOMAINS.has(first)) return { code: first, locked: true }
  }

  // 4. localStorage devops.last_tenant（不锁定）
  const last = session.getLastTenant()
  if (last) return { code: last, locked: false }

  // 5. 兜底
  return { code: 'default', locked: false }
}
```

- [ ] **Step 7: 写租户解析测试并验证**

`src/test/tenant.test.ts`：

```ts
import { describe, it, expect, beforeEach } from 'vitest'
import { resolveTenantEntry } from '@/lib/tenant'

beforeEach(() => localStorage.clear())

describe('resolveTenantEntry', () => {
  it('query param wins and locks', () => {
    expect(resolveTenantEntry('?tenantCode=acme', '/login', 'admin.example.com')).toEqual({ code: 'acme', locked: true })
  })
  it('path /t/{code}/login wins over subdomain and locks', () => {
    expect(resolveTenantEntry('', '/t/foo/login', 'bar.admin.example.com')).toEqual({ code: 'foo', locked: true })
  })
  it('subdomain first segment, not reserved', () => {
    expect(resolveTenantEntry('', '/login', 'acme.admin.example.com')).toEqual({ code: 'acme', locked: true })
  })
  it('reserved subdomain not taken', () => {
    expect(resolveTenantEntry('', '/login', 'www.admin.example.com').code).toBe('default')
  })
  it('localhost and IP not taken', () => {
    expect(resolveTenantEntry('', '/login', 'localhost').code).toBe('default')
    expect(resolveTenantEntry('', '/login', '127.0.0.1').code).toBe('default')
  })
  it('falls back to last_tenant unlocked', () => {
    localStorage.setItem('devops.last_tenant', 'prev')
    expect(resolveTenantEntry('', '/login', 'localhost')).toEqual({ code: 'prev', locked: false })
  })
  it('falls back to default', () => {
    expect(resolveTenantEntry('', '/login', 'localhost')).toEqual({ code: 'default', locked: false })
  })
})
```

Run: `npm run test:run -- src/test/tenant.test.ts` → Expected: PASS。

- [ ] **Step 8: 创建 `src/lib/captcha.ts` 与 `src/lib/errors.ts`**

`src/lib/captcha.ts`：

```ts
import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

export function captchaEnabled(c: CaptchaChallenge | null | undefined): boolean {
  if (!c) return false
  return c.enabled !== false
}

export function buildCaptchaAnswer(c: CaptchaChallenge, a: CaptchaAnswer): Record<string, unknown> {
  const base: Record<string, unknown> = { captcha_id: c.id }
  switch (c.type) {
    case 'click':
      base.captcha_points = a.points ?? []
      break
    case 'slide':
    case 'dragdrop':
      base.captcha_x = a.x ?? 0
      base.captcha_y = a.y ?? 0
      break
    case 'rotate':
      base.captcha_angle = a.angle ?? 0
      break
  }
  return base
}
```

`src/lib/errors.ts`：

```ts
export function mapLoginError(raw: string): string {
  const msg = raw.toLowerCase()
  if (msg.includes('incorrect username or password')) return '账号或密码错误'
  if (msg.includes('user does not exist')) return '账号不存在或已被删除'
  if (msg.includes('tenant not found')) return '租户不存在，请重新选择'
  if (msg.includes('tenant is disabled')) return '租户已被禁用，请联系管理员'
  if (msg.includes('too many') || msg.includes('rate limit')) return '操作过于频繁，请稍后再试'
  return raw
}
```

- [ ] **Step 9: 写 captcha + errors 测试并验证**

`src/test/captcha.test.ts`：

```ts
import { describe, it, expect } from 'vitest'
import { captchaEnabled, buildCaptchaAnswer } from '@/lib/captcha'
import type { CaptchaChallenge } from '@/lib/types'

const base = (over: Partial<CaptchaChallenge>): CaptchaChallenge => ({
  id: 'c1', type: 'slide', image: 'img', width: 300, height: 150, ...over,
})

describe('captchaEnabled', () => {
  it('enabled=false skips', () => expect(captchaEnabled(base({ enabled: false }))).toBe(false))
  it('missing enabled treated as on', () => expect(captchaEnabled(base({}))).toBe(true))
  it('null challenge skipped', () => expect(captchaEnabled(null)).toBe(false))
})

describe('buildCaptchaAnswer', () => {
  it('click maps captcha_points', () => {
    expect(buildCaptchaAnswer(base({ type: 'click' }), { points: [{ x: 1, y: 2 }] }))
      .toEqual({ captcha_id: 'c1', captcha_points: [{ x: 1, y: 2 }] })
  })
  it('slide maps captcha_x/captcha_y', () => {
    expect(buildCaptchaAnswer(base({ type: 'slide' }), { x: 10, y: 20 }))
      .toEqual({ captcha_id: 'c1', captcha_x: 10, captcha_y: 20 })
  })
  it('dragdrop maps captcha_x/captcha_y', () => {
    expect(buildCaptchaAnswer(base({ type: 'dragdrop' }), { x: 5, y: 6 }))
      .toEqual({ captcha_id: 'c1', captcha_x: 5, captcha_y: 6 })
  })
  it('rotate maps captcha_angle (0 when no drag)', () => {
    expect(buildCaptchaAnswer(base({ type: 'rotate' }), {})).toEqual({ captcha_id: 'c1', captcha_angle: 0 })
    expect(buildCaptchaAnswer(base({ type: 'rotate' }), { angle: 90 })).toEqual({ captcha_id: 'c1', captcha_angle: 90 })
  })
})
```

`src/test/errors.test.ts`：

```ts
import { describe, it, expect } from 'vitest'
import { mapLoginError } from '@/lib/errors'

describe('mapLoginError', () => {
  it('maps the five known backend errors', () => {
    expect(mapLoginError('incorrect username or password')).toBe('账号或密码错误')
    expect(mapLoginError('user does not exist')).toBe('账号不存在或已被删除')
    expect(mapLoginError('tenant not found')).toBe('租户不存在，请重新选择')
    expect(mapLoginError('tenant is disabled')).toBe('租户已被禁用，请联系管理员')
    expect(mapLoginError('too many requests')).toBe('操作过于频繁，请稍后再试')
    expect(mapLoginError('rate limit exceeded')).toBe('操作过于频繁，请稍后再试')
  })
  it('falls back to raw message', () => {
    expect(mapLoginError('something else')).toBe('something else')
  })
})
```

Run: `npm run test:run -- src/test/captcha.test.ts src/test/errors.test.ts` → Expected: PASS。

- [ ] **Step 10: 创建 `src/lib/datetime.ts`**

```ts
export function formatDateTime(v: string | number | null | undefined): string {
  if (v === null || v === undefined || v === '') return '-'
  const d = new Date(typeof v === 'number' ? v : v.replace(' ', 'T'))
  if (Number.isNaN(d.getTime())) return String(v)
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`
}

export function formatDate(v: string | number | null | undefined): string {
  return formatDateTime(v).slice(0, 10)
}
```

`src/test/datetime.test.ts`：

```ts
import { describe, it, expect } from 'vitest'
import { formatDateTime, formatDate } from '@/lib/datetime'

describe('formatDateTime', () => {
  it('formats unix millis', () => {
    expect(formatDateTime(1730000000000)).toMatch(/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$/)
  })
  it('formats string dates', () => {
    expect(formatDateTime('2026-01-02 03:04:05')).toBe('2026-01-02 03:04:05')
  })
  it('returns dash for null/empty', () => {
    expect(formatDateTime(null)).toBe('-')
    expect(formatDateTime('')).toBe('-')
  })
})

describe('formatDate', () => {
  it('returns date part only', () => {
    expect(formatDate('2026-01-02 03:04:05')).toBe('2026-01-02')
  })
})
```

Run: `npm run test:run -- src/test/datetime.test.ts` → Expected: PASS。

- [ ] **Step 11: 创建 `src/lib/list.ts`（useListPage，竞态防护 + 末页回退）**

```ts
import { useCallback, useRef, useState } from 'react'
import type { PageResult } from '@/lib/types'

export interface ListPageConfig<T> {
  fetcher: (params: { page_num: number; page_size: number; [k: string]: unknown }) => Promise<PageResult<T>>
  defaultQuery?: Record<string, unknown>
  pageSize?: number
  exportPath?: string
}

export interface ListPageState<T> {
  query: Record<string, unknown>
  rows: T[]
  total: number
  page: number
  pageSize: number
  loading: boolean
  exporting: boolean
  selected: string[]
  setSelected: (ids: string[]) => void
  setQuery: (patch: Record<string, unknown>) => void
  load: () => Promise<void>
  search: () => Promise<void>
  reset: () => Promise<void>
  setPage: (p: number) => Promise<void>
  setPageSize: (s: number) => Promise<void>
  exportData: () => Promise<void>
  afterMutate: () => Promise<void>
}

export function useListPage<T extends { id: string }>(cfg: ListPageConfig<T>): ListPageState<T> {
  const [query, setQueryState] = useState<Record<string, unknown>>({ ...(cfg.defaultQuery ?? {}) })
  const [rows, setRows] = useState<T[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPageState] = useState(1)
  const [pageSize, setPageSizeState] = useState(cfg.pageSize ?? 10)
  const [loading, setLoading] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const seq = useRef(0)

  const load = useCallback(async () => {
    const mySeq = ++seq.current
    setLoading(true)
    try {
      const res = await cfg.fetcher({
        page_num: page,
        page_size: Math.min(pageSize, 1000),
        ...query,
      })
      if (mySeq !== seq.current) return // 竞态：过期响应丢弃
      setRows(res.list)
      setTotal(res.total)
      // 删除末页末行后回退一页
      if (res.list.length === 0 && res.total > 0 && page > 1) {
        setPageState(page - 1)
      }
    } finally {
      if (mySeq === seq.current) setLoading(false)
    }
  }, [cfg, page, pageSize, query])

  const search = useCallback(async () => {
    setPageState(1)
  }, [])

  const reset = useCallback(async () => {
    setQueryState({ ...(cfg.defaultQuery ?? {}) })
    setPageState(1)
  }, [cfg])

  const setPage = useCallback(async (p: number) => {
    setPageState(p)
  }, [])

  const setPageSize = useCallback(async (s: number) => {
    setPageSizeState(s)
    setPageState(1)
  }, [])

  const exportData = useCallback(async () => {
    if (!cfg.exportPath) return
    setExporting(true)
    try {
      const params = new URLSearchParams(
        Object.entries({ page_num: '1', page_size: '1000', ...query }).map(([k, v]) => [k, String(v)]),
      )
      const token = localStorage.getItem('devops.access_token')
      const tenantId = localStorage.getItem('devops.tenant_id')
      const res = await fetch(`${cfg.exportPath}?${params}`, {
        headers: {
          Accept: 'application/json',
          ...(token ? { Authorization: `Bearer ${token}` } : {}),
          ...(tenantId ? { 'X-Tenant-ID': tenantId } : {}),
        },
      })
      const blob = await res.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = 'export.xlsx'
      a.click()
      URL.revokeObjectURL(url)
    } finally {
      setExporting(false)
    }
  }, [cfg, query])

  const afterMutate = useCallback(async () => {
    setSelected([])
    await load()
  }, [load])

  return {
    query, rows, total, page, pageSize, loading, exporting, selected,
    setSelected,
    setQuery: (patch) => setQueryState((q) => ({ ...q, ...patch })),
    load, search, reset, setPage, setPageSize, exportData, afterMutate,
  }
}
```

- [ ] **Step 12: 写 useListPage 测试（竞态 + 上限 1000）并验证**

`src/test/list.test.tsx`：

```tsx
import { renderHook, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { useListPage } from '@/lib/list'
import type { PageResult } from '@/lib/types'

function page(list: Array<{ id: string }>, total: number, page: number, pageSize: number): PageResult<{ id: string }> {
  return { list, total, page_num: page, page_size: pageSize, pages: Math.ceil(total / pageSize) }
}

describe('useListPage', () => {
  it('loads page and exposes state', async () => {
    const fetcher = vi.fn().mockResolvedValue(page([{ id: '1' }], 1, 1, 10))
    const { result } = renderHook(() => useListPage({ fetcher }))
    await act(() => result.current.load())
    await waitFor(() => expect(result.current.rows).toEqual([{ id: '1' }]))
    expect(result.current.total).toBe(1)
    expect(fetcher).toHaveBeenCalledWith(expect.objectContaining({ page_num: 1, page_size: 10 }))
  })

  it('drops stale response on race', async () => {
    let resolveSecond: ((v: PageResult<{ id: string }>) => void) | null = null
    const fetcher = vi
      .fn()
      .mockImplementationOnce(
        () => new Promise((r) => setTimeout(() => r(page([{ id: 'slow' }]), 1, 1, 10)), 50),
      )
      .mockImplementationOnce(
        () =>
          new Promise<PageResult<{ id: string }>>((resolve) => {
            resolveSecond = resolve
          }),
      )
    const { result } = renderHook(() => useListPage({ fetcher }))
    const first = act(() => result.current.load())
    await act(() => result.current.setPage(2))
    const second = act(() => result.current.load())
    void first
    void second
    resolveSecond!(page([{ id: 'fast' }], 1, 2, 10))
    await waitFor(() => expect(result.current.rows).toEqual([{ id: 'fast' }]))
    // 第一页的迟到响应应被丢弃，rows 保持 fast
  })

  it('caps page_size at 1000', async () => {
    const fetcher = vi.fn().mockResolvedValue(page([], 0, 1, 10))
    const { result } = renderHook(() => useListPage({ fetcher, pageSize: 5000 }))
    await act(() => result.current.load())
    expect(fetcher).toHaveBeenCalledWith(expect.objectContaining({ page_size: 1000 }))
  })
})
```

Run: `npm run test:run -- src/test/list.test.tsx` → Expected: PASS。

- [ ] **Step 13: 创建 `src/lib/portal-session.ts`（portal 会话与单飞刷新）**

```ts
import type { PortalTokenPair } from '@/lib/types'

const KEY = 'portal_tokens'

export class PortalApiError extends Error {
  status: number
  msg: string
  constructor(status: number, msg: string) {
    super(msg)
    this.status = status
    this.msg = msg
  }
}

export const portalSession = {
  getTokens: (): PortalTokenPair | null => {
    try {
      const raw = localStorage.getItem(KEY)
      return raw ? (JSON.parse(raw) as PortalTokenPair) : null
    } catch {
      return null
    }
  },
  saveTokens: (t: PortalTokenPair): void => {
    localStorage.setItem(KEY, JSON.stringify(t))
  },
  clearTokens: (): void => {
    localStorage.removeItem(KEY)
  },
  isLoggedIn: (): boolean => localStorage.getItem(KEY) != null,
}

let refreshing: Promise<boolean> | null = null

function doRefresh(): Promise<boolean> {
  if (!refreshing) {
    refreshing = (async () => {
      const tokens = portalSession.getTokens()
      if (!tokens) return false
      const res = await fetch('/user/api/v1/auth/refresh', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
        body: JSON.stringify({ refresh_token: tokens.refresh_token }),
      })
      if (!res.ok) return false
      const envelope = (await res.json()) as { code: number; data?: PortalTokenPair }
      if (envelope.code !== 200 || !envelope.data) return false
      portalSession.saveTokens(envelope.data)
      return true
    })().finally(() => {
      refreshing = null
    })
  }
  return refreshing
}

interface PortalRequestOptions extends RequestInit {
  auth?: boolean
  query?: Record<string, unknown>
}

function withQuery(path: string, query: Record<string, unknown> | undefined): string {
  if (!query) return path
  const qs = Object.entries(query)
    .filter(([, v]) => v !== undefined && v !== null && v !== '')
    .map(([k, v]) => `${encodeURIComponent(k)}=${encodeURIComponent(String(v))}`)
    .join('&')
  return qs ? `${path}?${qs}` : path
}

export async function portalRequest<T>(path: string, opts: PortalRequestOptions = {}): Promise<T> {
  const auth = opts.auth !== false
  const headers = new Headers(opts.headers)
  headers.set('Accept', 'application/json')
  if (opts.body) headers.set('Content-Type', 'application/json')
  const tokens = portalSession.getTokens()
  if (auth && tokens) headers.set('Authorization', `Bearer ${tokens.access_token}`)

  const url = withQuery(path, opts.query)
  let res = await fetch(url, { ...opts, headers })
  if (res.status === 401 && auth) {
    const ok = await doRefresh()
    if (!ok) {
      portalSession.clearTokens()
      throw new PortalApiError(401, '登录已过期，请重新登录')
    }
    headers.set('Authorization', `Bearer ${portalSession.getTokens()!.access_token}`)
    res = await fetch(url, { ...opts, headers })
    if (res.status === 401) {
      portalSession.clearTokens()
      throw new PortalApiError(401, '登录已过期，请重新登录')
    }
  }

  let envelope: { code: number; msg?: string; data?: unknown }
  try {
    envelope = (await res.json()) as { code: number; msg?: string; data?: unknown }
  } catch {
    throw new PortalApiError(res.status, `响应解析失败（HTTP ${res.status}）`)
  }
  if (envelope.code !== 200) {
    throw new PortalApiError(res.status, envelope.msg || `请求失败（HTTP ${res.status}）`)
  }
  return envelope.data as T
}
```

- [ ] **Step 14: 写 portal-session 测试并验证**

`src/test/portal-session.test.ts`：

```ts
import { describe, it, expect, beforeEach, vi } from 'vitest'
import { portalRequest, PortalApiError, portalSession } from '@/lib/portal-session'

const tokens = () => ({ access_token: 'a', refresh_token: 'r', expires_in: 3600, issued_at: Date.now() })

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('portal tokens storage', () => {
  it('saves/loads/clears portal_tokens', () => {
    portalSession.saveTokens(tokens())
    expect(portalSession.getTokens()!.access_token).toBe('a')
    portalSession.clearTokens()
    expect(portalSession.getTokens()).toBeNull()
    expect(portalSession.isLoggedIn()).toBe(false)
  })
})

describe('portalRequest', () => {
  function ok(data: unknown) {
    return new Response(JSON.stringify({ code: 200, data }), { status: 200 })
  }
  it('unwraps data', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(ok({ p: 1 }))
    await expect(portalRequest<{ p: number }>('/x')).resolves.toEqual({ p: 1 })
  })
  it('single-flight refresh then retry once', async () => {
    portalSession.saveTokens(tokens())
    let refreshCalls = 0
    vi.spyOn(global, 'fetch').mockImplementation(async (url, init) => {
      if ((url as string).includes('/auth/refresh')) {
        refreshCalls++
        return ok({ ...tokens(), access_token: 'n', refresh_token: 'r2' })
      }
      const auth = new Headers((init as RequestInit).headers).get('Authorization')
      if (auth === 'Bearer a') return new Response('', { status: 401 })
      return ok('done')
    })
    const [r1, r2] = await Promise.all([portalRequest<string>('/x'), portalRequest<string>('/y')])
    expect(r1).toBe('done')
    expect(r2).toBe('done')
    expect(refreshCalls).toBe(1)
  })
  it('clears tokens and throws 401 PortalApiError when refresh fails', async () => {
    portalSession.saveTokens(tokens())
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/auth/refresh')) {
        return new Response(JSON.stringify({ code: 401, msg: 'expired' }), { status: 200 })
      }
      return new Response('', { status: 401 })
    })
    const err = await portalRequest('/x').catch((e) => e)
    expect(err).toBeInstanceOf(PortalApiError)
    expect((err as PortalApiError).msg).toBe('登录已过期，请重新登录')
    expect(portalSession.isLoggedIn()).toBe(false)
  })
})
```

Run: `npm run test:run -- src/test/portal-session.test.ts` → Expected: PASS。

- [ ] **Step 15: 创建 `src/lib/toast.ts`、`src/lib/confirm.tsx`、`src/lib/permission.tsx`，并给 appearance 追加 hook**

`src/lib/toast.ts`：

```ts
import { toast as sonnerToast } from 'sonner'

export function toast(msg: string): void {
  sonnerToast(msg)
}

export function toastSuccess(msg: string): void {
  sonnerToast.success(msg)
}

export function toastError(msg: string): void {
  sonnerToast.error(msg)
}
```

`src/lib/confirm.tsx`：

```tsx
import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

interface ConfirmOptions {
  title: string
  description?: string
  confirmText?: string
  danger?: boolean
}

interface ConfirmContextValue {
  confirm: (opts: ConfirmOptions) => Promise<boolean>
}

const ConfirmContext = createContext<ConfirmContextValue | null>(null)

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<(ConfirmOptions & { resolve: (v: boolean) => void }) | null>(null)

  const confirm = useCallback((opts: ConfirmOptions) => {
    return new Promise<boolean>((resolve) => {
      setState({ ...opts, resolve })
    })
  }, [])

  const settle = (v: boolean) => {
    state?.resolve(v)
    setState(null)
  }

  return (
    <ConfirmContext.Provider value={{ confirm }}>
      {children}
      <AlertDialog open={state != null} onOpenChange={(o) => { if (!o) settle(false) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{state?.title}</AlertDialogTitle>
            {state?.description && (
              <AlertDialogDescription>{state.description}</AlertDialogDescription>
            )}
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>取消</AlertDialogCancel>
            <AlertDialogAction
              className={state?.danger ? 'bg-destructive text-white' : undefined}
              onClick={() => settle(true)}
            >
              {state?.confirmText ?? '确定'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </ConfirmContext.Provider>
  )
}

export function useConfirm(): (opts: ConfirmOptions) => Promise<boolean> {
  const ctx = useContext(ConfirmContext)
  if (!ctx) throw new Error('useConfirm must be used within ConfirmProvider')
  return ctx.confirm
}
```

`src/lib/permission.tsx`：

```tsx
import { createContext, useContext, useMemo, type ReactNode } from 'react'

interface PermissionsContextValue {
  permissions: string[]
  has: (p: string) => boolean
}

// 权限码精确匹配，不支持通配符
const PermissionsContext = createContext<PermissionsContextValue>({
  permissions: [],
  has: () => false,
})

export function PermissionsProvider({
  permissions,
  children,
}: {
  permissions: string[]
  children: ReactNode
}) {
  const value = useMemo<PermissionsContextValue>(
    () => ({
      permissions,
      has: (p: string) => permissions.includes(p),
    }),
    [permissions],
  )
  return <PermissionsContext.Provider value={value}>{children}</PermissionsContext.Provider>
}

export function usePermissions(): PermissionsContextValue {
  return useContext(PermissionsContext)
}
```

`src/lib/appearance.ts` 末尾追加：

```ts
import { useCallback, useEffect, useState } from 'react'

export function useAppearance(): { appearance: Appearance; update: (patch: Partial<Appearance>) => void } {
  const [appearance, setAppearance] = useState<Appearance>(() => loadAppearance())

  useEffect(() => {
    return watchSystemTheme((a) => {
      setAppearance(a)
      applyAppearance(a)
    })
  }, [])

  const update = useCallback((patch: Partial<Appearance>) => {
    setAppearance((prev) => {
      const next = { ...prev, ...patch }
      saveAppearance(next)
      applyAppearance(next)
      return next
    })
  }, [])

  return { appearance, update }
}
```

- [ ] **Step 16: 写 permission / confirm / appearance hook 测试并验证**

`src/test/permission.test.tsx`：

```tsx
import { render, screen } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { PermissionsProvider, usePermissions } from '@/lib/permission'

function Probe({ code }: { code: string }) {
  const { has } = usePermissions()
  return <div>{has(code) ? 'granted' : 'denied'}</div>
}

describe('permissions (exact match)', () => {
  it('matches exact permission code', () => {
    render(
      <PermissionsProvider permissions={['system:user:create']}>
        <Probe code="system:user:create" />
      </PermissionsProvider>,
    )
    expect(screen.getByText('granted')).toBeInTheDocument()
  })
  it('denies non-exact codes (no wildcard)', () => {
    render(
      <PermissionsProvider permissions={['system:user:create']}>
        <Probe code="system:user:*" />
        <Probe code="system:user:create:extra" />
      </PermissionsProvider>,
    )
    expect(screen.getAllByText('denied')).toHaveLength(2)
  })
})
```

`src/test/confirm.test.tsx`：

```tsx
import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { ConfirmProvider, useConfirm } from '@/lib/confirm'

function Probe() {
  const confirm = useConfirm()
  return (
    <button
      onClick={async () => {
        const ok = await confirm({ title: '确认删除？', danger: true })
        document.body.dataset.result = String(ok)
      }}
    >
      ask
    </button>
  )
}

describe('useConfirm', () => {
  it('resolves true on confirm', async () => {
    render(
      <ConfirmProvider>
        <Probe />
      </ConfirmProvider>,
    )
    fireEvent.click(screen.getByText('ask'))
    expect(screen.getByText('确认删除？')).toBeInTheDocument()
    fireEvent.click(screen.getByText('确定'))
    expect(document.body.dataset.result).toBe('true')
  })
  it('resolves false on cancel', async () => {
    render(
      <ConfirmProvider>
        <Probe />
      </ConfirmProvider>,
    )
    fireEvent.click(screen.getByText('ask'))
    fireEvent.click(screen.getByText('取消'))
    expect(document.body.dataset.result).toBe('false')
  })
})
```

`src/test/appearance.test.tsx`：

```tsx
import { renderHook, act } from '@testing-library/react'
import { describe, it, expect } from 'vitest'
import { useAppearance, loadAppearance } from '@/lib/appearance'

describe('useAppearance', () => {
  it('persists patch to localStorage and applies to html', () => {
    const { result } = renderHook(() => useAppearance())
    act(() => result.current.update({ theme: 'dark', railCollapsed: true }))
    expect(result.current.appearance.theme).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.classList.contains('rail-collapsed')).toBe(true)
    expect(loadAppearance().theme).toBe('dark')
  })
})
```

Run: `npm run test:run -- src/test/permission.test.tsx src/test/confirm.test.tsx src/test/appearance.test.tsx` → Expected: PASS。

- [ ] **Step 17: 创建 `src/lib/options.ts`（下拉选项缓存）**

```ts
import { useCallback, useEffect, useState } from 'react'

const cache = new Map<string, Promise<unknown>>()

export function loadOptions<T>(key: string, loader: () => Promise<T[]>): Promise<T[]> {
  let p = cache.get(key) as Promise<T[]> | undefined
  if (!p) {
    p = loader()
    cache.set(key, p)
  }
  return p
}

export function invalidateOptions(key?: string): void {
  if (key) cache.delete(key)
  else cache.clear()
}

export function useOptions<T>(key: string, loader: () => Promise<T[]>): {
  data: T[]
  loading: boolean
  reload: () => void
} {
  const [data, setData] = useState<T[]>([])
  const [loading, setLoading] = useState(true)
  const [tick, setTick] = useState(0)

  useEffect(() => {
    let alive = true
    setLoading(true)
    loadOptions<T>(key, loader)
      .then((d) => {
        if (alive) setData(d)
      })
      .finally(() => {
        if (alive) setLoading(false)
      })
    return () => {
      alive = false
    }
  }, [key, tick])

  const reload = useCallback(() => {
    cache.delete(key)
    setTick((t) => t + 1)
  }, [key])

  return { data, loading, reload }
}
```

- [ ] **Step 18: 全量校验并提交**

```bash
npm run test:run
npm run typecheck
npm run build
```

Expected: 全部测试 PASS；typecheck 零错误；build 成功。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): framework-agnostic contract layer with vitest contract tests"
```

### Task 3: Admin API 层 + 会话 store + 动态路由 + 守卫 + 最小页面

**Files:**
- Create: `devops-frontend/src/api/admin/auth.ts`
- Create: `devops-frontend/src/api/admin/system.ts`
- Create: `devops-frontend/src/api/portal/auth.ts`
- Create: `devops-frontend/src/store/session.tsx`
- Create: `devops-frontend/src/router/routes.tsx`
- Create: `devops-frontend/src/router/guards.tsx`
- Modify: `devops-frontend/src/App.tsx`（替换 Task 1 最小根为真实路由编排）
- Modify: `devops-frontend/src/main.tsx`（挂 ConfirmProvider/Toaster）
- Create: `devops-frontend/src/views/Login.tsx`（最小可登录版，Task 4 完整化）
- Create: `devops-frontend/src/views/Dashboard.tsx`、`src/views/NoAccess.tsx`、`src/views/NotFound.tsx`（Task 6 完整化 Dashboard）
- Create: `src/views/system/{Users,Roles,Menus,Depts,Posts,Tenants,Dict,Configs}.tsx`、`src/views/monitor/{LoginLogs,OperationLogs}.tsx`（最小占位组件，Task 8-12 逐个替换为完整实现）
- Create: `src/components/layout/AppShell.tsx`（最小版，Task 5 完整化）
- Test: `src/test/api-system.test.ts`、`src/test/routes.test.tsx`、`src/test/guards.test.tsx`

**Interfaces:**
- Consumes: `request<T>()`、`session`、`PortalApiError`/`portalRequest`、`portalSession`（Task 2）；shadcn 组件；`ConfirmProvider`、`applyAppearance`。
- Produces:
  - `adminAuth`（`@/api/admin/auth`）：`getCaptcha(tenantCode?: string): Promise<CaptchaChallenge>`、`login(body: { username: string; password: string; tenant_code: string } & Record<string, unknown>): Promise<LoginResult>`、`refresh(refresh_token: string): Promise<TokenPair>`、`logout(): Promise<void>`、`getInfo(): Promise<AuthInfo>`、`getRouters(): Promise<RouterNode[]>`、`updateProfile(body: { nickname?; email?; mobile?; gender?; avatar? }): Promise<void>`、`updatePassword(body: { old_password; new_password }): Promise<void>`
  - `adminSystem` + `parseDeptIDsJson(s?: string): string[]` + `encodeDeptIDsJson(ids: string[]): string` + 各 Write 类型（`UserWrite`/`RoleWrite`/`MenuWrite`/`DeptWrite`/`PostWrite`/`TenantWrite`/`DictTypeWrite`/`DictItemWrite`/`ConfigWrite`，全部 snake_case 字段）——完整实现见下方代码。
  - `portalAuth`（`@/api/portal/auth`）：`getCaptcha(): Promise<CaptchaChallenge>`、`login(body): Promise<PortalTokenPair>`、`register(body): Promise<PortalTokenPair>`、`refresh(refresh_token): Promise<PortalTokenPair>`、`logout(): Promise<void>`、`getProfile(): Promise<PublicProfile>`、`updateProfile(body: { nickname?; email?; mobile?; gender? }): Promise<void>`、`updatePassword(body: { old_password; new_password }): Promise<void>`
  - `SessionProvider`/`useSession(): { authInfo: AuthInfo | null; routes: RouterNode[]; loading: boolean; bootstrap(): Promise<void>; reload(): Promise<void>; logout(): Promise<void> }`
  - `COMPONENT_MAP: Record<string, ComponentType>`（11 键）；`buildDynamicRoutes(nodes: RouterNode[]): DynamicRouteDef[]`（`{ path: string; element: ReactElement }`）；`flattenMenus(nodes: RouterNode[]): RouterNode[]`（扁平可见菜单，供 Sidebar/CommandPalette）
  - `RequireAuth`/`RequireGuest`、`installUnauthorizedHandler(): () => void`（401 → 清会话 → 跳 `/login?tenantCode={code}&redirect={path}`）

- [ ] **Step 1: 创建 `src/api/admin/auth.ts`**

```ts
import { request } from '@/lib/http'
import type { AuthInfo, CaptchaChallenge, LoginResult, RouterNode, TokenPair } from '@/lib/types'

const BASE = '/system/api/v1/auth'

export const adminAuth = {
  getCaptcha(tenantCode?: string): Promise<CaptchaChallenge> {
    return request<CaptchaChallenge>(`${BASE}/captcha`, { auth: false, query: { tenant_code: tenantCode } })
  },
  login(
    body: { username: string; password: string; tenant_code: string } & Record<string, unknown>,
  ): Promise<LoginResult> {
    return request<LoginResult>(`${BASE}/login`, { method: 'POST', auth: false, body: JSON.stringify(body) })
  },
  refresh(refresh_token: string): Promise<TokenPair> {
    return request<TokenPair>(`${BASE}/refresh`, {
      method: 'POST',
      auth: false,
      body: JSON.stringify({ refresh_token }),
    })
  },
  logout(): Promise<void> {
    return request<void>(`${BASE}/logout`, { method: 'POST' })
  },
  getInfo(): Promise<AuthInfo> {
    return request<AuthInfo>(`${BASE}/info`)
  },
  getRouters(): Promise<RouterNode[]> {
    return request<RouterNode[]>(`${BASE}/routers`)
  },
  updateProfile(body: {
    nickname?: string
    email?: string
    mobile?: string
    gender?: string
    avatar?: string
  }): Promise<void> {
    return request<void>(`${BASE}/profile`, { method: 'PUT', body: JSON.stringify(body) })
  },
  updatePassword(body: { old_password: string; new_password: string }): Promise<void> {
    return request<void>(`${BASE}/profile/password`, { method: 'PUT', body: JSON.stringify(body) })
  },
}
```

- [ ] **Step 2: 创建 `src/api/admin/system.ts`（全部模块，写接口 snake_case）**

```ts
import { request } from '@/lib/http'
import type {
  DictItem, DictType, LoginLog, OperationLog, PageResult,
  SystemConfig, SystemDept, SystemMenu, SystemPost, SystemRole, SystemTenant, SystemUser,
} from '@/lib/types'

// 角色 data_scope_dept_ids：JSON 数组字符串（非逗号分隔）
export function parseDeptIDsJson(s?: string): string[] {
  if (!s) return []
  try {
    const v = JSON.parse(s)
    return Array.isArray(v) ? v.map(String) : []
  } catch {
    return []
  }
}

export function encodeDeptIDsJson(ids: string[]): string {
  return JSON.stringify(ids)
}

export type ListQuery = { page_num?: number; page_size?: number } & Record<string, unknown>

export interface UserWrite {
  username: string
  nickname: string
  dept_id: string
  post_ids: string[]
  role_ids: string[]
  email?: string
  mobile?: string
  gender?: string
  avatar?: string
  remark?: string
  status?: number
}

export interface RoleWrite {
  name: string
  code: string
  sort: number
  data_scope: number
  data_scope_dept_ids?: string
  status: number
  remark?: string
}

export interface MenuWrite {
  name: string
  permission?: string
  type: number
  sort: number
  parent_id: string
  path?: string
  icon?: string
  component?: string
  component_name?: string
  status: number
  visible: boolean
  keep_alive?: boolean
  always_show?: boolean
}

export interface DeptWrite {
  name: string
  parent_id: string
  sort: number
  leader_user_id?: string
  phone?: string
  email?: string
  status: number
}

export interface PostWrite {
  code: string
  name: string
  sort: number
  status: number
  remark?: string
}

export interface TenantWrite {
  name: string
  code: string
  contact_name: string
  contact_mobile: string
  expire_time?: string
  remark?: string
  admin_username?: string
  admin_password?: string
}

export interface DictTypeWrite {
  name: string
  type: string
  status: number
  remark?: string
}

export interface DictItemWrite {
  dict_type: string
  label: string
  value: string
  sort: number
}

export interface ConfigWrite {
  config_key: string
  config_name: string
  config_value: string
  type: number
  visible: boolean
  remark?: string
}

export const adminSystem = {
  // ── 用户 ──
  listUsers: (q: ListQuery) => request<PageResult<SystemUser>>('/system/api/v1/system/users', { query: q }),
  createUser: (u: UserWrite) =>
    request<void>('/system/api/v1/system/users', { method: 'POST', body: JSON.stringify(u) }),
  updateUser: (id: string, u: Partial<UserWrite>) =>
    request<void>(`/system/api/v1/system/users/${id}`, { method: 'PUT', body: JSON.stringify(u) }),
  updateUserPassword: (id: string, p: { password: string }) =>
    request<void>(`/system/api/v1/system/users/${id}/password`, { method: 'PUT', body: JSON.stringify(p) }),
  deleteUsers: (ids: string[]) =>
    request<void>('/system/api/v1/system/users', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 角色 ──
  listRoles: (q: ListQuery) => request<PageResult<SystemRole>>('/system/api/v1/system/roles', { query: q }),
  listAllRoles: () => request<SystemRole[]>('/system/api/v1/system/roles/all'),
  createRole: (r: RoleWrite) =>
    request<void>('/system/api/v1/system/roles', { method: 'POST', body: JSON.stringify(r) }),
  updateRole: (id: string, r: Partial<RoleWrite>) =>
    request<void>(`/system/api/v1/system/roles/${id}`, { method: 'PUT', body: JSON.stringify(r) }),
  assignRoleMenus: (id: string, menu_ids: string[]) =>
    request<void>(`/system/api/v1/system/roles/${id}/menus`, { method: 'PUT', body: JSON.stringify({ menu_ids }) }),
  deleteRoles: (ids: string[]) =>
    request<void>('/system/api/v1/system/roles', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 菜单 ──
  listMenus: () => request<SystemMenu[]>('/system/api/v1/system/menus'),
  createMenu: (m: MenuWrite) =>
    request<void>('/system/api/v1/system/menus', { method: 'POST', body: JSON.stringify(m) }),
  updateMenu: (id: string, m: Partial<MenuWrite>) =>
    request<void>(`/system/api/v1/system/menus/${id}`, { method: 'PUT', body: JSON.stringify(m) }),
  deleteMenu: (id: string) => request<void>(`/system/api/v1/system/menus/${id}`, { method: 'DELETE' }),

  // ── 部门 ──
  listDepts: () => request<SystemDept[]>('/system/api/v1/system/depts'),
  createDept: (d: DeptWrite) =>
    request<void>('/system/api/v1/system/depts', { method: 'POST', body: JSON.stringify(d) }),
  updateDept: (id: string, d: Partial<DeptWrite>) =>
    request<void>(`/system/api/v1/system/depts/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
  deleteDept: (id: string) => request<void>(`/system/api/v1/system/depts/${id}`, { method: 'DELETE' }),

  // ── 岗位 ──
  listPosts: (q: ListQuery) => request<PageResult<SystemPost>>('/system/api/v1/system/posts', { query: q }),
  listAllPosts: () => request<SystemPost[]>('/system/api/v1/system/posts/all'),
  createPost: (p: PostWrite) =>
    request<void>('/system/api/v1/system/posts', { method: 'POST', body: JSON.stringify(p) }),
  updatePost: (id: string, p: Partial<PostWrite>) =>
    request<void>(`/system/api/v1/system/posts/${id}`, { method: 'PUT', body: JSON.stringify(p) }),
  deletePosts: (ids: string[]) =>
    request<void>('/system/api/v1/system/posts', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 租户 ──
  listTenants: (q: ListQuery) => request<PageResult<SystemTenant>>('/system/api/v1/platform/tenants', { query: q }),
  createTenant: (t: TenantWrite) =>
    request<void>('/system/api/v1/platform/tenants', { method: 'POST', body: JSON.stringify(t) }),
  updateTenant: (id: string, t: Partial<TenantWrite>) =>
    request<void>(`/system/api/v1/platform/tenants/${id}`, { method: 'PUT', body: JSON.stringify(t) }),
  deleteTenants: (ids: string[]) =>
    request<void>('/system/api/v1/platform/tenants', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 字典 ──
  listDictTypes: (q: ListQuery) => request<PageResult<DictType>>('/system/api/v1/system/dict/types', { query: q }),
  createDictType: (d: DictTypeWrite) =>
    request<void>('/system/api/v1/system/dict/types', { method: 'POST', body: JSON.stringify(d) }),
  updateDictType: (id: string, d: Partial<DictTypeWrite>) =>
    request<void>(`/system/api/v1/system/dict/types/${id}`, { method: 'PUT', body: JSON.stringify(d) }),
  deleteDictTypes: (ids: string[]) =>
    request<void>('/system/api/v1/system/dict/types', { method: 'DELETE', body: JSON.stringify({ ids }) }),
  listDictItems: (dictType: string) =>
    request<DictItem[]>('/system/api/v1/system/dict/items', { query: { dict_type: dictType } }),
  createDictItem: (i: DictItemWrite) =>
    request<void>('/system/api/v1/system/dict/items', { method: 'POST', body: JSON.stringify(i) }),
  updateDictItem: (id: string, i: Partial<DictItemWrite>) =>
    request<void>(`/system/api/v1/system/dict/items/${id}`, { method: 'PUT', body: JSON.stringify(i) }),
  deleteDictItems: (ids: string[]) =>
    request<void>('/system/api/v1/system/dict/items', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 参数配置 ──
  listConfigs: (q: ListQuery) => request<PageResult<SystemConfig>>('/system/api/v1/system/configs', { query: q }),
  createConfig: (c: ConfigWrite) =>
    request<void>('/system/api/v1/system/configs', { method: 'POST', body: JSON.stringify(c) }),
  updateConfig: (id: string, c: Partial<ConfigWrite>) =>
    request<void>(`/system/api/v1/system/configs/${id}`, { method: 'PUT', body: JSON.stringify(c) }),
  deleteConfigs: (ids: string[]) =>
    request<void>('/system/api/v1/system/configs', { method: 'DELETE', body: JSON.stringify({ ids }) }),

  // ── 日志 ──
  listLoginLogs: (q: ListQuery) => request<PageResult<LoginLog>>('/system/api/v1/system/loginlogs', { query: q }),
  cleanLoginLogs: () => request<void>('/system/api/v1/system/loginlogs', { method: 'DELETE' }),
  removeLoginLogs: (ids: string[]) =>
    request<void>('/system/api/v1/system/loginlogs/batch', { method: 'DELETE', body: JSON.stringify({ ids }) }),
  listOperationLogs: (q: ListQuery) =>
    request<PageResult<OperationLog>>('/system/api/v1/system/operationlogs', { query: q }),
  cleanOperationLogs: () => request<void>('/system/api/v1/system/operationlogs', { method: 'DELETE' }),
  removeOperationLogs: (ids: string[]) =>
    request<void>('/system/api/v1/system/operationlogs/batch', { method: 'DELETE', body: JSON.stringify({ ids }) }),
}
```

- [ ] **Step 3: 创建 `src/api/portal/auth.ts`**

```ts
import { portalRequest } from '@/lib/portal-session'
import type { CaptchaChallenge, PortalTokenPair, PublicProfile } from '@/lib/types'

const BASE = '/user/api/v1'

export const portalAuth = {
  getCaptcha(): Promise<CaptchaChallenge> {
    return portalRequest<CaptchaChallenge>(`${BASE}/auth/captcha`, { auth: false })
  },
  login(body: { username: string; password: string } & Record<string, unknown>): Promise<PortalTokenPair> {
    return portalRequest<PortalTokenPair>(`${BASE}/auth/login`, {
      method: 'POST', auth: false, body: JSON.stringify(body),
    })
  },
  register(body: {
    username: string
    nickname: string
    email: string
    password: string
  } & Record<string, unknown>): Promise<PortalTokenPair> {
    return portalRequest<PortalTokenPair>(`${BASE}/auth/register`, {
      method: 'POST', auth: false, body: JSON.stringify(body),
    })
  },
  refresh(refresh_token: string): Promise<PortalTokenPair> {
    return portalRequest<PortalTokenPair>(`${BASE}/auth/refresh`, {
      method: 'POST', auth: false, body: JSON.stringify({ refresh_token }),
    })
  },
  logout(): Promise<void> {
    return portalRequest<void>(`${BASE}/auth/logout`, { method: 'POST' })
  },
  getProfile(): Promise<PublicProfile> {
    return portalRequest<PublicProfile>(`${BASE}/profile`)
  },
  updateProfile(body: { nickname?: string; email?: string; mobile?: string; gender?: string }): Promise<void> {
    return portalRequest<void>(`${BASE}/profile`, { method: 'PUT', body: JSON.stringify(body) })
  },
  updatePassword(body: { old_password: string; new_password: string }): Promise<void> {
    return portalRequest<void>(`${BASE}/profile/password`, { method: 'PUT', body: JSON.stringify(body) })
  },
}
```

- [ ] **Step 4: 写 admin API 写接口命名契约测试（红→绿）**

`src/test/api-system.test.ts`：

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { adminSystem, encodeDeptIDsJson, parseDeptIDsJson } from '@/api/admin/system'

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

function ok() {
  return new Response(JSON.stringify({ code: 200, data: null }), { status: 200 })
}

describe('admin system API write mapping', () => {
  it('createUser sends snake_case body', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.createUser({
      username: 'u', nickname: 'n', dept_id: '1', post_ids: ['2'], role_ids: ['3'], status: 1,
    })
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body).toMatchObject({ username: 'u', dept_id: '1', post_ids: ['2'], role_ids: ['3'] })
  })

  it('createConfig sends snake_case keys', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.createConfig({
      config_key: 'k', config_name: 'n', config_value: 'v', type: 2, visible: true,
    })
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body).toMatchObject({ config_key: 'k', config_name: 'n', config_value: 'v', type: 2, visible: true })
    expect(body.configKey).toBeUndefined()
  })

  it('updateUserPassword targets /password with snake_case', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.updateUserPassword('9', { password: 'p' })
    expect(spy.mock.calls[0]![0]).toBe('/system/api/v1/system/users/9/password')
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body).toEqual({ password: 'p' })
  })
})

describe('dept IDs JSON codec', () => {
  it('encode/parse round-trip as JSON array string', () => {
    const s = encodeDeptIDsJson(['1', '22'])
    expect(s).toBe('["1","22"]')
    expect(parseDeptIDsJson(s)).toEqual(['1', '22'])
  })
  it('parse handles empty/invalid', () => {
    expect(parseDeptIDsJson('')).toEqual([])
    expect(parseDeptIDsJson(undefined)).toEqual([])
    expect(parseDeptIDsJson('not json')).toEqual([])
  })
})

describe('ID handling stays string', () => {
  it('deleteUsers passes ids verbatim', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.deleteUsers(['1234567890123456789'])
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body.ids).toEqual(['1234567890123456789'])
  })
})
```

Run: `npm run test:run -- src/test/api-system.test.ts` → Expected: PASS。

- [ ] **Step 5: 创建 `src/store/session.tsx`**

```tsx
import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { adminAuth } from '@/api/admin/auth'
import { session } from '@/lib/session'
import { toastError } from '@/lib/toast'
import type { AuthInfo, RouterNode } from '@/lib/types'

interface SessionContextValue {
  authInfo: AuthInfo | null
  routes: RouterNode[]
  loading: boolean
  bootstrap: () => Promise<void>
  reload: () => Promise<void>
  logout: () => Promise<void>
}

const SessionContext = createContext<SessionContextValue | null>(null)

export function SessionProvider({ children }: { children: ReactNode }) {
  const [authInfo, setAuthInfo] = useState<AuthInfo | null>(null)
  const [routes, setRoutes] = useState<RouterNode[]>([])
  const [loading, setLoading] = useState(true)

  const bootstrap = useCallback(async () => {
    setLoading(true)
    try {
      if (!session.isLoggedIn()) {
        setAuthInfo(null)
        setRoutes([])
        return
      }
      const [info, routerNodes] = await Promise.all([adminAuth.getInfo(), adminAuth.getRouters()])
      setAuthInfo(info)
      setRoutes(routerNodes)
    } catch (e) {
      toastError((e as Error).message || '会话信息加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void bootstrap()
  }, [bootstrap])

  const reload = useCallback(async () => {
    await bootstrap()
  }, [bootstrap])

  const logout = useCallback(async () => {
    try {
      await adminAuth.logout()
    } catch {
      // 后端登出失败不影响本地清理
    }
    session.clearSession()
    setAuthInfo(null)
    setRoutes([])
  }, [])

  return (
    <SessionContext.Provider value={{ authInfo, routes, loading, bootstrap, reload, logout }}>
      {children}
    </SessionContext.Provider>
  )
}

export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext)
  if (!ctx) throw new Error('useSession must be used within SessionProvider')
  return ctx
}
```

- [ ] **Step 6: 创建 `src/router/routes.tsx`（COMPONENT_MAP + 动态路由构建）**

```tsx
import { createElement, type ComponentType, type ReactElement } from 'react'
import type { RouterNode } from '@/lib/types'
import Dashboard from '@/views/Dashboard'
import Users from '@/views/system/Users'
import Roles from '@/views/system/Roles'
import Menus from '@/views/system/Menus'
import Depts from '@/views/system/Depts'
import Posts from '@/views/system/Posts'
import Tenants from '@/views/system/Tenants'
import Dict from '@/views/system/Dict'
import Configs from '@/views/system/Configs'
import LoginLogs from '@/views/monitor/LoginLogs'
import OperationLogs from '@/views/monitor/OperationLogs'

// component 键 → 页面组件（spec §4.4 的 11 键）
export const COMPONENT_MAP: Record<string, ComponentType> = {
  'dashboard/index': Dashboard,
  'system/user/index': Users,
  'system/role/index': Roles,
  'system/menu/index': Menus,
  'system/dept/index': Depts,
  'system/post/index': Posts,
  'system/dict/index': Dict,
  'system/config/index': Configs,
  'system/loginlog/index': LoginLogs,
  'system/operationlog/index': OperationLogs,
  'platform/tenant/index': Tenants,
}

export interface DynamicRouteDef {
  path: string
  element: ReactElement
}

export function buildDynamicRoutes(nodes: RouterNode[]): DynamicRouteDef[] {
  const defs: DynamicRouteDef[] = []
  const walk = (list: RouterNode[]) => {
    for (const n of list) {
      if (n.component && COMPONENT_MAP[n.component]) {
        const Comp = COMPONENT_MAP[n.component]!
        defs.push({ path: n.path, element: createElement(Comp) })
      } else if (n.component) {
        // 未注册的 component 键：跳过并告警，不虚构菜单
        console.warn(`[router] unregistered component key: ${n.component} (path=${n.path})`)
      }
      if (n.children?.length) walk(n.children)
    }
  }
  walk(nodes)
  return defs
}

export function flattenMenus(nodes: RouterNode[]): RouterNode[] {
  const out: RouterNode[] = []
  const walk = (list: RouterNode[]) => {
    for (const n of list) {
      out.push(n)
      if (n.children?.length) walk(n.children)
    }
  }
  walk(nodes)
  return out.filter((n) => n.visible)
}
```

- [ ] **Step 7: 写路由映射测试（先创建 Step 8 的最小页面与 AppShell，再跑测试）**

`src/test/routes.test.tsx`：

```tsx
import { describe, it, expect, vi } from 'vitest'
import { COMPONENT_MAP, buildDynamicRoutes, flattenMenus } from '@/router/routes'
import type { RouterNode } from '@/lib/types'

const node = (over: Partial<RouterNode>): RouterNode => ({
  id: '1', name: 'n', path: '/x', component: 'system/user/index', sort: 1, visible: true, ...over,
})

describe('COMPONENT_MAP', () => {
  it('covers all 11 spec keys', () => {
    expect(Object.keys(COMPONENT_MAP).sort()).toEqual([
      'dashboard/index',
      'system/user/index',
      'system/role/index',
      'system/menu/index',
      'system/dept/index',
      'system/post/index',
      'system/dict/index',
      'system/config/index',
      'system/loginlog/index',
      'system/operationlog/index',
      'platform/tenant/index',
    ].sort())
  })
})

describe('buildDynamicRoutes', () => {
  it('maps registered components to route defs', () => {
    const defs = buildDynamicRoutes([node({ path: '/system/user', component: 'system/user/index' })])
    expect(defs).toHaveLength(1)
    expect(defs[0]!.path).toBe('/system/user')
  })

  it('skips unregistered component keys with console.warn', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const defs = buildDynamicRoutes([node({ path: '/mystery', component: 'mystery/index' })])
    expect(defs).toHaveLength(0)
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('mystery/index'))
    warn.mockRestore()
  })

  it('walks children', () => {
    const defs = buildDynamicRoutes([
      node({
        path: '/system',
        component: '', // 目录节点无页面
        children: [node({ path: '/system/user', component: 'system/user/index' })],
      }),
    ])
    expect(defs.map((d) => d.path)).toEqual(['/system/user'])
  })
})

describe('flattenMenus', () => {
  it('filters invisible nodes and flattens tree', () => {
    const menus = flattenMenus([
      node({
        name: 'parent',
        path: '/p',
        children: [
          node({ id: '2', name: 'child', path: '/p/c' }),
          node({ id: '3', name: 'hidden', path: '/p/h', visible: false }),
        ],
      }),
    ])
    expect(menus.map((m) => m.name)).toEqual(['parent', 'child'])
  })
})
```

- [ ] **Step 8: 创建最小页面与最小 AppShell（后续任务逐个替换）**

`src/components/layout/AppShell.tsx`（Task 5 替换）：

```tsx
import { Outlet } from 'react-router-dom'

export default function AppShell() {
  return (
    <div className="min-h-screen">
      <main className="p-4">
        <Outlet />
      </main>
    </div>
  )
}
```

`src/views/Dashboard.tsx`（Task 6 替换）：

```tsx
export default function Dashboard() {
  return <div className="p-4">工作台</div>
}
```

`src/views/NoAccess.tsx`：

```tsx
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function NoAccess() {
  return (
    <Card className="mx-auto mt-16 max-w-md">
      <CardHeader><CardTitle>403</CardTitle></CardHeader>
      <CardContent className="text-muted-foreground">没有权限访问该页面</CardContent>
    </Card>
  )
}
```

`src/views/NotFound.tsx`：

```tsx
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function NotFound() {
  return (
    <Card className="mx-auto mt-16 max-w-md">
      <CardHeader><CardTitle>404</CardTitle></CardHeader>
      <CardContent className="text-muted-foreground">页面不存在</CardContent>
    </Card>
  )
}
```

最小 CRUD 占位（Task 8-12 逐个替换为完整实现）：对 `Users/Roles/Menus/Depts/Posts/Tenants/Dict/Configs`（在 `src/views/system/`）与 `LoginLogs/OperationLogs`（在 `src/views/monitor/`）各创建，模式统一：

```tsx
// src/views/system/Users.tsx（Task 8 替换为完整实现）
export default function Users() {
  return <div className="p-4">用户管理</div>
}
```

其余文件同样模式，文案分别为：角色管理/菜单管理/部门管理/岗位管理/租户管理/字典管理/参数配置（system），登录日志/操作日志（monitor）。

`src/views/Login.tsx`（最小可登录版，Task 4 完整化验证码与租户锁定 UI）：

```tsx
import { useState, type FormEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { adminAuth } from '@/api/admin/auth'
import { session } from '@/lib/session'
import { mapLoginError } from '@/lib/errors'
import { resolveTenantEntry } from '@/lib/tenant'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

export default function Login() {
  const [search] = useSearchParams()
  const navigate = useNavigate()
  const entry = resolveTenantEntry(window.location.search, window.location.pathname, window.location.hostname)
  const tenantCode = search.get('tenantCode') ?? entry.code
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const redirect = search.get('redirect') || '/'

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    setBusy(true)
    try {
      const result = await adminAuth.login({ username, password, tenant_code: tenantCode })
      session.saveSession(result, result.tenant)
      navigate(redirect, { replace: true })
    } catch (err) {
      setError(mapLoginError((err as Error).message))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>登录 DevOps 管理平台</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} className="space-y-4">
            {error && <p className="text-destructive text-sm">{error}</p>}
            <Field>
              <FieldLabel>账号</FieldLabel>
              <Input value={username} onChange={(e) => setUsername(e.target.value)} required />
            </Field>
            <Field>
              <FieldLabel>密码</FieldLabel>
              <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
            </Field>
            <Button type="submit" className="w-full" disabled={busy}>登录</Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
```

- [ ] **Step 9: 创建 `src/router/guards.tsx`**

```tsx
import { type ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { session } from '@/lib/session'
import { useSession } from '@/store/session'

function currentTenantCode(): string {
  return (
    session.getTenantCode() ||
    new URLSearchParams(window.location.search).get('tenantCode') ||
    'default'
  )
}

export function RequireAuth({ children }: { children: ReactNode }) {
  const { loading } = useSession()
  const location = useLocation()
  if (loading) return <div className="p-8 text-sm text-muted-foreground">加载中…</div>
  if (!session.isLoggedIn()) {
    const params = new URLSearchParams({
      tenantCode: currentTenantCode(),
      redirect: location.pathname + location.search,
    })
    return <Navigate to={`/login?${params.toString()}`} replace />
  }
  return <>{children}</>
}

export function RequireGuest({ children }: { children: ReactNode }) {
  const { loading } = useSession()
  if (loading) return <div className="p-8 text-sm text-muted-foreground">加载中…</div>
  if (session.isLoggedIn()) return <Navigate to="/" replace />
  return <>{children}</>
}

// 401 未授权处理：清理会话并跳登录（携带 tenantCode 与 redirect）
export function installUnauthorizedHandler(): () => void {
  return session.onUnauthorized(() => {
    const { pathname, search } = window.location
    if (pathname.startsWith('/login')) return
    const params = new URLSearchParams({
      tenantCode: currentTenantCode(),
      redirect: pathname + search,
    })
    window.location.assign(`/login?${params.toString()}`)
  })
}
```

- [ ] **Step 10: 编排 `src/App.tsx` 与 `src/main.tsx` 最终版**

`src/App.tsx`：

```tsx
import { useEffect } from 'react'
import { BrowserRouter, useRoutes } from 'react-router-dom'
import { COMPONENT_MAP, buildDynamicRoutes } from '@/router/routes'
import { RequireAuth, RequireGuest, installUnauthorizedHandler } from '@/router/guards'
import { SessionProvider, useSession } from '@/store/session'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import AppShell from '@/components/layout/AppShell'
import Login from '@/views/Login'
import NotFound from '@/views/NotFound'
import Dashboard from '@/views/Dashboard'
import type { ReactNode } from 'react'

function RouterContent() {
  const { routes } = useSession()
  const dynamic = buildDynamicRoutes(routes)
  const routesConfig = [
    { path: '/login', element: <RequireGuest><Login /></RequireGuest> },
    { path: '/t/:code/login', element: <RequireGuest><Login /></RequireGuest> },
    {
      path: '/',
      element: (
        <RequireAuth>
          <AppShell />
        </RequireAuth>
      ),
      children: [
        // 工作台静态置于首位（spec §4.4：dashboard 静态提供，后端 /auth/routers 不下发）
        { index: true, element: <Dashboard /> },
        ...dynamic.map((d) => ({
          path: d.path.replace(/^\//, ''),
          element: d.element,
        })),
        { path: '*', element: <NotFound /> },
      ],
    },
    { path: '/403', element: <NotFound /> },
    { path: '*', element: <NotFound /> },
  ]
  return useRoutes(routesConfig)
}

function PermissionsGate({ children }: { children: ReactNode }) {
  const { authInfo } = useSession()
  return (
    <PermissionsProvider permissions={authInfo?.permissions ?? []}>
      {children}
    </PermissionsProvider>
  )
}

export default function App() {
  useEffect(() => installUnauthorizedHandler(), [])
  return (
    <BrowserRouter>
      <SessionProvider>
        <PermissionsGate>
          <ConfirmProvider>
            <RouterContent />
          </ConfirmProvider>
        </PermissionsGate>
      </SessionProvider>
    </BrowserRouter>
  )
}
```

注意：403 页 `NoAccess` 由动态路由承载业务路径权限不足时展示——实现上在任何动态路由组件内捕获 403 ApiError 时渲染 `<NoAccess />`（各 CRUD 页面 `catch` 中调用 `toastError` 并对 `code === 403` 设置页面级错误态渲染 `<NoAccess />`，由 Task 8 样板统一落地）。`/403` 直达路由改为：`import NoAccess from '@/views/NoAccess'` 并把 `{ path: '/403', element: <NotFound /> }` 替换为 `{ path: '/403', element: <NoAccess /> }`。

`src/main.tsx` 最终版：

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Toaster } from '@/components/ui/sonner'
import { applyAppearance } from '@/lib/appearance'
import App from './App'

applyAppearance()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
    <Toaster position="top-center" richColors />
  </StrictMode>,
)
```

- [ ] **Step 11: 写守卫测试（红→绿）**

`src/test/guards.test.tsx`：

```tsx
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Routes, Route } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { RequireAuth } from '@/router/guards'
import { session } from '@/lib/session'
import { SessionProvider } from '@/store/session'

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <SessionProvider>
        <Routes>
          <Route
            path="/protected"
            element={
              <RequireAuth>
                <div>secret</div>
              </RequireAuth>
            }
          />
          <Route path="/login" element={<div>login page</div>} />
        </Routes>
      </SessionProvider>
    </MemoryRouter>,
  )
}

describe('RequireAuth', () => {
  it('renders children when logged in', async () => {
    session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
    renderAt('/protected')
    await waitFor(() => expect(screen.getByText('secret')).toBeInTheDocument())
  })

  it('redirects to /login with tenantCode+redirect when not logged in', async () => {
    renderAt('/protected')
    await waitFor(() => expect(screen.getByText('login page')).toBeInTheDocument())
  })
})
```

Run: `npm run test:run -- src/test/routes.test.tsx src/test/guards.test.tsx` → Expected: PASS。

- [ ] **Step 12: 全量校验并提交**

```bash
npm run test:run
npm run typecheck
npm run build
```

Expected: 全部 PASS；typecheck 零错误；build 成功。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): admin API layer, session store, dynamic router, auth guards and minimal pages"
```

### Task 4: Admin 登录页完整化（验证码弹框 + 租户解析锁定 + 错误映射）

**Files:**
- Create: `devops-frontend/src/components/business/CaptchaDialog.tsx`
- Modify: `devops-frontend/src/views/Login.tsx`（替换 Task 3 的最小版本）
- Test: `src/test/captcha-dialog.test.tsx`、`src/test/login-page.test.tsx`

**Interfaces:**
- Consumes: `adminAuth.getCaptcha/login`（Task 3）、`captchaEnabled`/`buildCaptchaAnswer`（Task 2）、`resolveTenantEntry`（Task 2）、`mapLoginError`（Task 2）、`session.saveSession/setLastUser`、shadcn Dialog/Button/Input/Field、`toastError`。
- Produces: `CaptchaDialog({ open, challenge, onAnswer, onRefresh, onCancel })` — 展示四题型交互，作答经 `onAnswer(answer: CaptchaAnswer)` 交回；`onRefresh()` 拉新题（挑战一次性消费）。登录页完整行为：未登录可访问、已登录跳工作台、点击「登录」才拉题弹框、验证码校验失败弹框内提示并自动换题、凭据错误关闭弹框在登录卡显示映射文案、登录成功 `saveSession` + `setLastUser` + 跳 redirect。

- [ ] **Step 1: 创建 `src/components/business/CaptchaDialog.tsx`（四题型交互）**

```tsx
import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle,
} from '@/components/ui/dialog'
import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

interface CaptchaDialogProps {
  open: boolean
  challenge: CaptchaChallenge | null
  onAnswer: (answer: CaptchaAnswer) => void
  onRefresh: () => void
  onCancel: () => void
}

// 四题型统一交互组件：click 点击取点 / slide+dragdrop 拖拽拼图 / rotate 拖拽旋转
export function CaptchaDialog({ open, challenge, onAnswer, onRefresh, onCancel }: CaptchaDialogProps) {
  const [points, setPoints] = useState<Array<{ x: number; y: number }>>([])
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const [angle, setAngle] = useState(0)
  const dragging = useRef(false)
  const dragStart = useRef({ x: 0, y: 0 })

  useEffect(() => {
    if (open) {
      setPoints([])
      setOffset({ x: 0, y: 0 })
      setAngle(0)
    }
  }, [open, challenge?.id])

  const submit = useCallback(() => {
    if (!challenge) return
    switch (challenge.type) {
      case 'click':
        onAnswer({ points })
        break
      case 'slide':
      case 'dragdrop': {
        const bx = challenge.thumb_x ?? 0
        const by = challenge.thumb_y ?? 0
        onAnswer({ x: bx + offset.x, y: by + offset.y })
        break
      }
      case 'rotate':
        onAnswer({ angle })
        break
    }
  }, [challenge, points, offset, angle, onAnswer])

  if (!challenge) return null

  const imgW = challenge.width
  const imgH = challenge.height

  const onImageClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (challenge.type !== 'click') return
    const rect = e.currentTarget.getBoundingClientRect()
    const scaleX = imgW / rect.width
    const scaleY = imgH / rect.height
    setPoints((p) => [
      ...p,
      { x: Math.round((e.clientX - rect.left) * scaleX), y: Math.round((e.clientY - rect.top) * scaleY) },
    ])
  }

  const onDragStart = (e: React.MouseEvent | React.TouchEvent) => {
    dragging.current = true
    const pt = 'touches' in e ? e.touches[0]! : e
    dragStart.current = { x: pt.clientX - offset.x, y: pt.clientY - offset.y }
  }
  const onDragMove = (e: React.MouseEvent | React.TouchEvent) => {
    if (!dragging.current) return
    const pt = 'touches' in e ? (e.touches[0] ?? e.changedTouches[0]) : e
    if (!pt) return
    if (challenge.type === 'rotate') {
      const dy = pt.clientY - dragStart.current.y
      dragStart.current = { x: pt.clientX, y: pt.clientY }
      setAngle(((offset.y + dy) % 360 + 360) % 360)
    } else {
      setOffset({
        x: Math.max(0, Math.min(pt.clientX - dragStart.current.x, imgW - (challenge.thumb_width ?? 0))),
        y: Math.max(0, pt.clientY - dragStart.current.y),
      })
    }
  }
  const onDragEnd = () => {
    dragging.current = false
  }

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onCancel() }}>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>安全验证</DialogTitle>
          <DialogDescription>
            {challenge.type === 'click' && `请依次点击：${challenge.text ?? ''}`}
            {challenge.type === 'slide' && '拖动滑块完成拼图'}
            {challenge.type === 'dragdrop' && '将拼图块拖到正确位置'}
            {challenge.type === 'rotate' && '拖动下方滑块旋转图片至正向'}
          </DialogDescription>
        </DialogHeader>
        <div className="relative select-none" style={{ width: imgW, maxWidth: '100%', aspectRatio: `${imgW} / ${imgH}` }}>
          <img src={challenge.image} alt="captcha" className="w-full rounded-md" onClick={onImageClick} />
          {(challenge.type === 'slide' || challenge.type === 'dragdrop') && challenge.thumb && (
            <img
              src={challenge.thumb}
              alt="puzzle piece"
              className="absolute cursor-grab"
              style={{
                left: (challenge.thumb_x ?? 0) + offset.x,
                top: (challenge.thumb_y ?? 0) + offset.y,
                width: challenge.thumb_width,
                height: challenge.thumb_height,
              }}
              onMouseDown={onDragStart}
              onMouseMove={onDragMove}
              onMouseUp={onDragEnd}
              onMouseLeave={onDragEnd}
              onTouchStart={onDragStart}
              onTouchMove={onDragMove}
              onTouchEnd={onDragEnd}
            />
          )}
          {challenge.type === 'click' &&
            points.map((p, i) => (
              <span
                key={i}
                className="bg-primary pointer-events-none absolute h-4 w-4 -translate-x-1/2 -translate-y-1/2 rounded-full opacity-70"
                style={{ left: `${(p.x / imgW) * 100}%`, top: `${(p.y / imgH) * 100}%` }}
              />
            ))}
        </div>
        {challenge.type === 'rotate' && challenge.thumb && (
          <div className="flex flex-col items-center gap-2">
            <img
              src={challenge.thumb}
              alt="rotate target"
              style={{ width: challenge.thumb_size ?? 120, transform: `rotate(${angle}deg)` }}
              className="rounded-md"
            />
            <input
              type="range"
              min={0}
              max={360}
              value={angle}
              onChange={(e) => setAngle(Number(e.target.value))}
              className="w-full"
              aria-label="旋转角度"
            />
          </div>
        )}
        <div className="flex gap-2">
          <Button variant="outline" className="flex-1" onClick={onRefresh}>换一题</Button>
          <Button className="flex-1" onClick={submit}>提交</Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 2: 写验证码弹框四题型作答映射测试（红→绿）**

`src/test/captcha-dialog.test.tsx`：

```tsx
import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { CaptchaDialog } from '@/components/business/CaptchaDialog'
import { buildCaptchaAnswer } from '@/lib/captcha'
import type { CaptchaChallenge } from '@/lib/types'

const challenge = (over: Partial<CaptchaChallenge>): CaptchaChallenge => ({
  id: 'c1', type: 'slide', image: 'data:image/png;base64,x', width: 300, height: 150,
  thumb: 'data:image/png;base64,t', thumb_x: 10, thumb_y: 20, thumb_width: 60, thumb_height: 60,
  ...over,
})

function Harness({ challenge, onAnswer }: { challenge: CaptchaChallenge; onAnswer: (a: unknown) => void }) {
  return (
    <CaptchaDialog
      open
      challenge={challenge}
      onAnswer={(a) => onAnswer(a)}
      onRefresh={() => {}}
      onCancel={() => {}}
    />
  )
}

describe('CaptchaDialog interactions', () => {
  it('click type collects points and submits captcha_points', () => {
    const onAnswer = vi.fn()
    const c = challenge({ type: 'click', text: 'circle,star' })
    render(<Harness challenge={c} onAnswer={onAnswer} />)
    expect(screen.getByText(/circle,star/)).toBeInTheDocument()
    fireEvent.click(screen.getByAltText('captcha'))
    fireEvent.click(screen.getByText('提交'))
    expect(onAnswer).toHaveBeenCalledWith({
      points: [expect.objectContaining({ x: expect.any(Number), y: expect.any(Number) })],
    })
  })

  it('slide type submits captcha_x/captcha_y = base + offset via buildCaptchaAnswer', () => {
    const onAnswer = vi.fn()
    render(<Harness challenge={challenge({ type: 'slide' })} onAnswer={onAnswer} />)
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 10, clientY: 20 })
    fireEvent.mouseMove(piece, { clientX: 40, clientY: 20 })
    fireEvent.mouseUp(piece)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { x: number; y: number }
    // 作答经 buildCaptchaAnswer 映射为后端契约字段
    const mapped = buildCaptchaAnswer(challenge({ type: 'slide' }), a)
    expect(mapped).toEqual({ captcha_id: 'c1', captcha_x: a.x, captcha_y: a.y })
  })

  it('rotate type defaults angle 0 when untouched', () => {
    const onAnswer = vi.fn()
    render(<Harness challenge={challenge({ type: 'rotate', thumb_size: 100 })} onAnswer={onAnswer} />)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { angle: number }
    expect(buildCaptchaAnswer(challenge({ type: 'rotate' }), a)).toEqual({ captcha_id: 'c1', captcha_angle: 0 })
  })

  it('dragdrop type submits captcha_x/captcha_y', () => {
    const onAnswer = vi.fn()
    render(<Harness challenge={challenge({ type: 'dragdrop' })} onAnswer={onAnswer} />)
    const piece = screen.getByAltText('puzzle piece')
    fireEvent.mouseDown(piece, { clientX: 0, clientY: 0 })
    fireEvent.mouseMove(piece, { clientX: 15, clientY: 15 })
    fireEvent.mouseUp(piece)
    fireEvent.click(screen.getByText('提交'))
    const a = onAnswer.mock.calls[0]![0] as { x: number; y: number }
    expect(buildCaptchaAnswer(challenge({ type: 'dragdrop' }), a)).toMatchObject({ captcha_id: 'c1' })
  })
})
```

Run: `npm run test:run -- src/test/captcha-dialog.test.tsx` → Expected: PASS。

- [ ] **Step 3: 完整化 `src/views/Login.tsx`（拉题弹框 + 锁定 + 映射 + redirect）**

```tsx
import { useCallback, useState, type FormEvent } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { adminAuth } from '@/api/admin/auth'
import { session } from '@/lib/session'
import { buildCaptchaAnswer, captchaEnabled } from '@/lib/captcha'
import { mapLoginError } from '@/lib/errors'
import { resolveTenantEntry } from '@/lib/tenant'
import { toastError } from '@/lib/toast'
import { CaptchaDialog } from '@/components/business/CaptchaDialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

export default function Login() {
  const [search] = useSearchParams()
  const navigate = useNavigate()
  const entry = resolveTenantEntry(window.location.search, window.location.pathname, window.location.hostname)
  const tenantCode = search.get('tenantCode') ?? entry.code
  // 前三项命中即锁定：不渲染租户输入，仅账号 + 密码
  const locked = entry.locked || !!search.get('tenantCode')
  const [manualTenant, setManualTenant] = useState(tenantCode)

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [challenge, setChallenge] = useState<CaptchaChallenge | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)

  const redirect = search.get('redirect') || '/'
  const effectiveTenant = locked ? tenantCode : manualTenant || 'default'

  const doLogin = useCallback(
    async (answer?: CaptchaAnswer) => {
      setError(null)
      setBusy(true)
      try {
        const body: Record<string, unknown> = {
          username,
          password,
          tenant_code: effectiveTenant,
        }
        if (challenge && answer) {
          Object.assign(body, buildCaptchaAnswer(challenge, answer))
        }
        const result = await adminAuth.login(body as { username: string; password: string; tenant_code: string })
        session.saveSession(result, result.tenant)
        session.setLastUser(username)
        navigate(redirect, { replace: true })
      } catch (err) {
        const msg = (err as Error).message
        // 400 = 参数/验证码错误：留在弹框内提示并自动换题（挑战一次性消费）
        if (challenge && answer && (err as { code?: number }).code === 400) {
          toastError('验证码校验失败，已为你更换新题')
          const next = await adminAuth.getCaptcha(effectiveTenant)
          setChallenge(next)
          setDialogOpen(true)
        } else {
          // 其他失败（凭据错误、限流等）关闭弹框，错误显示在登录卡
          setDialogOpen(false)
          setChallenge(null)
          setError(mapLoginError(msg))
        }
      } finally {
        setBusy(false)
      }
    },
    [username, password, effectiveTenant, challenge, redirect, navigate],
  )

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    try {
      const c = await adminAuth.getCaptcha(effectiveTenant)
      if (captchaEnabled(c)) {
        setChallenge(c)
        setDialogOpen(true)
        return
      }
      await doLogin()
    } catch (err) {
      setError(mapLoginError((err as Error).message))
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>登录 DevOps 管理平台</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={onSubmit} className="space-y-4">
            {error && <p className="text-destructive text-sm">{error}</p>}
            {!locked && (
              <Field>
                <FieldLabel>租户编码</FieldLabel>
                <Input value={manualTenant} onChange={(e) => setManualTenant(e.target.value)} />
              </Field>
            )}
            <Field>
              <FieldLabel>账号</FieldLabel>
              <Input value={username} onChange={(e) => setUsername(e.target.value)} required />
            </Field>
            <Field>
              <FieldLabel>密码</FieldLabel>
              <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
            </Field>
            <Button type="submit" className="w-full" disabled={busy}>登录</Button>
          </form>
        </CardContent>
      </Card>
      <CaptchaDialog
        open={dialogOpen}
        challenge={challenge}
        onAnswer={(a) => void doLogin(a)}
        onRefresh={() => {
          void adminAuth.getCaptcha(effectiveTenant).then(setChallenge)
        }}
        onCancel={() => {
          setDialogOpen(false)
          setChallenge(null)
        }}
      />
    </div>
  )
}
```

- [ ] **Step 4: 写登录页行为测试（红→绿）**

`src/test/login-page.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Login from '@/views/Login'
import { session } from '@/lib/session'

function ok(data: unknown) {
  return new Response(JSON.stringify({ code: 200, data }), { status: 200 })
}
function bizError(code: number, msg: string) {
  return new Response(JSON.stringify({ code, msg }), { status: 200 })
}

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

function renderLogin(initial = '/login') {
  return render(
    <MemoryRouter initialEntries={[initial]}>
      <Login />
    </MemoryRouter>,
  )
}

describe('Login page', () => {
  it('renders tenant input when unlocked (no query/path/subdomain/last)', () => {
    renderLogin('/login')
    expect(screen.getByLabelText('租户编码')).toBeInTheDocument()
    expect(screen.getByText('登录 DevOps 管理平台')).toBeInTheDocument()
  })

  it('locks tenant input when ?tenantCode= present', () => {
    renderLogin('/login?tenantCode=acme')
    expect(screen.queryByLabelText('租户编码')).not.toBeInTheDocument()
  })

  it('shows mapped credential error on login card (closes captcha dialog)', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/captcha'))
        return ok({ id: 'c', type: 'slide', image: 'i', width: 1, height: 1, enabled: false })
      if ((url as string).includes('/login')) return bizError(401, 'incorrect username or password')
      throw new Error('unexpected')
    })
    renderLogin('/login')
    fireEvent.change(screen.getByLabelText('账号'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByText('登录'))
    await waitFor(() => expect(screen.getByText('账号或密码错误')).toBeInTheDocument())
    expect(screen.queryByText('安全验证')).not.toBeInTheDocument()
  })

  it('skips captcha when enabled=false and logs in directly', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/captcha'))
        return ok({ id: 'c', type: 'slide', image: 'i', width: 1, height: 1, enabled: false })
      if ((url as string).includes('/login'))
        return ok({
          access_token: 'a', refresh_token: 'r', expires_in: 1,
          tenant: { id: '1', name: 'ACME', code: 'acme' },
        })
      throw new Error('unexpected')
    })
    renderLogin('/login?tenantCode=acme')
    fireEvent.change(screen.getByLabelText('账号'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByText('登录'))
    await waitFor(() => expect(session.isLoggedIn()).toBe(true))
    expect(localStorage.getItem('devops.last_tenant')).toBe('acme')
    expect(screen.queryByText('安全验证')).not.toBeInTheDocument()
  })

  it('pulls captcha on submit and shows dialog when enabled', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/captcha'))
        return ok({ id: 'c', type: 'click', image: 'i', width: 100, height: 50, text: 'circle' })
      throw new Error('unexpected')
    })
    renderLogin('/login')
    fireEvent.change(screen.getByLabelText('账号'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByText('登录'))
    await waitFor(() => expect(screen.getByText('安全验证')).toBeInTheDocument())
  })
})
```

Run: `npm run test:run -- src/test/login-page.test.tsx` → Expected: PASS。

- [ ] **Step 5: 全量校验并提交**

```bash
npm run test:run
npm run typecheck
```

Expected: 全部 PASS，零错误。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): admin login page with captcha dialog, tenant lock and error mapping"
```

### Task 5: Admin AppShell 布局（Sidebar / Topbar / CommandPalette / 用户菜单 / 外观 / 密码 / 移动抽屉）

**Files:**
- Create: `src/components/layout/Sidebar.tsx`
- Create: `src/components/layout/Topbar.tsx`
- Create: `src/components/layout/CommandPalette.tsx`
- Create: `src/components/layout/AppearanceDialog.tsx`
- Create: `src/components/layout/PasswordDialog.tsx`
- Create: `src/components/layout/MobileDrawer.tsx`
- Modify: `src/components/layout/AppShell.tsx`（替换 Task 3 最小版）
- Test: `src/test/layout.test.tsx`

**Interfaces:**
- Consumes: `useSession()`（Task 3：`routes`/`authInfo`/`logout`）、`flattenMenus`（Task 3）、`useAppearance`（Task 2）、`adminAuth.updatePassword`（Task 3）、`useConfirm`、`toastSuccess`/`toastError`、shadcn `Sheet`/`DropdownMenu`/`Breadcrumb`/`Command`/`Dialog`、lucide-react 图标。
- Produces: `AppShell`（`Sidebar + Topbar + <Outlet />`，glass 风格容器）；`CommandPalette({ open, onOpenChange })`；`AppearanceDialog({ open, onOpenChange })`；`PasswordDialog({ open, onOpenChange })`；`Sidebar({ collapsed }: { collapsed?: boolean })`；`MobileDrawer({ open, onOpenChange })`。菜单渲染规则：工作台（`dashboard/index`）置于首位；分组（有 children 的目录）可折叠。

- [ ] **Step 1: 创建 `src/components/layout/Sidebar.tsx`**

```tsx
import { useState } from 'react'
import { NavLink } from 'react-router-dom'
import { ChevronRight } from 'lucide-react'
import { useSession } from '@/store/session'
import type { RouterNode } from '@/lib/types'

function isLeaf(n: RouterNode): boolean {
  return !n.children?.length
}

// 工作台置于首位：component === dashboard/index 的节点排最前
function sortWithDashboardFirst(nodes: RouterNode[]): RouterNode[] {
  return [...nodes].sort((a, b) => {
    const ad = a.component === 'dashboard/index' ? -1 : 0
    const bd = b.component === 'dashboard/index' ? -1 : 0
    if (ad !== bd) return ad - bd
    return a.sort - b.sort
  })
}

function MenuLink({ node, railWide }: { node: RouterNode; railWide: boolean }) {
  return (
    <NavLink
      to={node.path}
      className={({ isActive }) =>
        `flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors hover:bg-accent ${isActive ? 'bg-accent text-accent-foreground font-medium' : 'text-foreground/80'}`
      }
    >
      <span className="h-4 w-4 shrink-0" aria-hidden />
      <span className={railWide ? 'rail-wide' : ''}>{node.name}</span>
    </NavLink>
  )
}

function MenuGroup({ node, railWide }: { node: RouterNode; railWide: boolean }) {
  const [open, setOpen] = useState(true)
  return (
    <div className="space-y-1">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground/80 hover:bg-accent"
      >
        <ChevronRight className={`h-4 w-4 transition-transform ${open ? 'rotate-90' : ''}`} />
        <span className={railWide ? 'rail-wide' : ''}>{node.name}</span>
      </button>
      {open && (
        <div className="ml-4 space-y-1 border-l border-border pl-2">
          {node.children!.filter((c) => c.visible).map((c) =>
            isLeaf(c)
              ? <MenuLink key={c.id} node={c} railWide={railWide} />
              : <MenuGroup key={c.id} node={c} railWide={railWide} />,
          )}
        </div>
      )}
    </div>
  )
}

export function Sidebar({ collapsed = false }: { collapsed?: boolean }) {
  const { routes } = useSession()
  const visible = sortWithDashboardFirst(routes.filter((n) => n.visible))
  return (
    <aside className="flex h-full flex-col gap-1 p-3">
      {visible.map((n) =>
        isLeaf(n)
          ? <MenuLink key={n.id} node={n} railWide={!collapsed} />
          : <MenuGroup key={n.id} node={n} railWide={!collapsed} />,
      )}
    </aside>
  )
}
```

- [ ] **Step 2: 创建 `src/components/layout/CommandPalette.tsx`**

```tsx
import { useNavigate } from 'react-router-dom'
import {
  CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList,
} from '@/components/ui/command'
import { flattenMenus } from '@/router/routes'
import { useSession } from '@/store/session'

export function CommandPalette({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const { routes } = useSession()
  const navigate = useNavigate()
  const menus = flattenMenus(routes).filter((n) => !n.children?.length)

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <CommandInput placeholder="搜索菜单…" />
      <CommandList>
        <CommandEmpty>无匹配菜单</CommandEmpty>
        <CommandGroup heading="菜单">
          {menus.map((m) => (
            <CommandItem
              key={m.id}
              onSelect={() => {
                onOpenChange(false)
                navigate(m.path)
              }}
            >
              {m.name}
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
```

- [ ] **Step 3: 创建 `src/components/layout/AppearanceDialog.tsx`**

```tsx
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useAppearance } from '@/lib/appearance'
import type { Density, Theme } from '@/lib/types'

export function AppearanceDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const { appearance, update } = useAppearance()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-sm">
        <DialogHeader><DialogTitle>外观设置</DialogTitle></DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>主题</Label>
            <Select value={appearance.theme} onValueChange={(v) => update({ theme: v as Theme })}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="light">浅色</SelectItem>
                <SelectItem value="dark">深色</SelectItem>
                <SelectItem value="auto">跟随系统</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>密度</Label>
            <Select value={appearance.density} onValueChange={(v) => update({ density: v as Density })}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="comfy">宽松</SelectItem>
                <SelectItem value="regular">标准</SelectItem>
                <SelectItem value="compact">紧凑</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-center justify-between">
            <Label>侧边栏折叠</Label>
            <Switch checked={appearance.railCollapsed} onCheckedChange={(v) => update({ railCollapsed: v })} />
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 4: 创建 `src/components/layout/PasswordDialog.tsx`**

```tsx
import { useState, type FormEvent } from 'react'
import { adminAuth } from '@/api/admin/auth'
import { toastError, toastSuccess } from '@/lib/toast'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

export function PasswordDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const [oldPassword, setOld] = useState('')
  const [newPassword, setNew] = useState('')
  const [confirm, setConfirm] = useState('')

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (newPassword !== confirm) {
      toastError('两次输入的密码不一致')
      return
    }
    try {
      await adminAuth.updatePassword({ old_password: oldPassword, new_password: newPassword })
      toastSuccess('密码修改成功')
      onOpenChange(false)
      setOld('')
      setNew('')
      setConfirm('')
    } catch (err) {
      toastError((err as Error).message)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-sm">
        <DialogHeader><DialogTitle>修改密码</DialogTitle></DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4">
          <Field><FieldLabel>原密码</FieldLabel><Input type="password" value={oldPassword} onChange={(e) => setOld(e.target.value)} required /></Field>
          <Field><FieldLabel>新密码</FieldLabel><Input type="password" value={newPassword} onChange={(e) => setNew(e.target.value)} required /></Field>
          <Field><FieldLabel>确认新密码</FieldLabel><Input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} required /></Field>
          <Button type="submit" className="w-full">提交</Button>
        </form>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 5: 创建 `src/components/layout/MobileDrawer.tsx`**

```tsx
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { Sidebar } from '@/components/layout/Sidebar'

export function MobileDrawer({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="left" className="w-64 p-0">
        <SheetHeader className="px-4 py-3">
          <SheetTitle>导航菜单</SheetTitle>
        </SheetHeader>
        <Sidebar collapsed={false} />
      </SheetContent>
    </Sheet>
  )
}
```

- [ ] **Step 6: 创建 `src/components/layout/Topbar.tsx`**

```tsx
import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { Command, Menu as MenuIcon, UserRound } from 'lucide-react'
import {
  Breadcrumb, BreadcrumbItem, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { flattenMenus } from '@/router/routes'
import { useSession } from '@/store/session'
import { CommandPalette } from '@/components/layout/CommandPalette'
import { AppearanceDialog } from '@/components/layout/AppearanceDialog'
import { PasswordDialog } from '@/components/layout/PasswordDialog'
import { MobileDrawer } from '@/components/layout/MobileDrawer'
import { useConfirm } from '@/lib/confirm'

export function Topbar() {
  const { authInfo, routes, logout } = useSession()
  const confirm = useConfirm()
  const location = useLocation()
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [appearanceOpen, setAppearanceOpen] = useState(false)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)

  // Ctrl/⌘ + K 命令面板
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPaletteOpen((o) => !o)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const menus = flattenMenus(routes)
  const current = menus.find((m) => m.path !== '/' && location.pathname.startsWith(m.path))
  const tenantLabel = authInfo ? `${authInfo.tenant.name}（${authInfo.tenant.code}）` : ''

  const onLogout = async () => {
    const ok = await confirm({ title: '确认退出登录？', danger: true, confirmText: '退出' })
    if (!ok) return
    await logout()
    window.location.assign('/login')
  }

  return (
    <header className="flex items-center gap-3 px-4 py-3">
      <Button variant="ghost" size="icon" className="md:hidden" onClick={() => setDrawerOpen(true)} aria-label="打开菜单">
        <MenuIcon className="h-5 w-5" />
      </Button>
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbPage asChild><Link to="/">工作台</Link></BreadcrumbPage>
          </BreadcrumbItem>
          {current && (
            <>
              <BreadcrumbSeparator />
              <BreadcrumbItem><BreadcrumbPage>{current.name}</BreadcrumbPage></BreadcrumbItem>
            </>
          )}
        </BreadcrumbList>
      </Breadcrumb>
      <span className="text-muted-foreground hidden text-xs md:inline">{tenantLabel}</span>
      <div className="ml-auto flex items-center gap-2">
        <Button variant="ghost" size="sm" onClick={() => setPaletteOpen(true)} aria-label="打开命令面板">
          <Command className="h-4 w-4" /> Ctrl+K
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="sm" aria-label="用户菜单">
              <UserRound className="h-4 w-4" />
              {authInfo?.user.nickname || authInfo?.user.username}
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>{authInfo?.user.username}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => setPasswordOpen(true)}>修改密码</DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setAppearanceOpen(true)}>外观设置</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => void onLogout()}>退出登录</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} />
      <AppearanceDialog open={appearanceOpen} onOpenChange={setAppearanceOpen} />
      <PasswordDialog open={passwordOpen} onOpenChange={setPasswordOpen} />
      <MobileDrawer open={drawerOpen} onOpenChange={setDrawerOpen} />
    </header>
  )
}
```

- [ ] **Step 7: 替换 `src/components/layout/AppShell.tsx` 为完整版**

```tsx
import { Outlet } from 'react-router-dom'
import { Sidebar } from '@/components/layout/Sidebar'
import { Topbar } from '@/components/layout/Topbar'
import { useAppearance } from '@/lib/appearance'

export default function AppShell() {
  const { appearance } = useAppearance()
  return (
    <div className="flex min-h-screen flex-col">
      <Topbar />
      <div className="flex flex-1">
        {!appearance.railCollapsed && (
          <nav className="w-60 shrink-0 border-r border-border">
            <Sidebar collapsed={false} />
          </nav>
        )}
        <main className="flex-1 p-4">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
```

- [ ] **Step 8: 写布局测试（红→绿）**

`src/test/layout.test.tsx`：

```tsx
import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SessionProvider } from '@/store/session'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import { Sidebar } from '@/components/layout/Sidebar'
import { Topbar } from '@/components/layout/Topbar'
import { AppearanceDialog } from '@/components/layout/AppearanceDialog'
import { PasswordDialog } from '@/components/layout/PasswordDialog'
import { saveAppearance, applyAppearance } from '@/lib/appearance'
import { adminAuth } from '@/api/admin/auth'
import { session } from '@/lib/session'
import type { AuthInfo, RouterNode } from '@/lib/types'

vi.mock('@/api/admin/auth', () => ({
  adminAuth: {
    getInfo: vi.fn(),
    getRouters: vi.fn(),
    logout: vi.fn().mockResolvedValue(undefined),
    updatePassword: vi.fn().mockResolvedValue(undefined),
  },
}))

const authInfo: AuthInfo = {
  user: { id: '1', username: 'admin', nickname: '管理员' },
  tenant: { id: '1', name: 'ACME', code: 'acme' },
  roles: ['admin'],
  permissions: ['system:user:create'],
}

const routers: RouterNode[] = [
  { id: 'd', name: '工作台', path: '/dashboard', component: 'dashboard/index', sort: 99, visible: true },
  {
    id: 's', name: '系统管理', path: '/system', component: '', sort: 1, visible: true,
    children: [{ id: 'u', name: '用户管理', path: '/system/user', component: 'system/user/index', sort: 1, visible: true }],
  },
]

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
  vi.mocked(adminAuth.getInfo).mockResolvedValue(authInfo)
  vi.mocked(adminAuth.getRouters).mockResolvedValue(routers)
})

function renderWithProviders(ui: React.ReactElement, path = '/system/user') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="*" element={
          <SessionProvider>
            <PermissionsProvider permissions={authInfo.permissions}>
              <ConfirmProvider>{ui}</ConfirmProvider>
            </PermissionsProvider>
          </SessionProvider>
        } />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Sidebar', () => {
  it('renders dashboard first and groups', async () => {
    renderWithProviders(<Sidebar collapsed={false} />)
    await screen.findByText('工作台')
    expect(screen.getByText('系统管理')).toBeInTheDocument()
    expect(screen.getByText('用户管理')).toBeInTheDocument()
    // 工作台在用户管理之前
    const all = screen.getAllByRole('link').map((el) => el.textContent)
    expect(all.findIndex((t) => t?.includes('工作台')))
      .toBeLessThan(all.findIndex((t) => t?.includes('用户管理')))
  })
})

describe('Topbar', () => {
  it('shows tenant info and user menu', async () => {
    renderWithProviders(<Topbar />)
    await screen.findByText(/ACME/)
    expect(screen.getByLabelText('用户菜单')).toBeInTheDocument()
    expect(screen.getByLabelText('打开命令面板')).toBeInTheDocument()
  })

  it('opens command palette via Ctrl+K and closes on select', async () => {
    renderWithProviders(<Topbar />)
    await screen.findByText('用户管理')
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    expect(screen.getByPlaceholderText('搜索菜单…')).toBeInTheDocument()
    fireEvent.click(screen.getByText('用户管理'))
    expect(screen.queryByPlaceholderText('搜索菜单…')).not.toBeInTheDocument()
  })
})

describe('AppearanceDialog', () => {
  it('applies density to html dataset', () => {
    saveAppearance({ theme: 'auto', density: 'regular', railCollapsed: false })
    render(
      <MemoryRouter>
        <AppearanceDialog open onOpenChange={() => {}} />
      </MemoryRouter>,
    )
    applyAppearance()
    expect(document.documentElement.dataset.density).toBe('regular')
  })
})

describe('PasswordDialog', () => {
  it('rejects mismatched confirmation with error toast (no api call)', async () => {
    const toastModule = await import('@/lib/toast')
    const toastSpy = vi.spyOn(toastModule, 'toastError')
    render(
      <MemoryRouter>
        <PasswordDialog open onOpenChange={() => {}} />
      </MemoryRouter>,
    )
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'a' } })
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'b' } })
    fireEvent.click(screen.getByText('提交'))
    expect(toastSpy).toHaveBeenCalledWith('两次输入的密码不一致')
    expect(adminAuth.updatePassword).not.toHaveBeenCalled()
  })
})
```

Run: `npm run test:run -- src/test/layout.test.tsx` → Expected: PASS。

- [ ] **Step 9: 全量校验并提交**

```bash
npm run test:run
npm run typecheck
npm run build
```

Expected: 全部 PASS；typecheck 零错误；build 成功。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): admin AppShell with sidebar/topbar/command palette/user menu/appearance/password dialogs/mobile drawer"
```

### Task 6: Dashboard 工作台（真实数据）

**Files:**
- Modify: `src/views/Dashboard.tsx`（替换 Task 3 最小版）
- Test: `src/test/dashboard.test.tsx`

**Interfaces:**
- Consumes: `useSession()`（`authInfo`/`routes`）、`adminSystem.listLoginLogs`（Task 3）、`formatDateTime`（Task 2）、shadcn `Card`/`Badge`/`Table`/`Separator`/`Skeleton`、`flattenMenus`。
- Produces: `Dashboard` 组件，展示：账号信息卡（用户名/昵称/角色）、租户卡、菜单统计卡（可见菜单节点数）、最近登录日志表（真实 `listLoginLogs({ page_num: 1, page_size: 10 })`）。只读真实接口，无虚构指标。

- [ ] **Step 1: 写失败测试**

`src/test/dashboard.test.tsx`：

```tsx
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SessionProvider } from '@/store/session'
import { PermissionsProvider } from '@/lib/permission'
import Dashboard from '@/views/Dashboard'
import { adminAuth } from '@/api/admin/auth'
import { adminSystem } from '@/api/admin/system'
import { session } from '@/lib/session'
import type { AuthInfo, RouterNode } from '@/lib/types'

vi.mock('@/api/admin/auth', () => ({
  adminAuth: {
    getInfo: vi.fn(),
    getRouters: vi.fn(),
    logout: vi.fn(),
    updatePassword: vi.fn(),
  },
}))

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listLoginLogs: vi.fn(),
  },
}))

const authInfo: AuthInfo = {
  user: { id: '1', username: 'admin', nickname: '管理员' },
  tenant: { id: '1', name: 'ACME', code: 'acme' },
  roles: ['admin'],
  permissions: [],
}

const routers: RouterNode[] = [
  { id: '1', name: '工作台', path: '/dashboard', component: 'dashboard/index', sort: 0, visible: true },
  {
    id: '2', name: '系统', path: '/system', component: '', sort: 1, visible: true,
    children: [{ id: '3', name: '用户', path: '/system/user', component: 'system/user/index', sort: 1, visible: true }],
  },
]

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
  vi.mocked(adminAuth.getInfo).mockResolvedValue(authInfo)
  vi.mocked(adminAuth.getRouters).mockResolvedValue(routers)
  vi.mocked(adminSystem.listLoginLogs).mockResolvedValue({
    list: [{ id: 'l1', username: 'admin', ip: '1.2.3.4', status: 1, msg: 'ok', loginAt: '2026-01-02 03:04:05' }],
    total: 1, page_num: 1, page_size: 10, pages: 1,
  })
})

describe('Dashboard', () => {
  it('shows real account info, menu count and recent login logs', async () => {
    render(
      <MemoryRouter>
        <SessionProvider>
          <PermissionsProvider permissions={authInfo.permissions}>
            <Dashboard />
          </PermissionsProvider>
        </SessionProvider>
      </MemoryRouter>,
    )
    await screen.findByText('管理员')
    await waitFor(() =>
      expect(adminSystem.listLoginLogs).toHaveBeenCalledWith({ page_num: 1, page_size: 10 }),
    )
    expect(await screen.findByText('1.2.3.4')).toBeInTheDocument()
    expect(screen.getByText(/菜单/)).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: 运行测试验证失败**

Run: `npm run test:run -- src/test/dashboard.test.tsx`
Expected: FAIL（Dashboard 仍为占位「工作台」文本）。

- [ ] **Step 3: 实现完整 Dashboard**

`src/views/Dashboard.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useSession } from '@/store/session'
import { formatDateTime } from '@/lib/datetime'
import { flattenMenus } from '@/router/routes'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { LoginLog } from '@/lib/types'

export default function Dashboard() {
  const { authInfo, routes } = useSession()
  const [logs, setLogs] = useState<LoginLog[]>([])
  const [logsLoading, setLogsLoading] = useState(true)

  useEffect(() => {
    let alive = true
    adminSystem
      .listLoginLogs({ page_num: 1, page_size: 10 })
      .then((r) => {
        if (alive) setLogs(r.list)
      })
      .finally(() => {
        if (alive) setLogsLoading(false)
      })
    return () => {
      alive = false
    }
  }, [])

  if (!authInfo) return <Skeleton className="h-64 w-full" />

  const menuCount = flattenMenus(routes).length

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader><CardTitle>账号</CardTitle></CardHeader>
          <CardContent className="space-y-1 text-sm">
            <div>{authInfo.user.nickname || authInfo.user.username}</div>
            <div className="text-muted-foreground">@{authInfo.user.username}</div>
            <div className="flex gap-1 pt-1">
              {authInfo.roles.map((r) => <Badge key={r} variant="secondary">{r}</Badge>)}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>租户</CardTitle></CardHeader>
          <CardContent className="text-sm">
            <div>{authInfo.tenant.name}</div>
            <div className="text-muted-foreground">{authInfo.tenant.code}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader><CardTitle>菜单</CardTitle></CardHeader>
          <CardContent className="text-sm">
            共 <span className="font-semibold">{menuCount}</span> 个可见菜单节点
          </CardContent>
        </Card>
      </div>
      <Separator />
      <Card>
        <CardHeader><CardTitle>最近登录日志</CardTitle></CardHeader>
        <CardContent>
          {logsLoading ? (
            <Skeleton className="h-32 w-full" />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>账号</TableHead>
                  <TableHead>IP</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>时间</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {logs.map((l) => (
                  <TableRow key={l.id}>
                    <TableCell>{l.username}</TableCell>
                    <TableCell>{l.ip}</TableCell>
                    <TableCell>
                      <Badge variant={l.status === 1 ? 'default' : 'destructive'}>
                        {l.status === 1 ? '成功' : '失败'}
                      </Badge>
                    </TableCell>
                    <TableCell>{formatDateTime(l.loginAt)}</TableCell>
                  </TableRow>
                ))}
                {logs.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={4} className="text-muted-foreground">暂无日志</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
```

- [ ] **Step 4: 运行测试验证通过**

Run: `npm run test:run -- src/test/dashboard.test.tsx` → Expected: PASS。

- [ ] **Step 5: 全量校验并提交**

```bash
npm run test:run
npm run typecheck
```

Expected: 全部 PASS，零错误。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): dashboard with real account/menu/login-log data"
```

### Task 7: 共享 CRUD UI 骨架（ListPage / FormDialog / SearchForm / PermissionButton）

**Files:**
- Create: `src/components/business/PermissionButton.tsx`
- Create: `src/components/business/SearchForm.tsx`
- Create: `src/components/business/ListPage.tsx`
- Create: `src/components/business/FormDialog.tsx`
- Test: `src/test/business-ui.test.tsx`

**Interfaces:**
- Consumes: `useListPage<T>()`/`ListPageState<T>`（Task 2）、`usePermissions`（Task 2）、shadcn `Table`/`Dialog`/`Field`/`Input`/`Select`/`Button`/`Checkbox`。
- Produces:
  - `PermissionButton({ permission, ...btnProps }: { permission: string } & ButtonProps)` — 无权限**不渲染**（返回 null）。
  - `interface SearchField { key: string; label: string; type?: 'input' | 'select'; options?: Array<{ label: string; value: string }> }`；`SearchForm({ fields, value, onSearch, onReset })` — `value` 为受控的 `list.query` 对象引用（写入直接 mutate 并触发回调）。
  - `interface ListColumn<T> { key: string; title: string; render?: (row: T) => ReactNode; className?: string }`；`ListPage<T>({ state, columns, selectable?, toolbar?, batchActions?, emptyText? }: { state: ListPageState<T>; ... })` — 含分页与批量操作条（`selectable` 且有选中时渲染 `已选 N 项 + batchActions`）。
  - `FormDialog({ open, onOpenChange, title, children, onSubmit, busy?, submitText? })` — Dialog + 内部 form 提交。

- [ ] **Step 1: 创建 `src/components/business/PermissionButton.tsx`**

```tsx
import type { ButtonProps } from '@/components/ui/button'
import { Button } from '@/components/ui/button'
import { usePermissions } from '@/lib/permission'

export function PermissionButton({ permission, ...rest }: ButtonProps & { permission: string }) {
  const { has } = usePermissions()
  if (!has(permission)) return null // 无权限：不渲染，而非置灰
  return <Button {...rest} />
}
```

- [ ] **Step 2: 创建 `src/components/business/SearchForm.tsx`**

```tsx
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

export interface SearchField {
  key: string
  label: string
  type?: 'input' | 'select'
  options?: Array<{ label: string; value: string }>
}

interface SearchFormProps {
  fields: SearchField[]
  value: Record<string, unknown>
  onSearch: () => void
  onReset: () => void
}

export function SearchForm({ fields, value, onSearch, onReset }: SearchFormProps) {
  const [draft, setDraft] = useState<Record<string, unknown>>({ ...value })

  const applyDraft = () => {
    // 输入框空串不进入查询（后端空值剔除兜底）
    const cleaned = Object.fromEntries(Object.entries(draft).filter(([, v]) => v !== ''))
    for (const k of Object.keys(value)) delete value[k]
    Object.assign(value, cleaned)
  }

  return (
    <div className="flex flex-wrap items-end gap-3 rounded-md p-3">
      {fields.map((f) => (
        <Field key={f.key} className="min-w-40">
          <FieldLabel>{f.label}</FieldLabel>
          {f.type === 'select' ? (
            <Select
              value={(draft[f.key] as string) ?? ''}
              onValueChange={(v) => setDraft((d) => ({ ...d, [f.key]: v }))}
            >
              <SelectTrigger><SelectValue placeholder="全部" /></SelectTrigger>
              <SelectContent>
                <SelectItem value="">全部</SelectItem>
                {f.options!.map((o) => (
                  <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          ) : (
            <Input
              value={(draft[f.key] as string) ?? ''}
              onChange={(e) => setDraft((d) => ({ ...d, [f.key]: e.target.value }))}
            />
          )}
        </Field>
      ))}
      <Button onClick={() => { applyDraft(); onSearch() }}>查询</Button>
      <Button
        variant="outline"
        onClick={() => {
          for (const k of Object.keys(draft)) draft[k] = ''
          setDraft({ ...draft })
          applyDraft()
          onReset()
        }}
      >
        重置
      </Button>
    </div>
  )
}
```

- [ ] **Step 3: 创建 `src/components/business/ListPage.tsx`**

```tsx
import { type ReactNode } from 'react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { ListPageState } from '@/lib/list'

export interface ListColumn<T> {
  key: string
  title: string
  render?: (row: T) => ReactNode
  className?: string
}

interface ListPageProps<T> {
  state: ListPageState<T>
  columns: ListColumn<T>[]
  selectable?: boolean
  toolbar?: ReactNode
  batchActions?: ReactNode
  emptyText?: string
}

export function ListPage<T extends { id: string }>({
  state, columns, selectable = false, toolbar, batchActions, emptyText = '暂无数据',
}: ListPageProps<T>) {
  const allSelected = selectable && state.rows.length > 0 && state.rows.every((r) => state.selected.includes(r.id))
  const toggleAll = () => {
    state.setSelected(allSelected ? [] : state.rows.map((r) => r.id))
  }
  const toggleOne = (id: string) => {
    state.setSelected(
      state.selected.includes(id)
        ? state.selected.filter((s) => s !== id)
        : [...state.selected, id],
    )
  }

  return (
    <div className="space-y-3">
      {toolbar}
      {selectable && state.selected.length > 0 && (
        <div className="flex items-center gap-2 rounded-md p-2 text-sm">
          已选 {state.selected.length} 项
          {batchActions}
        </div>
      )}
      <div className="overflow-x-auto rounded-md">
        <Table>
          <TableHeader>
            <TableRow>
              {selectable && (
                <TableHead className="w-10">
                  <Checkbox checked={allSelected} onCheckedChange={toggleAll} aria-label="全选" />
                </TableHead>
              )}
              {columns.map((c) => <TableHead key={c.key} className={c.className}>{c.title}</TableHead>)}
            </TableRow>
          </TableHeader>
          <TableBody>
            {state.rows.map((row) => (
              <TableRow key={row.id}>
                {selectable && (
                  <TableCell>
                    <Checkbox
                      checked={state.selected.includes(row.id)}
                      onCheckedChange={() => toggleOne(row.id)}
                      aria-label={`选中 ${row.id}`}
                    />
                  </TableCell>
                )}
                {columns.map((c) => (
                  <TableCell key={c.key} className={c.className}>
                    {c.render ? c.render(row) : String((row as Record<string, unknown>)[c.key] ?? '-')}
                  </TableCell>
                ))}
              </TableRow>
            ))}
            {state.rows.length === 0 && !state.loading && (
              <TableRow>
                <TableCell colSpan={columns.length + (selectable ? 1 : 0)} className="text-muted-foreground">
                  {emptyText}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <div className="flex items-center justify-between">
        <span className="text-muted-foreground text-sm">共 {state.total} 条</span>
        <div className="flex items-center gap-2">
          <Select value={String(state.pageSize)} onValueChange={(v) => void state.setPageSize(Number(v))}>
            <SelectTrigger className="w-28"><SelectValue /></SelectTrigger>
            <SelectContent>
              {[10, 20, 50, 100].map((s) => <SelectItem key={s} value={String(s)}>{s} 条/页</SelectItem>)}
            </SelectContent>
          </Select>
          <Button
            variant="outline" size="sm"
            disabled={state.page <= 1 || state.loading}
            onClick={() => void state.setPage(state.page - 1)}
          >
            上一页
          </Button>
          <span className="text-sm">{state.page}</span>
          <Button
            variant="outline" size="sm"
            disabled={state.loading || state.page * state.pageSize >= state.total}
            onClick={() => void state.setPage(state.page + 1)}
          >
            下一页
          </Button>
        </div>
      </div>
    </div>
  )
}
```

- [ ] **Step 4: 创建 `src/components/business/FormDialog.tsx`**

```tsx
import { type FormEvent, type ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'

interface FormDialogProps {
  open: boolean
  onOpenChange: (o: boolean) => void
  title: string
  children: ReactNode
  onSubmit: (e: FormEvent) => void
  busy?: boolean
  submitText?: string
}

export function FormDialog({
  open, onOpenChange, title, children, onSubmit, busy, submitText = '保存',
}: FormDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader><DialogTitle>{title}</DialogTitle></DialogHeader>
        <form onSubmit={onSubmit} className="space-y-4">
          {children}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
            <Button type="submit" disabled={busy}>{submitText}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 5: 写共享骨架测试（红→绿）**

`src/test/business-ui.test.tsx`：

```tsx
import { render, screen, fireEvent, renderHook, act } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { PermissionButton } from '@/components/business/PermissionButton'
import { SearchForm } from '@/components/business/SearchForm'
import { ListPage } from '@/components/business/ListPage'
import { useListPage } from '@/lib/list'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'

function granted(ui: React.ReactElement) {
  return render(
    <PermissionsProvider permissions={['system:user:delete']}>
      <ConfirmProvider>{ui}</ConfirmProvider>
    </PermissionsProvider>,
  )
}

describe('PermissionButton', () => {
  it('renders when granted', () => {
    granted(<PermissionButton permission="system:user:delete" danger>删除</PermissionButton>)
    expect(screen.getByText('删除')).toBeInTheDocument()
  })
  it('does NOT render (hidden) when denied', () => {
    render(
      <PermissionsProvider permissions={[]}>
        <PermissionButton permission="system:user:delete">删除</PermissionButton>
      </PermissionsProvider>,
    )
    expect(screen.queryByText('删除')).not.toBeInTheDocument()
  })
})

describe('SearchForm', () => {
  it('collects input values and triggers search', () => {
    const onSearch = vi.fn()
    const value: Record<string, unknown> = {}
    render(
      <SearchForm
        fields={[{ key: 'username', label: '账号' }]}
        value={value} onSearch={onSearch} onReset={() => {}}
      />,
    )
    fireEvent.change(screen.getByLabelText('账号'), { target: { value: 'admin' } })
    fireEvent.click(screen.getByText('查询'))
    expect(onSearch).toHaveBeenCalled()
    expect(value.username).toBe('admin')
  })
})

describe('ListPage', () => {
  it('renders rows via custom render and default text', async () => {
    const fetcher = vi.fn().mockResolvedValue({ list: [{ id: '1', name: '张三' }], total: 1, page_num: 1, page_size: 10, pages: 1 })
    const { result } = renderHook(() => useListPage<{ id: string; name: string }>({ fetcher }))
    await act(() => result.current.load())
    render(
      <ListPage
        state={result.current}
        columns={[
          { key: 'name', title: '姓名' },
          { key: 'op', title: '操作', render: (r) => <button>编辑 {r.id}</button> },
        ]}
      />,
    )
    expect(screen.getByText('张三')).toBeInTheDocument()
    expect(screen.getByText('编辑 1')).toBeInTheDocument()
    expect(screen.getByText('共 1 条')).toBeInTheDocument()
  })

  it('selection supports select-all and batch bar', async () => {
    const fetcher = vi.fn().mockResolvedValue({ list: [{ id: '1' }, { id: '2' }], total: 2, page_num: 1, page_size: 10, pages: 1 })
    const { result } = renderHook(() => useListPage<{ id: string }>({ fetcher }))
    await act(() => result.current.load())
    render(
      <ListPage
        state={result.current}
        selectable
        columns={[{ key: 'id', title: 'ID' }]}
        batchActions={<button>批量删除</button>}
      />,
    )
    fireEvent.click(screen.getByLabelText('全选'))
    expect(screen.getByText('已选 2 项')).toBeInTheDocument()
    expect(screen.getByText('批量删除')).toBeInTheDocument()
  })

  it('pagination buttons page through', async () => {
    const fetcher = vi.fn().mockResolvedValue({ list: [{ id: '1' }], total: 25, page_num: 1, page_size: 10, pages: 3 })
    const { result } = renderHook(() => useListPage<{ id: string }>({ fetcher }))
    await act(() => result.current.load())
    render(<ListPage state={result.current} columns={[{ key: 'id', title: 'ID' }]} />)
    fireEvent.click(screen.getByText('下一页'))
    await act(() => result.current.load())
    expect(fetcher).toHaveBeenLastCalledWith(expect.objectContaining({ page_num: 2 }))
  })
})
```

Run: `npm run test:run -- src/test/business-ui.test.tsx` → Expected: PASS。

- [ ] **Step 6: 全量校验并提交**

```bash
npm run test:run
npm run typecheck
```

Expected: 全部 PASS，零错误。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): shared CRUD skeleton (ListPage/SearchForm/FormDialog/PermissionButton)"
```

### Task 8: Users + Posts 页面（CRUD 模式样板）

**Files:**
- Modify: `src/views/system/Users.tsx`（替换 Task 3 最小版）
- Modify: `src/views/system/Posts.tsx`（替换 Task 3 最小版）
- Test: `src/test/users-page.test.tsx`、`src/test/posts-page.test.tsx`

**Interfaces:**
- Consumes: `adminSystem.listUsers/createUser/updateUser/updateUserPassword/deleteUsers/listAllPosts/listAllRoles/listDepts`、`UserWrite`（Task 3）；`useListPage`/`ListPage`/`SearchForm`/`FormDialog`/`PermissionButton`（Task 7）；`useConfirm`、`toastSuccess`/`toastError`。
- Produces: `Users` 页面（列表/查询/新增/编辑/重置密码/删除；`deptId` 必填——写接口空串 400，表单校验在前；`code === 403` 时渲染 NoAccess）；`Posts` 页面（简单 CRUD）。两者为后续 CRUD 页面的样板模式：`useListPage + useEffect(load, [page,pageSize,query]) + SearchForm + ListPage + FormDialog + confirm 删除`。

- [ ] **Step 1: 写 Users 页失败测试**

`src/test/users-page.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Users from '@/views/system/Users'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { PageResult, SystemUser } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listUsers: vi.fn(),
    createUser: vi.fn().mockResolvedValue(undefined),
    updateUser: vi.fn().mockResolvedValue(undefined),
    updateUserPassword: vi.fn().mockResolvedValue(undefined),
    deleteUsers: vi.fn().mockResolvedValue(undefined),
    listAllPosts: vi.fn().mockResolvedValue([]),
    listAllRoles: vi.fn().mockResolvedValue([]),
    listDepts: vi.fn().mockResolvedValue([]),
  },
  parseDeptIDsJson: (s?: string) => (s ? JSON.parse(s) : []),
  encodeDeptIDsJson: (ids: string[]) => JSON.stringify(ids),
}))

function page(list: SystemUser[], total = list.length): PageResult<SystemUser> {
  return { list, total, page_num: 1, page_size: 10, pages: 1 }
}

const user = (over: Partial<SystemUser>): SystemUser => ({
  id: '1', username: 'admin', nickname: '管理员', deptId: '10', postIds: [], roleIds: [], status: 1, ...over,
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listUsers).mockResolvedValue(page([user({})]))
})

function renderUsers() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:user:create', 'system:user:update', 'system:user:delete', 'system:user:update-password']}>
        <ConfirmProvider>
          <Users />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Users page', () => {
  it('loads list and renders rows', async () => {
    renderUsers()
    await screen.findByText('admin')
    expect(adminSystem.listUsers).toHaveBeenCalledWith(expect.objectContaining({ page_num: 1 }))
  })

  it('requires deptId before create (empty dept rejected client-side)', async () => {
    vi.mocked(adminSystem.listDepts).mockResolvedValue([])
    renderUsers()
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('新增用户'))
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u1' } })
    fireEvent.change(screen.getByLabelText('昵称'), { target: { value: 'n1' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(screen.getByText('部门必填')).toBeInTheDocument())
    expect(adminSystem.createUser).not.toHaveBeenCalled()
  })

  it('create sends snake_case payload with dept_id', async () => {
    vi.mocked(adminSystem.listDepts).mockResolvedValue([
      { id: '10', name: '总部', parentId: '0', sort: 1, status: 1 },
    ])
    renderUsers()
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('新增用户'))
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u1' } })
    fireEvent.change(screen.getByLabelText('昵称'), { target: { value: 'n1' } })
    fireEvent.change(screen.getByLabelText('部门'), { target: { value: '10' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createUser).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createUser).mock.calls[0]![0]
    expect(body).toMatchObject({ username: 'u1', dept_id: '10', post_ids: [], role_ids: [] })
  })

  it('reset password calls dedicated endpoint', async () => {
    renderUsers()
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('重置密码'))
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'np123' } })
    fireEvent.click(screen.getByText('确定重置'))
    await waitFor(() =>
      expect(adminSystem.updateUserPassword).toHaveBeenCalledWith('1', { password: 'np123' }),
    )
  })

  it('delete confirms then calls deleteUsers', async () => {
    renderUsers()
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('删除'))
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deleteUsers).toHaveBeenCalledWith(['1']))
  })
})
```

- [ ] **Step 2: 运行验证失败**

Run: `npm run test:run -- src/test/users-page.test.tsx`
Expected: FAIL（占位页面）。

- [ ] **Step 3: 实现完整 Users 页面**

`src/views/system/Users.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import type { SystemDept, SystemPost, SystemRole, SystemUser } from '@/lib/types'

interface UserForm {
  username: string
  nickname: string
  dept_id: string
  email: string
  mobile: string
  gender: string
  remark: string
  post_ids: string[]
  role_ids: string[]
  status: number
}

const emptyForm: UserForm = {
  username: '', nickname: '', dept_id: '', email: '', mobile: '', gender: '1',
  remark: '', post_ids: [], role_ids: [], status: 1,
}

export default function Users() {
  const confirm = useConfirm()
  const list = useListPage<SystemUser>({ fetcher: (p) => adminSystem.listUsers(p) })
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemUser | null>(null)
  const [form, setForm] = useState<UserForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [pwdUser, setPwdUser] = useState<SystemUser | null>(null)
  const [newPassword, setNewPassword] = useState('')
  const [depts, setDepts] = useState<SystemDept[]>([])
  const [posts, setPosts] = useState<SystemPost[]>([])
  const [roles, setRoles] = useState<SystemRole[]>([])

  useEffect(() => { void list.load() }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => {
    void adminSystem.listDepts().then(setDepts)
    void adminSystem.listAllPosts().then(setPosts)
    void adminSystem.listAllRoles().then(setRoles)
  }, [])

  const openCreate = () => {
    setEditing(null)
    setForm({ ...emptyForm })
    setFormError(null)
    setFormOpen(true)
  }

  const openEdit = (u: SystemUser) => {
    setEditing(u)
    setForm({
      username: u.username, nickname: u.nickname, dept_id: u.deptId,
      email: u.email ?? '', mobile: u.mobile ?? '', gender: u.gender ?? '1', remark: u.remark ?? '',
      post_ids: u.postIds ?? [], role_ids: u.roleIds ?? [], status: u.status,
    })
    setFormError(null)
    setFormOpen(true)
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    // deptId 必填：写接口空串 400，前端先行校验
    if (!form.dept_id) {
      setFormError('部门必填')
      return
    }
    setBusy(true)
    try {
      const payload = {
        username: form.username, nickname: form.nickname, dept_id: form.dept_id,
        email: form.email || undefined, mobile: form.mobile || undefined, gender: form.gender,
        remark: form.remark || undefined, post_ids: form.post_ids, role_ids: form.role_ids,
        status: form.status,
      }
      if (editing) await adminSystem.updateUser(editing.id, payload)
      else await adminSystem.createUser(payload)
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await list.afterMutate()
    } catch (err) {
      setFormError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const doResetPassword = async () => {
    if (!pwdUser) return
    try {
      await adminSystem.updateUserPassword(pwdUser.id, { password: newPassword })
      toastSuccess('密码已重置')
      setPwdUser(null)
      setNewPassword('')
    } catch (err) {
      toastError((err as Error).message)
    }
  }

  const doDelete = async (u: SystemUser) => {
    const ok = await confirm({ title: `确认删除用户「${u.username}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.deleteUsers([u.id])
    toastSuccess('删除成功')
    await list.afterMutate()
  }

  const columns: ListColumn<SystemUser>[] = [
    { key: 'username', title: '用户名' },
    { key: 'nickname', title: '昵称' },
    {
      key: 'status', title: '状态',
      render: (r) => <Badge variant={r.status === 1 ? 'default' : 'secondary'}>{r.status === 1 ? '正常' : '停用'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (r) => (
        <div className="flex gap-1">
          <PermissionButton permission="system:user:update" variant="ghost" size="sm" onClick={() => openEdit(r)}>编辑</PermissionButton>
          <PermissionButton permission="system:user:update-password" variant="ghost" size="sm" onClick={() => setPwdUser(r)}>重置密码</PermissionButton>
          <PermissionButton permission="system:user:delete" variant="ghost" size="sm" onClick={() => void doDelete(r)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'username', label: '用户名' }, { key: 'nickname', label: '昵称' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list}
        columns={columns}
        toolbar={<PermissionButton permission="system:user:create" onClick={openCreate}>新增用户</PermissionButton>}
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑用户' : '新增用户'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field>
          <FieldLabel>用户名</FieldLabel>
          <Input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} disabled={!!editing} required />
        </Field>
        <Field>
          <FieldLabel>昵称</FieldLabel>
          <Input value={form.nickname} onChange={(e) => setForm({ ...form, nickname: e.target.value })} required />
        </Field>
        <Field>
          <FieldLabel>部门</FieldLabel>
          <select
            className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
            value={form.dept_id}
            onChange={(e) => setForm({ ...form, dept_id: e.target.value })}
            aria-label="部门"
          >
            <option value="">请选择部门</option>
            {depts.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
          </select>
        </Field>
        <Field>
          <FieldLabel>邮箱</FieldLabel>
          <Input value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
        </Field>
        <Field>
          <FieldLabel>手机号</FieldLabel>
          <Input value={form.mobile} onChange={(e) => setForm({ ...form, mobile: e.target.value })} />
        </Field>
        <Field>
          <FieldLabel>岗位（多选）</FieldLabel>
          <div className="flex flex-wrap gap-2">
            {posts.map((p) => (
              <label key={p.id} className="flex items-center gap-1 text-sm">
                <input
                  type="checkbox"
                  checked={form.post_ids.includes(p.id)}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      post_ids: e.target.checked ? [...form.post_ids, p.id] : form.post_ids.filter((x) => x !== p.id),
                    })
                  }
                />
                {p.name}
              </label>
            ))}
          </div>
        </Field>
        <Field>
          <FieldLabel>角色（多选）</FieldLabel>
          <div className="flex flex-wrap gap-2">
            {roles.map((r) => (
              <label key={r.id} className="flex items-center gap-1 text-sm">
                <input
                  type="checkbox"
                  checked={form.role_ids.includes(r.id)}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      role_ids: e.target.checked ? [...form.role_ids, r.id] : form.role_ids.filter((x) => x !== r.id),
                    })
                  }
                />
                {r.name}
              </label>
            ))}
          </div>
        </Field>
      </FormDialog>
      <Dialog open={pwdUser != null} onOpenChange={(o) => { if (!o) setPwdUser(null) }}>
        <DialogContent className="max-w-sm">
          <DialogHeader><DialogTitle>重置密码（{pwdUser?.username}）</DialogTitle></DialogHeader>
          <Field>
            <FieldLabel>新密码</FieldLabel>
            <Input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
          </Field>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPwdUser(null)}>取消</Button>
            <Button onClick={() => void doResetPassword()}>确定重置</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
```

- [ ] **Step 4: 运行 Users 测试通过**

Run: `npm run test:run -- src/test/users-page.test.tsx` → Expected: PASS。

- [ ] **Step 5: 写 Posts 页测试（红→绿）**

`src/test/posts-page.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Posts from '@/views/system/Posts'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { PageResult, SystemPost } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listPosts: vi.fn(),
    createPost: vi.fn().mockResolvedValue(undefined),
    updatePost: vi.fn().mockResolvedValue(undefined),
    deletePosts: vi.fn().mockResolvedValue(undefined),
  },
}))

const post = (over: Partial<SystemPost>): SystemPost => ({
  id: '1', code: 'ceo', name: 'CEO', sort: 1, status: 1, ...over,
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listPosts).mockResolvedValue({
    list: [post({})], total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<SystemPost>)
})

function renderPosts() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:post:create', 'system:post:update', 'system:post:delete']}>
        <ConfirmProvider>
          <Posts />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Posts page', () => {
  it('loads and renders posts', async () => {
    renderPosts()
    await screen.findByText('CEO')
    expect(adminSystem.listPosts).toHaveBeenCalled()
  })

  it('creates a post with snake_case payload', async () => {
    renderPosts()
    await screen.findByText('CEO')
    fireEvent.click(screen.getByText('新增岗位'))
    fireEvent.change(screen.getByLabelText('岗位编码'), { target: { value: 'cto' } })
    fireEvent.change(screen.getByLabelText('岗位名称'), { target: { value: 'CTO' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() =>
      expect(adminSystem.createPost).toHaveBeenCalledWith(
        expect.objectContaining({ code: 'cto', name: 'CTO', sort: expect.any(Number), status: 1 }),
      ),
    )
  })

  it('deletes with confirm', async () => {
    renderPosts()
    await screen.findByText('CEO')
    fireEvent.click(screen.getByText('删除'))
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deletePosts).toHaveBeenCalledWith(['1']))
  })
})
```

- [ ] **Step 6: 实现完整 Posts 页面**

`src/views/system/Posts.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import type { SystemPost } from '@/lib/types'

interface PostForm { code: string; name: string; sort: number; status: number; remark: string }
const emptyForm: PostForm = { code: '', name: '', sort: 1, status: 1, remark: '' }

export default function Posts() {
  const confirm = useConfirm()
  const list = useListPage<SystemPost>({ fetcher: (p) => adminSystem.listPosts(p) })
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemPost | null>(null)
  const [form, setForm] = useState<PostForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => { void list.load() }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.code || !form.name) {
      setFormError('编码与名称必填')
      return
    }
    setBusy(true)
    try {
      if (editing) await adminSystem.updatePost(editing.id, form)
      else await adminSystem.createPost(form)
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await list.afterMutate()
    } catch (err) {
      setFormError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const doDelete = async (p: SystemPost) => {
    const ok = await confirm({ title: `确认删除岗位「${p.name}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.deletePosts([p.id])
    toastSuccess('删除成功')
    await list.afterMutate()
  }

  const columns: ListColumn<SystemPost>[] = [
    { key: 'code', title: '编码' },
    { key: 'name', title: '名称' },
    { key: 'sort', title: '排序' },
    {
      key: 'status', title: '状态',
      render: (r) => <Badge variant={r.status === 1 ? 'default' : 'secondary'}>{r.status === 1 ? '正常' : '停用'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (r) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:post:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(r)
              setForm({ code: r.code, name: r.name, sort: r.sort, status: r.status, remark: r.remark ?? '' })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton permission="system:post:delete" variant="ghost" size="sm" onClick={() => void doDelete(r)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'code', label: '编码' }, { key: 'name', label: '名称' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns}
        toolbar={
          <PermissionButton
            permission="system:post:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormOpen(true) }}
          >新增岗位</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑岗位' : '新增岗位'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field>
          <FieldLabel>岗位编码</FieldLabel>
          <Input value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} disabled={!!editing} required />
        </Field>
        <Field>
          <FieldLabel>岗位名称</FieldLabel>
          <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
        </Field>
        <Field>
          <FieldLabel>排序</FieldLabel>
          <Input type="number" value={form.sort} onChange={(e) => setForm({ ...form, sort: Number(e.target.value) })} />
        </Field>
        <div className="flex items-center justify-between">
          <FieldLabel>状态</FieldLabel>
          <Switch checked={form.status === 1} onCheckedChange={(v) => setForm({ ...form, status: v ? 1 : 0 })} />
        </div>
        <Field>
          <FieldLabel>备注</FieldLabel>
          <Input value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} />
        </Field>
      </FormDialog>
    </div>
  )
}
```

- [ ] **Step 7: 运行 Posts 测试通过并提交**

Run: `npm run test:run -- src/test/users-page.test.tsx src/test/posts-page.test.tsx` → Expected: PASS。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): Users and Posts CRUD pages (list/search/form-dialog/reset-password/permission-guarded)"
```

### Task 9: Roles 页面（分配菜单树 + data_scope_dept_ids JSON 编解码 + 内置不可删）

**Files:**
- Modify: `src/views/system/Roles.tsx`（替换 Task 3 最小版）
- Test: `src/test/roles-page.test.tsx`

**Interfaces:**
- Consumes: `adminSystem.listRoles/createRole/updateRole/assignRoleMenus/deleteRoles/listMenus/listDepts`、`RoleWrite`、`parseDeptIDsJson`/`encodeDeptIDsJson`（Task 3）；CRUD 样板（Task 8 模式）。
- Produces: `Roles` 页面：列表/新增/编辑（含 `data_scope` 与 `data_scope_dept_ids` JSON 编解码往返）/分配菜单（树形复选，提交 `menu_ids`）/删除（**内置角色不可删除**：`builtin === true` 时删除按钮不渲染，且 confirm 路径不可达）。

- [ ] **Step 1: 写失败测试**

`src/test/roles-page.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Roles from '@/views/system/Roles'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { PageResult, SystemMenu, SystemRole } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listRoles: vi.fn(),
    createRole: vi.fn().mockResolvedValue(undefined),
    updateRole: vi.fn().mockResolvedValue(undefined),
    assignRoleMenus: vi.fn().mockResolvedValue(undefined),
    deleteRoles: vi.fn().mockResolvedValue(undefined),
    listMenus: vi.fn(),
    listDepts: vi.fn().mockResolvedValue([]),
  },
  parseDeptIDsJson: (s?: string) => (s ? JSON.parse(s) : []),
  encodeDeptIDsJson: (ids: string[]) => JSON.stringify(ids),
}))

const role = (over: Partial<SystemRole>): SystemRole => ({
  id: '1', name: '管理员', code: 'admin', sort: 1, dataScope: 1, dataScopeDeptIds: [], status: 1, ...over,
})

const menuTree: SystemMenu[] = [
  {
    id: '1', name: '系统', path: '/system', component: '', sort: 1, visible: true, type: 1, parentId: '0', status: 1,
    children: [
      { id: '2', name: '用户', path: '/system/user', component: 'system/user/index', sort: 1, visible: true, type: 2, parentId: '1', status: 1 },
    ],
  },
]

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listRoles).mockResolvedValue({
    list: [role({})], total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<SystemRole>)
  vi.mocked(adminSystem.listMenus).mockResolvedValue(menuTree)
})

function renderRoles() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:role:create', 'system:role:update', 'system:role:assign', 'system:role:delete']}>
        <ConfirmProvider>
          <Roles />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Roles page', () => {
  it('creates role with data_scope_dept_ids as JSON array string', async () => {
    renderRoles()
    await screen.findByText('管理员')
    fireEvent.click(screen.getByText('新增角色'))
    fireEvent.change(screen.getByLabelText('角色名称'), { target: { value: '运维' } })
    fireEvent.change(screen.getByLabelText('角色编码'), { target: { value: 'ops' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createRole).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createRole).mock.calls[0]![0]
    expect(body).toMatchObject({ name: '运维', code: 'ops', sort: expect.any(Number), data_scope: expect.any(Number) })
    // 空部门集编码为 "[]"
    expect(body.data_scope_dept_ids).toBe('[]')
  })

  it('edit round-trips data_scope_dept_ids through JSON codec', async () => {
    vi.mocked(adminSystem.listRoles).mockResolvedValue({
      list: [role({ id: '9', dataScope: 2, dataScopeDeptIds: ['10', '11'] })],
      total: 1, page_num: 1, page_size: 10, pages: 1,
    } as PageResult<SystemRole>)
    renderRoles()
    await screen.findByText('管理员')
    fireEvent.click(screen.getByText('编辑'))
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() =>
      expect(adminSystem.updateRole).toHaveBeenCalledWith('9', expect.objectContaining({ data_scope: 2, data_scope_dept_ids: '["10","11"]' })),
    )
  })

  it('assign menus submits menu_ids from tree selection', async () => {
    renderRoles()
    await screen.findByText('管理员')
    fireEvent.click(screen.getByText('分配菜单'))
    await screen.findByText('系统')
    fireEvent.click(screen.getByLabelText('选菜单 用户'))
    fireEvent.click(screen.getByText('确定分配'))
    await waitFor(() => expect(adminSystem.assignRoleMenus).toHaveBeenCalledWith('1', ['2']))
  })

  it('hides delete for builtin roles', async () => {
    vi.mocked(adminSystem.listRoles).mockResolvedValue({
      list: [role({ builtin: true })], total: 1, page_num: 1, page_size: 10, pages: 1,
    } as PageResult<SystemRole>)
    renderRoles()
    await screen.findByText('管理员')
    expect(screen.queryByText('删除')).not.toBeInTheDocument()
  })

  it('deletes non-builtin roles with confirm', async () => {
    renderRoles()
    await screen.findByText('管理员')
    fireEvent.click(screen.getByText('删除'))
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deleteRoles).toHaveBeenCalledWith(['1']))
  })
})
```

- [ ] **Step 2: 运行验证失败**

Run: `npm run test:run -- src/test/roles-page.test.tsx` → Expected: FAIL。

- [ ] **Step 3: 实现完整 Roles 页面**

`src/views/system/Roles.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { adminSystem, parseDeptIDsJson } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import type { SystemDept, SystemMenu, SystemRole } from '@/lib/types'

interface RoleForm {
  name: string; code: string; sort: number; data_scope: number; status: number; remark: string; dept_ids: string[]
}
const emptyForm: RoleForm = { name: '', code: '', sort: 1, data_scope: 1, status: 1, remark: '', dept_ids: [] }

function MenuTreeCheckbox({
  nodes, selected, onToggle,
}: {
  nodes: SystemMenu[]
  selected: Set<string>
  onToggle: (id: string) => void
}) {
  return (
    <div className="space-y-1">
      {nodes.map((n) => (
        <div key={n.id}>
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" checked={selected.has(n.id)} onChange={() => onToggle(n.id)} aria-label={`选菜单 ${n.name}`} />
            {n.name}
          </label>
          {n.children?.length && (
            <div className="ml-4">
              <MenuTreeCheckbox nodes={n.children} selected={selected} onToggle={onToggle} />
            </div>
          )}
        </div>
      ))}
    </div>
  )
}

export default function Roles() {
  const confirm = useConfirm()
  const list = useListPage<SystemRole>({ fetcher: (p) => adminSystem.listRoles(p) })
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemRole | null>(null)
  const [form, setForm] = useState<RoleForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [assignOpen, setAssignOpen] = useState(false)
  const [assignRole, setAssignRole] = useState<SystemRole | null>(null)
  const [menus, setMenus] = useState<SystemMenu[]>([])
  const [selectedMenus, setSelectedMenus] = useState<Set<string>>(new Set())
  const [depts, setDepts] = useState<SystemDept[]>([])

  useEffect(() => { void list.load() }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { void adminSystem.listDepts().then(setDepts) }, [])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.name || !form.code) {
      setFormError('名称与编码必填')
      return
    }
    setBusy(true)
    try {
      const payload = {
        name: form.name, code: form.code, sort: form.sort,
        data_scope: form.data_scope,
        data_scope_dept_ids: JSON.stringify(form.dept_ids), // JSON 数组字符串，非逗号分隔
        status: form.status,
        remark: form.remark || undefined,
      }
      if (editing) await adminSystem.updateRole(editing.id, payload)
      else await adminSystem.createRole(payload)
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await list.afterMutate()
    } catch (err) {
      setFormError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const doAssign = async () => {
    if (!assignRole) return
    try {
      await adminSystem.assignRoleMenus(assignRole.id, [...selectedMenus])
      toastSuccess('菜单分配成功')
      setAssignOpen(false)
    } catch (err) {
      toastError((err as Error).message)
    }
  }

  const doDelete = async (r: SystemRole) => {
    if (r.builtin) return // 内置角色不可删除（双保险，UI 已隐藏）
    const ok = await confirm({ title: `确认删除角色「${r.name}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.deleteRoles([r.id])
    toastSuccess('删除成功')
    await list.afterMutate()
  }

  const columns: ListColumn<SystemRole>[] = [
    { key: 'name', title: '名称' },
    { key: 'code', title: '编码' },
    { key: 'sort', title: '排序' },
    {
      key: 'status', title: '状态',
      render: (r) => <Badge variant={r.status === 1 ? 'default' : 'secondary'}>{r.status === 1 ? '正常' : '停用'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (r) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:role:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(r)
              setForm({
                name: r.name, code: r.code, sort: r.sort, data_scope: r.dataScope,
                status: r.status, remark: r.remark ?? '',
                dept_ids: parseDeptIDsJson(
                  r.dataScopeDeptIds?.length ? JSON.stringify(r.dataScopeDeptIds) : undefined,
                ),
              })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton
            permission="system:role:assign" variant="ghost" size="sm"
            onClick={() => {
              setAssignRole(r)
              setSelectedMenus(new Set())
              setAssignOpen(true)
              void adminSystem.listMenus().then(setMenus)
            }}
          >分配菜单</PermissionButton>
          {!r.builtin && (
            <PermissionButton permission="system:role:delete" variant="ghost" size="sm" onClick={() => void doDelete(r)}>删除</PermissionButton>
          )}
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'name', label: '名称' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns}
        toolbar={
          <PermissionButton
            permission="system:role:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormOpen(true) }}
          >新增角色</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑角色' : '新增角色'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field><FieldLabel>角色名称</FieldLabel><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></Field>
        <Field><FieldLabel>角色编码</FieldLabel><Input value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} disabled={!!editing} required /></Field>
        <Field><FieldLabel>排序</FieldLabel><Input type="number" value={form.sort} onChange={(e) => setForm({ ...form, sort: Number(e.target.value) })} /></Field>
        <Field>
          <FieldLabel>数据权限范围</FieldLabel>
          <select
            className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
            value={String(form.data_scope)}
            onChange={(e) => setForm({ ...form, data_scope: Number(e.target.value) })}
            aria-label="数据权限范围"
          >
            <option value="1">全部数据</option>
            <option value="2">自定义部门</option>
            <option value="3">本部门</option>
            <option value="4">本部门及以下</option>
            <option value="5">仅本人</option>
          </select>
        </Field>
        {form.data_scope === 2 && (
          <Field>
            <FieldLabel>自定义部门</FieldLabel>
            <div className="flex flex-wrap gap-2">
              {depts.map((d) => (
                <label key={d.id} className="flex items-center gap-1 text-sm">
                  <input
                    type="checkbox"
                    checked={form.dept_ids.includes(d.id)}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        dept_ids: e.target.checked ? [...form.dept_ids, d.id] : form.dept_ids.filter((x) => x !== d.id),
                      })
                    }
                  />
                  {d.name}
                </label>
              ))}
            </div>
          </Field>
        )}
        <div className="flex items-center justify-between">
          <FieldLabel>状态</FieldLabel>
          <Switch checked={form.status === 1} onCheckedChange={(v) => setForm({ ...form, status: v ? 1 : 0 })} />
        </div>
        <Field><FieldLabel>备注</FieldLabel><Input value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} /></Field>
      </FormDialog>
      <Dialog open={assignOpen} onOpenChange={setAssignOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader><DialogTitle>分配菜单（{assignRole?.name}）</DialogTitle></DialogHeader>
          <div className="max-h-80 overflow-auto">
            <MenuTreeCheckbox
              nodes={menus}
              selected={selectedMenus}
              onToggle={(id) => {
                setSelectedMenus((s) => {
                  const n = new Set(s)
                  if (n.has(id)) n.delete(id)
                  else n.add(id)
                  return n
                })
              }}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setAssignOpen(false)}>取消</Button>
            <Button onClick={() => void doAssign()}>确定分配</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
```

- [ ] **Step 4: 运行测试通过并提交**

Run: `npm run test:run -- src/test/roles-page.test.tsx` → Expected: PASS。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): Roles page with menu-assign tree, data_scope_dept_ids JSON codec and builtin-delete guard"
```

### Task 10: Menus + Depts 页面（树形级联）

**Files:**
- Modify: `src/views/system/Menus.tsx`（替换 Task 3 最小版）
- Modify: `src/views/system/Depts.tsx`（替换 Task 3 最小版）
- Test: `src/test/menus-page.test.tsx`、`src/test/depts-page.test.tsx`

**Interfaces:**
- Consumes: `adminSystem.listMenus/createMenu/updateMenu/deleteMenu/listDepts/createDept/updateDept/deleteDept`、`MenuWrite`/`DeptWrite`（Task 3）；CRUD 样板（Task 8）；`ListPage`（Task 7，树页直接自管行状态，不经 `useListPage`——传入静态 `ListPageState` 兼容对象）。
- Produces: `Menus`（树表：类型 1目录/2菜单/3按钮；按钮类型必填权限标识 `permission`；「新增子项」按 `parent_id` 级联；`visible`/`keep_alive`/`always_show` 开关）；`Depts`（树表：`parent_id` 级联、「新增子项」）。

- [ ] **Step 1: 写 Menus 失败测试**

`src/test/menus-page.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Menus from '@/views/system/Menus'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { SystemMenu } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listMenus: vi.fn(),
    createMenu: vi.fn().mockResolvedValue(undefined),
    updateMenu: vi.fn().mockResolvedValue(undefined),
    deleteMenu: vi.fn().mockResolvedValue(undefined),
  },
}))

const menu = (over: Partial<SystemMenu>): SystemMenu => ({
  id: '1', name: '系统', path: '/system', component: '', sort: 1, visible: true, type: 1, parentId: '0', status: 1, ...over,
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listMenus).mockResolvedValue([
    menu({
      children: [
        menu({ id: '2', name: '用户', path: '/system/user', component: 'system/user/index', type: 2, parentId: '1' }),
      ],
    }),
  ])
})

function renderMenus() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:menu:create', 'system:menu:update', 'system:menu:delete']}>
        <ConfirmProvider>
          <Menus />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Menus page', () => {
  it('renders flattened tree rows', async () => {
    renderMenus()
    await screen.findByText('系统')
    expect(screen.getByText('用户')).toBeInTheDocument()
  })

  it('button type requires permission identifier', async () => {
    renderMenus()
    await screen.findByText('系统')
    fireEvent.click(screen.getByText('新增菜单'))
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: 'btn' } })
    // 类型选按钮（3）但不填权限标识
    fireEvent.change(screen.getByLabelText('类型'), { target: { value: '3' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(screen.getByText('按钮类型必须填写权限标识')).toBeInTheDocument())
    expect(adminSystem.createMenu).not.toHaveBeenCalled()
  })

  it('create child cascades parent_id', async () => {
    renderMenus()
    await screen.findByText('系统')
    const addChildren = screen.getAllByText('新增子项')
    fireEvent.click(addChildren[0]!)
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: 'child' } })
    fireEvent.change(screen.getByLabelText('类型'), { target: { value: '2' } })
    fireEvent.change(screen.getByLabelText('路径'), { target: { value: '/system/role' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createMenu).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createMenu).mock.calls[0]![0]
    expect(body.parent_id).toBe('1')
    expect(body).toMatchObject({ name: 'child', type: 2, visible: true })
  })

  it('deletes with confirm by id', async () => {
    renderMenus()
    await screen.findByText('系统')
    fireEvent.click(screen.getAllByText('删除')[0]!)
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deleteMenu).toHaveBeenCalledWith('1'))
  })
})
```

- [ ] **Step 2: 运行验证失败**

Run: `npm run test:run -- src/test/menus-page.test.tsx` → Expected: FAIL。

- [ ] **Step 3: 实现完整 Menus 页面**

`src/views/system/Menus.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import type { ListPageState } from '@/lib/list'
import type { SystemMenu } from '@/lib/types'

interface MenuForm {
  name: string
  permission: string
  type: number
  sort: number
  parent_id: string
  path: string
  icon: string
  component: string
  component_name: string
  status: number
  visible: boolean
  keep_alive: boolean
  always_show: boolean
}

const emptyForm: MenuForm = {
  name: '', permission: '', type: 1, sort: 1, parent_id: '0', path: '', icon: '',
  component: '', component_name: '', status: 1, visible: true, keep_alive: false, always_show: false,
}

function flatten(n: SystemMenu, depth = 0): Array<{ node: SystemMenu; depth: number }> {
  const out: Array<{ node: SystemMenu; depth: number }> = [{ node: n, depth }]
  for (const c of n.children ?? []) out.push(...flatten(c, depth + 1))
  return out
}

function staticState(rows: SystemMenu[], loading: boolean): ListPageState<SystemMenu> {
  return {
    query: {}, rows, total: rows.length, page: 1, pageSize: rows.length,
    loading, exporting: false, selected: [],
    setSelected: () => {}, setQuery: () => {}, load: () => Promise.resolve(),
    search: () => Promise.resolve(), reset: () => Promise.resolve(),
    setPage: () => Promise.resolve(), setPageSize: () => Promise.resolve(),
    exportData: () => Promise.resolve(), afterMutate: () => Promise.resolve(),
  }
}

export default function Menus() {
  const confirm = useConfirm()
  const [rows, setRows] = useState<Array<{ node: SystemMenu; depth: number }>>([])
  const [loading, setLoading] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemMenu | null>(null)
  const [form, setForm] = useState<MenuForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      const tree = await adminSystem.listMenus()
      setRows(tree.flatMap((n) => flatten(n)))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (form.type === 3 && !form.permission) {
      setFormError('按钮类型必须填写权限标识')
      return
    }
    setBusy(true)
    try {
      const payload = {
        name: form.name,
        permission: form.permission || undefined,
        type: form.type, sort: form.sort, parent_id: form.parent_id,
        path: form.path || undefined, icon: form.icon || undefined,
        component: form.component || undefined, component_name: form.component_name || undefined,
        status: form.status, visible: form.visible,
        keep_alive: form.keep_alive, always_show: form.always_show,
      }
      if (editing) await adminSystem.updateMenu(editing.id, payload)
      else await adminSystem.createMenu(payload)
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await load()
    } catch (err) {
      setFormError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const doDelete = async (n: SystemMenu) => {
    const ok = await confirm({ title: `确认删除菜单「${n.name}」？（子级将级联处理）`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.deleteMenu(n.id)
    toastSuccess('删除成功')
    await load()
  }

  const columns: ListColumn<SystemMenu>[] = [
    {
      key: 'name', title: '名称',
      render: (n) => {
        const depth = rows.find((r) => r.node.id === n.id)?.depth ?? 0
        return (
          <span style={{ paddingLeft: depth * 16 }}>
            {depth > 0 && '└ '}
            {n.name}
            <Badge variant="outline" className="ml-2">{n.type === 1 ? '目录' : n.type === 2 ? '菜单' : '按钮'}</Badge>
          </span>
        )
      },
    },
    { key: 'permission', title: '权限标识', render: (n) => n.permission ?? '-' },
    { key: 'path', title: '路径', render: (n) => n.path || '-' },
    {
      key: 'op', title: '操作',
      render: (n) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:menu:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(n)
              setForm({
                name: n.name, permission: n.permission ?? '', type: n.type, sort: n.sort,
                parent_id: n.parentId, path: n.path ?? '', icon: n.icon ?? '',
                component: n.component ?? '', component_name: n.componentName ?? '',
                status: n.status, visible: n.visible, keep_alive: n.keepAlive ?? false,
                always_show: n.alwaysShow ?? false,
              })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton
            permission="system:menu:create" variant="ghost" size="sm"
            onClick={() => { setEditing(null); setForm({ ...emptyForm, parent_id: n.id }); setFormOpen(true) }}
          >新增子项</PermissionButton>
          <PermissionButton permission="system:menu:delete" variant="ghost" size="sm" onClick={() => void doDelete(n)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <ListPage
        state={staticState(rows.map((r) => r.node), loading)}
        columns={columns}
        toolbar={
          <PermissionButton
            permission="system:menu:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormOpen(true) }}
          >新增菜单</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑菜单' : '新增菜单'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field>
          <FieldLabel>类型</FieldLabel>
          <select
            className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
            value={String(form.type)}
            onChange={(e) => setForm({ ...form, type: Number(e.target.value) })}
            aria-label="类型"
          >
            <option value="1">目录</option>
            <option value="2">菜单</option>
            <option value="3">按钮</option>
          </select>
        </Field>
        <Field><FieldLabel>名称</FieldLabel><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></Field>
        <Field><FieldLabel>权限标识</FieldLabel><Input value={form.permission} onChange={(e) => setForm({ ...form, permission: e.target.value })} /></Field>
        <Field><FieldLabel>排序</FieldLabel><Input type="number" value={form.sort} onChange={(e) => setForm({ ...form, sort: Number(e.target.value) })} /></Field>
        <Field><FieldLabel>路径</FieldLabel><Input value={form.path} onChange={(e) => setForm({ ...form, path: e.target.value })} /></Field>
        <Field><FieldLabel>图标</FieldLabel><Input value={form.icon} onChange={(e) => setForm({ ...form, icon: e.target.value })} /></Field>
        {form.type === 2 && (
          <>
            <Field><FieldLabel>组件</FieldLabel><Input value={form.component} onChange={(e) => setForm({ ...form, component: e.target.value })} /></Field>
            <Field><FieldLabel>组件名</FieldLabel><Input value={form.component_name} onChange={(e) => setForm({ ...form, component_name: e.target.value })} /></Field>
          </>
        )}
        <div className="flex items-center justify-between"><FieldLabel>可见</FieldLabel><Switch checked={form.visible} onCheckedChange={(v) => setForm({ ...form, visible: v })} /></div>
        <div className="flex items-center justify-between"><FieldLabel>缓存</FieldLabel><Switch checked={form.keep_alive} onCheckedChange={(v) => setForm({ ...form, keep_alive: v })} /></div>
        <div className="flex items-center justify-between"><FieldLabel>常显</FieldLabel><Switch checked={form.always_show} onCheckedChange={(v) => setForm({ ...form, always_show: v })} /></div>
      </FormDialog>
    </div>
  )
}
```

- [ ] **Step 4: 写 Depts 失败测试**

`src/test/depts-page.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Depts from '@/views/system/Depts'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { SystemDept } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listDepts: vi.fn(),
    createDept: vi.fn().mockResolvedValue(undefined),
    updateDept: vi.fn().mockResolvedValue(undefined),
    deleteDept: vi.fn().mockResolvedValue(undefined),
  },
}))

const dept = (over: Partial<SystemDept>): SystemDept => ({
  id: '1', name: '总部', parentId: '0', sort: 1, status: 1, ...over,
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listDepts).mockResolvedValue([
    dept({ id: '1', name: '总部' }),
    dept({ id: '2', name: '研发部', parentId: '1' }),
  ])
})

function renderDepts() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:dept:create', 'system:dept:update', 'system:dept:delete']}>
        <ConfirmProvider>
          <Depts />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Depts page', () => {
  it('renders depts with parent cascade depth', async () => {
    renderDepts()
    await screen.findByText('总部')
    expect(screen.getByText('研发部')).toBeInTheDocument()
  })

  it('create child dept cascades parent_id (snake_case)', async () => {
    renderDepts()
    await screen.findByText('总部')
    fireEvent.click(screen.getAllByText('新增子项')[0]!)
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '测试部' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createDept).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createDept).mock.calls[0]![0]
    expect(body).toMatchObject({ name: '测试部', parent_id: '1', sort: expect.any(Number), status: 1 })
  })

  it('deletes with confirm by id', async () => {
    renderDepts()
    await screen.findByText('总部')
    fireEvent.click(screen.getAllByText('删除')[0]!)
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deleteDept).toHaveBeenCalledWith('1'))
  })
})
```

- [ ] **Step 5: 运行验证失败**

Run: `npm run test:run -- src/test/depts-page.test.tsx` → Expected: FAIL。

- [ ] **Step 6: 实现完整 Depts 页面**

`src/views/system/Depts.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import type { ListPageState } from '@/lib/list'
import type { SystemDept } from '@/lib/types'

interface DeptForm { name: string; parent_id: string; sort: number; phone: string; email: string; status: number }
const emptyForm: DeptForm = { name: '', parent_id: '0', sort: 1, phone: '', email: '', status: 1 }

function flatten(depts: SystemDept[], parentId = '0', depth = 0): Array<{ node: SystemDept; depth: number }> {
  const out: Array<{ node: SystemDept; depth: number }> = []
  for (const d of depts.filter((x) => x.parentId === parentId)) {
    out.push({ node: d, depth })
    out.push(...flatten(depts, d.id, depth + 1))
  }
  return out
}

function staticState(rows: SystemDept[], loading: boolean): ListPageState<SystemDept> {
  return {
    query: {}, rows, total: rows.length, page: 1, pageSize: rows.length,
    loading, exporting: false, selected: [],
    setSelected: () => {}, setQuery: () => {}, load: () => Promise.resolve(),
    search: () => Promise.resolve(), reset: () => Promise.resolve(),
    setPage: () => Promise.resolve(), setPageSize: () => Promise.resolve(),
    exportData: () => Promise.resolve(), afterMutate: () => Promise.resolve(),
  }
}

export default function Depts() {
  const confirm = useConfirm()
  const [all, setAll] = useState<SystemDept[]>([])
  const [loading, setLoading] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemDept | null>(null)
  const [form, setForm] = useState<DeptForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      setAll(await adminSystem.listDepts())
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => { void load() }, [])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.name) {
      setFormError('名称必填')
      return
    }
    setBusy(true)
    try {
      const payload = {
        name: form.name, parent_id: form.parent_id, sort: form.sort,
        phone: form.phone || undefined, email: form.email || undefined, status: form.status,
      }
      if (editing) await adminSystem.updateDept(editing.id, payload)
      else await adminSystem.createDept(payload)
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await load()
    } catch (err) {
      setFormError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const doDelete = async (d: SystemDept) => {
    const ok = await confirm({ title: `确认删除部门「${d.name}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.deleteDept(d.id)
    toastSuccess('删除成功')
    await load()
  }

  const flat = flatten(all)
  const columns: ListColumn<SystemDept>[] = [
    {
      key: 'name', title: '名称',
      render: (d) => {
        const depth = flat.find((r) => r.node.id === d.id)?.depth ?? 0
        return <span style={{ paddingLeft: depth * 16 }}>{depth > 0 && '└ '}{d.name}</span>
      },
    },
    { key: 'phone', title: '电话', render: (d) => d.phone ?? '-' },
    {
      key: 'status', title: '状态',
      render: (d) => <Badge variant={d.status === 1 ? 'default' : 'secondary'}>{d.status === 1 ? '正常' : '停用'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (d) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:dept:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(d)
              setForm({ name: d.name, parent_id: d.parentId, sort: d.sort, phone: d.phone ?? '', email: d.email ?? '', status: d.status })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton
            permission="system:dept:create" variant="ghost" size="sm"
            onClick={() => { setEditing(null); setForm({ ...emptyForm, parent_id: d.id }); setFormOpen(true) }}
          >新增子项</PermissionButton>
          <PermissionButton permission="system:dept:delete" variant="ghost" size="sm" onClick={() => void doDelete(d)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <ListPage
        state={staticState(flat.map((r) => r.node), loading)}
        columns={columns}
        toolbar={
          <PermissionButton
            permission="system:dept:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormOpen(true) }}
          >新增部门</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑部门' : '新增部门'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field><FieldLabel>名称</FieldLabel><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></Field>
        <Field><FieldLabel>电话</FieldLabel><Input value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} /></Field>
        <Field><FieldLabel>邮箱</FieldLabel><Input value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} /></Field>
        <Field><FieldLabel>排序</FieldLabel><Input type="number" value={form.sort} onChange={(e) => setForm({ ...form, sort: Number(e.target.value) })} /></Field>
        <div className="flex items-center justify-between">
          <FieldLabel>状态</FieldLabel>
          <Switch checked={form.status === 1} onCheckedChange={(v) => setForm({ ...form, status: v ? 1 : 0 })} />
        </div>
      </FormDialog>
    </div>
  )
}
```

- [ ] **Step 7: 运行测试通过并提交**

Run: `npm run test:run -- src/test/menus-page.test.tsx src/test/depts-page.test.tsx` → Expected: PASS。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): Menus and Depts tree pages with parent cascade and button-permission validation"
```

### Task 11: Tenants + Dict（双面板）+ Configs（内置控件映射）

**Files:**
- Modify: `src/views/system/Tenants.tsx`（替换 Task 3 最小版）
- Modify: `src/views/system/Dict.tsx`（替换 Task 3 最小版）
- Modify: `src/views/system/Configs.tsx`（替换 Task 3 最小版）
- Test: `src/test/tenants-page.test.tsx`、`src/test/dict-page.test.tsx`、`src/test/configs-page.test.tsx`

**Interfaces:**
- Consumes: `adminSystem.listTenants/createTenant/updateTenant/deleteTenants/listDictTypes/createDictType/updateDictType/deleteDictTypes/listDictItems/createDictItem/updateDictItem/deleteDictItems/listConfigs/createConfig/updateConfig/deleteConfigs` 及各 Write 类型（Task 3）；`useOptions`（Task 2）；CRUD 样板（Task 8）。
- Produces: `Tenants`（分页 CRUD，`contact_name`/`contact_mobile`/`expire_time` snake_case）；`Dict`（**双面板**：左类型列表 + 右数据项列表，选中类型加载其数据项；各自 CRUD）；`Configs`（分页 CRUD；**内置（type=1）不可删除**；内置键控件映射：`sys.account.registerUser`/`sys.captcha.enabled` → Switch，`sys.captcha.type` → 字典 `sys_captcha_type` 下拉，其余 Input）。

- [ ] **Step 1: 写 Tenants 失败测试**

`src/test/tenants-page.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Tenants from '@/views/system/Tenants'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { PageResult, SystemTenant } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listTenants: vi.fn(),
    createTenant: vi.fn().mockResolvedValue(undefined),
    updateTenant: vi.fn().mockResolvedValue(undefined),
    deleteTenants: vi.fn().mockResolvedValue(undefined),
  },
}))

const tenant = (over: Partial<SystemTenant>): SystemTenant => ({
  id: '1', name: 'ACME', code: 'acme', contactName: '张三', contactMobile: '13800000000', ...over,
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listTenants).mockResolvedValue({
    list: [tenant({})], total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<SystemTenant>)
})

function renderTenants() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['platform:tenant:create', 'platform:tenant:update', 'platform:tenant:delete']}>
        <ConfirmProvider>
          <Tenants />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Tenants page', () => {
  it('renders tenants', async () => {
    renderTenants()
    await screen.findByText('ACME')
    expect(screen.getByText('acme')).toBeInTheDocument()
  })

  it('creates tenant with snake_case payload', async () => {
    renderTenants()
    await screen.findByText('ACME')
    fireEvent.click(screen.getByText('新增租户'))
    fireEvent.change(screen.getByLabelText('租户名称'), { target: { value: 'Beta' } })
    fireEvent.change(screen.getByLabelText('租户编码'), { target: { value: 'beta' } })
    fireEvent.change(screen.getByLabelText('联系人'), { target: { value: '李四' } })
    fireEvent.change(screen.getByLabelText('联系电话'), { target: { value: '139' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createTenant).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createTenant).mock.calls[0]![0]
    expect(body).toMatchObject({ name: 'Beta', code: 'beta', contact_name: '李四', contact_mobile: '139' })
  })

  it('deletes with confirm', async () => {
    renderTenants()
    await screen.findByText('ACME')
    fireEvent.click(screen.getByText('删除'))
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deleteTenants).toHaveBeenCalledWith(['1']))
  })
})
```

- [ ] **Step 2: 实现完整 Tenants 页面**

`src/views/system/Tenants.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import type { SystemTenant } from '@/lib/types'

interface TenantForm {
  name: string; code: string; contact_name: string; contact_mobile: string
  expire_time: string; remark: string; admin_username: string; admin_password: string
}
const emptyForm: TenantForm = {
  name: '', code: '', contact_name: '', contact_mobile: '',
  expire_time: '', remark: '', admin_username: '', admin_password: '',
}

export default function Tenants() {
  const confirm = useConfirm()
  const list = useListPage<SystemTenant>({ fetcher: (p) => adminSystem.listTenants(p) })
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemTenant | null>(null)
  const [form, setForm] = useState<TenantForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => { void list.load() }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.name || !form.code || !form.contact_name) {
      setFormError('名称/编码/联系人必填')
      return
    }
    setBusy(true)
    try {
      const base = {
        name: form.name, code: form.code,
        contact_name: form.contact_name, contact_mobile: form.contact_mobile,
        expire_time: form.expire_time || undefined, remark: form.remark || undefined,
      }
      if (editing) await adminSystem.updateTenant(editing.id, base)
      else await adminSystem.createTenant({
        ...base, admin_username: form.admin_username, admin_password: form.admin_password,
      })
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await list.afterMutate()
    } catch (err) {
      setFormError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const doDelete = async (t: SystemTenant) => {
    const ok = await confirm({ title: `确认删除租户「${t.name}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.deleteTenants([t.id])
    toastSuccess('删除成功')
    await list.afterMutate()
  }

  const columns: ListColumn<SystemTenant>[] = [
    { key: 'name', title: '名称' },
    { key: 'code', title: '编码' },
    { key: 'contactName', title: '联系人' },
    { key: 'contactMobile', title: '联系电话' },
    {
      key: 'op', title: '操作',
      render: (t) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="platform:tenant:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(t)
              setForm({
                name: t.name, code: t.code, contact_name: t.contactName, contact_mobile: t.contactMobile,
                expire_time: t.expireTime ?? '', remark: t.remark ?? '', admin_username: '', admin_password: '',
              })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton permission="platform:tenant:delete" variant="ghost" size="sm" onClick={() => void doDelete(t)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'name', label: '名称' }, { key: 'code', label: '编码' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns}
        toolbar={
          <PermissionButton
            permission="platform:tenant:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormOpen(true) }}
          >新增租户</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑租户' : '新增租户'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field><FieldLabel>租户名称</FieldLabel><Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></Field>
        <Field><FieldLabel>租户编码</FieldLabel><Input value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} disabled={!!editing} required /></Field>
        <Field><FieldLabel>联系人</FieldLabel><Input value={form.contact_name} onChange={(e) => setForm({ ...form, contact_name: e.target.value })} required /></Field>
        <Field><FieldLabel>联系电话</FieldLabel><Input value={form.contact_mobile} onChange={(e) => setForm({ ...form, contact_mobile: e.target.value })} /></Field>
        <Field><FieldLabel>过期时间（YYYY-MM-DD）</FieldLabel><Input value={form.expire_time} onChange={(e) => setForm({ ...form, expire_time: e.target.value })} placeholder="2027-12-31" /></Field>
        {!editing && (
          <>
            <Field><FieldLabel>管理员账号</FieldLabel><Input value={form.admin_username} onChange={(e) => setForm({ ...form, admin_username: e.target.value })} required /></Field>
            <Field><FieldLabel>管理员密码</FieldLabel><Input type="password" value={form.admin_password} onChange={(e) => setForm({ ...form, admin_password: e.target.value })} required /></Field>
          </>
        )}
        <Field><FieldLabel>备注</FieldLabel><Textarea value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} /></Field>
      </FormDialog>
    </div>
  )
}
```

- [ ] **Step 3: 写 Dict 失败测试**

`src/test/dict-page.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Dict from '@/views/system/Dict'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { DictItem, DictType, PageResult } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listDictTypes: vi.fn(),
    createDictType: vi.fn().mockResolvedValue(undefined),
    updateDictType: vi.fn().mockResolvedValue(undefined),
    deleteDictTypes: vi.fn().mockResolvedValue(undefined),
    listDictItems: vi.fn(),
    createDictItem: vi.fn().mockResolvedValue(undefined),
    updateDictItem: vi.fn().mockResolvedValue(undefined),
    deleteDictItems: vi.fn().mockResolvedValue(undefined),
  },
}))

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listDictTypes).mockResolvedValue({
    list: [{ id: '1', name: '验证码类型', type: 'sys_captcha_type', status: 1 } as DictType],
    total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<DictType>)
  vi.mocked(adminSystem.listDictItems).mockResolvedValue([
    { id: 'i1', dictType: 'sys_captcha_type', label: '滑动', value: 'slide', sort: 1 } as DictItem,
  ])
})

function renderDict() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:dict:create', 'system:dict:update', 'system:dict:delete']}>
        <ConfirmProvider>
          <Dict />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Dict page (two panels)', () => {
  it('loads type list then items of selected type', async () => {
    renderDict()
    await screen.findByText('验证码类型')
    fireEvent.click(screen.getByText('验证码类型'))
    await screen.findByText('滑动')
    expect(adminSystem.listDictItems).toHaveBeenCalledWith('sys_captcha_type')
  })

  it('creates dict item with snake_case payload', async () => {
    renderDict()
    await screen.findByText('验证码类型')
    fireEvent.click(screen.getByText('验证码类型'))
    await screen.findByText('滑动')
    fireEvent.click(screen.getByText('新增数据项'))
    fireEvent.change(screen.getByLabelText('标签'), { target: { value: '旋转' } })
    fireEvent.change(screen.getByLabelText('值'), { target: { value: 'rotate' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createDictItem).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createDictItem).mock.calls[0]![0]
    expect(body).toMatchObject({ dict_type: 'sys_captcha_type', label: '旋转', value: 'rotate', sort: expect.any(Number) })
  })

  it('creates dict type with snake_case payload', async () => {
    renderDict()
    await screen.findByText('验证码类型')
    fireEvent.click(screen.getByText('新增类型'))
    fireEvent.change(screen.getByLabelText('类型名称'), { target: { value: '状态' } })
    fireEvent.change(screen.getByLabelText('类型编码'), { target: { value: 'sys_status' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createDictType).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createDictType).mock.calls[0]![0]
    expect(body).toMatchObject({ name: '状态', type: 'sys_status', status: 1 })
  })
})
```

- [ ] **Step 4: 实现完整 Dict 双面板页面**

`src/views/system/Dict.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import type { ListPageState } from '@/lib/list'
import type { DictItem, DictType } from '@/lib/types'

interface TypeForm { name: string; type: string; status: number; remark: string }
interface ItemForm { label: string; value: string; sort: number }

function staticState(rows: DictItem[], loading: boolean): ListPageState<DictItem> {
  return {
    query: {}, rows, total: rows.length, page: 1, pageSize: rows.length,
    loading, exporting: false, selected: [],
    setSelected: () => {}, setQuery: () => {}, load: () => Promise.resolve(),
    search: () => Promise.resolve(), reset: () => Promise.resolve(),
    setPage: () => Promise.resolve(), setPageSize: () => Promise.resolve(),
    exportData: () => Promise.resolve(), afterMutate: () => Promise.resolve(),
  }
}

export default function Dict() {
  const confirm = useConfirm()
  const typeList = useListPage<DictType>({ fetcher: (p) => adminSystem.listDictTypes(p) })
  const [selected, setSelected] = useState<DictType | null>(null)
  const [items, setItems] = useState<DictItem[]>([])
  const [itemsLoading, setItemsLoading] = useState(false)
  const [typeFormOpen, setTypeFormOpen] = useState(false)
  const [editingType, setEditingType] = useState<DictType | null>(null)
  const [typeForm, setTypeForm] = useState<TypeForm>({ name: '', type: '', status: 1, remark: '' })
  const [itemFormOpen, setItemFormOpen] = useState(false)
  const [editingItem, setEditingItem] = useState<DictItem | null>(null)
  const [itemForm, setItemForm] = useState<ItemForm>({ label: '', value: '', sort: 1 })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => { void typeList.load() }, [typeList.page, typeList.pageSize, typeList.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const loadItems = async (t: DictType) => {
    setSelected(t)
    setItemsLoading(true)
    try {
      setItems(await adminSystem.listDictItems(t.type))
    } finally {
      setItemsLoading(false)
    }
  }

  const submitType = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!typeForm.name || !typeForm.type) {
      setFormError('类型名称与编码必填')
      return
    }
    setBusy(true)
    try {
      const payload = {
        name: typeForm.name, type: typeForm.type, status: typeForm.status,
        remark: typeForm.remark || undefined,
      }
      if (editingType) await adminSystem.updateDictType(editingType.id, payload)
      else await adminSystem.createDictType(payload)
      toastSuccess(editingType ? '更新成功' : '创建成功')
      setTypeFormOpen(false)
      await typeList.afterMutate()
    } catch (err) {
      setFormError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const submitItem = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!selected) return
    if (!itemForm.label || !itemForm.value) {
      setFormError('标签与值必填')
      return
    }
    setBusy(true)
    try {
      const payload = {
        dict_type: selected.type, label: itemForm.label, value: itemForm.value, sort: itemForm.sort,
      }
      if (editingItem) await adminSystem.updateDictItem(editingItem.id, payload)
      else await adminSystem.createDictItem(payload)
      toastSuccess(editingItem ? '更新成功' : '创建成功')
      setItemFormOpen(false)
      await loadItems(selected)
    } catch (err) {
      setFormError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const doDeleteType = async (t: DictType) => {
    const ok = await confirm({ title: `确认删除字典类型「${t.name}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.deleteDictTypes([t.id])
    toastSuccess('删除成功')
    if (selected?.id === t.id) {
      setSelected(null)
      setItems([])
    }
    await typeList.afterMutate()
  }

  const doDeleteItem = async (i: DictItem) => {
    const ok = await confirm({ title: `确认删除数据项「${i.label}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.deleteDictItems([i.id])
    toastSuccess('删除成功')
    if (selected) await loadItems(selected)
  }

  const typeColumns: ListColumn<DictType>[] = [
    { key: 'name', title: '名称' },
    { key: 'type', title: '编码' },
    {
      key: 'status', title: '状态',
      render: (t) => <Badge variant={t.status === 1 ? 'default' : 'secondary'}>{t.status === 1 ? '正常' : '停用'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (t) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:dict:update" variant="ghost" size="sm"
            onClick={() => {
              setEditingType(t)
              setTypeForm({ name: t.name, type: t.type, status: t.status, remark: t.remark ?? '' })
              setFormError(null)
              setTypeFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton permission="system:dict:delete" variant="ghost" size="sm" onClick={() => void doDeleteType(t)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  const itemColumns: ListColumn<DictItem>[] = [
    { key: 'label', title: '标签' },
    { key: 'value', title: '值' },
    { key: 'sort', title: '排序' },
    {
      key: 'op', title: '操作',
      render: (i) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:dict:update" variant="ghost" size="sm"
            onClick={() => {
              setEditingItem(i)
              setItemForm({ label: i.label, value: i.value, sort: i.sort })
              setFormError(null)
              setItemFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton permission="system:dict:delete" variant="ghost" size="sm" onClick={() => void doDeleteItem(i)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader><CardTitle>字典类型</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <SearchForm
            fields={[{ key: 'name', label: '名称' }]}
            value={typeList.query} onSearch={() => void typeList.search()} onReset={() => void typeList.reset()}
          />
          <ListPage
            state={typeList}
            columns={typeColumns}
            toolbar={
              <PermissionButton
                permission="system:dict:create"
                onClick={() => { setEditingType(null); setTypeForm({ name: '', type: '', status: 1, remark: '' }); setTypeFormOpen(true) }}
              >新增类型</PermissionButton>
            }
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>数据项{selected ? `（${selected.name}）` : ''}</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          {!selected && <p className="text-muted-foreground text-sm">请先在左侧选择一个字典类型</p>}
          {selected && (
            <ListPage
              state={staticState(items, itemsLoading)}
              columns={itemColumns}
              toolbar={
                <PermissionButton
                  permission="system:dict:create"
                  onClick={() => { setEditingItem(null); setItemForm({ label: '', value: '', sort: 1 }); setItemFormOpen(true) }}
                >新增数据项</PermissionButton>
              }
            />
          )}
        </CardContent>
      </Card>
      <FormDialog open={typeFormOpen} onOpenChange={setTypeFormOpen} title={editingType ? '编辑字典类型' : '新增字典类型'} onSubmit={submitType} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field><FieldLabel>类型名称</FieldLabel><Input value={typeForm.name} onChange={(e) => setTypeForm({ ...typeForm, name: e.target.value })} required /></Field>
        <Field><FieldLabel>类型编码</FieldLabel><Input value={typeForm.type} onChange={(e) => setTypeForm({ ...typeForm, type: e.target.value })} disabled={!!editingType} required /></Field>
        <div className="flex items-center justify-between">
          <FieldLabel>状态</FieldLabel>
          <Switch checked={typeForm.status === 1} onCheckedChange={(v) => setTypeForm({ ...typeForm, status: v ? 1 : 0 })} />
        </div>
        <Field><FieldLabel>备注</FieldLabel><Input value={typeForm.remark} onChange={(e) => setTypeForm({ ...typeForm, remark: e.target.value })} /></Field>
      </FormDialog>
      <FormDialog open={itemFormOpen} onOpenChange={setItemFormOpen} title={editingItem ? '编辑数据项' : '新增数据项'} onSubmit={submitItem} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field><FieldLabel>标签</FieldLabel><Input value={itemForm.label} onChange={(e) => setItemForm({ ...itemForm, label: e.target.value })} required /></Field>
        <Field><FieldLabel>值</FieldLabel><Input value={itemForm.value} onChange={(e) => setItemForm({ ...itemForm, value: e.target.value })} required /></Field>
        <Field><FieldLabel>排序</FieldLabel><Input type="number" value={itemForm.sort} onChange={(e) => setItemForm({ ...itemForm, sort: Number(e.target.value) })} /></Field>
      </FormDialog>
    </div>
  )
}
```

- [ ] **Step 5: 写 Configs 失败测试**

`src/test/configs-page.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Configs from '@/views/system/Configs'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import { invalidateOptions } from '@/lib/options'
import type { PageResult, SystemConfig } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listConfigs: vi.fn(),
    createConfig: vi.fn().mockResolvedValue(undefined),
    updateConfig: vi.fn().mockResolvedValue(undefined),
    deleteConfigs: vi.fn().mockResolvedValue(undefined),
    listDictItems: vi.fn().mockResolvedValue([
      { id: '1', dictType: 'sys_captcha_type', label: '滑动', value: 'slide', sort: 1 },
      { id: '2', dictType: 'sys_captcha_type', label: '旋转', value: 'rotate', sort: 2 },
    ]),
  },
}))

const config = (over: Partial<SystemConfig>): SystemConfig => ({
  id: '1', configKey: 'sys.account.registerUser', configName: '是否允许自注册',
  configValue: 'true', type: 1, visible: true, ...over,
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  invalidateOptions()
  vi.mocked(adminSystem.listConfigs).mockResolvedValue({
    list: [
      config({}),
      config({ id: '2', configKey: 'sys.captcha.type', configName: '验证码类型', configValue: 'slide', type: 1 }),
      config({ id: '3', configKey: 'custom.key', configName: '自定义', configValue: 'v1', type: 2 }),
    ],
    total: 3, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<SystemConfig>)
})

function renderConfigs() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:config:create', 'system:config:update', 'system:config:delete']}>
        <ConfirmProvider>
          <Configs />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Configs page', () => {
  it('renders rows with builtin badge', async () => {
    renderConfigs()
    await screen.findByText('sys.account.registerUser')
    expect(screen.getByText('内置')).toBeInTheDocument()
  })

  it('builtin rows cannot be deleted (no delete button)', async () => {
    renderConfigs()
    await screen.findByText('sys.account.registerUser')
    // type=1 两行无删除按钮；type=2 一行有
    expect(screen.getAllByText('删除')).toHaveLength(1)
  })

  it('edits builtin switch key and sends snake_case', async () => {
    renderConfigs()
    await screen.findByText('sys.account.registerUser')
    fireEvent.click(screen.getAllByText('编辑')[0]!)
    const switches = screen.getAllByRole('switch')
    fireEvent.click(switches[switches.length - 1]!)
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.updateConfig).toHaveBeenCalled())
    const [id, body] = vi.mocked(adminSystem.updateConfig).mock.calls[0]!
    expect(id).toBe('1')
    expect(body).toMatchObject({ config_key: 'sys.account.registerUser' })
    expect(Object.keys(body)).not.toContain('configKey')
  })

  it('captcha type renders dict-driven select', async () => {
    renderConfigs()
    await screen.findByText('sys.captcha.type')
    fireEvent.click(screen.getAllByText('编辑')[1]!)
    const select = await screen.findByLabelText('参数值')
    expect(select.tagName).toBe('SELECT')
    expect(screen.getByText('滑动')).toBeInTheDocument()
  })
})
```

- [ ] **Step 6: 实现完整 Configs 页面**

`src/views/system/Configs.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useOptions } from '@/lib/options'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import type { DictItem, SystemConfig } from '@/lib/types'

interface ConfigForm {
  config_key: string; config_name: string; config_value: string
  type: number; visible: boolean; remark: string
}
const emptyForm: ConfigForm = { config_key: '', config_name: '', config_value: '', type: 2, visible: true, remark: '' }

// 内置键 → 控件映射
function controlFor(key: string): 'switch' | 'captcha-type-select' | 'input' {
  if (key === 'sys.account.registerUser' || key === 'sys.captcha.enabled') return 'switch'
  if (key === 'sys.captcha.type') return 'captcha-type-select'
  return 'input'
}

export default function Configs() {
  const confirm = useConfirm()
  const list = useListPage<SystemConfig>({ fetcher: (p) => adminSystem.listConfigs(p) })
  const captchaTypeOptions = useOptions<DictItem>(
    'dict:sys_captcha_type',
    () => adminSystem.listDictItems('sys_captcha_type'),
  )
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemConfig | null>(null)
  const [form, setForm] = useState<ConfigForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => { void list.load() }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.config_key || !form.config_name) {
      setFormError('键与名称必填')
      return
    }
    setBusy(true)
    try {
      const payload = {
        config_key: form.config_key, config_name: form.config_name, config_value: form.config_value,
        type: form.type, visible: form.visible, remark: form.remark || undefined,
      }
      if (editing) await adminSystem.updateConfig(editing.id, payload)
      else await adminSystem.createConfig(payload)
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await list.afterMutate()
    } catch (err) {
      setFormError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const doDelete = async (c: SystemConfig) => {
    if (c.type === 1) return // 内置不可删除（双保险）
    const ok = await confirm({ title: `确认删除参数「${c.configName}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.deleteConfigs([c.id])
    toastSuccess('删除成功')
    await list.afterMutate()
  }

  const columns: ListColumn<SystemConfig>[] = [
    { key: 'configKey', title: '键' },
    { key: 'configName', title: '名称' },
    { key: 'configValue', title: '值' },
    {
      key: 'type', title: '类型',
      render: (c) => <Badge variant={c.type === 1 ? 'secondary' : 'outline'}>{c.type === 1 ? '内置' : '自定义'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (c) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:config:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(c)
              setForm({
                config_key: c.configKey, config_name: c.configName, config_value: c.configValue,
                type: c.type, visible: c.visible, remark: c.remark ?? '',
              })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          {c.type !== 1 && (
            <PermissionButton permission="system:config:delete" variant="ghost" size="sm" onClick={() => void doDelete(c)}>删除</PermissionButton>
          )}
        </div>
      ),
    },
  ]

  const control = editing ? controlFor(editing.configKey) : 'input'

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'config_key', label: '键' }, { key: 'config_name', label: '名称' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns}
        toolbar={
          <PermissionButton
            permission="system:config:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormOpen(true) }}
          >新增参数</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑参数' : '新增参数'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field>
          <FieldLabel>参数键</FieldLabel>
          <Input value={form.config_key} onChange={(e) => setForm({ ...form, config_key: e.target.value })} disabled={!!editing} required />
        </Field>
        <Field>
          <FieldLabel>参数名称</FieldLabel>
          <Input value={form.config_name} onChange={(e) => setForm({ ...form, config_name: e.target.value })} required />
        </Field>
        <Field>
          <FieldLabel>参数值</FieldLabel>
          {control === 'switch' && (
            <Switch
              checked={form.config_value === 'true'}
              onCheckedChange={(v) => setForm({ ...form, config_value: v ? 'true' : 'false' })}
              aria-label="参数值"
            />
          )}
          {control === 'captcha-type-select' && (
            <select
              className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
              value={form.config_value}
              onChange={(e) => setForm({ ...form, config_value: e.target.value })}
              aria-label="参数值"
            >
              {captchaTypeOptions.data.map((o) => (
                <option key={o.id} value={o.value}>{o.label}</option>
              ))}
            </select>
          )}
          {control === 'input' && (
            <Input value={form.config_value} onChange={(e) => setForm({ ...form, config_value: e.target.value })} aria-label="参数值" />
          )}
        </Field>
        <div className="flex items-center justify-between">
          <FieldLabel>可见</FieldLabel>
          <Switch checked={form.visible} onCheckedChange={(v) => setForm({ ...form, visible: v })} />
        </div>
        <Field><FieldLabel>备注</FieldLabel><Input value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} /></Field>
      </FormDialog>
    </div>
  )
}
```

- [ ] **Step 7: 运行全部三个测试通过并提交**

Run: `npm run test:run -- src/test/tenants-page.test.tsx src/test/dict-page.test.tsx src/test/configs-page.test.tsx` → Expected: PASS。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): Tenants, Dict two-panel and Configs pages with builtin guards and control mapping"
```

### Task 12: 日志页面（LoginLogs + OperationLogs 含详情抽屉）

**Files:**
- Modify: `src/views/monitor/LoginLogs.tsx`（替换 Task 3 最小版）
- Modify: `src/views/monitor/OperationLogs.tsx`（替换 Task 3 最小版）
- Test: `src/test/logs-pages.test.tsx`

**Interfaces:**
- Consumes: `adminSystem.listLoginLogs/cleanLoginLogs/removeLoginLogs/listOperationLogs/cleanOperationLogs/removeOperationLogs`（Task 3）；`useListPage`/`ListPage`/`SearchForm`/`PermissionButton`/`useConfirm`；`formatDateTime`（Task 2）；shadcn `Sheet`（详情抽屉）。
- Produces: `LoginLogs`（列表 + 查询 + 清空 `system:loginlog:clean` + 批量删除 `system:loginlog:remove`，可选行选择）；`OperationLogs`（列表 + 查询 + 清空/批量删除 + **详情抽屉**展示 request/response）。日志页无新增/编辑。

- [ ] **Step 1: 写失败测试**

`src/test/logs-pages.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import LoginLogs from '@/views/monitor/LoginLogs'
import OperationLogs from '@/views/monitor/OperationLogs'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { LoginLog, OperationLog, PageResult } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listLoginLogs: vi.fn(),
    cleanLoginLogs: vi.fn().mockResolvedValue(undefined),
    removeLoginLogs: vi.fn().mockResolvedValue(undefined),
    listOperationLogs: vi.fn(),
    cleanOperationLogs: vi.fn().mockResolvedValue(undefined),
    removeOperationLogs: vi.fn().mockResolvedValue(undefined),
  },
}))

const loginLog: LoginLog = { id: 'l1', username: 'admin', ip: '1.2.3.4', status: 1, msg: 'ok', loginAt: '2026-01-02 03:04:05' }
const opLog: OperationLog = {
  id: 'o1', username: 'admin', module: 'user', action: 'create', method: 'POST',
  uri: '/system/api/v1/system/users', ip: '1.2.3.4', statusCode: 200,
  request: '{"username":"u1"}', response: '{"ok":true}', createdAt: '2026-01-02 03:04:05',
}

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listLoginLogs).mockResolvedValue({
    list: [loginLog], total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<LoginLog>)
  vi.mocked(adminSystem.listOperationLogs).mockResolvedValue({
    list: [opLog], total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<OperationLog>)
})

function wrap(ui: React.ReactElement) {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:loginlog:clean', 'system:loginlog:remove', 'system:operationlog:clean', 'system:operationlog:remove']}>
        <ConfirmProvider>{ui}</ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('LoginLogs page', () => {
  it('renders login logs', async () => {
    wrap(<LoginLogs />)
    await screen.findByText('admin')
    expect(adminSystem.listLoginLogs).toHaveBeenCalledWith(expect.objectContaining({ page_num: 1 }))
  })

  it('clean requires confirm and calls cleanLoginLogs', async () => {
    wrap(<LoginLogs />)
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('清空'))
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.cleanLoginLogs).toHaveBeenCalled())
  })

  it('batch remove sends selected ids', async () => {
    wrap(<LoginLogs />)
    await screen.findByText('admin')
    fireEvent.click(screen.getByLabelText('全选'))
    fireEvent.click(screen.getByText('批量删除'))
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.removeLoginLogs).toHaveBeenCalledWith(['l1']))
  })
})

describe('OperationLogs page', () => {
  it('renders op logs and opens detail drawer with request/response', async () => {
    wrap(<OperationLogs />)
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('详情'))
    expect(await screen.findByText(/"username":"u1"/)).toBeInTheDocument()
    expect(screen.getByText(/"ok":true/)).toBeInTheDocument()
  })

  it('batch remove sends selected ids', async () => {
    wrap(<OperationLogs />)
    await screen.findByText('admin')
    fireEvent.click(screen.getByLabelText('全选'))
    fireEvent.click(screen.getByText('批量删除'))
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.removeOperationLogs).toHaveBeenCalledWith(['o1']))
  })
})
```

- [ ] **Step 2: 实现完整 LoginLogs 页面**

`src/views/monitor/LoginLogs.tsx`：

```tsx
import { useEffect } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastSuccess } from '@/lib/toast'
import { formatDateTime } from '@/lib/datetime'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { LoginLog } from '@/lib/types'

export default function LoginLogs() {
  const confirm = useConfirm()
  const list = useListPage<LoginLog>({ fetcher: (p) => adminSystem.listLoginLogs(p) })

  useEffect(() => { void list.load() }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const doClean = async () => {
    const ok = await confirm({ title: '确认清空全部登录日志？', description: '此操作不可恢复', danger: true, confirmText: '清空' })
    if (!ok) return
    await adminSystem.cleanLoginLogs()
    toastSuccess('已清空')
    await list.afterMutate()
  }

  const doBatchRemove = async () => {
    const ok = await confirm({ title: `确认删除选中的 ${list.selected.length} 条日志？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.removeLoginLogs(list.selected)
    toastSuccess('删除成功')
    await list.afterMutate()
  }

  const columns: ListColumn<LoginLog>[] = [
    { key: 'username', title: '账号' },
    { key: 'ip', title: 'IP' },
    {
      key: 'status', title: '状态',
      render: (r) => <Badge variant={r.status === 1 ? 'default' : 'destructive'}>{r.status === 1 ? '成功' : '失败'}</Badge>,
    },
    { key: 'msg', title: '消息' },
    { key: 'loginAt', title: '时间', render: (r) => formatDateTime(r.loginAt) },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'username', label: '账号' }, { key: 'ip', label: 'IP' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns} selectable
        toolbar={
          <div className="flex gap-2">
            <PermissionButton permission="system:loginlog:clean" variant="destructive" onClick={() => void doClean()}>清空</PermissionButton>
            <PermissionButton permission="system:loginlog:remove" variant="destructive" onClick={() => void doBatchRemove()}>批量删除</PermissionButton>
          </div>
        }
        batchActions={<Button size="sm" variant="destructive" onClick={() => void doBatchRemove()}>删除选中</Button>}
      />
    </div>
  )
}
```

- [ ] **Step 3: 实现完整 OperationLogs 页面（含详情抽屉）**

`src/views/monitor/OperationLogs.tsx`：

```tsx
import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastSuccess } from '@/lib/toast'
import { formatDateTime } from '@/lib/datetime'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import type { OperationLog } from '@/lib/types'

export default function OperationLogs() {
  const confirm = useConfirm()
  const list = useListPage<OperationLog>({ fetcher: (p) => adminSystem.listOperationLogs(p) })
  const [detail, setDetail] = useState<OperationLog | null>(null)

  useEffect(() => { void list.load() }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const doClean = async () => {
    const ok = await confirm({ title: '确认清空全部操作日志？', description: '此操作不可恢复', danger: true, confirmText: '清空' })
    if (!ok) return
    await adminSystem.cleanOperationLogs()
    toastSuccess('已清空')
    await list.afterMutate()
  }

  const doBatchRemove = async () => {
    const ok = await confirm({ title: `确认删除选中的 ${list.selected.length} 条日志？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.removeOperationLogs(list.selected)
    toastSuccess('删除成功')
    await list.afterMutate()
  }

  const columns: ListColumn<OperationLog>[] = [
    { key: 'username', title: '账号' },
    { key: 'module', title: '模块' },
    { key: 'action', title: '动作' },
    { key: 'method', title: '方法' },
    { key: 'uri', title: 'URI', className: 'max-w-48 truncate' },
    { key: 'statusCode', title: '状态码' },
    { key: 'createdAt', title: '时间', render: (r) => formatDateTime(r.createdAt) },
    {
      key: 'op', title: '操作',
      render: (r) => <Button variant="ghost" size="sm" onClick={() => setDetail(r)}>详情</Button>,
    },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'username', label: '账号' }, { key: 'module', label: '模块' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns} selectable
        toolbar={
          <div className="flex gap-2">
            <PermissionButton permission="system:operationlog:clean" variant="destructive" onClick={() => void doClean()}>清空</PermissionButton>
            <PermissionButton permission="system:operationlog:remove" variant="destructive" onClick={() => void doBatchRemove()}>批量删除</PermissionButton>
          </div>
        }
        batchActions={<Button size="sm" variant="destructive" onClick={() => void doBatchRemove()}>删除选中</Button>}
      />
      <Sheet open={detail != null} onOpenChange={(o) => { if (!o) setDetail(null) }}>
        <SheetContent className="w-96 overflow-auto">
          <SheetHeader><SheetTitle>操作日志详情</SheetTitle></SheetHeader>
          {detail && (
            <div className="space-y-2 text-sm">
              <div><span className="text-muted-foreground">账号：</span>{detail.username}</div>
              <div><span className="text-muted-foreground">模块/动作：</span>{detail.module} / {detail.action}</div>
              <div><span className="text-muted-foreground">方法：</span>{detail.method} {detail.uri}</div>
              <div><span className="text-muted-foreground">IP：</span>{detail.ip}</div>
              <div>
                <span className="text-muted-foreground">状态码：</span>
                <Badge variant={detail.statusCode === 200 ? 'default' : 'destructive'}>{detail.statusCode ?? '-'}</Badge>
              </div>
              <div><span className="text-muted-foreground">时间：</span>{formatDateTime(detail.createdAt)}</div>
              <div>
                <div className="text-muted-foreground">请求体：</div>
                <pre className="bg-muted mt-1 overflow-auto rounded p-2 text-xs">{detail.request ?? '-'}</pre>
              </div>
              <div>
                <div className="text-muted-foreground">响应体：</div>
                <pre className="bg-muted mt-1 overflow-auto rounded p-2 text-xs">{detail.response ?? '-'}</pre>
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  )
}
```

- [ ] **Step 4: 运行测试通过并提交**

Run: `npm run test:run -- src/test/logs-pages.test.tsx` → Expected: PASS。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): login/operation log pages with clean/batch-remove and op-log detail drawer"
```

### Task 13: Portal 基座（hash 路由 + portal session + 验证码 + 静态玻璃首页 + Docs）

**Files:**
- Modify: `src/portal/App.tsx`（替换 Task 1 最小版）
- Create: `src/portal/useHashRoute.ts`
- Create: `src/portal/components/PortalDialog.tsx`
- Create: `src/portal/components/PortalCaptchaDialog.tsx`
- Create: `src/portal/pages/Home.tsx`
- Create: `src/portal/pages/Docs.tsx`
- Test: `src/test/portal-base.test.tsx`

**Interfaces:**
- Consumes: `portalSession`/`portalRequest`（Task 2）、`portalAuth.getCaptcha/logout`（Task 3）、`captchaEnabled`/`buildCaptchaAnswer`（Task 2）、shadcn Dialog/Button。
- Produces:
  - `useHashRoute(): { path: string; navigate(to: string): void }` — 解析 `location.hash`（`#/login` → `/login`），`navigate` 写 hash 并监听 `hashchange`。合法路由：`/`、`/login`、`/register`、`/profile`、`/docs`；未知 hash 落 `/`。
  - `PortalDialog({ open, onOpenChange, title, children })` — portal 侧对话框统一入口（继承入口根已应用的主题，不重命名 `data-slot`）。
  - `PortalCaptchaDialog({ open, challenge, onAnswer, onRefresh, onCancel })` — 四题型交互，作答经 `onAnswer(answer: CaptchaAnswer)` 交回。
  - `Home({ navigate, loggedIn })`：静态玻璃视觉营销首页，保留品牌文案 `Design World`、`Explore New Ideas`、`Explore Now`（点击 `navigate('/register')`）、`Sign In`。
  - `Docs`：`#/docs` 静态文档页（spec §5.3 明确的静态页，有确定内容）。
  - `PortalApp`：导航栏（Design World 品牌、Sign In/Sign Up/退出入口）+ hash 路由出口；已登录访问 `/login`/`/register` 渲染 Profile（Task 14 接入）；登出清理 `portal_tokens` 回 `/`。本任务的 Login/Register/Profile 路由先用 Task 14 将替换的最小占位——`loginPage`/`ProfileStub` 内联组件（在 Task 14 Step 4 中被删除）。

- [ ] **Step 1: 创建 `src/portal/useHashRoute.ts`**

```ts
import { useCallback, useEffect, useState } from 'react'

export const PORTAL_ROUTES = ['/', '/login', '/register', '/profile', '/docs'] as const

function parseHash(): string {
  const h = window.location.hash.replace(/^#/, '')
  if (!h || h === '/') return '/'
  return h.startsWith('/') ? h : `/${h}`
}

export function useHashRoute(): { path: string; navigate: (to: string) => void } {
  const [path, setPath] = useState(parseHash)

  useEffect(() => {
    const onHash = () => setPath(parseHash())
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  const navigate = useCallback((to: string) => {
    const normalized = to.startsWith('/') ? to : `/${to}`
    window.location.hash = `#${normalized}`
    setPath(normalized)
  }, [])

  return { path, navigate }
}
```

- [ ] **Step 2: 创建 `src/portal/components/PortalDialog.tsx` 与 `PortalCaptchaDialog.tsx`**

`PortalDialog.tsx`：

```tsx
import { type ReactNode } from 'react'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

// portal 对话框统一入口：继承入口根已应用的主题（html.dark），不得重命名 data-slot
export function PortalDialog({
  open, onOpenChange, title, children,
}: {
  open: boolean
  onOpenChange: (o: boolean) => void
  title: string
  children: ReactNode
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md">
        <DialogHeader><DialogTitle>{title}</DialogTitle></DialogHeader>
        {children}
      </DialogContent>
    </Dialog>
  )
}
```

`PortalCaptchaDialog.tsx`（复用 Task 4 的交互逻辑，独立 portal 封装）：

```tsx
import { useCallback, useEffect, useRef, useState } from 'react'
import { PortalDialog } from '@/portal/components/PortalDialog'
import { Button } from '@/components/ui/button'
import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

interface Props {
  open: boolean
  challenge: CaptchaChallenge | null
  onAnswer: (answer: CaptchaAnswer) => void
  onRefresh: () => void
  onCancel: () => void
}

export function PortalCaptchaDialog({ open, challenge, onAnswer, onRefresh, onCancel }: Props) {
  const [points, setPoints] = useState<Array<{ x: number; y: number }>>([])
  const [offset, setOffset] = useState({ x: 0, y: 0 })
  const [angle, setAngle] = useState(0)
  const dragging = useRef(false)
  const dragStart = useRef({ x: 0, y: 0 })

  useEffect(() => {
    if (open) {
      setPoints([])
      setOffset({ x: 0, y: 0 })
      setAngle(0)
    }
  }, [open, challenge?.id])

  const submit = useCallback(() => {
    if (!challenge) return
    switch (challenge.type) {
      case 'click':
        onAnswer({ points })
        break
      case 'slide':
      case 'dragdrop':
        onAnswer({ x: (challenge.thumb_x ?? 0) + offset.x, y: (challenge.thumb_y ?? 0) + offset.y })
        break
      case 'rotate':
        onAnswer({ angle })
        break
    }
  }, [challenge, points, offset, angle, onAnswer])

  if (!challenge) return null
  const imgW = challenge.width
  const imgH = challenge.height

  const onImageClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (challenge.type !== 'click') return
    const rect = e.currentTarget.getBoundingClientRect()
    setPoints((p) => [
      ...p,
      {
        x: Math.round(((e.clientX - rect.left) * imgW) / rect.width),
        y: Math.round(((e.clientY - rect.top) * imgH) / rect.height),
      },
    ])
  }

  const onDragStart = (e: React.MouseEvent | React.TouchEvent) => {
    dragging.current = true
    const pt = 'touches' in e ? e.touches[0]! : e
    dragStart.current = { x: pt.clientX - offset.x, y: pt.clientY - offset.y }
  }
  const onDragMove = (e: React.MouseEvent | React.TouchEvent) => {
    if (!dragging.current) return
    const pt = 'touches' in e ? (e.touches[0] ?? e.changedTouches[0]) : e
    if (!pt) return
    if (challenge.type === 'rotate') {
      const dy = pt.clientY - dragStart.current.y
      dragStart.current = { x: pt.clientX, y: pt.clientY }
      setAngle(((offset.y + dy) % 360 + 360) % 360)
    } else {
      setOffset({
        x: Math.max(0, Math.min(pt.clientX - dragStart.current.x, imgW - (challenge.thumb_width ?? 0))),
        y: Math.max(0, pt.clientY - dragStart.current.y),
      })
    }
  }
  const onDragEnd = () => {
    dragging.current = false
  }

  return (
    <PortalDialog open={open} onOpenChange={(o) => { if (!o) onCancel() }} title="安全验证">
      <p className="text-muted-foreground mb-2 text-sm">
        {challenge.type === 'click' && `请依次点击：${challenge.text ?? ''}`}
        {challenge.type === 'slide' && '拖动滑块完成拼图'}
        {challenge.type === 'dragdrop' && '将拼图块拖到正确位置'}
        {challenge.type === 'rotate' && '拖动滑块旋转图片至正向'}
      </p>
      <div className="relative select-none" style={{ width: imgW, maxWidth: '100%', aspectRatio: `${imgW} / ${imgH}` }}>
        <img src={challenge.image} alt="captcha" className="w-full rounded-md" onClick={onImageClick} />
        {(challenge.type === 'slide' || challenge.type === 'dragdrop') && challenge.thumb && (
          <img
            src={challenge.thumb} alt="puzzle piece" className="absolute cursor-grab"
            style={{
              left: (challenge.thumb_x ?? 0) + offset.x, top: (challenge.thumb_y ?? 0) + offset.y,
              width: challenge.thumb_width, height: challenge.thumb_height,
            }}
            onMouseDown={onDragStart} onMouseMove={onDragMove} onMouseUp={onDragEnd} onMouseLeave={onDragEnd}
            onTouchStart={onDragStart} onTouchMove={onDragMove} onTouchEnd={onDragEnd}
          />
        )}
        {challenge.type === 'click' &&
          points.map((p, i) => (
            <span
              key={i}
              className="bg-primary pointer-events-none absolute h-4 w-4 -translate-x-1/2 -translate-y-1/2 rounded-full opacity-70"
              style={{ left: `${(p.x / imgW) * 100}%`, top: `${(p.y / imgH) * 100}%` }}
            />
          ))}
      </div>
      {challenge.type === 'rotate' && challenge.thumb && (
        <div className="flex flex-col items-center gap-2">
          <img
            src={challenge.thumb} alt="rotate target"
            style={{ width: challenge.thumb_size ?? 120, transform: `rotate(${angle}deg)` }}
            className="rounded-md"
          />
          <input type="range" min={0} max={360} value={angle} onChange={(e) => setAngle(Number(e.target.value))} className="w-full" aria-label="旋转角度" />
        </div>
      )}
      <div className="mt-3 flex gap-2">
        <Button variant="outline" className="flex-1" onClick={onRefresh}>换一题</Button>
        <Button className="flex-1" onClick={submit}>提交</Button>
      </div>
    </PortalDialog>
  )
}
```

- [ ] **Step 3: 创建 `src/portal/pages/Home.tsx` 与 `Docs.tsx`**

`Home.tsx`：

```tsx
import { type ReactNode } from 'react'

interface HomeProps {
  navigate: (to: string) => void
  loggedIn: boolean
}

// 静态玻璃视觉营销首页（替代 Three.js hero，spec §1.3/§5.3），保留品牌文案
export function Home({ navigate, loggedIn }: HomeProps) {
  return (
    <div className="relative flex min-h-[70vh] flex-col items-center justify-center overflow-hidden">
      {/* 静态玻璃背景层（纯 CSS，无 WebGL） */}
      <div aria-hidden className="pointer-events-none absolute inset-0">
        <div className="bg-primary/10 absolute -left-24 top-10 h-72 w-72 rounded-full blur-3xl" />
        <div className="bg-primary/15 absolute right-0 bottom-0 h-96 w-96 rounded-full blur-3xl" />
      </div>
      <main className="relative z-10 mx-4 max-w-2xl rounded-3xl p-10 text-center">
        <h1 className="text-5xl font-bold tracking-tight">Design World</h1>
        <p className="text-muted-foreground mt-4 text-xl">Explore New Ideas</p>
        <div className="mt-8 flex items-center justify-center gap-3">
          {!loggedIn && (
            <>
              <button
                className="bg-primary text-primary-foreground hover:bg-primary/90 rounded-full px-8 py-3 text-lg font-medium shadow-lg transition-transform hover:scale-105"
                onClick={() => navigate('/register')}
              >
                Explore Now
              </button>
              <button
                className="border-border hover:bg-accent rounded-full border px-8 py-3 text-lg transition-colors"
                onClick={() => navigate('/login')}
              >
                Sign In
              </button>
            </>
          )}
          {loggedIn && (
            <button
              className="bg-primary text-primary-foreground rounded-full px-8 py-3 text-lg font-medium"
              onClick={() => navigate('/profile')}
            >
              个人资料
            </button>
          )}
        </div>
      </main>
    </div>
  )
}

export function HomeSection({ children }: { children: ReactNode }) {
  return <section className="mx-auto max-w-3xl px-4 py-8">{children}</section>
}
```

`Docs.tsx`：

```tsx
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

// #/docs 静态文档页（spec §5.3 明确的静态页，有确定内容）
export function Docs() {
  return (
    <Card className="mx-auto my-10 max-w-2xl">
      <CardHeader><CardTitle>产品文档</CardTitle></CardHeader>
      <CardContent className="space-y-3 text-sm">
        <p>Design World 平台的产品说明与使用指引将在此发布。</p>
        <p>当前内容：平台简介、账号注册与登录指引、个人资料管理说明。</p>
      </CardContent>
    </Card>
  )
}
```

- [ ] **Step 4: 替换 `src/portal/App.tsx`（本任务先接 Home/Docs；Login/Register/Profile 由 Task 14 填充）**

`src/portal/App.tsx`：

```tsx
import { useCallback, useState, type ReactNode } from 'react'
import { useHashRoute } from '@/portal/useHashRoute'
import { portalSession } from '@/lib/portal-session'
import { portalAuth } from '@/api/portal/auth'
import { Home } from '@/portal/pages/Home'
import { Docs } from '@/portal/pages/Docs'
import { Button } from '@/components/ui/button'

export default function PortalApp() {
  const { path, navigate } = useHashRoute()
  const [loggedIn, setLoggedIn] = useState(portalSession.isLoggedIn())

  const refreshLoggedIn = useCallback(() => setLoggedIn(portalSession.isLoggedIn()), [])
  const logout = useCallback(async () => {
    try {
      await portalAuth.logout()
    } catch {
      // 本地清理优先
    }
    portalSession.clearTokens()
    setLoggedIn(false)
    navigate('/')
  }, [navigate])

  // Task 14 将用真实页面组件替换以下内联占位
  const loginPage = (
    <div className="mx-auto max-w-sm p-6">
      <h2 className="mb-4 text-2xl font-semibold">Sign In</h2>
      <p className="text-muted-foreground text-sm">Task 14 完整化</p>
    </div>
  )

  function ProfileStub() {
    return <div className="mx-auto max-w-sm p-6">个人资料（Task 14 完整化）</div>
  }

  let page: ReactNode
  switch (path) {
    case '/login':
      page = loggedIn ? <ProfileStub /> : loginPage
      break
    case '/register':
      page = loggedIn ? <ProfileStub /> : loginPage
      break
    case '/profile':
      page = <ProfileStub />
      break
    case '/docs':
      page = <Docs />
      break
    default:
      page = <Home navigate={navigate} loggedIn={loggedIn} />
  }

  return (
    <div className="flex min-h-screen flex-col">
      <nav className="flex items-center justify-between px-6 py-4">
        <button className="text-xl font-bold" onClick={() => navigate('/')}>Design World</button>
        <div className="flex items-center gap-2">
          <Button variant="ghost" onClick={() => navigate('/docs')}>文档</Button>
          {loggedIn ? (
            <>
              <Button variant="ghost" onClick={() => navigate('/profile')}>个人资料</Button>
              <Button variant="outline" onClick={() => void logout()}>退出</Button>
            </>
          ) : (
            <>
              <Button variant="ghost" onClick={() => navigate('/login')}>Sign In</Button>
              <Button onClick={() => navigate('/register')}>Sign Up</Button>
            </>
          )}
        </div>
      </nav>
      <div className="flex-1">{page}</div>
    </div>
  )
}
```

- [ ] **Step 5: 写 Portal 基座测试（红→绿）**

`src/test/portal-base.test.tsx`：

```tsx
import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useHashRoute } from '@/portal/useHashRoute'
import PortalApp from '@/portal/App'
import { portalSession } from '@/lib/portal-session'

vi.mock('@/api/portal/auth', () => ({
  portalAuth: { logout: vi.fn().mockResolvedValue(undefined) },
}))

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  window.location.hash = ''
})

function RouteProbe() {
  const { path, navigate } = useHashRoute()
  return (
    <div>
      <span data-testid="path">{path}</span>
      <button onClick={() => navigate('/login')}>go</button>
    </div>
  )
}

describe('useHashRoute', () => {
  it('parses location.hash and navigates', () => {
    window.location.hash = '#/login'
    render(<RouteProbe />)
    expect(screen.getByTestId('path').textContent).toBe('/login')
    fireEvent.click(screen.getByText('go'))
    expect(window.location.hash).toBe('#/login')
  })
  it('normalizes bare hash to /', () => {
    window.location.hash = ''
    render(<RouteProbe />)
    expect(screen.getByTestId('path').textContent).toBe('/')
  })
})

describe('PortalApp shell', () => {
  it('shows brand copy and Sign In/Sign Up nav when logged out', () => {
    render(<PortalApp />)
    expect(screen.getAllByText('Design World').length).toBeGreaterThan(0)
    expect(screen.getByText('Explore New Ideas')).toBeInTheDocument()
    expect(screen.getByText('Explore Now')).toBeInTheDocument()
    expect(screen.getByText('Sign In')).toBeInTheDocument()
    expect(screen.getByText('Sign Up')).toBeInTheDocument()
  })

  it('Explore Now navigates to #/register', () => {
    render(<PortalApp />)
    fireEvent.click(screen.getByText('Explore Now'))
    expect(window.location.hash).toBe('#/register')
  })

  it('renders docs route', () => {
    window.location.hash = '#/docs'
    render(<PortalApp />)
    expect(screen.getByText('产品文档')).toBeInTheDocument()
  })

  it('logout clears portal_tokens and returns home', async () => {
    portalSession.saveTokens({ access_token: 'a', refresh_token: 'r', expires_in: 1, issued_at: Date.now() })
    window.location.hash = '#/profile'
    render(<PortalApp />)
    fireEvent.click(screen.getByText('退出'))
    await vi.waitFor(() => expect(portalSession.isLoggedIn()).toBe(false))
  })
})
```

Run: `npm run test:run -- src/test/portal-base.test.tsx` → Expected: PASS。

- [ ] **Step 6: 全量校验并提交**

```bash
npm run test:run
npm run typecheck
npm run build
```

Expected: 全部 PASS；typecheck 零错误；build 成功（双入口产物）。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): portal base with hash router, portal session, captcha dialog, static glass home and docs"
```

### Task 14: Portal Login / Register / Profile 页面

**Files:**
- Create: `src/portal/pages/Login.tsx`
- Create: `src/portal/pages/Register.tsx`
- Create: `src/portal/pages/Profile.tsx`
- Modify: `src/portal/App.tsx`（移除 Task 13 的内联占位，接入真实页面）
- Test: `src/test/portal-pages.test.tsx`（并更新 `src/test/portal-base.test.tsx` 的登出用例）

**Interfaces:**
- Consumes: `portalAuth.login/register/getProfile/updateProfile/updatePassword/getCaptcha/logout`（Task 3）、`portalSession.saveTokens/getTokens/clearTokens/isLoggedIn`（Task 2）、`PortalCaptchaDialog`（Task 13）、`buildCaptchaAnswer`/`captchaEnabled`（Task 2）、`PortalApiError`（Task 2）、`formatDateTime`（Task 2）、shadcn 表单组件、`toastError`/`toastSuccess`。
- Produces: `PortalLoginPage({ navigate, onLoggedIn })`（`Sign In` 标题；用户名/密码 + 验证码拉题弹框）；`PortalRegisterPage({ navigate, onRegistered })`（`Sign Up`；用户名/昵称/邮箱/密码/确认密码 + 验证码；两次密码不一致提示 `两次输入的密码不一致`；注册即登录）；`ProfilePage({ navigate, onLoggedOut })`（展示/编辑 `PublicProfile`：昵称/邮箱/手机号/性别/注册时间；编辑资料与修改密码两个表单；校验消息：邮箱格式、手机号格式、两次密码不一致；401 跳 `/login`）。

- [ ] **Step 1: 创建 `src/portal/pages/Login.tsx`**

```tsx
import { useState, type FormEvent } from 'react'
import { portalAuth } from '@/api/portal/auth'
import { portalSession } from '@/lib/portal-session'
import { buildCaptchaAnswer, captchaEnabled } from '@/lib/captcha'
import { toastError } from '@/lib/toast'
import { PortalCaptchaDialog } from '@/portal/components/PortalCaptchaDialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

interface Props {
  navigate: (to: string) => void
  onLoggedIn: () => void
}

export function PortalLoginPage({ navigate, onLoggedIn }: Props) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [challenge, setChallenge] = useState<CaptchaChallenge | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)

  const doLogin = async (answer?: CaptchaAnswer) => {
    setError(null)
    setBusy(true)
    try {
      const body: Record<string, unknown> = { username, password }
      if (challenge && answer) Object.assign(body, buildCaptchaAnswer(challenge, answer))
      const tokens = await portalAuth.login(body as { username: string; password: string })
      portalSession.saveTokens(tokens)
      onLoggedIn()
      navigate('/profile')
    } catch (err) {
      const code = (err as { status?: number }).status
      if (challenge && answer && code === 400) {
        // 验证码校验失败：弹框内提示并自动换题
        toastError('验证码校验失败，已为你更换新题')
        setChallenge(await portalAuth.getCaptcha())
        setDialogOpen(true)
      } else {
        setDialogOpen(false)
        setChallenge(null)
        setError((err as Error).message)
      }
    } finally {
      setBusy(false)
    }
  }

  const onSubmit = async (e: FormEvent) => {
    e.preventDefault()
    try {
      const c = await portalAuth.getCaptcha()
      if (captchaEnabled(c)) {
        setChallenge(c)
        setDialogOpen(true)
        return
      }
      await doLogin()
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <Card className="mx-auto my-10 max-w-sm">
      <CardHeader><CardTitle>Sign In</CardTitle></CardHeader>
      <CardContent>
        <form onSubmit={onSubmit} className="space-y-4">
          {error && <p className="text-destructive text-sm">{error}</p>}
          <Field><FieldLabel>用户名</FieldLabel><Input value={username} onChange={(e) => setUsername(e.target.value)} required /></Field>
          <Field><FieldLabel>密码</FieldLabel><Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required /></Field>
          <Button type="submit" className="w-full" disabled={busy}>Sign In</Button>
        </form>
      </CardContent>
      <PortalCaptchaDialog
        open={dialogOpen} challenge={challenge}
        onAnswer={(a) => void doLogin(a)}
        onRefresh={() => void portalAuth.getCaptcha().then(setChallenge)}
        onCancel={() => { setDialogOpen(false); setChallenge(null) }}
      />
    </Card>
  )
}
```

- [ ] **Step 2: 创建 `src/portal/pages/Register.tsx`**

```tsx
import { useState, type FormEvent } from 'react'
import { portalAuth } from '@/api/portal/auth'
import { portalSession } from '@/lib/portal-session'
import { buildCaptchaAnswer, captchaEnabled } from '@/lib/captcha'
import { toastError } from '@/lib/toast'
import { PortalCaptchaDialog } from '@/portal/components/PortalCaptchaDialog'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import type { CaptchaAnswer, CaptchaChallenge } from '@/lib/types'

interface Props {
  navigate: (to: string) => void
  onRegistered: () => void
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function PortalRegisterPage({ navigate, onRegistered }: Props) {
  const [form, setForm] = useState({ username: '', nickname: '', email: '', password: '', confirm: '' })
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [challenge, setChallenge] = useState<CaptchaChallenge | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [payload, setPayload] = useState<Record<string, unknown> | null>(null)

  const finish = async (withAnswer?: CaptchaAnswer) => {
    if (!payload) return
    setBusy(true)
    try {
      if (withAnswer) Object.assign(payload, withAnswer)
      const tokens = await portalAuth.register(payload as { username: string; nickname: string; email: string; password: string })
      portalSession.saveTokens(tokens)
      onRegistered()
      navigate('/profile')
    } catch (err) {
      const status = (err as { status?: number }).status
      if (withAnswer && status === 400) {
        // 验证码校验失败：弹框内提示并自动换题（挑战一次性消费）
        toastError('验证码校验失败，已为你更换新题')
        setChallenge(await portalAuth.getCaptcha())
        setDialogOpen(true)
      } else {
        setDialogOpen(false)
        setChallenge(null)
        setPayload(null)
        setError((err as Error).message)
      }
    } finally {
      setBusy(false)
    }
  }

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError(null)
    if (form.password !== form.confirm) {
      setError('两次输入的密码不一致')
      return
    }
    if (!EMAIL_RE.test(form.email)) {
      setError('邮箱格式不正确')
      return
    }
    const base: Record<string, unknown> = {
      username: form.username, nickname: form.nickname, email: form.email, password: form.password,
    }
    try {
      const c = await portalAuth.getCaptcha()
      if (captchaEnabled(c)) {
        setPayload(base)
        setChallenge(c)
        setDialogOpen(true)
        return
      }
      const tokens = await portalAuth.register(base as { username: string; nickname: string; email: string; password: string })
      portalSession.saveTokens(tokens)
      onRegistered()
      navigate('/profile')
    } catch (err) {
      setError((err as Error).message)
    }
  }

  return (
    <Card className="mx-auto my-10 max-w-sm">
      <CardHeader><CardTitle>Sign Up</CardTitle></CardHeader>
      <CardContent>
        <form onSubmit={submit} className="space-y-4">
          {error && <p className="text-destructive text-sm">{error}</p>}
          <Field><FieldLabel>用户名</FieldLabel><Input value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} required /></Field>
          <Field><FieldLabel>昵称</FieldLabel><Input value={form.nickname} onChange={(e) => setForm({ ...form, nickname: e.target.value })} required /></Field>
          <Field><FieldLabel>邮箱</FieldLabel><Input type="email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} required /></Field>
          <Field><FieldLabel>密码</FieldLabel><Input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required /></Field>
          <Field><FieldLabel>确认密码</FieldLabel><Input type="password" value={form.confirm} onChange={(e) => setForm({ ...form, confirm: e.target.value })} required /></Field>
          <Button type="submit" className="w-full" disabled={busy}>立即注册</Button>
        </form>
      </CardContent>
      <PortalCaptchaDialog
        open={dialogOpen} challenge={challenge}
        onAnswer={(a) => void finish(buildCaptchaAnswer(challenge!, a) as unknown as CaptchaAnswer)}
        onRefresh={() => void portalAuth.getCaptcha().then(setChallenge)}
        onCancel={() => { setDialogOpen(false); setChallenge(null); setPayload(null) }}
      />
    </Card>
  )
}
```

- [ ] **Step 3: 创建 `src/portal/pages/Profile.tsx`**

```tsx
import { useEffect, useState, type FormEvent } from 'react'
import { portalAuth } from '@/api/portal/auth'
import { portalSession, PortalApiError } from '@/lib/portal-session'
import { toastError, toastSuccess } from '@/lib/toast'
import { formatDateTime } from '@/lib/datetime'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import type { PublicProfile } from '@/lib/types'

interface Props {
  navigate: (to: string) => void
  onLoggedOut: () => void
}

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
const MOBILE_RE = /^1\d{10}$/

export function ProfilePage({ navigate, onLoggedOut }: Props) {
  const [profile, setProfile] = useState<PublicProfile | null>(null)
  const [editForm, setEditForm] = useState({ nickname: '', email: '', mobile: '', gender: '1' })
  const [editError, setEditError] = useState<string | null>(null)
  const [pwdForm, setPwdForm] = useState({ old_password: '', new_password: '', confirm: '' })
  const [pwdError, setPwdError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    portalAuth
      .getProfile()
      .then((p) => {
        setProfile(p)
        setEditForm({ nickname: p.nickname, email: p.email, mobile: p.mobile, gender: p.gender || '1' })
      })
      .catch((e) => {
        if (e instanceof PortalApiError && e.status === 401) {
          navigate('/login')
          return
        }
        toastError((e as Error).message)
      })
  }, [navigate])

  const submitProfile = async (e: FormEvent) => {
    e.preventDefault()
    setEditError(null)
    if (!editForm.nickname) {
      setEditError('昵称必填')
      return
    }
    if (editForm.email && !EMAIL_RE.test(editForm.email)) {
      setEditError('邮箱格式不正确')
      return
    }
    if (editForm.mobile && !MOBILE_RE.test(editForm.mobile)) {
      setEditError('手机号格式不正确')
      return
    }
    setBusy(true)
    try {
      await portalAuth.updateProfile(editForm)
      toastSuccess('资料已更新')
      setProfile(await portalAuth.getProfile())
    } catch (err) {
      setEditError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const submitPassword = async (e: FormEvent) => {
    e.preventDefault()
    setPwdError(null)
    if (pwdForm.new_password !== pwdForm.confirm) {
      setPwdError('两次输入的密码不一致')
      return
    }
    setBusy(true)
    try {
      await portalAuth.updatePassword({ old_password: pwdForm.old_password, new_password: pwdForm.new_password })
      toastSuccess('密码修改成功')
      setPwdForm({ old_password: '', new_password: '', confirm: '' })
    } catch (err) {
      setPwdError((err as Error).message)
    } finally {
      setBusy(false)
    }
  }

  const logout = async () => {
    try {
      await portalAuth.logout()
    } catch {
      // 本地清理优先
    }
    portalSession.clearTokens()
    onLoggedOut()
    navigate('/')
  }

  if (!profile) return <Skeleton className="mx-auto my-10 h-64 w-full max-w-md" />

  return (
    <div className="mx-auto my-10 max-w-md space-y-4">
      <Card>
        <CardHeader className="flex items-center justify-between">
          <CardTitle>个人资料</CardTitle>
          <Button variant="outline" size="sm" onClick={() => void logout()}>退出</Button>
        </CardHeader>
        <CardContent className="space-y-1 text-sm">
          <div>用户名：{profile.username}</div>
          <div>注册时间：{formatDateTime(profile.created_at)}</div>
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>编辑资料</CardTitle></CardHeader>
        <CardContent>
          <form onSubmit={submitProfile} className="space-y-4">
            {editError && <p className="text-destructive text-sm">{editError}</p>}
            <Field><FieldLabel>昵称</FieldLabel><Input value={editForm.nickname} onChange={(e) => setEditForm({ ...editForm, nickname: e.target.value })} required /></Field>
            <Field><FieldLabel>邮箱</FieldLabel><Input type="email" value={editForm.email} onChange={(e) => setEditForm({ ...editForm, email: e.target.value })} /></Field>
            <Field><FieldLabel>手机号</FieldLabel><Input value={editForm.mobile} onChange={(e) => setEditForm({ ...editForm, mobile: e.target.value })} /></Field>
            <Field>
              <FieldLabel>性别</FieldLabel>
              <Select value={editForm.gender} onValueChange={(v) => setEditForm({ ...editForm, gender: v })}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="1">男</SelectItem>
                  <SelectItem value="2">女</SelectItem>
                </SelectContent>
              </Select>
            </Field>
            <Button type="submit" disabled={busy}>保存资料</Button>
          </form>
        </CardContent>
      </Card>
      <Separator />
      <Card>
        <CardHeader><CardTitle>修改密码</CardTitle></CardHeader>
        <CardContent>
          <form onSubmit={submitPassword} className="space-y-4">
            {pwdError && <p className="text-destructive text-sm">{pwdError}</p>}
            <Field><FieldLabel>原密码</FieldLabel><Input type="password" value={pwdForm.old_password} onChange={(e) => setPwdForm({ ...pwdForm, old_password: e.target.value })} required /></Field>
            <Field><FieldLabel>新密码</FieldLabel><Input type="password" value={pwdForm.new_password} onChange={(e) => setPwdForm({ ...pwdForm, new_password: e.target.value })} required /></Field>
            <Field><FieldLabel>确认新密码</FieldLabel><Input type="password" value={pwdForm.confirm} onChange={(e) => setPwdForm({ ...pwdForm, confirm: e.target.value })} required /></Field>
            <Button type="submit" disabled={busy}>修改密码</Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
```

- [ ] **Step 4: 更新 `src/portal/App.tsx` 接入真实页面（移除占位）**

`src/portal/App.tsx` 最终版：

```tsx
import { useCallback, useState } from 'react'
import { useHashRoute } from '@/portal/useHashRoute'
import { portalSession } from '@/lib/portal-session'
import { portalAuth } from '@/api/portal/auth'
import { Home } from '@/portal/pages/Home'
import { Docs } from '@/portal/pages/Docs'
import { PortalLoginPage } from '@/portal/pages/Login'
import { PortalRegisterPage } from '@/portal/pages/Register'
import { ProfilePage } from '@/portal/pages/Profile'
import { Button } from '@/components/ui/button'

export default function PortalApp() {
  const { path, navigate } = useHashRoute()
  const [loggedIn, setLoggedIn] = useState(portalSession.isLoggedIn())

  const refreshLoggedIn = useCallback(() => setLoggedIn(portalSession.isLoggedIn()), [])
  const logout = useCallback(async () => {
    try {
      await portalAuth.logout()
    } catch {
      // 本地清理优先
    }
    portalSession.clearTokens()
    setLoggedIn(false)
    navigate('/')
  }, [navigate])

  let page: React.ReactNode
  switch (path) {
    case '/login':
      page = loggedIn
        ? <ProfilePage navigate={navigate} onLoggedOut={refreshLoggedIn} />
        : <PortalLoginPage navigate={navigate} onLoggedIn={refreshLoggedIn} />
      break
    case '/register':
      page = loggedIn
        ? <ProfilePage navigate={navigate} onLoggedOut={refreshLoggedIn} />
        : <PortalRegisterPage navigate={navigate} onRegistered={refreshLoggedIn} />
      break
    case '/profile':
      page = loggedIn
        ? <ProfilePage navigate={navigate} onLoggedOut={refreshLoggedIn} />
        : <PortalLoginPage navigate={navigate} onLoggedIn={refreshLoggedIn} />
      break
    case '/docs':
      page = <Docs />
      break
    default:
      page = <Home navigate={navigate} loggedIn={loggedIn} />
  }

  return (
    <div className="flex min-h-screen flex-col">
      <nav className="flex items-center justify-between px-6 py-4">
        <button className="text-xl font-bold" onClick={() => navigate('/')}>Design World</button>
        <div className="flex items-center gap-2">
          <Button variant="ghost" onClick={() => navigate('/docs')}>文档</Button>
          {loggedIn ? (
            <>
              <Button variant="ghost" onClick={() => navigate('/profile')}>个人资料</Button>
              <Button variant="outline" onClick={() => void logout()}>退出</Button>
            </>
          ) : (
            <>
              <Button variant="ghost" onClick={() => navigate('/login')}>Sign In</Button>
              <Button onClick={() => navigate('/register')}>Sign Up</Button>
            </>
          )}
        </div>
      </nav>
      <div className="flex-1">{page}</div>
    </div>
  )
}
```

同时更新 `src/test/portal-base.test.tsx` 的登出用例（已登录访问 `#/profile` 仍渲染 Profile，退出入口在 Profile 卡与导航栏均可）——Step 5 中该用例保持不变即可（退出按钮在 `#/profile` 页导航栏渲染，`getByText('退出')` 命中导航栏按钮）。

- [ ] **Step 5: 写 Portal 页面测试（红→绿）**

`src/test/portal-pages.test.tsx`：

```tsx
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { PortalLoginPage } from '@/portal/pages/Login'
import { PortalRegisterPage } from '@/portal/pages/Register'
import { ProfilePage } from '@/portal/pages/Profile'
import { portalAuth } from '@/api/portal/auth'
import { portalSession, PortalApiError } from '@/lib/portal-session'

vi.mock('@/api/portal/auth', () => ({
  portalAuth: {
    getCaptcha: vi.fn(),
    login: vi.fn(),
    register: vi.fn().mockResolvedValue({ access_token: 'a', refresh_token: 'r', expires_in: 1, issued_at: Date.now() }),
    getProfile: vi.fn(),
    updateProfile: vi.fn().mockResolvedValue(undefined),
    updatePassword: vi.fn().mockResolvedValue(undefined),
    logout: vi.fn().mockResolvedValue(undefined),
  },
}))

const navigate = vi.fn()
const noop = () => {}

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(portalAuth.getCaptcha).mockResolvedValue({
    id: 'c', type: 'slide', image: 'i', width: 100, height: 50, enabled: false,
  })
})

describe('Portal Login', () => {
  it('renders Sign In and skips captcha when disabled', async () => {
    vi.mocked(portalAuth.login).mockResolvedValue({ access_token: 'a', refresh_token: 'r', expires_in: 1, issued_at: Date.now() })
    render(<PortalLoginPage navigate={navigate} onLoggedIn={noop} />)
    expect(screen.getByText('Sign In')).toBeInTheDocument()
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getAllByText('Sign In').at(-1)!)
    await waitFor(() => expect(portalSession.isLoggedIn()).toBe(true))
    expect(navigate).toHaveBeenCalledWith('/profile')
  })
})

describe('Portal Register', () => {
  it('rejects mismatched passwords with spec message', () => {
    render(<PortalRegisterPage navigate={navigate} onRegistered={noop} />)
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'a' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'b' } })
    fireEvent.click(screen.getByText('立即注册'))
    expect(screen.getByText('两次输入的密码不一致')).toBeInTheDocument()
    expect(portalAuth.register).not.toHaveBeenCalled()
  })

  it('registers and logs in immediately', async () => {
    render(<PortalRegisterPage navigate={navigate} onRegistered={noop} />)
    fireEvent.change(screen.getByLabelText('用户名'), { target: { value: 'u' } })
    fireEvent.change(screen.getByLabelText('昵称'), { target: { value: 'n' } })
    fireEvent.change(screen.getByLabelText('邮箱'), { target: { value: 'a@b.co' } })
    fireEvent.change(screen.getByLabelText('密码'), { target: { value: 'p' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'p' } })
    fireEvent.click(screen.getByText('立即注册'))
    await waitFor(() => expect(portalAuth.register).toHaveBeenCalled())
    await waitFor(() => expect(portalSession.isLoggedIn()).toBe(true))
    expect(navigate).toHaveBeenCalledWith('/profile')
  })
})

describe('Portal Profile', () => {
  const profile = {
    id: 42, username: 'u', nickname: '昵称', email: 'a@b.co', mobile: '13800138000',
    gender: '1', avatar: '', created_at: 1730000000000,
  }

  it('loads profile with numeric id preserved and shows created_at', async () => {
    vi.mocked(portalAuth.getProfile).mockResolvedValue(profile)
    render(<ProfilePage navigate={navigate} onLoggedOut={noop} />)
    await screen.findByText('昵称')
    expect(screen.getByText(/注册时间/)).toBeInTheDocument()
    expect(portalAuth.getProfile).toHaveBeenCalled()
    // id 保持数字类型（PublicProfile.id: number），未经字符串转换
    const result = (await vi.mocked(portalAuth.getProfile).mock.results[0]!.value) as { id: number }
    expect(result.id).toBe(42)
  })

  it('redirects to login on 401', async () => {
    vi.mocked(portalAuth.getProfile).mockRejectedValue(new PortalApiError(401, '登录已过期，请重新登录'))
    render(<ProfilePage navigate={navigate} onLoggedOut={noop} />)
    await waitFor(() => expect(navigate).toHaveBeenCalledWith('/login'))
  })

  it('updates profile', async () => {
    vi.mocked(portalAuth.getProfile).mockResolvedValue(profile)
    render(<ProfilePage navigate={navigate} onLoggedOut={noop} />)
    await screen.findByText('昵称')
    fireEvent.click(screen.getByText('保存资料'))
    await waitFor(() =>
      expect(portalAuth.updateProfile).toHaveBeenCalledWith(expect.objectContaining({ nickname: '昵称' })),
    )
  })

  it('rejects mismatched password confirmation', async () => {
    vi.mocked(portalAuth.getProfile).mockResolvedValue(profile)
    render(<ProfilePage navigate={navigate} onLoggedOut={noop} />)
    await screen.findByText('昵称')
    fireEvent.change(screen.getByLabelText('新密码'), { target: { value: 'a' } })
    fireEvent.change(screen.getByLabelText('确认新密码'), { target: { value: 'b' } })
    fireEvent.click(screen.getByText('修改密码'))
    expect(screen.getByText('两次输入的密码不一致')).toBeInTheDocument()
    expect(portalAuth.updatePassword).not.toHaveBeenCalled()
  })
})
```

Run: `npm run test:run -- src/test/portal-pages.test.tsx src/test/portal-base.test.tsx` → Expected: PASS。

- [ ] **Step 6: 全量校验并提交**

```bash
npm run test:run
npm run typecheck
npm run build
```

Expected: 全部 PASS；typecheck 零错误；build 成功。

```bash
git add -A devops-frontend
git commit -m "feat(frontend): portal login/register/profile pages with captcha, preserved copy and validations"
```

### Task 15: 清理 Vue/GSAP/Three.js/自研样式与依赖

**Files:**
- Delete: `devops-frontend/env.d.ts`
- Delete: `src/main.ts`、`src/App.vue`、`src/router/index.ts`
- Delete: `src/api/auth.ts`、`src/api/http.ts`、`src/api/index.ts`、`src/api/session.ts`、`src/api/system.ts`
- Delete: `src/components/Icon.vue`、`src/components/TopoNet.vue`
- Delete: `src/components/business/` 旧文件（`FormDialog.vue`、`index.ts`、`ListPage.vue`、`PermissionButton.vue`、`SearchForm.vue`）
- Delete: `src/components/layout/*.vue`（`AppearanceDialog.vue`、`AppShell.vue`、`CommandPalette.vue`、`PasswordDialog.vue`、`Sidebar.vue`、`Topbar.vue`）
- Delete: `src/components/ui/*.vue` 与 `src/components/ui/index.ts`、`src/components/ui/types.ts`（**保留 shadcn 生成的 React `.tsx` 组件**）
- Delete: `src/composables/`（全部 `.ts`）
- Delete: `src/styles/`（`base.css`、`components.css`、`instruments.css`、`layout.css`、`materials.css`、`pages.css`、`tokens.css`）
- Delete: `src/views/*.vue`、`src/views/monitor/*.vue`、`src/views/system/*.vue`、`src/views/system/parts/MenuCheckTree.vue`
- Delete: `src/portal/main.ts`、`src/portal/api.ts`、`src/portal/captcha.ts`、`src/portal/pages.ts`、`src/portal/hero.ts`、`src/portal/style.css`
- Modify: `package.json`（移除依赖）
- Modify: `README.md`（若含前端栈描述，更新为 React 技术栈）

**Interfaces:**
- Consumes: Task 1-14 全部 React 实现（清理后仍需全部测试与构建通过）。
- Produces: 仅含 React 技术栈的 `devops-frontend`；`package.json` 无 `vue`、`vue-router`、`vue-tsc`、`@vitejs/plugin-vue`、`go-captcha-vue`、`gsap`、`three`、`@types/three`。

- [ ] **Step 1: 删除旧文件（逐目录执行）**

```powershell
cd D:\project\golang\devops\devops-frontend
Remove-Item env.d.ts, src\main.ts, src\App.vue, src\router\index.ts
Remove-Item src\api\auth.ts, src\api\http.ts, src\api\index.ts, src\api\session.ts, src\api\system.ts
Remove-Item src\components\Icon.vue, src\components\TopoNet.vue
Get-ChildItem src\components\business -Exclude *.tsx, *.ts | Where-Object { $_.Name -ne 'CaptchaDialog.tsx' -and $_.Name -ne 'PermissionButton.tsx' -and $_.Name -ne 'SearchForm.tsx' -and $_.Name -ne 'ListPage.tsx' -and $_.Name -ne 'FormDialog.tsx' } | Remove-Item
```

更直接、不易错的写法（逐个列名删除，只保留 React 实现）：

```powershell
Remove-Item src\components\business\FormDialog.vue, src\components\business\index.ts, src\components\business\ListPage.vue, src\components\business\PermissionButton.vue, src\components\business\SearchForm.vue
Remove-Item src\components\layout\*.vue
Get-ChildItem src\components\ui -Filter *.vue | Remove-Item
Remove-Item src\components\ui\index.ts, src\components\ui\types.ts -ErrorAction SilentlyContinue
Remove-Item src\composables -Recurse -Force
Remove-Item src\styles -Recurse -Force
Remove-Item src\views\*.vue, src\views\monitor\*.vue, src\views\system\*.vue
Remove-Item src\views\system\parts -Recurse -Force
Remove-Item src\portal\main.ts, src\portal\api.ts, src\portal\captcha.ts, src\portal\pages.ts, src\portal\hero.ts, src\portal\style.css
```

**警示**：`src/components/ui/` 中 shadcn 生成的 React `.tsx` 文件（`button.tsx`、`dialog.tsx` 等）**不得删除**——上面命令用 `-Filter *.vue` 精确匹配。`src/lib/`、`src/api/admin/`、`src/api/portal/`、`src/store/`、`src/router/routes.tsx`、`src/router/guards.tsx`、`src/test/` 均为 React 侧实现，不得删除。

- [ ] **Step 2: 移除 package.json 依赖**

```bash
cd devops-frontend
npm uninstall vue vue-router vue-tsc go-captcha-vue gsap three @types/three @vitejs/plugin-vue
```

- [ ] **Step 3: 清理残留引用检查**

```bash
npm run typecheck
```

Expected: 零错误。若报错，用 grep 定位对已删文件的残余 import：

```bash
grep -rn "from 'vue'" src
grep -rn "vue-router" src
grep -rn "composables/" src
grep -rn "styles/" src
```

Expected: 四条命令均无输出。

- [ ] **Step 4: 更新 README（若含前端栈描述）**

若 `README.md` 中存在描述 Vue/GSAP/Three.js 的段落，将其替换为：

```markdown
## 前端技术栈

- React 19 + TypeScript + Vite（双入口：`index.html` 后台管理 / `portal.html` C 端门户）
- Tailwind CSS 4（`@tailwindcss/vite`）+ tw-animate-css
- shadcn/ui（base-nova / Base UI 底座）+ solid-glass 玻璃质感组件
- React Router（admin 动态菜单路由）/ Hash 路由（portal）
- Vitest + React Testing Library（契约与页面测试）

质量命令：`npm run typecheck` / `npm run test:run` / `npm run build`
```

- [ ] **Step 5: 全量校验并提交**

```bash
npm run test:run
npm run typecheck
npm run build
```

Expected: 全部 PASS；typecheck 零错误；build 成功且无 Vue/GSAP/Three 产物进入 `dist`。

```bash
git add -A devops-frontend
git commit -m "chore(frontend): remove Vue/GSAP/Three.js/go-captcha-vue legacy code, styles and dependencies"
```

### Task 16: 最终验证（验收清单核对 + 冒烟）

**Files:**
- Modify: `devops-frontend/README.md`（仅当验证发现文档需补充质量命令/结构说明时）
- Test: 无新增（运行全量）

**Interfaces:**
- Consumes: Task 1-15 全部产出。
- Produces: 验收通过的前端迁移交付。

- [ ] **Step 1: 静态校验（spec §7.1）**

```bash
cd devops-frontend
npm run typecheck   # 零错误
npm run build       # dist/index.html 与 dist/portal.html 均生成
```

确认 `dist/index.html` 与 `dist/portal.html` 均存在且可独立加载。

- [ ] **Step 2: 全量契约/页面测试（spec §7.3）**

```bash
npm run test:run
```

Expected: 全部 PASS，覆盖：信封解包、Query 空值剔除、刷新单飞、会话版本守卫、菜单映射、四题型验证码映射与换题、租户解析、登录错误映射、portal 契约。

- [ ] **Step 3: 开发服务器浏览器冒烟（spec §7.2，需本地网关/后端可用）**

```bash
npm run dev
```

浏览器打开 `http://127.0.0.1:5173`，逐项核对：

- [ ] Admin 登录：验证码拉取与提交、登录成功跳工作台、错误文案映射正确、`?tenantCode=` / `/t/{code}/login` 入口解析与锁定正确。
- [ ] 工作台：真实接口数据加载渲染。
- [ ] CRUD：各模块列表加载、分页、新增/编辑/删除/重置密码、表单校验、内置项（内置角色/内置参数）不可删除、菜单与部门树级联、无权限按钮不渲染。
- [ ] 401：会话失效后清理会话、跳登录页并携带 `tenantCode` 与 `redirect`。
- [ ] 动态菜单：深链接刷新后回到对应页面（不落 404）。
- [ ] 主题：light/dark/auto 切换生效，`auto` 跟随系统，刷新无闪烁。
- [ ] Portal：`http://127.0.0.1:5173/portal.html` 首页静态玻璃视觉渲染、品牌文案完整、登录/注册/个人资料/文档路由跳转、验证码与校验消息、退出清理 `portal_tokens`。
- [ ] 代理：`/system/api` 与 `/user/api` 请求经网关成功；非 API 前缀（如 `/system/user`）刷新走 SPA fallback 不触发 401。

- [ ] **Step 4: 提交（仅当有文档/测试性修正）**

```bash
git add -A devops-frontend
git commit -m "docs(frontend): final verification pass updates"
```

---

## Self-Review 结论（已执行）

1. **Spec 覆盖**：§2.1 页面范围（admin 登录/工作台/8 模块 CRUD/两日志/403/404 + portal 5 页）→ Task 3-14；§3.1-3.5 技术栈/双入口/主题 → Task 1-2；§4.1-4.11 契约 → Task 2/3 + 契约测试（信封/空值/单飞/版本守卫/菜单映射/验证码四题型/租户解析/错误映射/portal）；§5.1 AppShell → Task 5；§5.2 CRUD 约束（权限码清单、snake_case、内置不可删、树级联、配置控件映射）→ Task 7-12；§5.3 portal（hash 路由/保留文案/验证码）→ Task 13-14；§6.3 清理清单 → Task 15；§7 验证标准 → 各任务测试步骤 + Task 16。无缺口。
2. **占位符扫描**：无 TBD/TODO/“后续实现”遗留。Task 13 的 portal 内联占位（`loginPage`/`ProfileStub`）在 Task 14 Step 4 明确替换为真实页面；`Docs` 页与 Dashboard 为 spec 明确定义的行为（§5.3 静态文档页、§5.2 只读真实数据），非占位骨架。
3. **类型一致性**：`ApiError(status, code, msg)`、`request<T>(path, opts)`、`session.saveSession(pair, tenant)`、`PortalApiError(status, msg)`、`portalRequest<T>(path, opts)`、`portalSession.saveTokens/getTokens/clearTokens/isLoggedIn`、`useListPage<T>() → ListPageState<T>`、`ListPage({ state, columns })`、`ListPageState` 静态兼容对象（树页/字典面板复用）、`PermissionButton(permission)`、`COMPONENT_MAP` 11 键、`buildDynamicRoutes`/`flattenMenus`、`PageResult<T>`、`CaptchaChallenge`/`CaptchaAnswer`/`buildCaptchaAnswer`/`captchaEnabled`、`resolveTenantEntry(search, pathname, hostname)`、`Appearance`/`applyAppearance`/`useAppearance`、`parseDeptIDsJson`/`encodeDeptIDsJson`、`adminAuth`（`@/api/admin/auth`）与 `adminSystem`（`@/api/admin/system`）与 `portalAuth`（`@/api/portal/auth`）导入路径在各任务间一致。














