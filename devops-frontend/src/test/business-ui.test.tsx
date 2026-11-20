import { render, screen, fireEvent, renderHook, act } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'
import { PermissionButton } from '@/components/business/PermissionButton'
import { SearchForm } from '@/components/business/SearchForm'
import { ListPage } from '@/components/business/ListPage'
import { useListPage } from '@/lib/list'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'

function granted(ui: React.ReactElement) {
  return render(
    <PermissionsProvider permissions={['system:user:delete']}>
      <ConfirmProvider>{ui}</ConfirmProvider>
    </PermissionsProvider>,
  )
}

describe('PermissionButton', () => {
  it('renders when granted', () => {
    granted(<PermissionButton permission="system:user:delete" danger>删除</PermissionButton>)
    expect(screen.getByText('删除')).toBeInTheDocument()
  })
  it('does NOT render (hidden) when denied', () => {
    render(
      <PermissionsProvider permissions={[]}>
        <PermissionButton permission="system:user:delete">删除</PermissionButton>
      </PermissionsProvider>,
    )
    expect(screen.queryByText('删除')).not.toBeInTheDocument()
  })
})

describe('SearchForm', () => {
  it('collects input values and triggers search', () => {
    const onSearch = vi.fn()
    const value: Record<string, unknown> = {}
    render(
      <SearchForm
        fields={[{ key: 'username', label: '账号' }]}
        value={value} onSearch={onSearch} onReset={() => {}}
      />,
    )
    fireEvent.change(screen.getByLabelText('账号'), { target: { value: 'admin' } })
    fireEvent.click(screen.getByText('查询'))
    expect(onSearch).toHaveBeenCalled()
    expect(value.username).toBe('admin')
  })
})

describe('ListPage', () => {
  it('renders rows via custom render and default text', async () => {
    const fetcher = vi.fn().mockResolvedValue({ list: [{ id: '1', name: '张三' }], total: 1, page_num: 1, page_size: 10, pages: 1 })
    const { result } = renderHook(() => useListPage<{ id: string; name: string }>({ fetcher }))
    await act(() => result.current.load())
    render(
      <ListPage
        state={result.current}
        columns={[
          { key: 'name', title: '姓名' },
          { key: 'op', title: '操作', render: (r) => <button>编辑 {r.id}</button> },
        ]}
      />,
    )
    expect(screen.getByText('张三')).toBeInTheDocument()
    expect(screen.getByText('编辑 1')).toBeInTheDocument()
    expect(screen.getByText('共 1 条')).toBeInTheDocument()
  })

  it('selection supports select-all and batch bar', async () => {
    const fetcher = vi.fn().mockResolvedValue({ list: [{ id: '1' }, { id: '2' }], total: 2, page_num: 1, page_size: 10, pages: 1 })
    const { result } = renderHook(() => useListPage<{ id: string }>({ fetcher }))
    await act(() => result.current.load())
    render(
      <ListPage
        state={result.current}
        selectable
        columns={[{ key: 'id', title: 'ID' }]}
        batchActions={<button>批量删除</button>}
      />,
    )
    fireEvent.click(screen.getByLabelText('全选'))
    expect(screen.getByText('已选 2 项')).toBeInTheDocument()
    expect(screen.getByText('批量删除')).toBeInTheDocument()
  })

  it('pagination buttons page through', async () => {
    const fetcher = vi.fn().mockResolvedValue({ list: [{ id: '1' }], total: 25, page_num: 1, page_size: 10, pages: 3 })
    const { result } = renderHook(() => useListPage<{ id: string }>({ fetcher }))
    await act(() => result.current.load())
    render(<ListPage state={result.current} columns={[{ key: 'id', title: 'ID' }]} />)
    fireEvent.click(screen.getByText('下一页'))
    await act(() => result.current.load())
    expect(fetcher).toHaveBeenLastCalledWith(expect.objectContaining({ page_num: 2 }))
  })
})
