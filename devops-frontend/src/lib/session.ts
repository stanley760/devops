export interface Session {
  access_token: string | null
  refresh_token: string | null
  tenant_id: string | null
  tenant_code: string | null
  loggedIn: boolean
  ver: number
}

const ACCESS = 'devops.access_token'
const REFRESH = 'devops.refresh_token'
const TENANT_ID = 'devops.tenant_id'
const TENANT_CODE = 'devops.tenant_code'
const VER = 'devops.session_ver'
const LAST_TENANT = 'devops.last_tenant'
const LAST_USER = 'devops.last_user'

function currentVer(): number {
  const raw = Number(localStorage.getItem(VER) ?? '0')
  return Number.isFinite(raw) ? raw : 0
}

function bumpVer(): number {
  const v = currentVer() + 1
  localStorage.setItem(VER, String(v))
  return v
}

let unauthorizedCb: (() => void) | null = null

export const session = {
  sessionVersion: (): number => currentVer(),
  isLoggedIn: (): boolean => localStorage.getItem(ACCESS) != null,
  getAccessToken: (): string | null => localStorage.getItem(ACCESS),
  getRefreshToken: (): string | null => localStorage.getItem(REFRESH),
  getTenantId: (): string | null => localStorage.getItem(TENANT_ID),
  getTenantCode: (): string | null => localStorage.getItem(TENANT_CODE),
  getSession: (): Session => ({
    access_token: localStorage.getItem(ACCESS),
    refresh_token: localStorage.getItem(REFRESH),
    tenant_id: localStorage.getItem(TENANT_ID),
    tenant_code: localStorage.getItem(TENANT_CODE),
    loggedIn: localStorage.getItem(ACCESS) != null,
    ver: currentVer(),
  }),
  saveSession(
    pair: { access_token: string; refresh_token: string },
    tenant: { id: string; code: string } | null,
  ): number {
    localStorage.setItem(ACCESS, pair.access_token)
    localStorage.setItem(REFRESH, pair.refresh_token)
    if (tenant) {
      localStorage.setItem(TENANT_ID, tenant.id)
      localStorage.setItem(TENANT_CODE, tenant.code)
      localStorage.setItem(LAST_TENANT, tenant.code)
    }
    return bumpVer()
  },
  saveTokens(pair: { access_token: string; refresh_token: string }): number {
    localStorage.setItem(ACCESS, pair.access_token)
    localStorage.setItem(REFRESH, pair.refresh_token)
    return bumpVer()
  },
  clearSession(): number {
    localStorage.removeItem(ACCESS)
    localStorage.removeItem(REFRESH)
    localStorage.removeItem(TENANT_ID)
    localStorage.removeItem(TENANT_CODE)
    return bumpVer()
  },
  setLastUser(username: string): void {
    localStorage.setItem(LAST_USER, JSON.stringify({ username }))
  },
  getLastTenant(): string | null {
    return localStorage.getItem(LAST_TENANT)
  },
  onUnauthorized(cb: () => void): () => void {
    unauthorizedCb = cb
    return () => {
      if (unauthorizedCb === cb) unauthorizedCb = null
    }
  },
  fireUnauthorized(): void {
    unauthorizedCb?.()
  },
}
