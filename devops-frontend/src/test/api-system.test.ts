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
  it('createUser targets /users and sends the backend camelCase body', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.createUser({
      username: 'u', nickname: 'n', deptId: '1', postIds: ['2'], roleIds: ['3'],
      password: 'pass123', gender: 2, status: 1,
    })
    expect(spy.mock.calls[0]![0]).toMatch(/\/api\/v1\/users$/)
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body).toMatchObject({
      username: 'u', nickname: 'n', deptId: '1', postIds: ['2'], roleIds: ['3'],
      password: 'pass123', gender: 2, status: 1,
    })
    expect(body.dept_id).toBeUndefined()
    expect(body.gender).toBe(2)
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

  it('updateUserPassword targets the dedicated password endpoint', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.updateUserPassword('9', { password: 'p' })
    expect(spy.mock.calls[0]![0]).toMatch(/\/api\/v1\/users\/9\/password$/)
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body).toEqual({ password: 'p' })
  })

  it('deleteUser deletes by single string id, no batch body', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.deleteUser('1234567890123456789')
    expect(spy.mock.calls[0]![0]).toMatch(/\/api\/v1\/users\/1234567890123456789$/)
    expect(spy.mock.calls[0]![1]!.method).toBe('DELETE')
    expect((spy.mock.calls[0]![1] as RequestInit).body).toBeUndefined()
  })

  it('deletePost deletes by single string id', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.deletePost('1234567890123456789')
    expect(spy.mock.calls[0]![0]).toMatch(/\/api\/v1\/posts\/1234567890123456789$/)
    expect(spy.mock.calls[0]![1]!.method).toBe('DELETE')
  })

  it('listMenuTree targets the menus endpoint that returns a nested tree', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.listMenuTree()
    expect(spy.mock.calls[0]![0]).toMatch(/\/api\/v1\/menus$/)
  })

  it('getRole targets the role detail endpoint', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.getRole('r1')
    expect(spy.mock.calls[0]![0]).toMatch(/\/api\/v1\/roles\/r1$/)
  })

  it('assignRoleMenus sends the backend menuIds payload', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.assignRoleMenus('r1', ['m1', 'm2'])
    expect(spy.mock.calls[0]![0]).toMatch(/\/api\/v1\/roles\/r1\/menus$/)
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body).toEqual({ menuIds: ['m1', 'm2'] })
    expect(body.menu_ids).toBeUndefined()
  })

  it('unused delete helpers target the backend single-id routes', async () => {
    const spy = vi.spyOn(global, 'fetch').mockImplementation(async () => ok())
    await adminSystem.deleteRole('1234567890123456789')
    await adminSystem.deleteTenant('2')
    await adminSystem.deleteDictType('3')
    await adminSystem.deleteDictItem('4')
    await adminSystem.deleteConfig('5')
    await adminSystem.removeLoginLogs(['6'])
    await adminSystem.removeOperationLogs(['7'])
    const urls = spy.mock.calls.map(([url]) => String(url))
    expect(urls.slice(0, 5)).toEqual([
      '/system/api/v1/roles/1234567890123456789',
      '/system/api/v1/tenants/2',
      '/system/api/v1/dict-types/3',
      '/system/api/v1/dict-data/4',
      '/system/api/v1/configs/5',
    ])
    expect(urls[5]).toMatch(/\/api\/v1\/login-logs\?(.*&)?ids=6(&|$)/)
    expect(urls[5]).not.toContain('/batch')
    expect(urls[6]).toMatch(/\/api\/v1\/operation-logs\?(.*&)?ids=7(&|$)/)
    expect(urls[6]).not.toContain('/batch')
    for (const [, init] of spy.mock.calls) {
      expect(init!.method).toBe('DELETE')
      expect((init as RequestInit).body).toBeUndefined()
    }
  })

  it('clean helpers target the dedicated /clean routes, batch delete stays on the collection', async () => {
    const spy = vi.spyOn(global, 'fetch').mockImplementation(async () => ok())
    await adminSystem.cleanLoginLogs()
    await adminSystem.cleanOperationLogs()
    await adminSystem.removeLoginLogs(['1'])
    await adminSystem.removeOperationLogs(['2'])
    const urls = spy.mock.calls.map(([url]) => String(url))
    expect(urls.slice(0, 2)).toEqual([
      '/system/api/v1/login-logs/clean',
      '/system/api/v1/operation-logs/clean',
    ])
    expect(urls[2]).toMatch(/\/api\/v1\/login-logs\?/)
    expect(urls[3]).toMatch(/\/api\/v1\/operation-logs\?/)
    for (const [, init] of spy.mock.calls) {
      expect(init!.method).toBe('DELETE')
    }
  })

  it('listAllPosts and listAllRoles use paginated list endpoints', async () => {
    const pageResponse = () =>
      new Response(JSON.stringify({ code: 200, data: { list: [], total: 0, page_num: 1, page_size: 10, pages: 0 } }), { status: 200 })
    const spy = vi.spyOn(global, 'fetch')
      .mockResolvedValueOnce(pageResponse())
      .mockResolvedValueOnce(pageResponse())
    await adminSystem.listAllPosts()
    await adminSystem.listAllRoles()
    expect(spy.mock.calls[0]![0]).toMatch(/\/api\/v1\/posts\?/)
    expect(spy.mock.calls[0]![0]).toContain('page_size=1000')
    expect(spy.mock.calls[1]![0]).toMatch(/\/api\/v1\/roles\?/)
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
  it('write payloads keep snowflake IDs as strings', async () => {
    const spy = vi.spyOn(global, 'fetch').mockResolvedValue(ok())
    await adminSystem.createUser({
      username: 'u', nickname: 'n', deptId: '1234567890123456789',
      postIds: ['9876543210987654321'], roleIds: ['5555555555555555555'],
      password: 'pass123',
    })
    const body = JSON.parse((spy.mock.calls[0]![1] as RequestInit).body as string)
    expect(body.deptId).toBe('1234567890123456789')
    expect(body.postIds).toEqual(['9876543210987654321'])
    expect(body.roleIds).toEqual(['5555555555555555555'])
  })
})
