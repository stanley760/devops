import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useLocation, useRoutes } from 'react-router-dom'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { RequireAuth, installUnauthorizedHandler } from '@/router/guards'
import { session } from '@/lib/session'
import { SessionProvider, useSession } from '@/store/session'
import { PermissionsProvider, usePermissions } from '@/lib/permission'
import Login from '@/views/Login'
import type { AuthInfo, RouterNode } from '@/lib/types'

let originalLocation: Location

const authInfo: AuthInfo = {
  user: { id: '1', username: 'alice' },
  tenant: { id: '1', name: 'Acme', code: 'acme' },
  roles: ['admin'],
  permissions: ['system:user:create'],
}

const routerNodes: RouterNode[] = [{
  id: '10',
  name: 'Users',
  path: '/system/users',
  component: 'system/user/index',
  sort: 1,
  visible: true,
}]

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <SessionProvider>
        <Routes>
          <Route
            path="/protected"
            element={
              <RequireAuth>
                <div>secret</div>
              </RequireAuth>
            }
          />
          <Route path="/login" element={<div>login page</div>} />
        </Routes>
      </SessionProvider>
    </MemoryRouter>,
  )
}

function ok(data: unknown) {
  return new Response(JSON.stringify({ code: 200, data }), { status: 200 })
}

function useLocationProbe() {
  return useLocation()
}

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
  originalLocation = window.location
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: { ...originalLocation, assign: vi.fn() },
  })
  installUnauthorizedHandler()
})

afterEach(() => {
  Object.defineProperty(window, 'location', { configurable: true, value: originalLocation })
})

describe('RequireAuth', () => {
  it('renders children when logged in', async () => {
    session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
    renderAt('/protected')
    await waitFor(() => expect(screen.getByText('secret')).toBeInTheDocument())
  })

  it('redirects with tenantCode+redirect query params (including via 401 handler)', async () => {
    session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
    const assign = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { pathname: '/protected', search: '?tab=1', assign },
    })
    session.fireUnauthorized()
    expect(assign).toHaveBeenCalledTimes(1)
    const target = new URL(assign.mock.calls[0]![0] as string, 'http://localhost')
    expect(target.pathname).toBe('/login')
    expect(target.searchParams.get('tenantCode')).toBe('acme')
    expect(target.searchParams.get('redirect')).toBe('/protected?tab=1')
  })

  it('redirects unauthenticated routes with tenantCode+redirect query params', async () => {
    function LoginProbe() {
      const location = useLocation()
      return (
        <div>
          <div>login page</div>
          <div data-testid="login-search">{location.search}</div>
        </div>
      )
    }
    render(
      <MemoryRouter initialEntries={['/protected?tab=1']}>
        <SessionProvider>
          <Routes>
            <Route path="/protected" element={<RequireAuth><div>secret</div></RequireAuth>} />
            <Route path="/login" element={<LoginProbe />} />
          </Routes>
        </SessionProvider>
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('login page')).toBeInTheDocument())
    expect(screen.getByTestId('login-search')).toHaveTextContent('?')
    const loginSearch = screen.getByTestId('login-search').textContent ?? ''
    const params = new URLSearchParams(loginSearch.replace(/^\?/, ''))
    expect(params.get('tenantCode')).toBe('default')
    expect(params.get('redirect')).toBe('/protected?tab=1')
  })

  it('uses last tenant after session clear', async () => {
    session.saveSession({ access_token: 'a', refresh_token: 'r' }, { id: '1', code: 'acme' })
    session.clearSession()
    const assign = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...window.location, pathname: '/protected', search: '', assign },
    })
    session.fireUnauthorized()
    const target = new URL(assign.mock.calls[0]![0] as string, 'http://localhost')
    expect(target.searchParams.get('tenantCode')).toBe('acme')
  })
})

describe('client-side login bootstrap', () => {
  it('loads dynamic routes and permissions before navigating', async () => {
    vi.spyOn(global, 'fetch').mockImplementation(async (url) => {
      const path = String(url)
      if (path.includes('/auth/login')) {
        return ok({ access_token: 'a', refresh_token: 'r', expires_in: 3600, tenant: authInfo.tenant })
      }
      if (path.includes('/auth/info')) return ok(authInfo)
      if (path.includes('/auth/routers')) return ok(routerNodes)
      return new Response('unexpected URL', { status: 500 })
    })

    function PermissionProbe() {
      const { has } = usePermissions()
      return <div data-testid="permission">{String(has('system:user:create'))}</div>
    }
    function ShellProbe() {
      const info = useSession().authInfo
      return (
        <PermissionsProvider permissions={info?.permissions ?? []}>
          <HomeProbe />
          <PermissionProbe />
        </PermissionsProvider>
      )
    }
    function DynamicProbe() {
      return <div>动态用户页</div>
    }
    function HomeProbe() {
      const { routes } = useSession()
      const location = useLocation()
      return (
        <div>
          <div data-testid="path">{location.pathname}</div>
          <div data-testid="routes">{String(routes.length)}</div>
        </div>
      )
    }
    function AppProbe() {
      return useRoutes([
        { path: '/login', element: <Login /> },
        {
          path: '/',
          element: (
            <RequireAuth>
              <ShellProbe />
            </RequireAuth>
          ),
          children: [{ path: 'system/users', element: <DynamicProbe /> }],
        },
      ])
    }

    render(
      <MemoryRouter initialEntries={['/login?redirect=/system/users']}>
        <SessionProvider>
          <AppProbe />
        </SessionProvider>
      </MemoryRouter>,
    )

    expect(await screen.findByRole('button', { name: '登录' })).toBeInTheDocument()
    expect(session.isLoggedIn()).toBe(false)
    const user = userEvent.setup()
    const inputs = document.querySelectorAll('input')
    await user.type(inputs[0] as HTMLInputElement, 'alice')
    await user.type(inputs[1] as HTMLInputElement, 'secret')
    await user.click(screen.getByRole('button', { name: '登录' }))

    expect(screen.getByTestId('path')).toHaveTextContent('/system/users')
    expect(screen.getByTestId('routes')).toHaveTextContent('1')
    expect(screen.getByTestId('permission')).toHaveTextContent('true')
    expect(vi.mocked(global.fetch)).toHaveBeenCalledWith(
      expect.stringContaining('/auth/info'),
      expect.anything(),
    )
    expect(vi.mocked(global.fetch)).toHaveBeenCalledWith(
      expect.stringContaining('/auth/routers'),
      expect.anything(),
    )
  })
})
