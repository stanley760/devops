/**
 * 认证编排：当前用户、权限集、动态菜单路由、会话版本。
 * 约定：退出与 401 都走 clearSession()，撤销动态路由与缓存；
 * 页面不得直接改认证存储或路由就绪标记。
 */
import { computed, ref, shallowRef, type Component } from 'vue'
import type { RouteRecordRaw } from 'vue-router'
import { authApi, type AuthInfo, type RouterNode, type AuthUser } from '@/api/auth'
import { session, sessionVersion as sessionVer, type TenantBrief } from '@/api/session'
import { ApiError } from '@/api/http'

/* ---------------- 组件注册表：后端菜单 component 字符串 → 懒加载 ---------------- */

const COMPONENT_MAP: Record<string, () => Promise<{ default: Component }>> = {
  'dashboard/index': () => import('@/views/Dashboard.vue'),
  'system/user/index': () => import('@/views/system/Users.vue'),
  'system/role/index': () => import('@/views/system/Roles.vue'),
  'system/menu/index': () => import('@/views/system/Menus.vue'),
  'system/dept/index': () => import('@/views/system/Depts.vue'),
  'system/post/index': () => import('@/views/system/Posts.vue'),
  'system/dict/index': () => import('@/views/system/Dict.vue'),
  'system/config/index': () => import('@/views/system/Configs.vue'),
  'system/loginlog/index': () => import('@/views/monitor/LoginLogs.vue'),
  'system/operationlog/index': () => import('@/views/monitor/OperationLogs.vue'),
  'platform/tenant/index': () => import('@/views/system/Tenants.vue'),
}

/**
 * 工作台：scaffold-vue 菜单集中的首页位，前端静态提供（后端 /auth/routers 无此菜单）。
 * 页面只读真实接口（账号 / 菜单 / 审计日志），不含虚构监控指标。
 */
const WORKBENCH_LEAF: NavLeaf = {
  key: 'workbench',
  label: '工作台',
  icon: 'ep:grid',
  path: '/',
  component: 'dashboard/index',
}

/* ---------------- 状态 ---------------- */

export interface NavLeaf {
  key: string
  label: string
  icon?: string
  path: string
  component: string
}

export interface NavGroup {
  key: string
  label: string
  icon?: string
  single?: boolean
  items: NavLeaf[]
}

const currentUser = ref<AuthUser | null>(null)
const currentTenant = ref<TenantBrief | null>(null)
const roles = ref<string[]>([])
const permissions = ref<string[]>([])
const navGroups = shallowRef<NavGroup[]>([])
const dynamicRoutes = shallowRef<RouteRecordRaw[]>([])
const componentPaths = shallowRef<Record<string, string>>({})
const routesReady = ref(false)
const authed = computed(() => Boolean(session.loggedIn))

export {
  authed,
  currentUser,
  currentTenant,
  roles,
  permissions,
  navGroups,
  dynamicRoutes,
  componentPaths,
  routesReady,
  sessionVer as sessionVersion,
}

export function hasPermission(code: string): boolean {
  return permissions.value.includes(code)
}

export function pathOf(component: string): string {
  return componentPaths.value[component] ?? ''
}

