import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Menus from '@/views/system/Menus'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { SystemMenu } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listMenus: vi.fn(),
    createMenu: vi.fn().mockResolvedValue(undefined),
    updateMenu: vi.fn().mockResolvedValue(undefined),
    deleteMenu: vi.fn().mockResolvedValue(undefined),
  },
}))

const menu = (over: Partial<SystemMenu>): SystemMenu => ({
  id: '1', name: '系统', path: '/system', component: '', sort: 1, visible: true, type: 1, parentId: '0', status: 1, ...over,
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listMenus).mockResolvedValue([
    menu({
      children: [
        menu({ id: '2', name: '用户', path: '/system/user', component: 'system/user/index', type: 2, parentId: '1' }),
      ],
    }),
  ])
})

function renderMenus() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:menu:create', 'system:menu:update', 'system:menu:delete']}>
        <ConfirmProvider>
          <Menus />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Menus page', () => {
  it('renders flattened tree rows', async () => {
    renderMenus()
    await screen.findByText('系统')
    expect(screen.getByText('用户')).toBeInTheDocument()
  })

  it('button type requires permission identifier', async () => {
    renderMenus()
    await screen.findByText('系统')
    fireEvent.click(screen.getByText('新增菜单'))
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: 'btn' } })
    // 类型选按钮（3）但不填权限标识
    fireEvent.change(screen.getByLabelText('类型'), { target: { value: '3' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(screen.getByText('按钮类型必须填写权限标识')).toBeInTheDocument())
    expect(adminSystem.createMenu).not.toHaveBeenCalled()
  })

  it('create child cascades parent_id', async () => {
    renderMenus()
    await screen.findByText('系统')
    const addChildren = screen.getAllByText('新增子项')
    fireEvent.click(addChildren[0]!)
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: 'child' } })
    fireEvent.change(screen.getByLabelText('类型'), { target: { value: '2' } })
    fireEvent.change(screen.getByLabelText('路径'), { target: { value: '/system/role' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createMenu).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createMenu).mock.calls[0]![0]
    expect(body.parent_id).toBe('1')
    expect(body).toMatchObject({ name: 'child', type: 2, visible: true })
  })

  it('deletes with confirm by id', async () => {
    renderMenus()
    await screen.findByText('系统')
    fireEvent.click(screen.getAllByText('删除')[0]!)
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deleteMenu).toHaveBeenCalledWith('1'))
  })
})
