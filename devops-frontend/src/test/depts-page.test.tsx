import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Depts from '@/views/system/Depts'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { SystemDept } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listDepts: vi.fn(),
    createDept: vi.fn().mockResolvedValue(undefined),
    updateDept: vi.fn().mockResolvedValue(undefined),
    deleteDept: vi.fn().mockResolvedValue(undefined),
  },
}))

const dept = (over: Partial<SystemDept>): SystemDept => ({
  id: '1', name: '总部', parentId: '0', sort: 1, status: 1, ...over,
})

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listDepts).mockResolvedValue([
    dept({ id: '1', name: '总部' }),
    dept({ id: '2', name: '研发部', parentId: '1' }),
  ])
})

function renderDepts() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:dept:create', 'system:dept:update', 'system:dept:delete']}>
        <ConfirmProvider>
          <Depts />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Depts page', () => {
  it('renders depts with parent cascade depth', async () => {
    renderDepts()
    await screen.findByText('总部')
    expect(screen.getByText('研发部')).toBeInTheDocument()
  })

  it('create child dept cascades parent_id (snake_case)', async () => {
    renderDepts()
    await screen.findByText('总部')
    fireEvent.click(screen.getAllByText('新增子项')[0]!)
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '测试部' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createDept).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createDept).mock.calls[0]![0]
    expect(body).toMatchObject({ name: '测试部', parent_id: '1', sort: expect.any(Number), status: 1 })
  })

  it('deletes with confirm by id', async () => {
    renderDepts()
    await screen.findByText('总部')
    fireEvent.click(screen.getAllByText('删除')[0]!)
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deleteDept).toHaveBeenCalledWith('1'))
  })
})
