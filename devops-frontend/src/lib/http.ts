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
