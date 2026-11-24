import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { useHashRoute } from '@/portal/useHashRoute'
import { portalSession } from '@/lib/portal-session'
import { portalAuth } from '@/api/portal/auth'
import { Home } from '@/portal/pages/Home'
import { Docs } from '@/portal/pages/Docs'
import { PortalLoginPage } from '@/portal/pages/Login'
import { PortalRegisterPage } from '@/portal/pages/Register'
import { ProfilePage } from '@/portal/pages/Profile'
import { Button } from '@/components/ui/button'

export default function PortalApp() {
  const { path, navigate } = useHashRoute()
  const [loggedIn, setLoggedIn] = useState(portalSession.isLoggedIn())

  // 登录态的唯一来源是本地会话存储；登录/注册/退出后调用它刷新 UI。
  const refreshLoggedIn = useCallback(() => setLoggedIn(portalSession.isLoggedIn()), [])

  // 路由变化时与本地会话重新对齐（例如登录页写入令牌后跳回 /profile）。
  useEffect(() => {
    refreshLoggedIn()
  }, [path, refreshLoggedIn])

  const logout = useCallback(async () => {
    // 带上 refresh_token：后端据此吊销刷新令牌，
    // 否则旧 refresh token 仍能换出新令牌，登出就不是真的登出。
    const refreshToken = portalSession.getTokens()?.refresh_token
    try {
      await portalAuth.logout(refreshToken)
    } catch {
      // 本地清理优先
    }
    portalSession.clearTokens()
    setLoggedIn(false)
    navigate('/')
  }, [navigate])

  let page: ReactNode
  switch (path) {
    case '/login':
      page = loggedIn
        ? <ProfilePage navigate={navigate} onLoggedOut={refreshLoggedIn} />
        : <PortalLoginPage navigate={navigate} onLoggedIn={refreshLoggedIn} />
      break
    case '/register':
      page = loggedIn
        ? <ProfilePage navigate={navigate} onLoggedOut={refreshLoggedIn} />
        : <PortalRegisterPage navigate={navigate} onRegistered={refreshLoggedIn} />
      break
    case '/profile':
      page = loggedIn
        ? <ProfilePage navigate={navigate} onLoggedOut={refreshLoggedIn} />
        : <PortalLoginPage navigate={navigate} onLoggedIn={refreshLoggedIn} />
      break
    case '/docs':
      page = <Docs />
      break
    default:
      page = <Home navigate={navigate} loggedIn={loggedIn} />
  }

  return (
    <div className="glass-panel flex min-h-screen flex-col rounded-none border-0 bg-panel/70">
      <nav className="glass-control sticky top-0 z-20 flex items-center justify-between rounded-none border-0 bg-panel/80 px-6 py-4">
        <button className="text-xl font-bold" onClick={() => navigate('/')}>Design World</button>
        <div className="flex items-center gap-2">
          <Button variant="ghost" className="glass-control" onClick={() => navigate('/docs')}>文档</Button>
          {loggedIn ? (
            <>
              <Button variant="ghost" className="glass-control" onClick={() => navigate('/profile')}>个人资料</Button>
              <Button variant="outline" className="glass-control" onClick={() => void logout()}>退出</Button>
            </>
          ) : (
            <>
              <Button variant="ghost" className="glass-control" onClick={() => navigate('/login')}>Sign In</Button>
              <Button onClick={() => navigate('/register')}>Sign Up</Button>
            </>
          )}
        </div>
      </nav>
      <div className="flex-1">{page}</div>
    </div>
  )
}
