import { Outlet } from 'react-router-dom'
import { Sidebar } from '@/components/layout/Sidebar'
import { Topbar } from '@/components/layout/Topbar'
import { useAppearance } from '@/lib/appearance'

export default function AppShell() {
  const { appearance } = useAppearance()
  return (
    <div className="glass-panel flex min-h-screen flex-col rounded-none border-0 bg-panel/70">
      <div className="glass-control sticky top-0 z-20 rounded-none border-0 bg-panel/80">
        <Topbar />
      </div>
      <div className="flex flex-1">
        <nav
          className={`glass-panel shrink-0 border-0 bg-panel/70 transition-[width] ${appearance.railCollapsed ? 'w-16' : 'w-60'}`}
          aria-label={appearance.railCollapsed ? '折叠导航' : '主导航'}
        >
          <Sidebar collapsed={appearance.railCollapsed} />
        </nav>
        <main className="glass-panel m-2 flex-1 rounded-xl p-4 shadow-none">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
