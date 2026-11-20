import { describe, it, expect, vi } from 'vitest'
import { COMPONENT_MAP, buildDynamicRoutes, flattenMenus } from '@/router/routes'
import type { RouterNode } from '@/lib/types'

const node = (over: Partial<RouterNode>): RouterNode => ({
  id: '1', name: 'n', path: '/x', component: 'system/user/index', sort: 1, visible: true, ...over,
})

describe('COMPONENT_MAP', () => {
  it('covers all 11 spec keys', () => {
    expect(Object.keys(COMPONENT_MAP).sort()).toEqual([
      'dashboard/index',
      'system/user/index',
      'system/role/index',
      'system/menu/index',
      'system/dept/index',
      'system/post/index',
      'system/dict/index',
      'system/config/index',
      'system/loginlog/index',
      'system/operationlog/index',
      'platform/tenant/index',
    ].sort())
  })
})

describe('buildDynamicRoutes', () => {
  it('maps registered components to route defs', () => {
    const defs = buildDynamicRoutes([node({ path: '/system/user', component: 'system/user/index' })])
    expect(defs).toHaveLength(1)
    expect(defs[0]!.path).toBe('/system/user')
  })

  it('skips unregistered component keys with console.warn', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const defs = buildDynamicRoutes([node({ path: '/mystery', component: 'mystery/index' })])
    expect(defs).toHaveLength(0)
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('mystery/index'))
    warn.mockRestore()
  })

  it('walks children', () => {
    const defs = buildDynamicRoutes([
      node({
        path: '/system',
        component: '', // 目录节点无页面
        children: [node({ path: '/system/user', component: 'system/user/index' })],
      }),
    ])
    expect(defs.map((d) => d.path)).toEqual(['/system/user'])
  })

  it('skips registered components with empty paths and walks their children', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const defs = buildDynamicRoutes([
      node({
        path: '',
        component: 'system/user/index',
        children: [node({ path: '/system/role', component: 'system/role/index' })],
      }),
    ])
    expect(defs.map((d) => d.path)).toEqual(['/system/role'])
    expect(warn).toHaveBeenCalledWith(expect.stringContaining('missing path'))
    warn.mockRestore()
  })
})

describe('flattenMenus', () => {
  it('filters invisible nodes and flattens tree', () => {
    const menus = flattenMenus([
      node({
        name: 'parent',
        path: '/p',
        children: [
          node({ id: '2', name: 'child', path: '/p/c' }),
          node({ id: '3', name: 'hidden', path: '/p/h', visible: false }),
          node({ id: '4', name: 'visible-child', path: '/p/v' }),
        ],
      }),
    ])
    expect(menus.map((m) => m.name)).toEqual(['parent', 'child', 'visible-child'])
  })

  it('excludes the whole subtree of a hidden parent', () => {
    const menus = flattenMenus([
      node({
        name: 'hidden-parent',
        path: '/hidden',
        visible: false,
        children: [
          node({ id: '2', name: 'child', path: '/hidden/child' }),
          node({
            id: '3',
            name: 'grandchild-parent',
            path: '/hidden/sub',
            children: [node({ id: '4', name: 'grandchild', path: '/hidden/sub/deep' })],
          }),
        ],
      }),
      node({ id: '5', name: 'shown', path: '/shown' }),
    ])
    expect(menus.map((m) => m.name)).toEqual(['shown'])
  })
})
