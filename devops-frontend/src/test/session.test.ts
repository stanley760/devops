import { describe, it, expect, beforeEach } from 'vitest'
import { session } from '@/lib/session'

beforeEach(() => localStorage.clear())

describe('admin session store', () => {
  it('saveSession writes tokens/tenant and bumps version', () => {
    const v0 = session.sessionVersion()
    const v1 = session.saveSession(
      { access_token: 'a', refresh_token: 'r' },
      { id: '1', code: 'acme' },
    )
    expect(v1).toBe(v0 + 1)
    expect(session.isLoggedIn()).toBe(true)
    expect(session.getTenantCode()).toBe('acme')
    expect(localStorage.getItem('devops.last_tenant')).toBe('acme')
  })

  it('clearSession removes token/tenant keys and bumps version', () => {
    session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
    const v = session.clearSession()
    expect(session.isLoggedIn()).toBe(false)
    expect(session.getTenantId()).toBeNull()
    expect(session.sessionVersion()).toBe(v)
  })

  it('setLastUser stores raw username', () => {
    session.setLastUser('alice')
    expect(localStorage.getItem('devops.last_user')).toBe('alice')
  })

  it('saveTokens rotates tokens only', () => {
    session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
    const before = session.getTenantCode()
    session.saveTokens({ access_token: 'a2', refresh_token: 'r2' })
    expect(session.getAccessToken()).toBe('a2')
    expect(session.getTenantCode()).toBe(before)
  })
})
