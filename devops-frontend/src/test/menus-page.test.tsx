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

  it('uses the backend default always_show=true for new menus', async () => {
    renderMenus()
    await screen.findByText('系统')
    fireEvent.click(screen.getByText('新增菜单'))
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: 'new' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createMenu).toHaveBeenCalled())
    expect(vi.mocked(adminSystem.createMenu).mock.calls[0]![0].always_show).toBe(true)
  })

  it('preserves an explicit always_show=false when editing', async () => {
    vi.mocked(adminSystem.listMenus).mockResolvedValueOnce([
      menu({ alwaysShow: false }),
    ])
    renderMenus()
    await screen.findByText('系统')
    fireEvent.click(screen.getAllByText('编辑')[0]!)
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.updateMenu).toHaveBeenCalled())
    expect(vi.mocked(adminSystem.updateMenu).mock.calls[0]![1].always_show).toBe(false)
  })

  it('sends both boolean flags when editing so toggling them off reaches the backend', async () => {
    vi.mocked(adminSystem.listMenus).mockResolvedValueOnce([
      menu({ keepAlive: true, alwaysShow: true }),
    ])
    renderMenus()
    await screen.findByText('系统')
    fireEvent.click(screen.getAllByText('编辑')[0]!)
    fireEvent.click(screen.getAllByLabelText('缓存')[1]!)
    fireEvent.click(screen.getAllByLabelText('常显')[1]!)
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.updateMenu).toHaveBeenCalled())
    expect(vi.mocked(adminSystem.updateMenu).mock.calls[0]![1]).toMatchObject({
      keep_alive: false,
      always_show: false,
    })
  })

  it('warns that menus with children cannot be deleted', async () => {
    renderMenus()
    await screen.findByText('系统')
    fireEvent.click(screen.getAllByText('删除')[0]!)
    expect(await screen.findByText(/子级.*无法删除/)).toBeInTheDocument()
  })

  it('deletes with confirm by id', async () => {
    renderMenus()
    await screen.findByText('系统')
    fireEvent.click(screen.getAllByText('删除')[0]!)
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deleteMenu).toHaveBeenCalledWith('1'))
  })
})
