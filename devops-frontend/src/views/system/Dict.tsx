import { useEffect, useState } from 'react'
import { adminSystem, type DictItemWrite, type DictTypeWrite } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import type { ListPageState } from '@/lib/list'
import type { DictItem, DictType } from '@/lib/types'

interface TypeForm { name: string; type: string; status: number; remark: string }
interface ItemForm { label: string; value: string; sort: number }

function staticState(rows: DictItem[], loading: boolean): ListPageState<DictItem> {
  return {
    query: {}, rows, total: rows.length, page: 1, pageSize: rows.length,
    loading, exporting: false, selected: [],
    setSelected: () => {}, setQuery: () => {}, load: () => Promise.resolve(),
    search: () => Promise.resolve(), reset: () => Promise.resolve(),
    setPage: () => Promise.resolve(), setPageSize: () => Promise.resolve(),
    exportData: () => Promise.resolve(), afterMutate: () => Promise.resolve(),
  }
}

export default function Dict() {
  const confirm = useConfirm()
  const typeList = useListPage<DictType>({ fetcher: (p) => adminSystem.listDictTypes(p) })
  const [selected, setSelected] = useState<DictType | null>(null)
  const [items, setItems] = useState<DictItem[]>([])
  const [itemsLoading, setItemsLoading] = useState(false)
  const [typeFormOpen, setTypeFormOpen] = useState(false)
  const [editingType, setEditingType] = useState<DictType | null>(null)
  const [typeForm, setTypeForm] = useState<TypeForm>({ name: '', type: '', status: 1, remark: '' })
  const [itemFormOpen, setItemFormOpen] = useState(false)
  const [editingItem, setEditingItem] = useState<DictItem | null>(null)
  const [itemForm, setItemForm] = useState<ItemForm>({ label: '', value: '', sort: 1 })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void typeList.load().catch((err: unknown) => {
      toastError(err instanceof Error ? err.message : '加载字典类型失败')
    })
  }, [typeList.page, typeList.pageSize, typeList.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const loadItems = async (t: DictType) => {
    setSelected(t)
    setItemsLoading(true)
    try {
      setItems(await adminSystem.listDictItems(t.type))
    } catch (err) {
      toastError(err instanceof Error ? err.message : '加载数据项失败')
    } finally {
      setItemsLoading(false)
    }
  }

  const submitType = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!typeForm.name || !typeForm.type) {
      setFormError('类型名称与编码必填')
      return
    }
    setBusy(true)
    try {
      const payload: DictTypeWrite = {
        name: typeForm.name, type: typeForm.type, status: typeForm.status,
      }
      if (typeForm.remark) payload.remark = typeForm.remark
      if (editingType) await adminSystem.updateDictType(editingType.id, payload)
      else await adminSystem.createDictType(payload)
      toastSuccess(editingType ? '更新成功' : '创建成功')
      setTypeFormOpen(false)
      await typeList.afterMutate()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const submitItem = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!selected) return
    if (!itemForm.label || !itemForm.value) {
      setFormError('标签与值必填')
      return
    }
    setBusy(true)
    try {
      const payload: DictItemWrite = {
        dict_type: selected.type, label: itemForm.label, value: itemForm.value, sort: itemForm.sort,
      }
      if (editingItem) await adminSystem.updateDictItem(editingItem.id, payload)
      else await adminSystem.createDictItem(payload)
      toastSuccess(editingItem ? '更新成功' : '创建成功')
      setItemFormOpen(false)
      await loadItems(selected)
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const doDeleteType = async (t: DictType) => {
    const ok = await confirm({ title: `确认删除字典类型「${t.name}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    try {
      await adminSystem.deleteDictType(t.id)
      toastSuccess('删除成功')
      if (selected?.id === t.id) {
        setSelected(null)
        setItems([])
      }
      await typeList.afterMutate()
    } catch (err) {
      toastError(err instanceof Error ? err.message : '删除失败')
    }
  }

  const doDeleteItem = async (i: DictItem) => {
    const ok = await confirm({ title: `确认删除数据项「${i.label}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    try {
      await adminSystem.deleteDictItem(i.id)
      toastSuccess('删除成功')
      if (selected) await loadItems(selected)
    } catch (err) {
      toastError(err instanceof Error ? err.message : '删除失败')
    }
  }

  const typeColumns: ListColumn<DictType>[] = [
    {
      key: 'name', title: '名称',
      render: (t) => (
        <button
          type="button"
          className={`p-0 text-left hover:underline ${selected?.id === t.id ? 'font-medium text-primary underline' : ''}`}
          onClick={() => void loadItems(t)}
        >
          {t.name}
        </button>
      ),
    },
    { key: 'type', title: '编码' },
    {
      key: 'status', title: '状态',
      render: (t) => <Badge variant={t.status === 1 ? 'default' : 'secondary'}>{t.status === 1 ? '正常' : '停用'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (t) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:dict:update" variant="ghost" size="sm"
            onClick={() => {
              setEditingType(t)
              setTypeForm({ name: t.name, type: t.type, status: t.status, remark: t.remark ?? '' })
              setFormError(null)
              setTypeFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton permission="system:dict:delete" variant="ghost" size="sm" onClick={() => void doDeleteType(t)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  const itemColumns: ListColumn<DictItem>[] = [
    { key: 'label', title: '标签' },
    { key: 'value', title: '值' },
    { key: 'sort', title: '排序' },
    {
      key: 'op', title: '操作',
      render: (i) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:dict:update" variant="ghost" size="sm"
            onClick={() => {
              setEditingItem(i)
              setItemForm({ label: i.label, value: i.value, sort: i.sort })
              setFormError(null)
              setItemFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton permission="system:dict:delete" variant="ghost" size="sm" onClick={() => void doDeleteItem(i)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader><CardTitle>字典类型</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          <SearchForm
            fields={[{ key: 'name', label: '名称' }]}
            value={typeList.query} onSearch={() => void typeList.search()} onReset={() => void typeList.reset()}
          />
          <ListPage
            state={typeList}
            columns={typeColumns}
            toolbar={
              <PermissionButton
                permission="system:dict:create"
                onClick={() => { setEditingType(null); setTypeForm({ name: '', type: '', status: 1, remark: '' }); setFormError(null); setTypeFormOpen(true) }}
              >新增类型</PermissionButton>
            }
          />
        </CardContent>
      </Card>
      <Card>
        <CardHeader><CardTitle>数据项{selected ? `（${selected.name}）` : ''}</CardTitle></CardHeader>
        <CardContent className="space-y-3">
          {!selected && <p className="text-muted-foreground text-sm">请先在左侧选择一个字典类型</p>}
          {selected && (
            <ListPage
              state={staticState(items, itemsLoading)}
              columns={itemColumns}
              toolbar={
                <PermissionButton
                  permission="system:dict:create"
                  onClick={() => { setEditingItem(null); setItemForm({ label: '', value: '', sort: 1 }); setFormError(null); setItemFormOpen(true) }}
                >新增数据项</PermissionButton>
              }
            />
          )}
        </CardContent>
      </Card>
      <FormDialog open={typeFormOpen} onOpenChange={setTypeFormOpen} title={editingType ? '编辑字典类型' : '新增字典类型'} onSubmit={submitType} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field><FieldLabel htmlFor="dict-type-name">类型名称</FieldLabel><Input id="dict-type-name" value={typeForm.name} onChange={(e) => setTypeForm({ ...typeForm, name: e.target.value })} required /></Field>
        <Field><FieldLabel htmlFor="dict-type-code">类型编码</FieldLabel><Input id="dict-type-code" value={typeForm.type} onChange={(e) => setTypeForm({ ...typeForm, type: e.target.value })} disabled={!!editingType} required /></Field>
        <div className="flex items-center justify-between">
          <FieldLabel htmlFor="dict-type-status">状态</FieldLabel>
          <Switch id="dict-type-status" checked={typeForm.status === 1} onCheckedChange={(v) => setTypeForm({ ...typeForm, status: v ? 1 : 0 })} />
        </div>
        <Field><FieldLabel htmlFor="dict-type-remark">备注</FieldLabel><Input id="dict-type-remark" value={typeForm.remark} onChange={(e) => setTypeForm({ ...typeForm, remark: e.target.value })} /></Field>
      </FormDialog>
      <FormDialog open={itemFormOpen} onOpenChange={setItemFormOpen} title={editingItem ? '编辑数据项' : '新增数据项'} onSubmit={submitItem} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field><FieldLabel htmlFor="dict-item-label">标签</FieldLabel><Input id="dict-item-label" value={itemForm.label} onChange={(e) => setItemForm({ ...itemForm, label: e.target.value })} required /></Field>
        <Field><FieldLabel htmlFor="dict-item-value">值</FieldLabel><Input id="dict-item-value" value={itemForm.value} onChange={(e) => setItemForm({ ...itemForm, value: e.target.value })} required /></Field>
        <Field><FieldLabel htmlFor="dict-item-sort">排序</FieldLabel><Input id="dict-item-sort" type="number" value={itemForm.sort} onChange={(e) => setItemForm({ ...itemForm, sort: Number(e.target.value) })} /></Field>
      </FormDialog>
    </div>
  )
}