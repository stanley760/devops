/**
 * C 端门户 API：对接 devops-user（/user/api/v1/...）。
 * 信封 {code,msg,data}，200 解包；token 存 localStorage（portal_*）。
 */

const BASE = '/user/api/v1'
const TOKEN_KEY = 'portal_tokens'

export interface TokenPair {
  access_token: string
  refresh_token: string
  expires_in: number
  issued_at: number
}

export interface PublicProfile {
  id: number
  username: string
  nickname: string
  email: string
  mobile: string
  gender: number
  avatar: string
  created_at: number
}

/** 行为式验证码挑战（与后端 service.CaptchaChallenge 对齐）。 */
export interface CaptchaChallenge {
  id: string
  type: 'click' | 'slide' | 'dragdrop' | 'rotate'
  image: string
  thumb?: string
  width: number
  height: number
  text?: string
}

/** 验证码作答（与后端 service.CaptchaInput 对齐）。 */
export interface CaptchaAnswer {
  captcha_id: string
  captcha_points?: Array<{ x: number; y: number }>
  captcha_x?: number
  captcha_y?: number
  captcha_angle?: number
}

export class PortalApiError extends Error {
  readonly code: number
  constructor(code: number, msg: string) {
    super(msg)
    this.name = 'PortalApiError'
    this.code = code
  }
}

export const portalSession = {
  get tokens(): TokenPair | null {
    try {
      const raw = localStorage.getItem(TOKEN_KEY)
      return raw ? (JSON.parse(raw) as TokenPair) : null
    } catch {
      return null
    }
  },
  save(pair: TokenPair): void {
    localStorage.setItem(TOKEN_KEY, JSON.stringify(pair))
  },
  clear(): void {
    localStorage.removeItem(TOKEN_KEY)
  },
  get loggedIn(): boolean {
    return !!this.tokens?.access_token
  },
}

interface Envelope<T> { code: number; msg: string; data: T }

async function request<T>(path: string, opts: { method?: string; body?: unknown; auth?: boolean } = {}): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (opts.body !== undefined) headers['Content-Type'] = 'application/json'
  if (opts.auth !== false) {
    const token = portalSession.tokens?.access_token
    if (token) headers.Authorization = `Bearer ${token}`
  }
  const res = await fetch(`${BASE}${path}`, {
    method: opts.method ?? 'GET',
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  })
  let payload: Envelope<T>
  try {
    payload = (await res.json()) as Envelope<T>
  } catch {
    throw new PortalApiError(res.status, `响应解析失败（HTTP ${res.status}）`)
  }
  const code = payload?.code ?? res.status
  if (code === 200) return payload.data

  // 401：尝试刷新一次后重试
  if (code === 401 && opts.auth !== false) {
    const ok = await tryRefresh()
    if (ok) return request<T>(path, { ...opts, auth: true })
    portalSession.clear()
    throw new PortalApiError(401, payload?.msg || '登录已过期，请重新登录')
  }
  throw new PortalApiError(code, payload?.msg || `请求失败（${code}）`)
}

let refreshing: Promise<boolean> | null = null
async function tryRefresh(): Promise<boolean> {
  if (!refreshing) {
    refreshing = (async () => {
      const rt = portalSession.tokens?.refresh_token
      if (!rt) return false
      try {
        const res = await fetch(`${BASE}/auth/refresh`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ refresh_token: rt }),
        })
        const payload = (await res.json()) as Envelope<TokenPair>
        if (payload?.code !== 200) return false
        portalSession.save(payload.data)
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

export const portalApi = {
  captcha(): Promise<CaptchaChallenge> {
    return request<CaptchaChallenge>('/auth/captcha', { auth: false })
  },
  register(body: { username: string; password: string; nickname?: string; email?: string; mobile?: string; captcha?: CaptchaAnswer }): Promise<TokenPair> {
    return request<TokenPair>('/auth/register', { method: 'POST', body, auth: false })
  },
  login(body: { username: string; password: string; captcha?: CaptchaAnswer }): Promise<TokenPair> {
    return request<TokenPair>('/auth/login', { method: 'POST', body, auth: false })
  },
  logout(): Promise<void> {
    return request<void>('/auth/logout', { method: 'POST' })
  },
  profile(): Promise<PublicProfile> {
    return request<PublicProfile>('/profile')
  },
  updateProfile(body: { nickname?: string; email?: string; mobile?: string; gender?: number; avatar?: string }): Promise<PublicProfile> {
    return request<PublicProfile>('/profile', { method: 'PUT', body })
  },
  changePassword(body: { old_password: string; new_password: string; keep_current_device?: boolean }): Promise<void> {
    return request<void>('/profile/password', { method: 'PUT', body })
  },
}
