import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Roles from '@/views/system/Roles'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { PageResult, SystemMenu, SystemRole } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listRoles: vi.fn(),
    createRole: vi.fn().mockResolvedValue(undefined),
    updateRole: vi.fn().mockResolvedValue(undefined),
    assignRoleMenus: vi.fn().mockResolvedValue(undefined),
    deleteRole: vi.fn().mockResolvedValue(undefined),
    listMenus: vi.fn(),
    listMenuTree: vi.fn(),
    getRole: vi.fn(),
    listDepts: vi.fn().mockResolvedValue([]),
  },
  parseDeptIDsJson: (s?: string) => (s ? JSON.parse(s) : []),
  encodeDeptIDsJson: (ids: string[]) => JSON.stringify(ids),
}))

function page(list: SystemRole[]): PageResult<SystemRole> {
  return { list, total: list.length, page_num: 1, page_size: 10, pages: 1 }
}

// Backend exposes role type 1 = builtin, 2 = custom; IDs are strings.
const role = (over: Partial<SystemRole>): SystemRole => ({
  id: '1', name: '管理员', code: 'admin', sort: 1, dataScope: 1, dataScopeDeptIds: '[]', status: 1, type: 2, ...over,
})

const menuTree: SystemMenu[] = [
  {
    id: '1', name: '系统', path: '/system', component: '', sort: 1, visible: true, type: 1, parentId: '0', status: 1,
    children: [
      { id: '2', name: '用户', path: '/system/user', component: 'system/user/index', sort: 1, visible: true, type: 2, parentId: '1', status: 1 },
    ],
  },
]

const allPermissions = [
  'system:role:list',
  'system:role:create',
  'system:role:update',
  'system:role:assign',
  'system:role:delete',
]

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listRoles).mockResolvedValue(page([role({})]))
  vi.mocked(adminSystem.listMenuTree).mockResolvedValue(menuTree)
  vi.mocked(adminSystem.getRole).mockResolvedValue({ ...role({}), menuIds: [] })
  vi.mocked(adminSystem.listDepts).mockResolvedValue([])
  vi.mocked(adminSystem.createRole).mockResolvedValue(undefined)
  vi.mocked(adminSystem.updateRole).mockResolvedValue(undefined)
  vi.mocked(adminSystem.assignRoleMenus).mockResolvedValue(undefined)
  vi.mocked(adminSystem.deleteRole).mockResolvedValue(undefined)
})

function renderRoles(permissions = allPermissions) {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={permissions}>
        <ConfirmProvider>
          <Roles />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Roles page', () => {
  it('creates role with data_scope_dept_ids encoded as a JSON array string', async () => {
    renderRoles()
    await screen.findByText('管理员')
    fireEvent.click(screen.getByText('新增角色'))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText('角色名称'), { target: { value: '运维' } })
    fireEvent.change(within(dialog).getByLabelText('角色编码'), { target: { value: 'ops' } })
    fireEvent.click(within(dialog).getByText('保存'))
    await waitFor(() => expect(adminSystem.createRole).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createRole).mock.calls[0]![0]
    expect(body).toMatchObject({
      name: '运维', code: 'ops', sort: expect.any(Number),
      data_scope: expect.any(Number), data_scope_dept_ids: '[]', status: 1,
    })
  })

  it('edit round-trips data_scope_dept_ids through the JSON codec', async () => {
    vi.mocked(adminSystem.listRoles).mockResolvedValue(
      page([role({ id: '9', dataScope: 2, dataScopeDeptIds: '["10","11"]' })]),
    )
    vi.mocked(adminSystem.listDepts).mockResolvedValue([
      { id: '10', name: '总部', parentId: '0', sort: 1, status: 1 },
      { id: '11', name: '研发部', parentId: '0', sort: 2, status: 1 },
    ])
    renderRoles()
    await screen.findByText('管理员')
    fireEvent.click(screen.getByText('编辑'))
    const dialog = await screen.findByRole('dialog')
    await waitFor(() => expect(within(dialog).getByLabelText('总部')).toBeChecked())
    expect(within(dialog).getByLabelText('研发部')).toBeChecked()
    fireEvent.click(within(dialog).getByText('保存'))
    await waitFor(() =>
      expect(adminSystem.updateRole).toHaveBeenCalledWith('9', expect.objectContaining({
        data_scope: 2,
        data_scope_dept_ids: '["10","11"]',
      })),
    )
  })

  it('loads the nested menu tree and preselects existing assignments', async () => {
    vi.mocked(adminSystem.getRole).mockResolvedValue({ ...role({}), menuIds: ['2'] })
    renderRoles()
    await screen.findByText('管理员')
    fireEvent.click(screen.getByText('分配菜单'))
    await screen.findByText('系统')
    expect(adminSystem.listMenuTree).toHaveBeenCalled()
    expect(adminSystem.getRole).toHaveBeenCalledWith('1')
    expect(screen.getByLabelText('选菜单 用户')).toBeChecked()
    fireEvent.click(screen.getByText('确定分配'))
    await waitFor(() => expect(adminSystem.assignRoleMenus).toHaveBeenCalledWith('1', ['2']))
  })

  it('closes the assignment dialog when the menu tree cannot be loaded', async () => {
    vi.mocked(adminSystem.listMenuTree).mockRejectedValue(new Error('tree down'))
    renderRoles()
    await screen.findByText('管理员')
    fireEvent.click(screen.getByText('分配菜单'))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(adminSystem.assignRoleMenus).not.toHaveBeenCalled()
  })

  it('hides delete for builtin roles (type === 1)', async () => {
    vi.mocked(adminSystem.listRoles).mockResolvedValue(page([role({ type: 1 })]))
    renderRoles()
    await screen.findByText('管理员')
    expect(screen.queryByText('删除')).not.toBeInTheDocument()
  })

  it('deletes a custom role with confirm using its string id', async () => {
    renderRoles()
    await screen.findByText('管理员')
    fireEvent.click(screen.getByText('删除'))
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deleteRole).toHaveBeenCalledWith('1'))
  })
})