/** 路径 → 路由名（/system/user → system-user）。 */
export function routeNameOf(path: string): string {
  return path.replace(/^\/+/, '').replace(/\//g, '-') || 'home'
}

/* ---------------- 会话清理 ---------------- */

let sessionRouteReset: (() => void) | null = null

export function setSessionRouteReset(fn: (() => void) | null): void {
  sessionRouteReset = fn
}

export function clearSession(): void {
  session.clear()
  sessionRouteReset?.()
  currentUser.value = null
  currentTenant.value = null
  roles.value = []
  permissions.value = []
  navGroups.value = []
  dynamicRoutes.value = []
  componentPaths.value = {}
  routesReady.value = false
}

/* ---------------- 菜单 → 路由 / 导航 ---------------- */

function flattenLeaf(
  nodes: RouterNode[],
  parentPath: string,
  groupKey: string,
  out: { leaves: NavLeaf[]; routes: RouteRecordRaw[]; compMap: Record<string, string> },
): void {
  for (const n of nodes) {
    const full = joinPath(parentPath, n.path)
    if (n.children?.length && !n.component) {
      flattenLeaf(n.children, full, groupKey, out)
      continue
    }
    if (!n.component || !n.visible) continue
    const loader = COMPONENT_MAP[n.component]
    if (!loader) {
      console.warn('[auth] 未注册的菜单组件：', n.component)
      continue
    }
    const name = routeNameOf(full)
    out.leaves.push({ key: n.id, label: n.name, icon: n.icon, path: full, component: n.component })
    out.compMap[n.component] = full
    out.routes.push({
      name,
      path: full,
      component: loader,
      meta: {
        title: n.name,
        group: groupKey,
        keepAlive: n.keepAlive,
        icon: n.icon,
        permission: undefined,
      },
    })
    if (n.children?.length) flattenLeaf(n.children, full, groupKey, out)
  }
}

function joinPath(parent: string, child: string): string {
  if (child.startsWith('/')) return child
  const base = parent.endsWith('/') ? parent.slice(0, -1) : parent
  return `${base}/${child}`.replace(/\/+/g, '/')
}

function buildNavAndRoutes(nodes: RouterNode[]): void {
  const groups: NavGroup[] = []
  const routes: RouteRecordRaw[] = []
  const compMap: Record<string, string> = { [WORKBENCH_LEAF.component]: WORKBENCH_LEAF.path }
  // 工作台是 root 的空路径静态子路由，这里只把它排进导航。

  for (const node of nodes) {
    const children = node.children ?? []
    if (children.length && !node.component) {
      const out = { leaves: [] as NavLeaf[], routes: [] as RouteRecordRaw[], compMap: {} as Record<string, string> }
      flattenLeaf(children, node.path, node.name, out)
      if (out.leaves.length) {
        groups.push({ key: node.id, label: node.name, icon: node.icon, items: out.leaves })
        routes.push(...out.routes)
        Object.assign(compMap, out.compMap)
      }
    } else if (node.component && node.visible && COMPONENT_MAP[node.component]) {
      const full = joinPath('', node.path)
      const name = routeNameOf(full)
      const leaf: NavLeaf = { key: node.id, label: node.name, icon: node.icon, path: full, component: node.component }
      groups.push({ key: node.id, label: node.name, icon: node.icon, single: true, items: [leaf] })
      compMap[node.component] = full
      routes.push({
        name,
        path: full,
        component: COMPONENT_MAP[node.component]!,
        meta: { title: node.name, group: node.name, icon: node.icon },
      })
    }
  }

  // 工作台排最前（单页组），其后为 /auth/routers 返回的真实菜单
  navGroups.value = [{ key: 'workbench', label: '工作台', single: true, items: [WORKBENCH_LEAF] }, ...groups]
  dynamicRoutes.value = routes
  componentPaths.value = compMap
}

/* ---------------- 会话期动作 ---------------- */

/** 登录：存 token → 拉用户与菜单（调用方负责跳转）。 */
export async function login(
  payload: { tenant_code?: string; username: string; password: string; captcha?: import('@/api/auth').CaptchaAnswer },
): Promise<void> {
  const ver = sessionVer()
  const result = await authApi.login(payload)
  session.save(result, result.tenant)
  await Promise.all([refreshUser(), loadRoutes()])
  if (ver !== sessionVer()) return
}

export async function refreshUser(): Promise<void> {
  const ver = sessionVer()
  const info: AuthInfo = await authApi.info()
  if (ver !== sessionVer()) return
  currentUser.value = info.user
  currentTenant.value = info.tenant
  roles.value = info.roles ?? []
  permissions.value = info.permissions ?? []
}

export async function loadRoutes(): Promise<void> {
  const ver = sessionVer()
  const routers = await authApi.routers()
  if (ver !== sessionVer()) return
  buildNavAndRoutes(routers)
  routesReady.value = true
}

/** 保证路由就绪（路由守卫调用；失败抛出由守卫处理）。 */
export async function ensureSessionData(): Promise<void> {
  const ver = sessionVer()
  await Promise.all([loadRoutes(), refreshUser()])
  if (ver !== sessionVer()) {
    throw new ApiError(401, '会话已切换')
  }
}

export async function logout(): Promise<void> {
  const ver = sessionVer()
  // 慢请求不能带旧 token 干掉新会话
  authApi.logout().catch(() => undefined)
  if (ver === sessionVer()) clearSession()
}

export function homePath(): string {
  return navGroups.value[0]?.items[0]?.path ?? '/'
}
