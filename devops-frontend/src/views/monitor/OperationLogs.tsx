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

const RESULT_OPTIONS = [
  { label: '成功', value: '0' },
  { label: '失败', value: '1' },
]

function durationMs(extra: string): number | null {
  try {
    const parsed = JSON.parse(extra) as { duration_ms?: number }
    return typeof parsed.duration_ms === 'number' ? parsed.duration_ms : null
  } catch {
    return null
  }
}

export default function OperationLogs() {
  const confirm = useConfirm()
  const list = useListPage<OperationLog>({ fetcher: (p) => adminSystem.listOperationLogs(p) })
  const [detail, setDetail] = useState<OperationLog | null>(null)

  useEffect(() => { void list.load() }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const doClean = async () => {
    const ok = await confirm({ title: '确认清空全部操作日志？', description: '此操作不可恢复', danger: true, confirmText: '清空' })
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
    { key: 'userName', title: '操作人' },
    { key: 'module', title: '模块' },
    { key: 'subModule', title: '子模块' },
    { key: 'requestMethod', title: '方法' },
    { key: 'requestUrl', title: 'URI', className: 'max-w-48 truncate' },
    { key: 'userIp', title: 'IP' },
    {
      key: 'result', title: '执行结果',
      render: (r) => <Badge variant={r.result === 0 ? 'default' : 'destructive'}>{r.result === 0 ? '成功' : '失败'}</Badge>,
    },
    { key: 'code', title: '状态码' },
    { key: 'createTime', title: '时间', render: (r) => formatDateTime(r.createTime) },
    {
      key: 'op', title: '操作',
      render: (r) => <Button variant="ghost" size="sm" onClick={() => setDetail(r)}>详情</Button>,
    },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[
          { key: 'user_name', label: '操作人' },
          { key: 'module', label: '模块' },
          { key: 'user_ip', label: 'IP' },
          { key: 'result', label: '执行结果', type: 'select', options: RESULT_OPTIONS },
        ]}
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
              <div><span className="text-muted-foreground">摘要：</span>{detail.contents || '-'}</div>
              <div><span className="text-muted-foreground">模块：</span>{[detail.module, detail.subModule].filter(Boolean).join(' / ') || '-'}</div>
              <div><span className="text-muted-foreground">方法：</span>{detail.requestMethod || '-'}</div>
              <div><span className="text-muted-foreground">路径：</span>{detail.requestUrl || '-'}</div>
              <div><span className="text-muted-foreground">操作人：</span>{detail.userName || '-'}（{detail.userTypeName || '-'}）</div>
              <div><span className="text-muted-foreground">IP：</span>{detail.userIp || '-'}</div>
              <div>
                <span className="text-muted-foreground">执行结果：</span>
                <Badge variant={detail.result === 0 ? 'default' : 'destructive'}>
                  {detail.result === 0 ? '成功' : detail.result === 1 ? '失败' : '-'}
                </Badge>
              </div>
              <div><span className="text-muted-foreground">状态码：</span>{detail.code || '-'}</div>
              <div><span className="text-muted-foreground">错误码：</span>{detail.result === 1 ? detail.code || '-' : '-'}</div>
              <div><span className="text-muted-foreground">耗时：</span>{durationMs(detail.extra) != null ? `${durationMs(detail.extra)} ms` : '-'}</div>
              <div><span className="text-muted-foreground">操作时间：</span>{formatDateTime(detail.createTime)}</div>
              <div>
                <div className="text-muted-foreground">附加信息：</div>
                <pre className="bg-muted mt-1 overflow-auto rounded p-2 text-xs">{detail.extra || '-'}</pre>
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </div>
  )
}
