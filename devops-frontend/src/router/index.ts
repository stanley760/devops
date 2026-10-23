/**
 * 路由：静态入口（登录 / 工作台 / 异常页）+ 会话后动态注册的业务菜单路由。
 * 动态路由全部来自 GET /system/api/v1/auth/routers，前端不虚构菜单。
 */
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { setUnauthorizedHandler } from '@/api/http'
import { session } from '@/api/session'
import {
  authed,
  clearSession,
  dynamicRoutes,
  ensureSessionData,
  homePath,
  routesReady,
  setSessionRouteReset,
} from '@/composables/auth'
import { pushToast } from '@/composables/toast'
import { resolveTenantEntry } from '@/composables/tenantEntry'

const AppShell = () => import('@/components/layout/AppShell.vue')

const staticRoutes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/Login.vue'),
    meta: { public: true, title: '登录' },
  },
  {
    // 路径带租户的登录入口：/t/{tenant_code}/login。租户由 URL 指定，登录框只剩账号密码。
    // 与 /login 共用组件，参数经路由注入（见 Login.vue 的 tenantEntry 解析）。
    path: '/t/:tenantCode/login',
    name: 'login-tenant',
    component: () => import('@/views/Login.vue'),
    meta: { public: true, title: '登录' },
  },
  {
    path: '/',
    name: 'root',
    component: AppShell,
    children: [
      {
        path: '',
        name: 'workbench',
        component: () => import('@/views/Dashboard.vue'),
        meta: { title: '工作台', group: '工作台' },
      },
      {
        path: 'no-access',
        name: 'no-access',
        component: () => import('@/views/NoAccess.vue'),
        meta: { title: '无权限', group: '系统' },
      },
      {
        path: ':pathMatch(.*)*',
        name: 'not-found',
        component: () => import('@/views/NotFound.vue'),
        meta: { title: '页面不存在', group: '系统' },
      },
    ],
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes: staticRoutes,
  scrollBehavior: () => ({ top: 0 }),
})

/* ---- 动态路由管理 ---- */

const removers: Array<() => void> = []

export function resetRoutes(): void {
  for (const rm of removers) rm()
  removers.length = 0
  registered = false
}

setSessionRouteReset(resetRoutes)

let loading: Promise<void> | null = null

/**
 * 路由是否已注册进 router。
 *
 * 不能拿 `routesReady` 当判断依据：它只表示「菜单数据拉到了」。
 * 登录流程直接调 `loadRoutes()` 会把它置 true，但**不会注册路由**；
 * 而守卫原先只在 `!routesReady` 时才调本函数，于是登录后注册被永久跳过，
 * 所有业务路径都落到 catch-all 上显示 404。所以这里单独跟踪注册状态。
 */
let registered = false

async function ensureRoutes(): Promise<void> {
  if (registered) return
  // 数据没拉过才去拉（页面刷新后首次进入受保护路由的场景）
  if (!routesReady.value) {
    if (!loading) {
      loading = ensureSessionData().finally(() => {
        loading = null
      })
    }
    await loading
    // 并发导航下，等待期间可能已被另一个调用注册完
    if (registered) return
  }
  // 注册是同步的，中间没有 await，不会出现重复注册
  for (const r of dynamicRoutes.value) {
    removers.push(router.addRoute('root', r))
  }
  registered = true
}

/* ---- 守卫 ---- */

router.beforeEach(async (to) => {
  if (to.meta.public) {
    // login 与 login-tenant（/t/:tenantCode/login）都是登录页：已登录就别再看登录框
    if (authed.value && (to.name === 'login' || to.name === 'login-tenant')) return { path: homePath() }
    return true
  }
  if (!authed.value) {
    // 未登录一律回登录页，并按兜底链带上租户编码：直接访问 / 时就是
    // /login?tenantCode=default（上次登过别的租户则带那个编码）。
    // 登录页只从地址解析租户、页面不提供租户输入；换租户 = 改地址栏 tenantCode 的值。
    const entry = resolveTenantEntry()
    return {
      name: 'login',
      query: {
        tenantCode: entry.code,
        // 首页不带 redirect：登录后本来就回工作台，带了反而多个无意义参数
        ...(to.fullPath === '/' ? {} : { redirect: to.fullPath }),
      },
    }
  }
  if (!routesReady.value) {
    try {
      await ensureRoutes()
    } catch {
      clearSession()
      pushToast('会话已失效，请重新登录', 'warn')
      return {
        name: 'login',
        query: { tenantCode: resolveTenantEntry().code, redirect: to.fullPath },
      }
    }
    // 重新按 path 匹配一次，让新注册的业务路由生效。
    //
    // 不能写 `{ ...to, replace: true }`：`to` 里带着本次匹配到的 name
    // （深链接首次进来时匹配到的是 catch-all 的 'not-found'），而 vue-router
    // 解析时 name 优先于 path，于是又回到 404 路由，注册了也白注册。
    // 只带 path，强制重新走一遍路径匹配。
    return { path: to.fullPath, replace: true }
  }
  // 菜单数据已就绪（登录流程拉的），但路由可能还没注册，补一次。
  // ensureRoutes 内部幂等，已注册则立即返回。
  await ensureRoutes()
  return true
})

router.afterEach((to) => {
  const title = (to.meta?.title as string) || ''
  document.title = title ? `${title} · DevOps 钛玻璃` : 'DevOps 钛玻璃控制台'
})

/* ---- 401 统一跳转 ---- */

let redirecting = false

setUnauthorizedHandler(() => {
  if (redirecting) return
  redirecting = true
  // 先取出租户再清会话：会话失效回登录页时尽量保住入口租户上下文，
  // 统一回 /login?tenantCode=xxx（登录页从地址解析租户，页面无租户输入）。
  const tenantCode = session.tenantCode
  clearSession()
  pushToast('会话已失效，请重新登录', 'warn')
  const current = router.currentRoute.value
  router
    .replace({
      name: 'login',
      query: { tenantCode: tenantCode || 'default', redirect: current.fullPath },
    })
    .finally(() => {
      redirecting = false
    })
})
