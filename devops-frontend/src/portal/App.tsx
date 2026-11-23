import { useCallback, useState, type ReactNode } from 'react'
import { useHashRoute } from '@/portal/useHashRoute'
import { portalSession } from '@/lib/portal-session'
import { portalAuth } from '@/api/portal/auth'
import { Home } from '@/portal/pages/Home'
import { Docs } from '@/portal/pages/Docs'
import { Button } from '@/components/ui/button'

export default function PortalApp() {
  const { path, navigate } = useHashRoute()
  const [loggedIn, setLoggedIn] = useState(portalSession.isLoggedIn())

  const refreshLoggedIn = useCallback(() => setLoggedIn(portalSession.isLoggedIn()), [])
  const logout = useCallback(async () => {
    try {
      await portalAuth.logout()
    } catch {
      // 本地清理优先
    }
    portalSession.clearTokens()
    setLoggedIn(false)
    navigate('/')
  }, [navigate])

  // Task 14 将用真实页面组件替换以下内联占位
  const loginPage = (
    <div className="mx-auto max-w-sm p-6">
      <h2 className="mb-4 text-2xl font-semibold">Sign In</h2>
      <p className="text-muted-foreground text-sm">Task 14 完整化</p>
    </div>
  )

  function ProfileStub() {
    return <div className="mx-auto max-w-sm p-6">个人资料（Task 14 完整化）</div>
  }

  let page: ReactNode
  switch (path) {
    case '/login':
      page = loggedIn ? <ProfileStub /> : loginPage
      break
    case '/register':
      page = loggedIn ? <ProfileStub /> : loginPage
      break
    case '/profile':
      page = <ProfileStub />
      break
    case '/docs':
      page = <Docs />
      break
    default:
      page = <Home navigate={navigate} loggedIn={loggedIn} />
  }

  return (
    <div className="flex min-h-screen flex-col">
      <nav className="flex items-center justify-between px-6 py-4">
        <button className="text-xl font-bold" onClick={() => navigate('/')}>Design World</button>
        <div className="flex items-center gap-2">
          <Button variant="ghost" onClick={() => navigate('/docs')}>文档</Button>
          {loggedIn ? (
            <>
              <Button variant="ghost" onClick={() => navigate('/profile')}>个人资料</Button>
              <Button variant="outline" onClick={() => void logout()}>退出</Button>
            </>
          ) : (
            <>
              <Button variant="ghost" onClick={() => navigate('/login')}>Sign In</Button>
              <Button onClick={() => navigate('/register')}>Sign Up</Button>
            </>
          )}
        </div>
      </nav>
      <div className="flex-1">{page}</div>
    </div>
  )
}
