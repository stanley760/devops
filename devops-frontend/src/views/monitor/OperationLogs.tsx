import { useEffect, useState } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastSuccess } from '@/lib/toast'
import { formatDateTime } from '@/lib/datetime'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import type { OperationLog } from '@/lib/types'

export default function OperationLogs() {
  const confirm = useConfirm()
  const list = useListPage<OperationLog>({ fetcher: (p) => adminSystem.listOperationLogs(p) })
  const [detail, setDetail] = useState<OperationLog | null>(null)

  useEffect(() => { void list.load() }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const doClean = async () => {
    const ok = await confirm({ title: '确认清空全部操作日志？', description: '此操作不可恢复', danger: true, confirmText: '确定' })
    if (!ok) return
    await adminSystem.cleanOperationLogs()
    toastSuccess('已清空')
    await list.afterMutate()
  }

  const doBatchRemove = async () => {
    const ok = await confirm({ title: `确认删除选中的 ${list.selected.length} 条日志？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.removeOperationLogs(list.selected)
    toastSuccess('删除成功')
    await list.afterMutate()
  }

  const columns: ListColumn<OperationLog>[] = [
    { key: 'username', title: '账号' },
    { key: 'module', title: '模块' },
    { key: 'action', title: '动作' },
    { key: 'method', title: '方法' },
    { key: 'uri', title: 'URI', className: 'max-w-48 truncate' },
    { key: 'statusCode', title: '状态码' },
    { key: 'createdAt', title: '时间', render: (r) => formatDateTime(r.createdAt) },
    {
      key: 'op', title: '操作',
      render: (r) => <Button variant="ghost" size="sm" onClick={() => setDetail(r)}>详情</Button>,
    },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'username', label: '账号' }, { key: 'module', label: '模块' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns} selectable
        toolbar={
          <div className="flex gap-2">
            <PermissionButton permission="system:operationlog:clean" variant="destructive" onClick={() => void doClean()}>清空</PermissionButton>
            <PermissionButton permission="system:operationlog:remove" variant="destructive" onClick={() => void doBatchRemove()}>批量删除</PermissionButton>
          </div>
        }
        batchActions={
          <PermissionButton permission="system:operationlog:remove" size="sm" variant="destructive" onClick={() => void doBatchRemove()}>
            删除选中
          </PermissionButton>
        }
      />
      <Sheet open={detail != null} onOpenChange={(o) => { if (!o) setDetail(null) }}>
        <SheetContent className="w-96 overflow-auto">
          <SheetHeader><SheetTitle>操作日志详情</SheetTitle></SheetHeader>
          {detail && (
            <div className="space-y-2 text-sm">
              <div><span className="text-muted-foreground">标题：</span>{detail.title ?? '-'}</div>
              <div><span className="text-muted-foreground">动作：</span>{detail.action || '-'}</div>
              <div><span className="text-muted-foreground">方法：</span>{detail.method || '-'}</div>
              <div><span className="text-muted-foreground">操作人：</span>{detail.operatorName ?? detail.username ?? '-'}</div>
              <div><span className="text-muted-foreground">IP：</span>{detail.ip || '-'}</div>
              <div>
                <span className="text-muted-foreground">状态：</span>
                <Badge variant={detail.status === 0 || detail.statusCode === 200 ? 'default' : 'destructive'}>
                  {detail.status === 0 ? '成功' : detail.status === 1 ? '失败' : detail.statusCode === 200 ? '成功' : detail.statusCode === undefined ? '-' : '失败'}
                </Badge>
              </div>
              <div><span className="text-muted-foreground">耗时：</span>{detail.costMs != null ? `${detail.costMs} ms` : '-'}</div>
              <div><span className="text-muted-foreground">操作时间：</span>{formatDateTime(detail.operTime ?? detail.createdAt)}</div>
              <div><span className="text-muted-foreground">错误信息：</span>{detail.errorMsg ?? '-'}</div>
              <div>
                <div className="text-muted-foreground">请求体：</div>
                <pre className="bg-muted mt-1 overflow-auto rounded p-2 text-xs">{detail.request ?? '-'}</pre>
              </div>
              <div>
                <div className="text-muted-foreground">响应体：</div>
                <pre className="bg-muted mt-1 overflow-auto rounded p-2 text-xs">{detail.response ?? '-'}</pre>
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  )
}
