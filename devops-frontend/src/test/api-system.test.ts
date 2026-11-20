import { describe, it, expect, vi, beforeEach } from 'vitest'
import { adminSystem, encodeDeptIDsJson, parseDeptIDsJson } from '@/api/admin/system'

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

function ok() {
  return new Response(JSON.stringify({ code: 200, data: null }), { status: 200 })
}

describe('admin system API write mapping', () => {
  it('createUser sends snake_case body', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.createUser({
      username: 'u', nickname: 'n', dept_id: '1', post_ids: ['2'], role_ids: ['3'], status: 1,
    })
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body).toMatchObject({ username: 'u', dept_id: '1', post_ids: ['2'], role_ids: ['3'] })
  })

  it('createConfig sends snake_case keys', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.createConfig({
      config_key: 'k', config_name: 'n', config_value: 'v', type: 2, visible: true,
    })
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body).toMatchObject({ config_key: 'k', config_name: 'n', config_value: 'v', type: 2, visible: true })
    expect(body.configKey).toBeUndefined()
  })

  it('updateUserPassword targets /password with snake_case', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.updateUserPassword('9', { password: 'p' })
    expect(spy.mock.calls[0]![0]).toBe('/system/api/v1/system/users/9/password')
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body).toEqual({ password: 'p' })
  })
})

describe('dept IDs JSON codec', () => {
  it('encode/parse round-trip as JSON array string', () => {
    const s = encodeDeptIDsJson(['1', '22'])
    expect(s).toBe('["1","22"]')
    expect(parseDeptIDsJson(s)).toEqual(['1', '22'])
  })
  it('parse handles empty/invalid', () => {
    expect(parseDeptIDsJson('')).toEqual([])
    expect(parseDeptIDsJson(undefined)).toEqual([])
    expect(parseDeptIDsJson('not json')).toEqual([])
  })
})

describe('ID handling stays string', () => {
  it('deleteUsers passes ids verbatim', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.deleteUsers(['1234567890123456789'])
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body.ids).toEqual(['1234567890123456789'])
  })
})
