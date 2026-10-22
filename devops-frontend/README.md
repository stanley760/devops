# devops-frontend

工业级运维管理平台前端 —— Vue 3 + TypeScript + Vite + Vue Router + GSAP，
**钛玻璃主题**（Apple 材质玻璃 × 工业仪表），对接 devops-system（用户权限系统）。

页面集合与菜单完全对齐 `GET /system/api/v1/auth/routers` 返回的真实菜单
（系统管理：用户 / 角色 / 菜单 / 部门 / 岗位 / 租户 / 字典 / 参数；日志：登录 / 操作）
与工作台首页；**不新增后端不存在的监控功能菜单**。

## 开发

```bash
npm install
npm run dev        # http://localhost:5173
npm run build      # vue-tsc --noEmit && vite build
npm run typecheck
```

前置（见根目录 README）：

```bash
cd devops-id && go run . server        # gRPC :9990
cd devops-system && go run . migrate   # 建表 + 种子
cd devops-system && go run . server    # HTTP :9999
```

开发代理：`/system` → `http://127.0.0.1:9999`（devops-system）、
`/user` → `http://127.0.0.1:10000`（devops-user）；
可用 `VITE_API_PROXY_TARGET` / `VITE_USER_PROXY_TARGET` 覆盖。

种子账号：租户 `default` · `admin / Admin@123`。验证码以登录页显示为准。

## 目录

| 路径 | 职责 |
| --- | --- |
| `src/api` | 统一 HTTP（信封解包 / 双 token 刷新 / X-Tenant-ID）、认证与系统管理接口、会话存储 |
| `src/router` | 静态入口（登录 / 工作台 / 异常页）+ 动态菜单路由、登录守卫、401 统一跳转 |
| `src/composables` | 动效（GSAP 统一入口）、外观、认证编排（COMPONENT_MAP）、列表状态、提示 / 确认 |
| `src/components/ui` | 按钮、输入、表格、弹窗、读数、铭牌等基础控件 |
| `src/components/business` | ListPage、SearchForm、FormDialog、PermissionButton |
| `src/components/layout` | AppShell、Sidebar、Topbar、CommandPalette、外观 / 密码弹窗 |
| `src/styles` | 令牌 → 基础 → 材质 → 控件 → 仪表 → 布局 → 页面 七层样式 |
| `src/views` | 登录、工作台、系统管理页（用户/角色/菜单/部门/岗位/字典/参数/租户）、日志页、异常页 |
| `DESIGN.md` | 钛玻璃设计规范（令牌 / 材质 / 版式 / 动效语态） |

## 关键约定

- **接口契约**：响应信封 `{code, msg, data}`；分页 `{list, total, page_num, page_size, pages}`；
  认证头 `Authorization: Bearer <access_token>` + `X-Tenant-ID`；登录需 `captcha`（按挑战类型作答）。
- **权限**：按钮级用 `PermissionButton code="system:user:create"` 等（与 Casbin 权限标识一致）；
  前端隐藏不能替代后端校验。
- **菜单**：`COMPONENT_MAP` 注册 `system/user/index` 等真实组件键；页面不改会话存储与路由就绪标记，
  退出 / 401 统一走 `clearSession()`。
- **动效**：一律 `composables/motion.ts`（EASE/DUR + 语态函数），尊重减少动态设置；
  组件不直接 import gsap（除 motion 模块与 TopoNet 画布）。
- **新增列表页**：复用 `ListPage` + `useListPage`；新增表单：复用 `FormDialog`；
  接口复用 `src/api/system.ts` 的类型与方法。
