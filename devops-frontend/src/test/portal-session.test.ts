import { describe, it, expect, beforeEach, vi } from 'vitest'
import { portalRequest, PortalApiError, portalSession } from '@/lib/portal-session'

const tokens = () => ({ access_token: 'a', refresh_token: 'r', expires_in: 3600, issued_at: Date.now() })

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('portal tokens storage', () => {
  it('saves/loads/clears portal_tokens', () => {
    portalSession.saveTokens(tokens())
    expect(portalSession.getTokens()!.access_token).toBe('a')
    portalSession.clearTokens()
    expect(portalSession.getTokens()).toBeNull()
    expect(portalSession.isLoggedIn()).toBe(false)
  })
})

describe('portalRequest', () => {
  function ok(data: unknown) {
    return new Response(JSON.stringify({ code: 200, data }), { status: 200 })
  }
  it('unwraps data', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(ok({ p: 1 }))
    await expect(portalRequest<{ p: number }>('/x')).resolves.toEqual({ p: 1 })
  })
  it('single-flight refresh then retry once', async () => {
    portalSession.saveTokens(tokens())
    let refreshCalls = 0
    vi.spyOn(global, 'fetch').mockImplementation(async (url, init) => {
      if ((url as string).includes('/auth/refresh')) {
        refreshCalls++
        return ok({ ...tokens(), access_token: 'n', refresh_token: 'r2' })
      }
      const auth = new Headers((init as RequestInit).headers).get('Authorization')
      if (auth === 'Bearer a') return new Response('', { status: 401 })
      return ok('done')
    })
    const [r1, r2] = await Promise.all([portalRequest<string>('/x'), portalRequest<string>('/y')])
    expect(r1).toBe('done')
    expect(r2).toBe('done')
    expect(refreshCalls).toBe(1)
  })
  it('clears tokens and throws 401 PortalApiError when refresh fails', async () => {
    portalSession.saveTokens(tokens())
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/auth/refresh')) {
        return new Response(JSON.stringify({ code: 401, msg: 'expired' }), { status: 200 })
      }
      return new Response('', { status: 401 })
    })
    const err = await portalRequest('/x').catch((e) => e)
    expect(err).toBeInstanceOf(PortalApiError)
    expect((err as PortalApiError).msg).toBe('登录已过期，请重新登录')
    expect(portalSession.isLoggedIn()).toBe(false)
  })
})
