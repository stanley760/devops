import { useEffect } from 'react'
import { adminSystem } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastSuccess } from '@/lib/toast'
import { formatDateTime } from '@/lib/datetime'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import type { LoginLog } from '@/lib/types'

export default function LoginLogs() {
  const confirm = useConfirm()
  const list = useListPage<LoginLog>({ fetcher: (p) => adminSystem.listLoginLogs(p) })

  useEffect(() => { void list.load() }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const doClean = async () => {
    const ok = await confirm({ title: '确认清空全部登录日志？', description: '此操作不可恢复', danger: true, confirmText: '清空' })
    if (!ok) return
    await adminSystem.cleanLoginLogs()
    toastSuccess('已清空')
    await list.afterMutate()
  }

  const doBatchRemove = async () => {
    const ok = await confirm({ title: `确认删除选中的 ${list.selected.length} 条日志？`, danger: true, confirmText: '删除' })
    if (!ok) return
    await adminSystem.removeLoginLogs(list.selected)
    toastSuccess('删除成功')
    await list.afterMutate()
  }

  const columns: ListColumn<LoginLog>[] = [
    { key: 'username', title: '账号' },
    { key: 'ip', title: 'IP' },
    {
      key: 'status', title: '状态',
      render: (r) => <Badge variant={r.status === 1 ? 'default' : 'destructive'}>{r.status === 1 ? '成功' : '失败'}</Badge>,
    },
    { key: 'msg', title: '消息' },
    { key: 'loginAt', title: '时间', render: (r) => formatDateTime(r.loginAt) },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'username', label: '账号' }, { key: 'ip', label: 'IP' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns} selectable
        toolbar={
          <div className="flex gap-2">
            <PermissionButton permission="system:loginlog:clean" variant="destructive" onClick={() => void doClean()}>清空</PermissionButton>
            <PermissionButton permission="system:loginlog:remove" variant="destructive" onClick={() => void doBatchRemove()}>批量删除</PermissionButton>
          </div>
        }
        batchActions={
          <PermissionButton permission="system:loginlog:remove" size="sm" variant="destructive" onClick={() => void doBatchRemove()}>
            删除选中
          </PermissionButton>
        }
      />
    </div>
  )
}
