import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

// #/docs 静态文档页（spec §5.3 明确的静态页，有确定内容）
export function Docs() {
  return (
    <Card className="glass-panel mx-auto my-10 max-w-2xl">
      <CardHeader><CardTitle>产品文档</CardTitle></CardHeader>
      <CardContent className="space-y-3 text-sm">
        <p>Design World 平台的产品说明与使用指引将在此发布。</p>
        <p>当前内容：平台简介、账号注册与登录指引、个人资料管理说明。</p>
      </CardContent>
    </Card>
  )
}
