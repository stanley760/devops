import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Tenants from '@/views/system/Tenants'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { PageResult, SystemTenant } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listTenants: vi.fn(),
    createTenant: vi.fn().mockResolvedValue(undefined),
    updateTenant: vi.fn().mockResolvedValue(undefined),
    deleteTenant: vi.fn().mockResolvedValue(undefined),
  },
}))

const tenant = (over: Partial<SystemTenant>): SystemTenant => ({
  id: '1', name: 'ACME', code: 'acme', contactName: '张三', contactMobile: '13800000000', ...over,
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listTenants).mockResolvedValue({
    list: [tenant({})], total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<SystemTenant>)
})

function renderTenants() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['platform:tenant:create', 'platform:tenant:update', 'platform:tenant:delete']}>
        <ConfirmProvider>
          <Tenants />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Tenants page', () => {
  it('renders tenants', async () => {
    renderTenants()
    await screen.findByText('ACME')
    expect(screen.getByText('acme')).toBeInTheDocument()
  })

  it('creates tenant with snake_case payload', async () => {
    renderTenants()
    await screen.findByText('ACME')
    fireEvent.click(screen.getByText('新增租户'))
    fireEvent.change(screen.getByLabelText('租户名称'), { target: { value: 'Beta' } })
    fireEvent.change(screen.getByLabelText('租户编码'), { target: { value: 'beta' } })
    fireEvent.change(screen.getByLabelText('联系人'), { target: { value: '李四' } })
    fireEvent.change(screen.getByLabelText('联系电话'), { target: { value: '139' } })
    fireEvent.change(screen.getByLabelText('管理员账号'), { target: { value: 'admin' } })
    fireEvent.change(screen.getByLabelText('管理员密码'), { target: { value: 'pass123' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createTenant).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createTenant).mock.calls[0]![0]
    expect(body).toMatchObject({ name: 'Beta', code: 'beta', contact_name: '李四', contact_mobile: '139' })
  })

  it('deletes with confirm using the real single-id route', async () => {
    renderTenants()
    await screen.findByText('ACME')
    fireEvent.click(screen.getByText('删除'))
    const dialog = await screen.findByRole('alertdialog')
    fireEvent.click(within(dialog).getByText('删除'))
    await waitFor(() => expect(adminSystem.deleteTenant).toHaveBeenCalledWith('1'))
  })
})
