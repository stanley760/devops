import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Dict from '@/views/system/Dict'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import type { DictItem, DictType, PageResult } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listDictTypes: vi.fn(),
    createDictType: vi.fn().mockResolvedValue(undefined),
    updateDictType: vi.fn().mockResolvedValue(undefined),
    deleteDictType: vi.fn().mockResolvedValue(undefined),
    listDictItems: vi.fn(),
    createDictItem: vi.fn().mockResolvedValue(undefined),
    updateDictItem: vi.fn().mockResolvedValue(undefined),
    deleteDictItem: vi.fn().mockResolvedValue(undefined),
  },
}))

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listDictTypes).mockResolvedValue({
    list: [{ id: '1', name: '验证码类型', type: 'sys_captcha_type', status: 1 } as DictType],
    total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<DictType>)
  vi.mocked(adminSystem.listDictItems).mockResolvedValue([
    { id: 'i1', dictType: 'sys_captcha_type', label: '滑动', value: 'slide', sort: 1 } as DictItem,
  ])
})

function renderDict() {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={['system:dict:create', 'system:dict:update', 'system:dict:delete']}>
        <ConfirmProvider>
          <Dict />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Dict page (two panels)', () => {
  it('loads type list then items of selected type', async () => {
    renderDict()
    await screen.findByText('验证码类型')
    fireEvent.click(screen.getByText('验证码类型'))
    await screen.findByText('滑动')
    expect(adminSystem.listDictItems).toHaveBeenCalledWith('sys_captcha_type')
  })

  it('creates dict item with snake_case payload', async () => {
    renderDict()
    await screen.findByText('验证码类型')
    fireEvent.click(screen.getByText('验证码类型'))
    await screen.findByText('滑动')
    fireEvent.click(screen.getByText('新增数据项'))
    fireEvent.change(screen.getByLabelText('标签'), { target: { value: '旋转' } })
    fireEvent.change(screen.getByLabelText('值'), { target: { value: 'rotate' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createDictItem).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createDictItem).mock.calls[0]![0]
    expect(body).toMatchObject({ dict_type: 'sys_captcha_type', label: '旋转', value: 'rotate', sort: expect.any(Number) })
  })

  it('creates dict type with snake_case payload', async () => {
    renderDict()
    await screen.findByText('验证码类型')
    fireEvent.click(screen.getByText('新增类型'))
    fireEvent.change(screen.getByLabelText('类型名称'), { target: { value: '状态' } })
    fireEvent.change(screen.getByLabelText('类型编码'), { target: { value: 'sys_status' } })
    fireEvent.click(screen.getByText('保存'))
    await waitFor(() => expect(adminSystem.createDictType).toHaveBeenCalled())
    const body = vi.mocked(adminSystem.createDictType).mock.calls[0]![0]
    expect(body).toMatchObject({ name: '状态', type: 'sys_status', status: 1 })
  })

  it('deletes a type and an item with confirm using single string IDs', async () => {
    const { unmount } = renderDict()
    await screen.findByText('验证码类型')
    fireEvent.click(screen.getByText('验证码类型'))
    await screen.findByText('滑动')

    const itemDelete = screen.getAllByText('删除')[1]!
    fireEvent.click(itemDelete)
    const itemDialog = await screen.findByRole('alertdialog')
    fireEvent.click(within(itemDialog).getByText('删除'))
    await waitFor(() => expect(adminSystem.deleteDictItem).toHaveBeenCalledWith('i1'))

    unmount()
    renderDict()
    await screen.findByText('验证码类型')
    fireEvent.click(screen.getAllByText('删除')[0]!)
    const typeDialog = await screen.findByRole('alertdialog')
    fireEvent.click(within(typeDialog).getByText('删除'))
    await waitFor(() => expect(adminSystem.deleteDictType).toHaveBeenCalledWith('1'))
  })
})
