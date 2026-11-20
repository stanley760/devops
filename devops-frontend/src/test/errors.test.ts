import { describe, it, expect } from 'vitest'
import { mapLoginError } from '@/lib/errors'

describe('mapLoginError', () => {
  it('maps the five known backend errors', () => {
    expect(mapLoginError('incorrect username or password')).toBe('账号或密码错误')
    expect(mapLoginError('user does not exist')).toBe('账号不存在或已被删除')
    expect(mapLoginError('tenant not found')).toBe('租户不存在，请重新选择')
    expect(mapLoginError('tenant is disabled')).toBe('租户已被禁用，请联系管理员')
    expect(mapLoginError('too many requests')).toBe('操作过于频繁，请稍后再试')
    expect(mapLoginError('rate limit exceeded')).toBe('操作过于频繁，请稍后再试')
  })
  it('falls back to raw message', () => {
    expect(mapLoginError('something else')).toBe('something else')
  })
})
