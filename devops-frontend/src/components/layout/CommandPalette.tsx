import { useNavigate } from 'react-router-dom'
import {
  Command, CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList,
} from '@/components/ui/command'
import { flattenMenus } from '@/router/routes'
import { useSession } from '@/store/session'

export function CommandPalette({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const { routes } = useSession()
  const navigate = useNavigate()
  const menus = flattenMenus(routes).filter((n) => !n.children?.length)

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange}>
      <Command>
        <CommandInput placeholder="搜索菜单…" />
        <CommandList>
          <CommandEmpty>无匹配菜单</CommandEmpty>
          <CommandGroup heading="菜单">
            {menus.map((m) => (
              <CommandItem
                key={m.id}
                value={m.name}
                onSelect={() => {
                  onOpenChange(false)
                  navigate(m.path)
                }}
              >
                {m.name}
              </CommandItem>
            ))}
          </CommandGroup>
        </CommandList>
      </Command>
    </CommandDialog>
  )
}
