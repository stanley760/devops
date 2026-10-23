/**
 * 登录入口的租户解析：查询参数 → 路径 → 子域 → 记住的租户 → 默认租户。
 *
 * 为什么要从 URL 带租户（而不是登录时选）：
 *   - 租户是「组织」不是「选项」。用户从组织入口进来，URL/域名本身就是身份的一部分；
 *     让人在登录框里挑租户既多一步操作，也容易登进错误的组织。
 *   - 查询参数 /login?tenantCode=acme（或路径 /t/acme/login、子域 acme.admin.example.com）
 *     一旦确定，登录框只剩账号 + 密码，与大型 SaaS 控制台的登录形态一致。
 *   - 换租户的方式就是改地址栏 tenantCode 的值，无需任何页面交互。
 *
 * 兜底链（依次短路，**不能**反过来）：
 *   1. 查询参数 ?tenantCode= —— 默认入口：未登录访问任意页都会被守卫带回
 *      /login?tenantCode=xxx（首页即 tenantCode=default）；
 *   2. 路径参数 /t/{code}/login —— 显式入口（分享链接、书签）；
 *   3. 子域首段 —— 通配域名部署（*.admin.example.com）时用户完全无感知；
 *   4. localStorage 记住的上次租户（devops.last_tenant）；
 *   5. 'default' —— 与后端对齐：LoginReq.tenant_code 为空时后端回退 tenant.default_code，
 *      前端显式带上 'default' 只是把这层兜底提前可见（验证码接口也依赖它定位租户配置）。
 *
 * 前三项命中即「锁定」：登录页不渲染租户输入，防止用户改掉 URL 指定的组织。
 */

/** 子域里不能当租户编码的保留段（平台自用入口）。 */
const RESERVED_SUBDOMAINS = new Set([
  'www',
  'admin',
  'app',
  'api',
  'console',
  'portal',
  'static',
  'cdn',
  'dev',
  'test',
  'staging',
])

const K_LAST_TENANT = 'devops.last_tenant'

const FALLBACK_TENANT = 'default'

export type TenantSource = 'query' | 'path' | 'host' | 'stored' | 'default'

export interface TenantEntry {
  /** 租户编码（登录请求提交的就是它）。 */
  code: string
  /** 来源，用于提示文案与排查。 */
  source: TenantSource
  /** 路径/子域带入时为 true：登录页锁定租户，不再提供选择。 */
  locked: boolean
}

/**
 * 从子域首段取租户编码：acme.admin.example.com → "acme"。
 *
 * 只认至少三级的域名：example.com 这种二级域名的首段是品牌不是租户；
 * localhost 与 IP 直连没有租户语义，一律返回 null。
 */
export function tenantFromHost(hostname: string): string | null {
  const host = hostname.toLowerCase()
  if (!host || host === 'localhost') return null
  if (/^\d{1,3}(\.\d{1,3}){3}$/.test(host)) return null
  const labels = host.split('.')
  if (labels.length < 3) return null
  const head = labels[0]
  if (!head || RESERVED_SUBDOMAINS.has(head)) return null
  return head
}

/** 从路径取租户编码：/t/acme/login → "acme"。 */
export function tenantFromPath(pathname: string): string | null {
  const m = /^\/t\/([^/]+)/.exec(pathname)
  if (!m) return null
  try {
    return decodeURIComponent(m[1]) || null
  } catch {
    return m[1]
  }
}

/**
 * 解析登录入口的租户。
 *
 * @param pathCode 路由参数 :tenantCode（/t/:tenantCode/login 匹配时传入，比再解析 pathname 更准）
 * @param queryCode 查询参数 ?tenantCode=（默认入口，优先级最高：守卫会把未登录访问统一带回这里）
 */
export function resolveTenantEntry(pathCode?: string | null, queryCode?: string | null): TenantEntry {
  const fromQuery = (queryCode ?? new URLSearchParams(window.location.search).get('tenantCode'))?.trim()
  if (fromQuery) return { code: fromQuery, source: 'query', locked: true }
  const fromPath = (pathCode ?? tenantFromPath(window.location.pathname))?.trim()
  if (fromPath) return { code: fromPath, source: 'path', locked: true }
  const fromHost = tenantFromHost(window.location.hostname)
  if (fromHost) return { code: fromHost, source: 'host', locked: true }
  const stored = window.localStorage.getItem(K_LAST_TENANT)?.trim()
  if (stored) return { code: stored, source: 'stored', locked: false }
  return { code: FALLBACK_TENANT, source: 'default', locked: false }
}

/** 记住本次登录的租户（下次访问没带 tenantCode 时的兜底值）。 */
export function rememberTenant(code: string): void {
  window.localStorage.setItem(K_LAST_TENANT, code.trim() || FALLBACK_TENANT)
}
