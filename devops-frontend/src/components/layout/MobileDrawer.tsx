import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import { RailContext, Sidebar } from '@/components/layout/Sidebar'

export function MobileDrawer({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="left" className="w-64 p-0">
        <SheetHeader className="px-4 py-3">
          <SheetTitle>导航菜单</SheetTitle>
        </SheetHeader>
        <RailContext.Provider value={{ collapsed: false, mobileDrawer: true }}>
          <Sidebar />\n        </RailContext.Provider>
      </SheetContent>
    </Sheet>
  )
}

