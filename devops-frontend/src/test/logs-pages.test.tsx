import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import LoginLogs from '@/views/monitor/LoginLogs'
import OperationLogs from '@/views/monitor/OperationLogs'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { LoginLog, OperationLog, PageResult } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listLoginLogs: vi.fn(),
    cleanLoginLogs: vi.fn().mockResolvedValue(undefined),
    removeLoginLogs: vi.fn().mockResolvedValue(undefined),
    listOperationLogs: vi.fn(),
    cleanOperationLogs: vi.fn().mockResolvedValue(undefined),
    removeOperationLogs: vi.fn().mockResolvedValue(undefined),
  },
}))

const loginLog: LoginLog = {
  id: 'l1', username: 'admin', userId: '9', ip: '1.2.3.4', userAgent: 'test-agent',
  geoCountry: '', geoProvince: '', geoCity: '', geoIsp: '',
  result: 1, code: 401, createTime: '2026-01-02 03:04:05',
}
const opLog: OperationLog = {
  id: 'o1', traceId: 't1', userId: '9', userName: 'admin', userType: 2,
  userTypeName: '管理员', module: 'system', subModule: 'user', bizId: '0',
  contents: '创建用户', extra: '{"duration_ms":12}', requestMethod: 'POST',
  requestUrl: '/system/api/v1/users', userIp: '1.2.3.4', userAgent: 'test-agent',
  geoCountry: '', geoProvince: '', geoCity: '', geoIsp: '',
  result: 0, code: 200, createTime: '2026-01-02 03:04:05',
}

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listLoginLogs).mockResolvedValue({
    list: [loginLog], total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<LoginLog>)
  vi.mocked(adminSystem.listOperationLogs).mockResolvedValue({
    list: [opLog], total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<OperationLog>)
})

function wrap(ui: React.ReactElement) {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:loginlog:list', 'system:loginlog:clean', 'system:loginlog:remove', 'system:operationlog:list', 'system:operationlog:clean', 'system:operationlog:remove']}>
        <ConfirmProvider>{ui}</ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('LoginLogs page', () => {
  it('renders login logs', async () => {
    wrap(<LoginLogs />)
    await screen.findByText('admin')
    expect(adminSystem.listLoginLogs).toHaveBeenCalledWith(expect.objectContaining({ page_num: 1 }))
  })

  it('clean requires confirm and calls cleanLoginLogs', async () => {
    wrap(<LoginLogs />)
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('清空'))
    const dialog = await screen.findByRole('alertdialog')
    fireEvent.click(within(dialog).getByText('清空'))
    await waitFor(() => expect(adminSystem.cleanLoginLogs).toHaveBeenCalled())
  })

  it('batch remove sends selected ids', async () => {
    wrap(<LoginLogs />)
    await screen.findByText('admin')
    fireEvent.click(screen.getByLabelText('全选'))
    fireEvent.click(screen.getByText('批量删除'))
    fireEvent.click(await screen.findByText('删除'))
    await waitFor(() => expect(adminSystem.removeLoginLogs).toHaveBeenCalledWith(['l1']))
  })
})

describe('OperationLogs page', () => {
  it('renders op logs and opens detail drawer with backend DTO fields', async () => {
    wrap(<OperationLogs />)
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('详情'))
    expect(await screen.findByText(/"duration_ms":12/)).toBeInTheDocument()
    expect(screen.getByText('创建用户')).toBeInTheDocument()
    expect(screen.getByText('12 ms')).toBeInTheDocument()
    expect(screen.getByText('操作时间：')).toBeInTheDocument()
    expect(screen.getByText(/管理员/)).toBeInTheDocument()
  })

  it('clean requires confirm and calls cleanOperationLogs', async () => {
    wrap(<OperationLogs />)
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('清空'))
    const dialog = await screen.findByRole('alertdialog')
    fireEvent.click(within(dialog).getByText('清空'))
    await waitFor(() => expect(adminSystem.cleanOperationLogs).toHaveBeenCalled())
  })

  it('batch remove sends selected ids', async () => {
    wrap(<OperationLogs />)
    await screen.findByText('admin')
    fireEvent.click(screen.getByLabelText('全选'))
    fireEvent.click(screen.getByText('批量删除'))
    fireEvent.click(await screen.findByText('删除'))
    await waitFor(() => expect(adminSystem.removeOperationLogs).toHaveBeenCalledWith(['o1']))
  })
})
