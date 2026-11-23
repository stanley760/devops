import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Users from '@/views/system/Users'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import { ApiError } from '@/lib/http'
import type { PageResult, SystemUser } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listUsers: vi.fn(),
    createUser: vi.fn().mockResolvedValue(undefined),
    updateUser: vi.fn().mockResolvedValue(undefined),
    getUser: vi.fn().mockResolvedValue(undefined),
    updateUserPassword: vi.fn().mockResolvedValue(undefined),
    deleteUser: vi.fn().mockResolvedValue(undefined),
    listAllPosts: vi.fn().mockResolvedValue([]),
    listAllRoles: vi.fn().mockResolvedValue([]),
    listDepts: vi.fn().mockResolvedValue([]),
  },
  parseDeptIDsJson: (s?: string) => (s ? JSON.parse(s) : []),
  encodeDeptIDsJson: (ids: string[]) => JSON.stringify(ids),
}))

function page(list: SystemUser[], total = list.length): PageResult<SystemUser> {
  return { list, total, page_num: 1, page_size: 10, pages: 1 }
}

// Backend DTO uses numeric gender 0/1/2 and string IDs.
const user = (over: Partial<SystemUser>): SystemUser => ({
  id: '1', username: 'admin', nickname: '管理员', deptId: '10', postIds: [], roleIds: [], gender: 1, status: 1, ...over,
})

const allPermissions = [
  'system:user:list',
  'system:user:query',
  'system:user:create',
  'system:user:update',
  'system:user:delete',
  'system:user:update-password',
]

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listUsers).mockResolvedValue(page([user({})]))
  vi.mocked(adminSystem.listDepts).mockResolvedValue([])
  vi.mocked(adminSystem.listAllPosts).mockResolvedValue([])
  vi.mocked(adminSystem.listAllRoles).mockResolvedValue([])
  vi.mocked(adminSystem.createUser).mockResolvedValue(undefined)
  vi.mocked(adminSystem.updateUser).mockResolvedValue(undefined)
  vi.mocked(adminSystem.getUser).mockImplementation(async (id) =>
    user({ id, roleIds: ['r1'], postIds: ['p1'] }),
  )
  vi.mocked(adminSystem.updateUserPassword).mockResolvedValue(undefined)
  vi.mocked(adminSystem.deleteUser).mockResolvedValue(undefined)
})

