import type { ButtonProps } from '@/components/ui/button'
import { Button } from '@/components/ui/button'
import { usePermissions } from '@/lib/permission'

export function PermissionButton({
  permission, danger, variant, ...rest
}: ButtonProps & { permission: string; danger?: boolean }) {
  const { has } = usePermissions()
  if (!has(permission)) return null // 无权限：不渲染，而非置灰
  return <Button variant={danger ? 'destructive' : variant} {...rest} />
}
