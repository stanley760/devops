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
// 记录共享刷新前后的会话版本：兄弟请求晚到 401 时据此判定版本变化来自本次刷新
let refreshFromVersion: number | null = null
let refreshToVersion: number | null = null

function doRefresh(): Promise<boolean> {
  if (!refreshing) {
    refreshFromVersion = session.sessionVersion()
    refreshToVersion = null
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
      refreshToVersion = session.sessionVersion()
      return true
    })().finally(() => {
      refreshing = null
    })
  }
  return refreshing
}

function isOwnRefresh(verAtStart: number): boolean {
  // refreshToVersion 仅在刷新成功后赋值；后续会话切换会再次 bump 版本，因此不会误判
  return refreshToVersion !== null && refreshFromVersion === verAtStart && session.sessionVersion() === refreshToVersion
}

interface Envelope {
  code: number
  msg?: string
  data?: unknown
}

async function readEnvelope(res: Response): Promise<Envelope | null> {
  try {
    return (await res.json()) as Envelope
  } catch {
    return null
  }
}

function isUnauthorized(res: Response, envelope: Envelope | null): boolean {
  return res.status === 401 || envelope?.code === 401
}

interface RequestOptions extends RequestInit {
  auth?: boolean
  query?: Record<string, unknown>
  /** blob = 二进制响应（导出等），跳过 JSON 信封解包但仍走统一认证与 401 刷新 */
  responseType?: 'json' | 'blob'
}

export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const auth = opts.auth !== false
  const verAtStart = session.sessionVersion()
  const headers = new Headers(opts.headers)
  headers.set('Accept', 'application/json')
  if (opts.body) headers.set('Content-Type', 'application/json')
  const applyAuth = (): void => {
    if (!auth) return
    const token = session.getAccessToken()
    if (token) headers.set('Authorization', `Bearer ${token}`)
    const tenantId = session.getTenantId()
    if (tenantId) headers.set('X-Tenant-ID', tenantId)
  }
  applyAuth()

  const send = async (): Promise<Response> =>
    fetch(`${path}${buildQuery(opts.query)}`, { ...opts, headers })
  const parse = async (res: Response): Promise<Envelope | null> =>
    opts.responseType === 'blob' ? null : readEnvelope(res)

  let res = await send()
  let envelope = await parse(res)

  // HTTP 401 或信封 code 401 均按未授权处理
  if (isUnauthorized(res, envelope) && auth) {
    // 会话版本守卫：版本变化若由本次共享刷新造成，属于同会话续期，不应报“会话已切换”
    if (verAtStart !== session.sessionVersion() && !isOwnRefresh(verAtStart)) {
      throw new ApiError(res.status, 401, '会话已切换，请重新操作')
    }
    if (!isOwnRefresh(verAtStart)) {
      const ok = await doRefresh()
      if (!ok) {
        session.clearSession()
        session.fireUnauthorized()
        throw new ApiError(res.status, 401, '登录已过期，请重新登录')
      }
    }
    // 只重试一次，并使用刷新后的 token
    headers.set('Authorization', `Bearer ${session.getAccessToken()}`)
    res = await send()
    envelope = await parse(res)
    if (isUnauthorized(res, envelope)) {
      session.clearSession()
      session.fireUnauthorized()
      throw new ApiError(res.status, 401, '登录已过期，请重新登录')
    }
  }

  if (opts.responseType === 'blob') {
    if (!res.ok) throw new ApiError(res.status, res.status, `请求失败（HTTP ${res.status}）`)
    return (await res.blob()) as T
  }
  if (!envelope) {
    throw new ApiError(res.status, res.status, `响应解析失败（HTTP ${res.status}）`)
  }
  if (envelope.code !== 200) {
    throw new ApiError(res.status, envelope.code, envelope.msg || `请求失败（HTTP ${res.status}）`)
  }
  return envelope.data as T
}
