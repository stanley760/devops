import { useState } from 'react'
import { NavLink } from 'react-router-dom'
import { ChevronRight } from 'lucide-react'
import { useSession } from '@/store/session'
import type { RouterNode } from '@/lib/types'

function isLeaf(n: RouterNode): boolean {
  return !n.children?.length
}

// 工作台置于首位：component === dashboard/index 的节点排最前
function sortWithDashboardFirst(nodes: RouterNode[]): RouterNode[] {
  return [...nodes].sort((a, b) => {
    const ad = a.component === 'dashboard/index' ? -1 : 0
    const bd = b.component === 'dashboard/index' ? -1 : 0
    if (ad !== bd) return ad - bd
    return a.sort - b.sort
  })
}

function MenuLink({ node, railWide }: { node: RouterNode; railWide: boolean }) {
  return (
    <NavLink
      to={node.path}
      className={({ isActive }) =>
        `flex items-center gap-2 rounded-md px-3 py-2 text-sm transition-colors hover:bg-accent ${isActive ? 'bg-accent text-accent-foreground font-medium' : 'text-foreground/80'}`
      }
    >
      <span className="h-4 w-4 shrink-0" aria-hidden />
      <span className={railWide ? 'rail-wide' : ''}>{node.name}</span>
    </NavLink>
  )
}

function MenuGroup({ node, railWide }: { node: RouterNode; railWide: boolean }) {
  const [open, setOpen] = useState(true)
  return (
    <div className="space-y-1">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm text-foreground/80 hover:bg-accent"
      >
        <ChevronRight className={`h-4 w-4 transition-transform ${open ? 'rotate-90' : ''}`} />
        <span className={railWide ? 'rail-wide' : ''}>{node.name}</span>
      </button>
      {open && (
        <div className="ml-4 space-y-1 border-l border-border pl-2">
          {node.children!.filter((c) => c.visible).map((c) =>
            isLeaf(c)
              ? <MenuLink key={c.id} node={c} railWide={railWide} />
              : <MenuGroup key={c.id} node={c} railWide={railWide} />,
          )}
        </div>
      )}
    </div>
  )
}

export function Sidebar({ collapsed = false }: { collapsed?: boolean }) {
  const { routes } = useSession()
  const visible = sortWithDashboardFirst(routes.filter((n) => n.visible))
  return (
    <aside className="flex h-full flex-col gap-1 p-3">
      {visible.map((n) =>
        isLeaf(n)
          ? <MenuLink key={n.id} node={n} railWide={!collapsed} />
          : <MenuGroup key={n.id} node={n} railWide={!collapsed} />,
      )}
    </aside>
  )
}
