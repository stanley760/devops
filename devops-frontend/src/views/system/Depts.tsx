import { useEffect, useState } from 'react'
import { adminSystem, type DeptWrite } from '@/api/admin/system'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import type { ListPageState } from '@/lib/list'
import type { SystemDept } from '@/lib/types'

interface DeptForm { name: string; parent_id: string; sort: number; phone: string; email: string; status: number }
const emptyForm: DeptForm = { name: '', parent_id: '0', sort: 1, phone: '', email: '', status: 1 }

function flatten(depts: SystemDept[], parentId = '0', depth = 0): Array<{ node: SystemDept; depth: number }> {
  const out: Array<{ node: SystemDept; depth: number }> = []
  const children = depts.filter((x) => x.parentId === parentId && x.id !== parentId)
  for (const d of children) {
    out.push({ node: d, depth })
    out.push(...flatten(depts, d.id, depth + 1))
  }
  if (parentId === '0') {
    const knownIds = new Set(depts.map((d) => d.id))
    for (const d of depts) {
      if (children.includes(d)) continue
      if (!d.parentId || d.parentId === d.id || !knownIds.has(d.parentId)) {
        out.push({ node: d, depth })
      }
    }
  }
  return out
}

function staticState(rows: SystemDept[], loading: boolean): ListPageState<SystemDept> {
  return {
    query: {}, rows, total: rows.length, page: 1, pageSize: rows.length,
    loading, exporting: false, selected: [],
    setSelected: () => {}, setQuery: () => {}, load: () => Promise.resolve(),
    search: () => Promise.resolve(), reset: () => Promise.resolve(),
    setPage: () => Promise.resolve(), setPageSize: () => Promise.resolve(),
    exportData: () => Promise.resolve(), afterMutate: () => Promise.resolve(),
  }
}

export default function Depts() {
  const confirm = useConfirm()
  const [all, setAll] = useState<SystemDept[]>([])
  const [loading, setLoading] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemDept | null>(null)
  const [form, setForm] = useState<DeptForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      setAll(await adminSystem.listDepts())
    } finally {
      setLoading(false)
    }
  }
  useEffect(() => { void load() }, [])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.name.trim()) {
      setFormError('名称必填')
      return
    }
    setBusy(true)
    try {
      const payload: DeptWrite = {
        name: form.name.trim(),
        parent_id: form.parent_id,
        sort: form.sort,
        status: form.status,
      }
      if (form.phone.trim()) payload.phone = form.phone.trim()
      if (form.email.trim()) payload.email = form.email.trim()
      if (editing) await adminSystem.updateDept(editing.id, payload)
      else await adminSystem.createDept(payload)
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await load()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const doDelete = async (d: SystemDept) => {
    const ok = await confirm({ title: `确认删除部门「${d.name}」？`, danger: true, confirmText: '确定' })
    if (!ok) return
    try {
      await adminSystem.deleteDept(d.id)
      toastSuccess('删除成功')
      await load()
    } catch (err) {
      toastError(err instanceof Error ? err.message : '删除失败')
    }
  }

  const flat = flatten(all)
  const columns: ListColumn<SystemDept>[] = [
    {
      key: 'name', title: '名称',
      render: (d) => {
        const depth = flat.find((r) => r.node.id === d.id)?.depth ?? 0
        return <span style={{ paddingLeft: depth * 16 }}>{depth > 0 && '└ '}<span>{d.name}</span></span>
      },
    },
    { key: 'phone', title: '电话', render: (d) => d.phone ?? '-' },
    {
      key: 'status', title: '状态',
      render: (d) => <Badge variant={d.status === 1 ? 'default' : 'secondary'}>{d.status === 1 ? '正常' : '停用'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (d) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:dept:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(d)
              setForm({ name: d.name, parent_id: d.parentId, sort: d.sort, phone: d.phone ?? '', email: d.email ?? '', status: d.status })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton
            permission="system:dept:create" variant="ghost" size="sm"
            onClick={() => { setEditing(null); setForm({ ...emptyForm, parent_id: d.id }); setFormError(null); setFormOpen(true) }}
          >新增子项</PermissionButton>
          <PermissionButton permission="system:dept:delete" variant="ghost" size="sm" onClick={() => void doDelete(d)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <ListPage
        state={staticState(flat.map((r) => r.node), loading)}
        columns={columns}
        toolbar={
          <PermissionButton
            permission="system:dept:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormError(null); setFormOpen(true) }}
          >新增部门</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑部门' : '新增部门'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field><FieldLabel htmlFor="dept-name">名称</FieldLabel><Input id="dept-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></Field>
        <Field><FieldLabel htmlFor="dept-phone">电话</FieldLabel><Input id="dept-phone" value={form.phone} onChange={(e) => setForm({ ...form, phone: e.target.value })} /></Field>
        <Field><FieldLabel htmlFor="dept-email">邮箱</FieldLabel><Input id="dept-email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} /></Field>
        <Field><FieldLabel htmlFor="dept-sort">排序</FieldLabel><Input id="dept-sort" type="number" value={form.sort} onChange={(e) => setForm({ ...form, sort: Number(e.target.value) })} /></Field>
        <div className="flex items-center justify-between">
          <FieldLabel htmlFor="dept-status">状态</FieldLabel>
          <Switch id="dept-status" checked={form.status === 1} onCheckedChange={(v) => setForm({ ...form, status: v ? 1 : 0 })} />
        </div>
      </FormDialog>
    </div>
  )
}
