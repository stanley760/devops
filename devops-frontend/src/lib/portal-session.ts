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
