import { portalRequest } from '@/lib/portal-session'
import type { CaptchaChallenge, PortalTokenPair, PublicProfile } from '@/lib/types'

const BASE = '/user/api/v1'

export const portalAuth = {
  getCaptcha(): Promise<CaptchaChallenge> {
    return portalRequest<CaptchaChallenge>(`${BASE}/auth/captcha`, { auth: false })
  },
  login(body: { username: string; password: string } & Record<string, unknown>): Promise<PortalTokenPair> {
    return portalRequest<PortalTokenPair>(`${BASE}/auth/login`, {
      method: 'POST', auth: false, body: JSON.stringify(body),
    })
  },
  register(body: {
    username: string
    nickname: string
    email: string
    password: string
  } & Record<string, unknown>): Promise<PortalTokenPair> {
    return portalRequest<PortalTokenPair>(`${BASE}/auth/register`, {
      method: 'POST', auth: false, body: JSON.stringify(body),
    })
  },
  refresh(refresh_token: string): Promise<PortalTokenPair> {
    return portalRequest<PortalTokenPair>(`${BASE}/auth/refresh`, {
      method: 'POST', auth: false, body: JSON.stringify({ refresh_token }),
    })
  },
  logout(refresh_token?: string): Promise<void> {
    return portalRequest<void>(`${BASE}/auth/logout`, {
      method: 'POST',
      body: JSON.stringify(refresh_token ? { refresh_token } : {}),
    })
  },
  getProfile(): Promise<PublicProfile> {
    return portalRequest<PublicProfile>(`${BASE}/profile`)
  },
  updateProfile(body: { nickname?: string; email?: string; mobile?: string; gender?: string }): Promise<void> {
    return portalRequest<void>(`${BASE}/profile`, { method: 'PUT', body: JSON.stringify(body) })
  },
  updatePassword(body: { old_password: string; new_password: string }): Promise<void> {
    return portalRequest<void>(`${BASE}/profile/password`, { method: 'PUT', body: JSON.stringify(body) })
  },
}
