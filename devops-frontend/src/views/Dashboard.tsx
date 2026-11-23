import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useSession } from '@/store/session'
import { usePermissions } from '@/lib/permission'
import { formatDateTime } from '@/lib/datetime'
import { flattenMenus } from '@/router/routes'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'
import { Skeleton } from '@/components/ui/skeleton'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import type { LoginLog, OperationLog } from '@/lib/types'

export default function Dashboard() {
  const { authInfo, routes } = useSession()
  const { has } = usePermissions()
  const canLoginLog = has('system:loginlog:list')
  const canOperationLog = has('system:operationlog:list')
  const [logs, setLogs] = useState<LoginLog[]>([])
  const [opLogs, setOpLogs] = useState<OperationLog[]>([])
  const [logsLoading, setLogsLoading] = useState(true)
  const [opLogsLoading, setOpLogsLoading] = useState(true)

  useEffect(() => {
    if (!canLoginLog) return
    let alive = true
    adminSystem
      .listLoginLogs({ page_num: 1, page_size: 10 })
      .then((r) => {
        if (alive) setLogs(r.list)
      })
      .catch(() => {
        if (alive) setLogs([])
      })
      .finally(() => {
        if (alive) setLogsLoading(false)
      })
    return () => {
      alive = false
    }
  }, [canLoginLog])

  useEffect(() => {
    if (!canOperationLog) return
    let alive = true
    adminSystem
      .listOperationLogs({ page_num: 1, page_size: 10 })
      .then((r) => {
        if (alive) setOpLogs(r.list)
      })
      .catch(() => {
        if (alive) setOpLogs([])
      })
      .finally(() => {
        if (alive) setOpLogsLoading(false)
      })
    return () => {
      alive = false
    }
  }, [canOperationLog])

  if (!authInfo) return <Skeleton className="h-64 w-full" />

  const menuCount = flattenMenus(routes).length

  return (
    <div className="space-y-4">
      <div className="grid gap-4 md:grid-cols-3">
        <Card className="glass-panel">
          <CardHeader><CardTitle>账号</CardTitle></CardHeader>
          <CardContent className="space-y-1 text-sm">
            <div>{authInfo.user.nickname || authInfo.user.username}</div>
            <div className="text-muted-foreground">@{authInfo.user.username}</div>
            <div className="flex gap-1 pt-1">
              {authInfo.roles.map((r) => <Badge key={r} variant="secondary">{r}</Badge>)}
            </div>
          </CardContent>
        </Card>
        <Card className="glass-panel">
          <CardHeader><CardTitle>租户</CardTitle></CardHeader>
          <CardContent className="text-sm">
            <div>{authInfo.tenant.name}</div>
            <div className="text-muted-foreground">{authInfo.tenant.code}</div>
          </CardContent>
        </Card>
        <Card className="glass-panel">
          <CardHeader><CardTitle>菜单</CardTitle></CardHeader>
          <CardContent className="text-sm">
            共 <span className="font-semibold">{menuCount}</span> 个可见菜单节点
          </CardContent>
        </Card>
      </div>
      <Separator />
      {canLoginLog && (
        <Card className="glass-panel">
          <CardHeader><CardTitle>最近登录日志</CardTitle></CardHeader>
          <CardContent>
            {logsLoading ? (
              <Skeleton className="h-32 w-full" />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>账号</TableHead>
                    <TableHead>IP</TableHead>
                    <TableHead>状态</TableHead>
                    <TableHead>时间</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {logs.map((l) => (
                    <TableRow key={l.id}>
                      <TableCell>{l.username}</TableCell>
                      <TableCell>{l.ip}</TableCell>
                      <TableCell>
                        <Badge variant={l.result === 0 ? 'default' : 'destructive'}>
                          {l.result === 0 ? '成功' : '失败'}
                        </Badge>
                      </TableCell>
                      <TableCell>{formatDateTime(l.createTime)}</TableCell>
                    </TableRow>
                  ))}
                  {logs.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={4} className="text-muted-foreground">暂无日志</TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      )}
      {canOperationLog && (
        <Card className="glass-panel">
          <CardHeader><CardTitle>最近操作日志</CardTitle></CardHeader>
          <CardContent>
            {opLogsLoading ? (
              <Skeleton className="h-32 w-full" />
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>账号</TableHead>
                    <TableHead>模块</TableHead>
                    <TableHead>动作</TableHead>
                    <TableHead>IP</TableHead>
                    <TableHead>时间</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {opLogs.map((l) => (
                    <TableRow key={l.id}>
                      <TableCell>{l.userName}</TableCell>
                      <TableCell>{l.module}</TableCell>
                      <TableCell>{l.contents}</TableCell>
                      <TableCell>{l.userIp}</TableCell>
                      <TableCell>{formatDateTime(l.createTime)}</TableCell>
                    </TableRow>
                  ))}
                  {opLogs.length === 0 && (
                    <TableRow>
                      <TableCell colSpan={5} className="text-muted-foreground">暂无日志</TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}
