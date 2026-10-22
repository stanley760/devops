/**
 * 统一 HTTP 客户端：devops 后端信封 {code,msg,data} 解包。
 * - 200 → 返回 data
 * - 401 → 尝试 refresh 轮换后重试一次，失败则清理会话并抛出
 * - 其余 → 抛 ApiError（携带业务 code 与 msg）
 */
import { session, sessionVersion } from './session'

export class ApiError extends Error {
  readonly code: number
  constructor(code: number, msg: string) {
    super(msg)
    this.name = 'ApiError'
    this.code = code
  }
}

export interface Envelope<T> {
  code: number
  msg: string
  data: T
}

export interface PageResult<T> {
  list: T[]
  total: number
  page_num: number
  page_size: number
  pages: number
}

export interface Query {
  [k: string]: string | number | boolean | undefined | null
}

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  body?: unknown
  query?: Query
  /** false = 匿名请求（登录 / 刷新 / 验证码等） */
  auth?: boolean
  /** 发起时的会话版本，过期响应不再触发跳转 */
  ver?: number
}

export type UnauthorizedHandler = () => void

let onUnauthorized: UnauthorizedHandler | null = null
let refreshing: Promise<boolean> | null = null

/** 注册会话失效回调（router 挂载登录跳转）。 */
export function setUnauthorizedHandler(fn: UnauthorizedHandler | null): void {
  onUnauthorized = fn
}

function buildUrl(path: string, query?: Query): string {
  if (!query) return path
  const usp = new URLSearchParams()
  for (const [k, v] of Object.entries(query)) {
    if (v === undefined || v === null || v === '') continue
    usp.set(k, String(v))
  }
  const qs = usp.toString()
  return qs ? `${path}?${qs}` : path
}

function authHeaders(auth: boolean): Record<string, string> {
  const h: Record<string, string> = {}
  if (!auth) return h
  const token = session.accessToken
  if (token) h.Authorization = `Bearer ${token}`
  const tenant = session.tenantId
  if (tenant) h['X-Tenant-ID'] = tenant
  return h
}

async function parseEnvelope<T>(res: Response, ver: number | undefined): Promise<T> {
  let payload: Envelope<T>
  try {
    payload = (await res.json()) as Envelope<T>
  } catch {
    throw new ApiError(res.status, `响应解析失败（HTTP ${res.status}）`)
  }
  const code = payload?.code ?? res.status
  if (code === 200) return payload.data
  // 会话已切换时，忽略旧会话的 401（避免互相顶跳）
  if (code === 401 && ver !== undefined && ver !== sessionVersion()) {
    throw new ApiError(401, '会话已切换')
  }
  throw new ApiError(code, payload?.msg || `请求失败（${code}）`)
}

/** 轮换 token；并发 401 只触发一次刷新。 */
async function refreshTokens(): Promise<boolean> {
  if (!refreshing) {
    refreshing = (async () => {
      const rt = session.refreshToken
      if (!rt) return false
      try {
        const res = await fetch('/system/api/v1/auth/refresh', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: rt }),
        })
        const payload = (await res.json()) as Envelope<{ access_token: string; refresh_token: string; expires_in: number }>
        if (payload?.code !== 200) return false
        session.saveTokens(payload.data)
        return true
      } catch {
        return false
      } finally {
        refreshing = null
      }
    })()
  }
  return refreshing
}

async function rawFetch(path: string, opts: RequestOptions): Promise<Response> {
  const method = opts.method ?? 'GET'
  const headers: Record<string, string> = {
    Accept: 'application/json',
    ...authHeaders(opts.auth !== false),
  }
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json'
  return fetch(buildUrl(path, opts.query), {
    method,
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })
}

/** 发起请求并解包信封。 */
export async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const ver = opts.ver ?? sessionVersion()
  const res = await rawFetch(path, opts)
  let code = res.status
  try {
    return await parseEnvelope<T>(res, ver)
  } catch (e) {
    if (!(e instanceof ApiError)) throw e
    code = e.code
    // 401：刷新后原请求重试一次
    if (code === 401 && opts.auth !== false) {
      const ok = await refreshTokens()
      if (ok && ver === sessionVersion()) {
        const res2 = await rawFetch(path, opts)
        return parseEnvelope<T>(res2, ver)
      }
      if (ver === sessionVersion()) {
        session.clear()
        onUnauthorized?.()
      }
    }
    throw e
  }
}

export const http = {
  get: <T>(path: string, query?: Query, opts?: RequestOptions) =>
    request<T>(path, { ...opts, method: 'GET', query }),
  post: <T>(path: string, body?: unknown, opts?: RequestOptions) =>
    request<T>(path, { ...opts, method: 'POST', body }),
  put: <T>(path: string, body?: unknown, opts?: RequestOptions) =>
    request<T>(path, { ...opts, method: 'PUT', body }),
  del: <T>(path: string, body?: unknown, opts?: RequestOptions) =>
    request<T>(path, { ...opts, method: 'DELETE', body }),
}
