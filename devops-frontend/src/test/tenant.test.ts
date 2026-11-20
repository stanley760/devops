import { describe, it, expect, beforeEach } from 'vitest'
import { resolveTenantEntry } from '@/lib/tenant'

beforeEach(() => localStorage.clear())

describe('resolveTenantEntry', () => {
  it('query param wins and locks', () => {
    expect(resolveTenantEntry('?tenantCode=acme', '/login', 'admin.example.com')).toEqual({ code: 'acme', locked: true })
  })
  it('path /t/{code}/login wins over subdomain and locks', () => {
    expect(resolveTenantEntry('', '/t/foo/login', 'bar.admin.example.com')).toEqual({ code: 'foo', locked: true })
  })
  it('subdomain first segment, not reserved', () => {
    expect(resolveTenantEntry('', '/login', 'acme.admin.example.com')).toEqual({ code: 'acme', locked: true })
  })
  it('reserved subdomain not taken', () => {
    expect(resolveTenantEntry('', '/login', 'www.admin.example.com').code).toBe('default')
  })
  it('localhost and IP not taken', () => {
    expect(resolveTenantEntry('', '/login', 'localhost').code).toBe('default')
    expect(resolveTenantEntry('', '/login', '127.0.0.1').code).toBe('default')
  })
  it('falls back to last_tenant unlocked', () => {
    localStorage.setItem('devops.last_tenant', 'prev')
    expect(resolveTenantEntry('', '/login', 'localhost')).toEqual({ code: 'prev', locked: false })
  })
  it('falls back to default', () => {
    expect(resolveTenantEntry('', '/login', 'localhost')).toEqual({ code: 'default', locked: false })
  })
})
