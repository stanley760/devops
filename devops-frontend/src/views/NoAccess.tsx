import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function NoAccess() {
  return (
    <Card className="mx-auto mt-16 max-w-md">
      <CardHeader><CardTitle>403</CardTitle></CardHeader>
      <CardContent className="text-muted-foreground">没有权限访问该页面</CardContent>
    </Card>
  )
}
