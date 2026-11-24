import { request } from '@/lib/http'
import type { AuthInfo, CaptchaChallenge, LoginResult, RouterNode, TokenPair } from '@/lib/types'

const BASE = '/system/api/v1/auth'

export const adminAuth = {
  getCaptcha(tenantCode?: string): Promise<CaptchaChallenge> {
    return request<CaptchaChallenge>(`${BASE}/captcha`, { auth: false, query: { tenant_code: tenantCode } })
  },
  login(
    body: { username: string; password: string; tenant_code: string } & Record<string, unknown>,
  ): Promise<LoginResult> {
    return request<LoginResult>(`${BASE}/login`, { method: 'POST', auth: false, body: JSON.stringify(body) })
  },
  refresh(refresh_token: string): Promise<TokenPair> {
    return request<TokenPair>(`${BASE}/refresh`, {
      method: 'POST',
      auth: false,
      body: JSON.stringify({ refresh_token }),
    })
  },
  logout(): Promise<void> {
    return request<void>(`${BASE}/logout`, { method: 'POST' })
  },
  getInfo(): Promise<AuthInfo> {
    return request<AuthInfo>(`${BASE}/info`)
  },
  getRouters(): Promise<RouterNode[]> {
    return request<RouterNode[]>(`${BASE}/routers`)
  },
  updateProfile(body: {
    nickname?: string
    email?: string
    mobile?: string
    gender?: number
    avatar?: string
  }): Promise<void> {
    return request<void>(`${BASE}/profile`, { method: 'PUT', body: JSON.stringify(body) })
  },
  updatePassword(body: { old_password: string; new_password: string }): Promise<void> {
    return request<void>(`${BASE}/profile/password`, { method: 'PUT', body: JSON.stringify(body) })
  },
}
