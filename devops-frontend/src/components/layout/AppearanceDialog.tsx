import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useAppearance } from '@/lib/appearance'
import type { Density, Theme } from '@/lib/types'

export function AppearanceDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (o: boolean) => void }) {
  const { appearance, update } = useAppearance()
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-sm">
        <DialogHeader><DialogTitle>外观设置</DialogTitle></DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>主题</Label>
            <Select value={appearance.theme} onValueChange={(v) => update({ theme: v as Theme })}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="light">浅色</SelectItem>
                <SelectItem value="dark">深色</SelectItem>
                <SelectItem value="auto">跟随系统</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="space-y-2">
            <Label>密度</Label>
            <Select value={appearance.density} onValueChange={(v) => update({ density: v as Density })}>
              <SelectTrigger><SelectValue /></SelectTrigger>
              <SelectContent>
                <SelectItem value="comfy">宽松</SelectItem>
                <SelectItem value="regular">标准</SelectItem>
                <SelectItem value="compact">紧凑</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-center justify-between">
            <Label>侧边栏折叠</Label>
            <Switch checked={appearance.railCollapsed} onCheckedChange={(v) => update({ railCollapsed: v })} />
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
