import { Outlet } from 'react-router-dom'
import { Sidebar } from '@/components/layout/Sidebar'
import { Topbar } from '@/components/layout/Topbar'
import { useAppearance } from '@/lib/appearance'

export default function AppShell() {
  const { appearance } = useAppearance()
  return (
    <div className="flex min-h-screen flex-col">
      <Topbar />
      <div className="flex flex-1">
        {!appearance.railCollapsed && (
          <nav className="w-60 shrink-0 border-r border-border">
            <Sidebar collapsed={false} />
          </nav>
        )}
        <main className="flex-1 p-4">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
