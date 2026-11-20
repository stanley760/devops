import { useEffect, useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { Command, Menu as MenuIcon, UserRound } from 'lucide-react'
import {
  Breadcrumb, BreadcrumbItem, BreadcrumbLink, BreadcrumbList, BreadcrumbPage, BreadcrumbSeparator,
} from '@/components/ui/breadcrumb'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel,
  DropdownMenuSeparator, DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { flattenMenus } from '@/router/routes'
import { useSession } from '@/store/session'
import { CommandPalette } from '@/components/layout/CommandPalette'
import { AppearanceDialog } from '@/components/layout/AppearanceDialog'
import { PasswordDialog } from '@/components/layout/PasswordDialog'
import { MobileDrawer } from '@/components/layout/MobileDrawer'
import { useConfirm } from '@/lib/confirm'

export function Topbar() {
  const { authInfo, routes, logout } = useSession()
  const confirm = useConfirm()
  const location = useLocation()
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [appearanceOpen, setAppearanceOpen] = useState(false)
  const [passwordOpen, setPasswordOpen] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)

  // Ctrl/⌘ + K 命令面板
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setPaletteOpen((o) => !o)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  const menus = flattenMenus(routes)
  const current = menus.find((m) => m.path !== '/' && location.pathname.startsWith(m.path))
  const tenantLabel = authInfo ? `${authInfo.tenant.name}（${authInfo.tenant.code}）` : ''

  const onLogout = async () => {
    const ok = await confirm({ title: '确认退出登录？', danger: true, confirmText: '退出' })
    if (!ok) return
    await logout()
    window.location.assign('/login')
  }

  return (
    <header className="flex items-center gap-3 px-4 py-3">
      <Button variant="ghost" size="icon" className="md:hidden" onClick={() => setDrawerOpen(true)} aria-label="打开菜单">
        <MenuIcon className="h-5 w-5" />
      </Button>
      <Breadcrumb>
        <BreadcrumbList>
          <BreadcrumbItem>
            <BreadcrumbLink render={<Link to="/" />}>工作台</BreadcrumbLink>
          </BreadcrumbItem>
          {current && (
            <>
              <BreadcrumbSeparator />
              <BreadcrumbItem><BreadcrumbPage>{current.name}</BreadcrumbPage></BreadcrumbItem>
            </>
          )}
        </BreadcrumbList>
      </Breadcrumb>
      <span className="text-muted-foreground hidden text-xs md:inline">{tenantLabel}</span>
      <div className="ml-auto flex items-center gap-2">
        <Button variant="ghost" size="sm" onClick={() => setPaletteOpen(true)} aria-label="打开命令面板">
          <Command className="h-4 w-4" /> Ctrl+K
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger render={
            <Button variant="ghost" size="sm" aria-label="用户菜单">
              <UserRound className="h-4 w-4" />
              {authInfo?.user.nickname || authInfo?.user.username}
            </Button>
          } />
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>{authInfo?.user.username}</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => setPasswordOpen(true)}>修改密码</DropdownMenuItem>
            <DropdownMenuItem onSelect={() => setAppearanceOpen(true)}>外观设置</DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem onSelect={() => void onLogout()}>退出登录</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
      <CommandPalette open={paletteOpen} onOpenChange={setPaletteOpen} />
      <AppearanceDialog open={appearanceOpen} onOpenChange={setAppearanceOpen} />
      <PasswordDialog open={passwordOpen} onOpenChange={setPasswordOpen} />
      <MobileDrawer open={drawerOpen} onOpenChange={setDrawerOpen} />
    </header>
  )
}
