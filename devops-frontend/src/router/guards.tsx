import { type ReactNode } from 'react'
import { Navigate, useLocation } from 'react-router-dom'
import { session } from '@/lib/session'
import { useSession } from '@/store/session'

function currentTenantCode(): string {
  return (
    session.getTenantCode() ||
    session.getLastTenant() ||
    new URLSearchParams(window.location.search).get('tenantCode') ||
    'default'
  )
}

export function RequireAuth({ children }: { children: ReactNode }) {
  const { loading } = useSession()
  const location = useLocation()
  if (loading) return <div className="p-8 text-sm text-muted-foreground">加载中…</div>
  if (!session.isLoggedIn()) {
    const params = new URLSearchParams({
      tenantCode: currentTenantCode(),
      redirect: location.pathname + location.search,
    })
    return <Navigate to={`/login?${params.toString()}`} replace />
  }
  return <>{children}</>
}

export function RequireGuest({ children }: { children: ReactNode }) {
  const { loading } = useSession()
  if (loading) return <div className="p-8 text-sm text-muted-foreground">加载中…</div>
  if (session.isLoggedIn()) return <Navigate to="/" replace />
  return <>{children}</>
}

// 401 未授权处理：清理会话并跳登录（携带 tenantCode 与 redirect）
export function installUnauthorizedHandler(): () => void {
  return session.onUnauthorized(() => {
    const { pathname, search } = window.location
    if (pathname.startsWith('/login')) return
    const params = new URLSearchParams({
      tenantCode: currentTenantCode(),
      redirect: pathname + search,
    })
    window.location.assign(`/login?${params.toString()}`)
  })
}
