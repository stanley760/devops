import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SessionProvider } from '@/store/session'
import { PermissionsProvider } from '@/lib/permission'
import Dashboard from '@/views/Dashboard'
import { adminAuth } from '@/api/admin/auth'
import { adminSystem } from '@/api/admin/system'
import { session } from '@/lib/session'
import type { AuthInfo, RouterNode } from '@/lib/types'

vi.mock('@/api/admin/auth', () => ({
  adminAuth: {
    getInfo: vi.fn(),
    getRouters: vi.fn(),
    logout: vi.fn(),
    updatePassword: vi.fn(),
  },
}))

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listLoginLogs: vi.fn(),
    listOperationLogs: vi.fn(),
  },
}))

const authInfo: AuthInfo = {
  user: { id: '1', username: 'admin', nickname: '管理员' },
  tenant: { id: '1', name: 'ACME', code: 'acme' },
  roles: ['admin'],
  permissions: ['system:loginlog:list', 'system:operationlog:list'],
}

const routers: RouterNode[] = [
  { id: '1', name: '工作台', path: '/dashboard', component: 'dashboard/index', sort: 0, visible: true },
  {
    id: '2', name: '系统', path: '/system', component: '', sort: 1, visible: true,
    children: [{ id: '3', name: '用户', path: '/system/user', component: 'system/user/index', sort: 1, visible: true }],
  },
]

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
  vi.mocked(adminAuth.getInfo).mockResolvedValue(authInfo)
  vi.mocked(adminAuth.getRouters).mockResolvedValue(routers)
  vi.mocked(adminSystem.listLoginLogs).mockResolvedValue({
    list: [{
      id: 'l1', username: 'admin', userId: '9', ip: '1.2.3.4', userAgent: 'test-agent',
      geoCountry: '', geoProvince: '', geoCity: '', geoIsp: '',
      result: 0, code: 200, createTime: '2026-01-02 03:04:05',
    }],
    total: 1, page_num: 1, page_size: 10, pages: 1,
  })
  vi.mocked(adminSystem.listOperationLogs).mockResolvedValue({
    list: [{
      id: 'o1', traceId: 't1', userId: '9', userName: 'admin', userType: 2,
      userTypeName: '管理员', module: 'system', subModule: 'user', bizId: '0',
      contents: '创建用户', extra: '{"duration_ms":12}', requestMethod: 'POST',
      requestUrl: '/system/api/v1/users', userIp: '5.6.7.8', userAgent: 'test-agent',
      geoCountry: '', geoProvince: '', geoCity: '', geoIsp: '',
      result: 0, code: 200, createTime: '2026-01-02 03:05:06',
    }],
    total: 1, page_num: 1, page_size: 10, pages: 1,
  })
})

describe('Dashboard', () => {
  it('shows real account info, menu count and recent login logs', async () => {
    render(
      <MemoryRouter>
        <SessionProvider>
          <PermissionsProvider permissions={authInfo.permissions}>
            <Dashboard />
          </PermissionsProvider>
        </SessionProvider>
      </MemoryRouter>,
    )
    await screen.findByText('管理员')
    await waitFor(() =>
      expect(adminSystem.listLoginLogs).toHaveBeenCalledWith({ page_num: 1, page_size: 10 }),
    )
    expect(await screen.findByText('1.2.3.4')).toBeInTheDocument()
    expect(screen.getByText('菜单')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(adminSystem.listOperationLogs).toHaveBeenCalledWith({ page_num: 1, page_size: 10 })
    expect(await screen.findByText('创建用户')).toBeInTheDocument()
  })

  it('hides log panels and skips log APIs without permissions', async () => {
    render(
      <MemoryRouter>
        <SessionProvider>
          <PermissionsProvider permissions={[]}>
            <Dashboard />
          </PermissionsProvider>
        </SessionProvider>
      </MemoryRouter>,
    )
    await screen.findByText('管理员')
    expect(adminSystem.listLoginLogs).not.toHaveBeenCalled()
    expect(adminSystem.listOperationLogs).not.toHaveBeenCalled()
    expect(screen.queryByText('最近登录日志')).not.toBeInTheDocument()
    expect(screen.queryByText('最近操作日志')).not.toBeInTheDocument()
  })
})
