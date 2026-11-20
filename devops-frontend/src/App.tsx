import { useEffect, type ReactNode } from 'react'
import { BrowserRouter, useRoutes } from 'react-router-dom'
import { buildDynamicRoutes } from '@/router/routes'
import { RequireAuth, RequireGuest, installUnauthorizedHandler } from '@/router/guards'
import { SessionProvider, useSession } from '@/store/session'
import { PermissionsProvider } from '@/lib/permission'
import { ConfirmProvider } from '@/lib/confirm'
import AppShell from '@/components/layout/AppShell'
import Login from '@/views/Login'
import NoAccess from '@/views/NoAccess'
import NotFound from '@/views/NotFound'
import Dashboard from '@/views/Dashboard'

function RouterContent() {
  const { routes } = useSession()
  const dynamic = buildDynamicRoutes(routes)
  const routesConfig = [
    { path: '/login', element: <RequireGuest><Login /></RequireGuest> },
    { path: '/t/:code/login', element: <RequireGuest><Login /></RequireGuest> },
    {
      path: '/',
      element: (
        <RequireAuth>
          <AppShell />
        </RequireAuth>
      ),
      children: [
        // 工作台静态置于首位（spec §4.4：dashboard 静态提供，后端 /auth/routers 不下发）
        { index: true, element: <Dashboard /> },
        ...dynamic.map((d) => ({
          path: d.path.replace(/^\/+/, ''),
          element: d.element,
        })),
        { path: '*', element: <NotFound /> },
      ],
    },
    { path: '/403', element: <NoAccess /> },
    { path: '*', element: <NotFound /> },
  ]
  return useRoutes(routesConfig)
}

function PermissionsGate({ children }: { children: ReactNode }) {
  const { authInfo } = useSession()
  return (
    <PermissionsProvider permissions={authInfo?.permissions ?? []}>
      {children}
    </PermissionsProvider>
  )
}

export default function App() {
  useEffect(() => installUnauthorizedHandler(), [])
  return (
    <BrowserRouter>
      <SessionProvider>
        <PermissionsGate>
          <ConfirmProvider>
            <RouterContent />
          </ConfirmProvider>
        </PermissionsGate>
      </SessionProvider>
    </BrowserRouter>
  )
}