function renderUsers(permissions = allPermissions) {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={permissions}>
        <ConfirmProvider>
          <Users />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

async function openCreateDialog() {
  fireEvent.click(screen.getByText('新增用户'))
  return await screen.findByRole('dialog')
}

describe('Users page', () => {
  it('loads list and renders rows with exact list permission', async () => {
    renderUsers()
    await screen.findByText('admin')
    expect(adminSystem.listUsers).toHaveBeenCalledWith(expect.objectContaining({ page_num: 1, page_size: 10 }))
  })

  it('skips the list request and renders no-access when the user lacks list permission', async () => {
    renderUsers([])
    expect(await screen.findByText('没有权限访问该页面')).toBeInTheDocument()
    expect(adminSystem.listUsers).not.toHaveBeenCalled()
  })

  it('renders NoAccess when the list request returns code 403', async () => {
    vi.mocked(adminSystem.listUsers).mockRejectedValueOnce(new ApiError(403, 403, 'forbidden'))
    renderUsers()
    expect(await screen.findByText('没有权限访问该页面')).toBeInTheDocument()
  })

  it('requires deptId before create (empty dept rejected client-side)', async () => {
    renderUsers()
    await screen.findByText('admin')
    const dialog = await openCreateDialog()
    fireEvent.change(within(dialog).getByLabelText('用户名'), { target: { value: 'u1' } })
    fireEvent.change(within(dialog).getByLabelText('昵称'), { target: { value: 'n1' } })
    fireEvent.change(within(dialog).getByLabelText('初始密码'), { target: { value: 'pass123' } })
    fireEvent.click(within(dialog).getByText('保存'))
    expect(await within(dialog).findByText('部门必填')).toBeInTheDocument()
    expect(adminSystem.createUser).not.toHaveBeenCalled()
  })

  it('create sends the backend camelCase payload with numeric gender and required password', async () => {
    vi.mocked(adminSystem.listDepts).mockResolvedValue([
      { id: '10', name: '总部', parentId: '0', sort: 1, status: 1 },
    ])
    renderUsers()
    await screen.findByText('admin')
    const dialog = await openCreateDialog()
    fireEvent.change(within(dialog).getByLabelText('用户名'), { target: { value: 'u1' } })
    fireEvent.change(within(dialog).getByLabelText('昵称'), { target: { value: 'n1' } })
    fireEvent.change(within(dialog).getByLabelText('初始密码'), { target: { value: 'pass123' } })
    fireEvent.change(within(dialog).getByLabelText('部门'), { target: { value: '10' } })
    fireEvent.change(within(dialog).getByLabelText('性别'), { target: { value: '2' } })
    fireEvent.click(within(dialog).getByText('保存'))
    await waitFor(() => expect(adminSystem.createUser).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createUser).mock.calls[0]![0]
    expect(body).toMatchObject({
      username: 'u1',
      nickname: 'n1',
      password: 'pass123',
      deptId: '10',
      postIds: [],
      roleIds: [],
      gender: 2,
      status: 1,
    })
    expect(body).not.toHaveProperty('dept_id')
  })

  it('edit loads user detail and preserves roleIds/postIds on update', async () => {
    vi.mocked(adminSystem.listAllRoles).mockResolvedValue([{ id: 'r1', name: '管理员' } as never])
    vi.mocked(adminSystem.listAllPosts).mockResolvedValue([{ id: 'p1', code: 'P1', name: '岗位1' } as never])
    renderUsers()
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('编辑'))
    const dialog = await screen.findByRole('dialog')
    await waitFor(() => expect(adminSystem.getUser).toHaveBeenCalledWith('1'))
    await waitFor(() =>
      expect(within(dialog).getByLabelText('管理员')).toBeChecked(),
    )
    expect(within(dialog).getByLabelText('岗位1')).toBeChecked()
    fireEvent.click(within(dialog).getByText('保存'))
    await waitFor(() => expect(adminSystem.updateUser).toHaveBeenCalledWith('1', expect.objectContaining({
      roleIds: ['r1'],
      postIds: ['p1'],
    })))
  })

  it('reset password calls the dedicated endpoint with a string id', async () => {
    renderUsers()
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('重置密码'))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText('新密码'), { target: { value: 'np123456' } })
    fireEvent.click(within(dialog).getByText('确定重置'))
    await waitFor(() =>
      expect(adminSystem.updateUserPassword).toHaveBeenCalledWith('1', { password: 'np123456' }),
    )
  })

  it('reset password rejects passwords shorter than 6 characters', async () => {
    renderUsers()
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('重置密码'))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText('新密码'), { target: { value: 'np123' } })
    fireEvent.click(within(dialog).getByText('确定重置'))
    expect(await within(dialog).findByText('密码至少 6 位')).toBeInTheDocument()
    expect(adminSystem.updateUserPassword).not.toHaveBeenCalled()
  })

  it('delete confirms then calls deleteUser with the string id', async () => {
    renderUsers()
    await screen.findByText('admin')
    fireEvent.click(screen.getByText('删除'))
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deleteUser).toHaveBeenCalledWith('1'))
  })

  it('hides write actions without their exact permissions', async () => {
    renderUsers(['system:user:list'])
    await screen.findByText('admin')
    expect(screen.queryByText('新增用户')).not.toBeInTheDocument()
    expect(screen.queryByText('编辑')).not.toBeInTheDocument()
    expect(screen.queryByText('重置密码')).not.toBeInTheDocument()
    expect(screen.queryByText('删除')).not.toBeInTheDocument()
  })
})
