import { useEffect, useState, type ReactNode } from 'react'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { ListPageState } from '@/lib/list'

export interface ListColumn<T> {
  key: string
  title: string
  render?: (row: T) => ReactNode
  className?: string
}

interface ListPageProps<T> {
  state: ListPageState<T>
  columns: ListColumn<T>[]
  selectable?: boolean
  toolbar?: ReactNode
  batchActions?: ReactNode
  emptyText?: string
}

export function ListPage<T extends { id: string }>({
  state, columns, selectable = false, toolbar, batchActions, emptyText = '暂无数据',
}: ListPageProps<T>) {
  const [selected, setSelectedLocal] = useState<string[]>(state.selected)
  useEffect(() => setSelectedLocal(state.selected), [state.selected])

  const setSelected = (ids: string[]) => {
    setSelectedLocal(ids)
    state.setSelected(ids)
  }
  const allSelected = selectable && state.rows.length > 0 && state.rows.every((r) => selected.includes(r.id))
  const toggleAll = () => {
    setSelected(allSelected ? [] : state.rows.map((r) => r.id))
  }
  const toggleOne = (id: string) => {
    setSelected(
      selected.includes(id)
        ? selected.filter((s) => s !== id)
        : [...selected, id],
    )
  }

  return (
    <div className="space-y-3">
      {toolbar}
      {selectable && selected.length > 0 && (
        <div className="flex items-center gap-2 rounded-md p-2 text-sm">
          已选 {selected.length} 项
          {batchActions}
        </div>
      )}
      <div className="overflow-x-auto rounded-md">
        <Table>
          <TableHeader>
            <TableRow>
              {selectable && (
                <TableHead className="w-10">
                  <Checkbox checked={allSelected} onCheckedChange={toggleAll} aria-label="全选" />
                </TableHead>
              )}
              {columns.map((c) => <TableHead key={c.key} className={c.className}>{c.title}</TableHead>)}
            </TableRow>
          </TableHeader>
          <TableBody>
            {state.rows.map((row) => (
              <TableRow key={row.id}>
                {selectable && (
                  <TableCell>
                    <Checkbox
                      checked={selected.includes(row.id)}
                      onCheckedChange={() => toggleOne(row.id)}
                      aria-label={`选中 ${row.id}`}
                    />
                  </TableCell>
                )}
                {columns.map((c) => (
                  <TableCell key={c.key} className={c.className}>
                    {c.render ? c.render(row) : String((row as Record<string, unknown>)[c.key] ?? '-')}
                  </TableCell>
                ))}
              </TableRow>
            ))}
            {state.rows.length === 0 && !state.loading && (
              <TableRow>
                <TableCell colSpan={columns.length + (selectable ? 1 : 0)} className="text-muted-foreground">
                  {emptyText}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>
      <div className="flex items-center justify-between">
        <span className="text-muted-foreground text-sm">共 {state.total} 条</span>
        <div className="flex items-center gap-2">
          <Select value={String(state.pageSize)} onValueChange={(v) => void state.setPageSize(Number(v))}>
            <SelectTrigger className="w-28"><SelectValue /></SelectTrigger>
            <SelectContent>
              {[10, 20, 50, 100].map((s) => <SelectItem key={s} value={String(s)}>{s} 条/页</SelectItem>)}
            </SelectContent>
          </Select>
          <Button
            variant="outline" size="sm"
            disabled={state.page <= 1 || state.loading}
            onClick={() => void state.setPage(state.page - 1)}
          >
            上一页
          </Button>
          <span className="text-sm">{state.page}</span>
          <Button
            variant="outline" size="sm"
            disabled={state.loading || state.page * state.pageSize >= state.total}
            onClick={() => void state.setPage(state.page + 1)}
          >
            下一页
          </Button>
        </div>
      </div>
    </div>
  )
}
