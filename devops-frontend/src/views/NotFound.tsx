import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

export default function NotFound() {
  return (
    <Card className="mx-auto mt-16 max-w-md">
      <CardHeader><CardTitle>404</CardTitle></CardHeader>
      <CardContent className="text-muted-foreground">页面不存在</CardContent>
    </Card>
  )
}
