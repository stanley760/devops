import { createContext, useContext, useState } from 'react'
import { NavLink } from 'react-router-dom'
import { ChevronRight } from 'lucide-react'
import { useSession } from '@/store/session'
import type { RouterNode } from '@/lib/types'

/** 侧边栏折叠开关；移动端由外壳抽屉处理。 */
export const RailContext = createContext<{ collapsed: boolean; mobileDrawer?: boolean }>({ collapsed: false })

function isLeaf(n: RouterNode): boolean {
  return !n.children?.length
}

// Total order: dashboard first, then by sort, then id so equal sorts are stable.
function sortWithDashboardFirst(nodes: RouterNode[]): RouterNode[] {
  return [...nodes].sort((a, b) => {
    const aDashboard = a.component === 'dashboard/index' ? 0 : 1
    const bDashboard = b.component === 'dashboard/index' ? 0 : 1
    if (aDashboard !== bDashboard) return aDashboard - bDashboard
    if (a.sort !== b.sort) return a.sort - b.sort
    return a.id.localeCompare(b.id)
  })
}

function MenuLink({ node, collapsed }: { node: RouterNode; collapsed: boolean }) {
  return (
    <NavLink
      to={node.path}
      title={collapsed ? node.name : undefined}
      aria-label={node.name}
      className={({ isActive }) =>
        `flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors hover:bg-accent ${collapsed ? 'justify-center px-2' : ''} ${isActive ? 'bg-accent text-accent-foreground font-medium' : 'text-foreground/80'}`
      }
    >
      <span className="grid h-4 w-4 shrink-0 place-items-center rounded-sm border border-border bg-muted text-[8px]" aria-hidden>
        {node.name.slice(0, 1)}
      </span>
      {!collapsed && <span>{node.name}</span>}
    </NavLink>
  )
}

function MenuGroup({ node, collapsed }: { node: RouterNode; collapsed: boolean }) {
  const [open, setOpen] = useState(!collapsed)
  return (
    <div className="space-y-1">
      <button
        type="button"
        title={collapsed ? node.name : undefined}
        aria-label={node.name}
        onClick={() => setOpen(!open)}
        className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground/80 hover:bg-accent ${collapsed ? 'justify-center px-2' : ''}`}
      >
        <ChevronRight className={`h-4 w-4 shrink-0 transition-transform ${open ? 'rotate-90' : ''}`} />
        {!collapsed && <span>{node.name}</span>}
      </button>
      {open && (
        <div className={collapsed ? 'space-y-1' : 'ml-4 space-y-1 border-l border-border pl-2'}>
          {node.children!.filter((c) => c.visible).map((c) =>
            isLeaf(c)
              ? <MenuLink key={c.id} node={c} collapsed={collapsed} />
              : <MenuGroup key={c.id} node={c} collapsed={collapsed} />,
          )}
        </div>
      )}
    </div>
  )
}

export function Sidebar({ collapsed: collapsedProp, mobileDrawer = false }: { collapsed?: boolean; mobileDrawer?: boolean }) {
  const { routes } = useSession()
  const rail = useContext(RailContext)
  const collapsed = collapsedProp ?? rail.collapsed
  const visible = sortWithDashboardFirst(routes.filter((n) => n.visible))
  return (
    <aside className={`glass-panel flex h-full flex-col gap-1 border-0 bg-transparent shadow-none ${collapsed ? 'w-16 items-center px-2' : 'w-60 p-3'}`} aria-label="导航菜单">
      {visible.map((n) =>
        isLeaf(n)
          ? <MenuLink key={n.id} node={n} collapsed={collapsed} />
          : <MenuGroup key={n.id} node={n} collapsed={collapsed} />,
      )}
    </aside>
  )
}

