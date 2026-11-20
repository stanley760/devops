import { Outlet } from 'react-router-dom'

export default function AppShell() {
  return (
    <div className="min-h-screen">
      <main className="p-4">
        <Outlet />
      </main>
    </div>
  )
}
