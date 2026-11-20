import { describe, it, expect, beforeEach, vi } from 'vitest'
import { request, ApiError, buildQuery } from '@/lib/http'
import { session } from '@/lib/session'

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('buildQuery', () => {
  it('skips undefined/null/empty values', () => {
    expect(buildQuery({ a: '1', b: undefined, c: null, d: '', e: 'ok' })).toBe('?a=1&e=ok')
  })
  it('returns empty string for empty object', () => {
    expect(buildQuery({})).toBe('')
  })
})

function envelopeOk(data: unknown, status = 200) {
  return new Response(JSON.stringify({ code: 200, msg: 'ok', data }), { status })
}

describe('request envelope', () => {
  it('unwraps data when code === 200', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(envelopeOk({ hello: 1 }))
    await expect(request<{ hello: number }>('/x')).resolves.toEqual({ hello: 1 })
  })

  it('throws ApiError with code/msg when code !== 200', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(
      new Response(JSON.stringify({ code: 400, msg: '参数错误' }), { status: 200 }),
    )
    const err = await request('/x').catch((e) => e)
    expect(err).toBeInstanceOf(ApiError)
    expect((err as ApiError).code).toBe(400)
    expect((err as ApiError).msg).toBe('参数错误')
  })

  it('throws parse-failure error for non-JSON response', async () => {
    vi.spyOn(global, 'fetch').mockResolvedValue(new Response('not json', { status: 502 }))
    const err = await request('/x').catch((e) => e)
    expect((err as ApiError).msg).toBe('响应解析失败（HTTP 502）')
  })

  it('sends Bearer + X-Tenant-ID when logged in', async () => {
    session.saveSession({ access_token: 'tok', refresh_token: 'r' }, { id: '42', code: 'acme' })
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(envelopeOk(null))
    await request('/x')
    const headers = new Headers((spy.mock.calls[0]![1] as RequestInit).headers)
    expect(headers.get('Authorization')).toBe('Bearer tok')
    expect(headers.get('X-Tenant-ID')).toBe('42')
  })

  it('single-flights concurrent 401 refresh and retries once', async () => {
    session.saveSession({ access_token: 'old', refresh_token: 'r1' }, { id: '42', code: 'acme' })
    let refreshCalls = 0
    vi.spyOn(global, 'fetch').mockImplementation(async (_url, init) => {
      const url = _url as string
      if (url.includes('/auth/refresh')) {
        refreshCalls++
        return envelopeOk({ access_token: 'new', refresh_token: 'r2', expires_in: 3600 })
      }
      const auth = new Headers((init as RequestInit).headers).get('Authorization')
      if (auth === 'Bearer old') return new Response('', { status: 401 })
      return envelopeOk('done')
    })
    const [r1, r2] = await Promise.all([request<string>('/x'), request<string>('/y')])
    expect(r1).toBe('done')
    expect(r2).toBe('done')
    expect(refreshCalls).toBe(1)
    expect(session.getAccessToken()).toBe('new')
  })

  it('clears session and fires unauthorized when refresh fails', async () => {
    session.saveSession({ access_token: 'old', refresh_token: 'bad' }, { id: '42', code: 'acme' })
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/auth/refresh')) {
        return new Response(JSON.stringify({ code: 401, msg: 'expired' }), { status: 200 })
      }
      return new Response('', { status: 401 })
    })
    let fired = false
    const off = session.onUnauthorized(() => {
      fired = true
    })
    const err = await request('/x').catch((e) => e)
    off()
    expect(fired).toBe(true)
    expect(session.isLoggedIn()).toBe(false)
    expect((err as ApiError).status).toBe(401)
  })

  it('treats stale-version 401 as session switched (no refresh)', async () => {
    session.saveSession({ access_token: 'old', refresh_token: 'r' }, { id: '42', code: 'acme' })
    let refreshCalls = 0
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      if ((url as string).includes('/auth/refresh')) {
        refreshCalls++
        return envelopeOk({ access_token: 'n', refresh_token: 'r2', expires_in: 1 })
      }
      return new Response('', { status: 401 })
    })
    const p = request('/x')
    // 模拟在请求飞行中会话被切换
    session.saveTokens({ access_token: 'other', refresh_token: 'other' })
    const err = await p.catch((e) => e)
    expect((err as ApiError).msg).toContain('会话已切换')
    expect(refreshCalls).toBe(0)
  })
})
