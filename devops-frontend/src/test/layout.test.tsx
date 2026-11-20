import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { SessionProvider } from '@/store/session'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import { Sidebar } from '@/components/layout/Sidebar'
import { Topbar } from '@/components/layout/Topbar'
import { AppearanceDialog } from '@/components/layout/AppearanceDialog'
import { PasswordDialog } from '@/components/layout/PasswordDialog'
import AppShell from '@/components/layout/AppShell'
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
  { id: 'u2', name: '用户2', path: '/user2', component: 'user/index', sort: 2, visible: true },
]

function LocationProbe() {
  const location = useLocation()
  return <div data-testid="location">{location.pathname}</div>
}

beforeEach(() => {
  localStorage.clear()
  vi.clearAllMocks()
  document.documentElement.className = ''
  document.documentElement.removeAttribute('data-density')
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
              <ConfirmProvider>
                {ui}
                <LocationProbe />
              </ConfirmProvider>
            </PermissionsProvider>
          </SessionProvider>
        } />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Sidebar', () => {
  it('renders dashboard first and groups with a stable total order', async () => {
    const nodes: RouterNode[] = [
      { id: 'z', name: 'Z', path: '/z', component: '', sort: 1, visible: true },
      { id: 'a', name: 'A', path: '/a', component: '', sort: 1, visible: true },
      routers[0]!,
    ]
    vi.mocked(adminAuth.getRouters).mockResolvedValue(nodes)
    renderWithProviders(<Sidebar collapsed={false} />)
    await screen.findByText('工作台')
    const all = screen.getAllByRole('link').map((el) => el.textContent?.replace(/^./, ''))
    expect(all[0]).toContain('工作台')
    expect(all.slice(1)).toEqual(['A', 'Z'])
  })

  it('renders a compact icon rail when collapsed', async () => {
    renderWithProviders(<Sidebar collapsed />)
    await screen.findByLabelText('工作台')
    expect(screen.getByLabelText('工作台')).toHaveTextContent('工')
    expect(screen.getByLabelText('工作台')).not.toHaveTextContent('工作台')
  })
})

describe('Topbar', () => {
  it('shows tenant info and user menu', async () => {
    renderWithProviders(<Topbar />)
    await screen.findByText(/ACME/)
    expect(screen.getByLabelText('用户菜单')).toBeInTheDocument()
    expect(screen.getByLabelText('打开命令面板')).toBeInTheDocument()
    expect(screen.getByLabelText('打开命令面板')).toHaveTextContent('Ctrl/Cmd+K')
  })

  it('opens command palette via Ctrl+K and Meta+K, then navigates on select', async () => {
    const view = renderWithProviders(<Topbar />)
    await screen.findByText(/ACME/)
    fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    const input = await screen.findByPlaceholderText('搜索菜单…')
    const menuItem = await screen.findByRole('option', { name: '用户管理' })
    expect(input).toBeInTheDocument()
    fireEvent.click(menuItem)
    await waitFor(() => expect(view.getByTestId('location')).toHaveTextContent('/system/user'))
    expect(screen.queryByPlaceholderText('搜索菜单…')).not.toBeInTheDocument()
    fireEvent.keyDown(window, { key: 'k', metaKey: true })
    expect(await screen.findByPlaceholderText('搜索菜单…')).toBeInTheDocument()
  })

  it('does not let /system/user resolve to the /system prefix', async () => {
    const view = renderWithProviders(<Topbar />, '/system/user')
    const current = await view.findByRole('link', { name: /用户管理/ })
    expect(current).toHaveAttribute('aria-current', 'page')
    expect(view.queryByRole('link', { name: /系统管理/ })).not.toBeInTheDocument()
  })

  it('does not let /user2 resolve to /user', async () => {
    vi.mocked(adminAuth.getRouters).mockResolvedValue([
      { id: 's', name: '系统管理', path: '/system', component: '', sort: 1, visible: true,
        children: [{ id: 'u', name: '用户管理', path: '/system/user', component: 'system/user/index', sort: 1, visible: true }] },
      { id: 'p', name: '用户', path: '/user', component: '', sort: 2, visible: true },
      { id: 'u2', name: '用户2', path: '/user2', component: 'user/index', sort: 3, visible: true },
    ])
    const view = renderWithProviders(<Topbar />, '/user2')
    await view.findByText(/ACME/)
    expect(view.getByRole('link', { name: /用户2/ })).toHaveAttribute('aria-current', 'page')
    expect(view.queryByRole('link', { name: /用户管理/ })).not.toBeInTheDocument()
  })
})

describe('AppShell', () => {
  it('uses solid-glass surfaces and keeps the rail visible when collapsed', async () => {
    saveAppearance({ theme: 'dark', density: 'compact', railCollapsed: true })
    const view = renderWithProviders(<AppShell />)
    await screen.findByText(/ACME/)
    expect(view.container.querySelector('.glass-panel')).toBeInTheDocument()
    expect(view.container.querySelector('.glass-control')).toBeInTheDocument()
    const rail = screen.getByLabelText('折叠导航')
    expect(rail).toHaveTextContent('工')
    expect(rail).not.toHaveTextContent('工作台')
  })
})

describe('AppearanceDialog', () => {
  it('applies theme and density from dialog interaction to html state', async () => {
    saveAppearance({ theme: 'auto', density: 'regular', railCollapsed: false })
    applyAppearance()
    const view = renderWithProviders(<AppearanceDialog open onOpenChange={() => {}} />)
    await view.findByRole('dialog')

    const triggers = view.getAllByRole('combobox')
    expect(triggers).toHaveLength(2)
    fireEvent.click(triggers[0]!)
    const darkOption = await screen.findByRole('option', { name: '深色' })
    fireEvent.click(darkOption)
    fireEvent.keyDown(darkOption, { key: 'Enter' })
    await waitFor(() => expect(document.documentElement.classList.contains('dark')).toBe(true))

    fireEvent.click(triggers[1]!)
    const compactOption = await screen.findByRole('option', { name: '紧凑' })
    fireEvent.click(compactOption)
    fireEvent.keyDown(compactOption, { key: 'Enter' })
    await waitFor(() => expect(document.documentElement.dataset.density).toBe('compact'))
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

  it('clears fields when reopened after a failed submit', async () => {
    vi.mocked(adminAuth.updatePassword).mockRejectedValueOnce(new Error('旧密码错误'))
    const first = render(<MemoryRouter><PasswordDialog open onOpenChange={() => {}} /></MemoryRouter>)
    let fields = first.getByRole('dialog').querySelectorAll('input[type="password"]')
    fireEvent.change(fields[0]!, { target: { value: 'old' } })
    fireEvent.change(fields[1]!, { target: { value: 'new' } })
    fireEvent.change(fields[2]!, { target: { value: 'new' } })
    fireEvent.submit(first.getByRole('dialog').querySelector('form')!)
    await waitFor(() => expect(adminAuth.updatePassword).toHaveBeenCalled())
    first.unmount()

    const reopened = render(<MemoryRouter><PasswordDialog open onOpenChange={() => {}} /></MemoryRouter>)
    fields = reopened.getByRole('dialog').querySelectorAll('input[type="password"]')
    expect(Array.from(fields).every((input) => (input as HTMLInputElement).value === '')).toBe(true)
  })
})


