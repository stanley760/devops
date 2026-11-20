import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react'
import { adminAuth } from '@/api/admin/auth'
import { session } from '@/lib/session'
import { toastError } from '@/lib/toast'
import type { AuthInfo, RouterNode } from '@/lib/types'

interface SessionContextValue {
  authInfo: AuthInfo | null
  routes: RouterNode[]
  loading: boolean
  bootstrap: () => Promise<void>
  reload: () => Promise<void>
  logout: () => Promise<void>
}

const SessionContext = createContext<SessionContextValue | null>(null)

export function SessionProvider({ children }: { children: ReactNode }) {
  const [authInfo, setAuthInfo] = useState<AuthInfo | null>(null)
  const [routes, setRoutes] = useState<RouterNode[]>([])
  const [loading, setLoading] = useState(true)

  const bootstrap = useCallback(async () => {
    setLoading(true)
    try {
      if (!session.isLoggedIn()) {
        setAuthInfo(null)
        setRoutes([])
        return
      }
      const [info, routerNodes] = await Promise.all([adminAuth.getInfo(), adminAuth.getRouters()])
      setAuthInfo(info)
      setRoutes(routerNodes)
    } catch (e) {
      toastError((e as Error).message || '会话信息加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void bootstrap()
  }, [bootstrap])

  const reload = useCallback(async () => {
    await bootstrap()
  }, [bootstrap])

  const logout = useCallback(async () => {
    try {
      await adminAuth.logout()
    } catch {
      // 后端登出失败不影响本地清理
    }
    session.clearSession()
    setAuthInfo(null)
    setRoutes([])
  }, [])

  return (
    <SessionContext.Provider value={{ authInfo, routes, loading, bootstrap, reload, logout }}>
      {children}
    </SessionContext.Provider>
  )
}

export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext)
  if (!ctx) throw new Error('useSession must be used within SessionProvider')
  return ctx
}
