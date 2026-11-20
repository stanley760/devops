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
