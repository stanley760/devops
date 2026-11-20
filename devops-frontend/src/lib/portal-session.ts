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
let refreshTokensAtStart: PortalTokenPair | null = null

function doRefresh(): Promise<boolean> {
  if (!refreshing) {
    refreshTokensAtStart = portalSession.getTokens()
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

function tokensUnchanged(tokens: PortalTokenPair | null): boolean {
  return (
    !!tokens &&
    !!refreshTokensAtStart &&
    tokens.access_token === refreshTokensAtStart.access_token &&
    tokens.refresh_token === refreshTokensAtStart.refresh_token
  )
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
  const tokensAtStart = portalSession.getTokens()
  if (auth && tokensAtStart) headers.set('Authorization', `Bearer ${tokensAtStart.access_token}`)

  const url = withQuery(path, opts.query)
  let res = await fetch(url, { ...opts, headers })
  let envelope: { code: number; msg?: string; data?: unknown }
  try {
    envelope = (await res.json()) as { code: number; msg?: string; data?: unknown }
  } catch {
    envelope = null as never
  }
  if ((res.status === 401 || envelope?.code === 401) && auth) {
    if (refreshing && tokensUnchanged(tokensAtStart)) {
      const ok = await refreshing
      if (!ok) {
        portalSession.clearTokens()
        throw new PortalApiError(401, '登录已过期，请重新登录')
      }
    } else if (!refreshing) {
      const ok = await doRefresh()
      if (!ok) {
        portalSession.clearTokens()
        throw new PortalApiError(401, '登录已过期，请重新登录')
      }
    }
    const next = portalSession.getTokens()
    if (!next) {
      throw new PortalApiError(401, '登录已过期，请重新登录')
    }
    headers.set('Authorization', `Bearer ${next.access_token}`)
    res = await fetch(url, { ...opts, headers })
    try {
      envelope = (await res.json()) as { code: number; msg?: string; data?: unknown }
    } catch {
      envelope = null as never
    }
    if (res.status === 401 || envelope?.code === 401) {
      portalSession.clearTokens()
      throw new PortalApiError(401, '登录已过期，请重新登录')
    }
  }
  if (!envelope) {
    throw new PortalApiError(res.status, `响应解析失败（HTTP ${res.status}）`)
  }
  if (envelope.code !== 200) {
    throw new PortalApiError(res.status, envelope.msg || `请求失败（HTTP ${res.status}）`)
  }
  return envelope.data as T
}
