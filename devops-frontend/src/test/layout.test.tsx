import { render, screen, fireEvent } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SessionProvider } from '@/store/session'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import { Sidebar } from '@/components/layout/Sidebar'
import { Topbar } from '@/components/layout/Topbar'
import { AppearanceDialog } from '@/components/layout/AppearanceDialog'
import { PasswordDialog } from '@/components/layout/PasswordDialog'
import { saveAppearance, applyAppearance } from '@/lib/appearance'
import { adminAuth } from '@/api/admin/auth'
import { session } from '@/lib/session'
import type { AuthInfo, RouterNode } from '@/lib/types'

vi.mock('@/api/admin/auth', () => ({
  adminAuth: {
    getInfo: vi.fn(),
    getRouters: vi.fn(),
    logout: vi.fn().mockResolvedValue(undefined),
    updatePassword: vi.fn().mockResolvedValue(undefined),
  },
}))

vi.mock('@/lib/toast', () => ({
  toast: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}))

const authInfo: AuthInfo = {
  user: { id: '1', username: 'admin', nickname: '管理员' },
  tenant: { id: '1', name: 'ACME', code: 'acme' },
  roles: ['admin'],
  permissions: ['system:user:create'],
}

const routers: RouterNode[] = [
  { id: 'd', name: '工作台', path: '/dashboard', component: 'dashboard/index', sort: 99, visible: true },
  {
    id: 's', name: '系统管理', path: '/system', component: '', sort: 1, visible: true,
    children: [{ id: 'u', name: '用户管理', path: '/system/user', component: 'system/user/index', sort: 1, visible: true }],
  },
]

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
  vi.mocked(adminAuth.getInfo).mockResolvedValue(authInfo)
  vi.mocked(adminAuth.getRouters).mockResolvedValue(routers)
})

function renderWithProviders(ui: React.ReactElement, path = '/system/user') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="*" element={
          <SessionProvider>
            <PermissionsProvider permissions={authInfo.permissions}>
              <ConfirmProvider>{ui}</ConfirmProvider>
            </PermissionsProvider>
          </SessionProvider>
        } />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Sidebar', () => {
  it('renders dashboard first and groups', async () => {
    renderWithProviders(<Sidebar collapsed={false} />)
    await screen.findByText('工作台')
    expect(screen.getByText('系统管理')).toBeInTheDocument()
    expect(screen.getByText('用户管理')).toBeInTheDocument()
    // 工作台在用户管理之前
    const all = screen.getAllByRole('link').map((el) => el.textContent)
    expect(all.findIndex((t) => t?.includes('工作台')))
      .toBeLessThan(all.findIndex((t) => t?.includes('用户管理')))
  })
})

describe('Topbar', () => {
  it('shows tenant info and user menu', async () => {
    renderWithProviders(<Topbar />)
    await screen.findByText(/ACME/)
    expect(screen.getByLabelText('用户菜单')).toBeInTheDocument()
    expect(screen.getByLabelText('打开命令面板')).toBeInTheDocument()
  })

  it('opens command palette via Ctrl+K and closes on select', async () => {
    renderWithProviders(<Topbar />)
    await screen.findByText(/ACME/)
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    expect(screen.getByPlaceholderText('搜索菜单…')).toBeInTheDocument()
    fireEvent.click(await screen.findByText('用户管理'))
    expect(screen.queryByPlaceholderText('搜索菜单…')).not.toBeInTheDocument()
  })
})

describe('AppearanceDialog', () => {
  it('applies density to html dataset', () => {
    saveAppearance({ theme: 'auto', density: 'regular', railCollapsed: false })
    render(
      <MemoryRouter>
        <AppearanceDialog open onOpenChange={() => {}} />
      </MemoryRouter>,
    )
    applyAppearance()
    expect(document.documentElement.dataset.density).toBe('regular')
  })
})

describe('PasswordDialog', () => {
  it('rejects mismatched confirmation with error toast (no api call)', async () => {
    const { toastError } = await import('@/lib/toast')
    render(
      <MemoryRouter>
        <PasswordDialog open onOpenChange={() => {}} />
      </MemoryRouter>,
    )
    const dialog = screen.getByRole('dialog')
    const fields = dialog.querySelectorAll('input[type="password"]')
    fireEvent.change(fields[1]!, { target: { value: 'a' } })
    fireEvent.change(fields[2]!, { target: { value: 'b' } })
    fireEvent.submit(dialog.querySelector('form')!)
    expect(toastError).toHaveBeenCalledWith('两次输入的密码不一致')
    expect(adminAuth.updatePassword).not.toHaveBeenCalled()
  })
})
