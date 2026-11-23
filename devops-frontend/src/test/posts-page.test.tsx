import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import Posts from '@/views/system/Posts'
import { adminSystem } from '@/api/admin/system'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import { ApiError } from '@/lib/http'
import type { PageResult, SystemPost } from '@/lib/types'

vi.mock('@/api/admin/system', () => ({
  adminSystem: {
    listPosts: vi.fn(),
    createPost: vi.fn().mockResolvedValue(undefined),
    updatePost: vi.fn().mockResolvedValue(undefined),
    deletePost: vi.fn().mockResolvedValue(undefined),
  },
}))

const post = (over: Partial<SystemPost>): SystemPost => ({
  id: '1', code: 'ceo', name: 'CEO', sort: 1, status: 1, ...over,
})

const allPermissions = [
  'system:post:list',
  'system:post:query',
  'system:post:create',
  'system:post:update',
  'system:post:delete',
]

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  vi.mocked(adminSystem.listPosts).mockResolvedValue({
    list: [post({})], total: 1, page_num: 1, page_size: 10, pages: 1,
  } as PageResult<SystemPost>)
  vi.mocked(adminSystem.createPost).mockResolvedValue(undefined)
  vi.mocked(adminSystem.updatePost).mockResolvedValue(undefined)
  vi.mocked(adminSystem.deletePost).mockResolvedValue(undefined)
})

function renderPosts(permissions = allPermissions) {
  return render(
    <MemoryRouter>
      <PermissionsProvider permissions={permissions}>
        <ConfirmProvider>
          <Posts />
        </ConfirmProvider>
      </PermissionsProvider>
    </MemoryRouter>,
  )
}

describe('Posts page', () => {
  it('loads and renders posts with the exact list permission', async () => {
    renderPosts()
    await screen.findByText('CEO')
    expect(adminSystem.listPosts).toHaveBeenCalledWith(expect.objectContaining({ page_num: 1, page_size: 10 }))
  })

  it('renders no-access and skips the list request without the list permission', async () => {
    renderPosts([])
    expect(await screen.findByText('没有权限访问该页面')).toBeInTheDocument()
    expect(adminSystem.listPosts).not.toHaveBeenCalled()
  })

  it('renders NoAccess when the list request returns code 403', async () => {
    vi.mocked(adminSystem.listPosts).mockRejectedValueOnce(new ApiError(403, 403, 'forbidden'))
    renderPosts()
    expect(await screen.findByText('没有权限访问该页面')).toBeInTheDocument()
  })

  it('creates a post with the backend camelCase save payload', async () => {
    renderPosts()
    await screen.findByText('CEO')
    fireEvent.click(screen.getByText('新增岗位'))
    const dialog = await screen.findByRole('dialog')
    fireEvent.change(within(dialog).getByLabelText('岗位编码'), { target: { value: 'cto' } })
    fireEvent.change(within(dialog).getByLabelText('岗位名称'), { target: { value: 'CTO' } })
    fireEvent.click(within(dialog).getByText('保存'))
    await waitFor(() =>
      expect(adminSystem.createPost).toHaveBeenCalledWith(
        expect.objectContaining({ code: 'cto', name: 'CTO', sort: expect.any(Number), status: 1 }),
      ),
    )
  })

  it('deletes with confirm using the string id', async () => {
    renderPosts()
    await screen.findByText('CEO')
    fireEvent.click(screen.getByText('删除'))
    fireEvent.click(await screen.findByText('确定'))
    await waitFor(() => expect(adminSystem.deletePost).toHaveBeenCalledWith('1'))
  })

  it('hides write actions without their exact permissions', async () => {
    renderPosts(['system:post:list'])
    await screen.findByText('CEO')
    expect(screen.queryByText('新增岗位')).not.toBeInTheDocument()
    expect(screen.queryByText('编辑')).not.toBeInTheDocument()
    expect(screen.queryByText('删除')).not.toBeInTheDocument()
  })
})
