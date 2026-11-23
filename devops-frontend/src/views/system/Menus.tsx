import { useEffect, useState } from 'react'
import { adminSystem, type MenuWrite } from '@/api/admin/system'
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
import type { SystemMenu } from '@/lib/types'

interface MenuForm {
  name: string
  permission: string
  type: number
  sort: number
  parent_id: string
  path: string
  icon: string
  component: string
  component_name: string
  status: number
  visible: boolean
  keep_alive: boolean
  always_show: boolean
}

const emptyForm: MenuForm = {
  name: '', permission: '', type: 1, sort: 1, parent_id: '0', path: '', icon: '',
  component: '', component_name: '', status: 1, visible: true, keep_alive: false, always_show: true,
}

function flatten(n: SystemMenu, depth = 0): Array<{ node: SystemMenu; depth: number }> {
  const out: Array<{ node: SystemMenu; depth: number }> = [{ node: n, depth }]
  for (const c of n.children ?? []) out.push(...flatten(c, depth + 1))
  return out
}

function staticState(rows: SystemMenu[], loading: boolean): ListPageState<SystemMenu> {
  return {
    query: {}, rows, total: rows.length, page: 1, pageSize: rows.length,
    loading, exporting: false, selected: [],
    setSelected: () => {}, setQuery: () => {}, load: () => Promise.resolve(),
    search: () => Promise.resolve(), reset: () => Promise.resolve(),
    setPage: () => Promise.resolve(), setPageSize: () => Promise.resolve(),
    exportData: () => Promise.resolve(), afterMutate: () => Promise.resolve(),
  }
}

export default function Menus() {
  const confirm = useConfirm()
  const [rows, setRows] = useState<Array<{ node: SystemMenu; depth: number }>>([])
  const [loading, setLoading] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemMenu | null>(null)
  const [form, setForm] = useState<MenuForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const load = async () => {
    setLoading(true)
    try {
      const tree = await adminSystem.listMenus()
      setRows(tree.flatMap((n) => flatten(n)))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { void load() }, [])

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (form.type === 3 && !form.permission.trim()) {
      setFormError('按钮类型必须填写权限标识')
      return
    }
    setBusy(true)
    try {
      const payload: MenuWrite = {
        name: form.name.trim(),
        type: form.type,
        sort: form.sort,
        parent_id: form.parent_id,
        status: form.status,
        visible: form.visible,
        keep_alive: form.keep_alive,
        always_show: form.always_show,
      }
      if (form.permission.trim()) payload.permission = form.permission.trim()
      if (form.path.trim()) payload.path = form.path.trim()
      if (form.icon.trim()) payload.icon = form.icon.trim()
      if (form.component.trim()) payload.component = form.component.trim()
      if (form.component_name.trim()) payload.component_name = form.component_name.trim()
      if (editing) await adminSystem.updateMenu(editing.id, payload)
      else await adminSystem.createMenu(payload)
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await load()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const doDelete = async (n: SystemMenu) => {
    const ok = await confirm({ title: `确认删除菜单「${n.name}」？存在子级，无法删除`, danger: true, confirmText: '确定' })
    if (!ok) return
    try {
      await adminSystem.deleteMenu(n.id)
      toastSuccess('删除成功')
      await load()
    } catch (err) {
      toastError(err instanceof Error ? err.message : '删除失败')
    }
  }

  const columns: ListColumn<SystemMenu>[] = [
    {
      key: 'name', title: '名称',
      render: (n) => {
        const depth = rows.find((r) => r.node.id === n.id)?.depth ?? 0
        return (
          <span style={{ paddingLeft: depth * 16 }}>
            {depth > 0 && '└ '}
            <span>{n.name}</span>
            <Badge variant="outline" className="ml-2">{n.type === 1 ? '目录' : n.type === 2 ? '菜单' : '按钮'}</Badge>
          </span>
        )
      },
    },
    { key: 'permission', title: '权限标识', render: (n) => n.permission ?? '-' },
    { key: 'path', title: '路径', render: (n) => n.path || '-' },
    {
      key: 'op', title: '操作',
      render: (n) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:menu:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(n)
              setForm({
                name: n.name, permission: n.permission ?? '', type: n.type, sort: n.sort,
                parent_id: n.parentId, path: n.path ?? '', icon: n.icon ?? '',
                component: n.component ?? '', component_name: n.componentName ?? '',
                status: n.status, visible: n.visible, keep_alive: n.keepAlive ?? false,
                always_show: n.alwaysShow ?? true,
              })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton
            permission="system:menu:create" variant="ghost" size="sm"
            onClick={() => { setEditing(null); setForm({ ...emptyForm, parent_id: n.id }); setFormError(null); setFormOpen(true) }}
          >新增子项</PermissionButton>
          <PermissionButton permission="system:menu:delete" variant="ghost" size="sm" onClick={() => void doDelete(n)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <ListPage
        state={staticState(rows.map((r) => r.node), loading)}
        columns={columns}
        toolbar={
          <PermissionButton
            permission="system:menu:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormError(null); setFormOpen(true) }}
          >新增菜单</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑菜单' : '新增菜单'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field>
          <FieldLabel htmlFor="menu-type">类型</FieldLabel>
          <select
            id="menu-type"
            className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
            value={String(form.type)}
            onChange={(e) => setForm({ ...form, type: Number(e.target.value) })}
          >
            <option value="1">目录</option>
            <option value="2">菜单</option>
            <option value="3">按钮</option>
          </select>
        </Field>
        <Field><FieldLabel htmlFor="menu-name">名称</FieldLabel><Input id="menu-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></Field>
        <Field><FieldLabel htmlFor="menu-permission">权限标识</FieldLabel><Input id="menu-permission" value={form.permission} onChange={(e) => setForm({ ...form, permission: e.target.value })} /></Field>
        <Field><FieldLabel htmlFor="menu-sort">排序</FieldLabel><Input id="menu-sort" type="number" value={form.sort} onChange={(e) => setForm({ ...form, sort: Number(e.target.value) })} /></Field>
        <Field><FieldLabel htmlFor="menu-path">路径</FieldLabel><Input id="menu-path" value={form.path} onChange={(e) => setForm({ ...form, path: e.target.value })} /></Field>
        <Field><FieldLabel htmlFor="menu-icon">图标</FieldLabel><Input id="menu-icon" value={form.icon} onChange={(e) => setForm({ ...form, icon: e.target.value })} /></Field>
        {form.type === 2 && (
          <>
            <Field><FieldLabel htmlFor="menu-component">组件</FieldLabel><Input id="menu-component" value={form.component} onChange={(e) => setForm({ ...form, component: e.target.value })} /></Field>
            <Field><FieldLabel htmlFor="menu-component-name">组件名</FieldLabel><Input id="menu-component-name" value={form.component_name} onChange={(e) => setForm({ ...form, component_name: e.target.value })} /></Field>
          </>
        )}
        <div className="flex items-center justify-between"><FieldLabel htmlFor="menu-visible">可见</FieldLabel><Switch id="menu-visible" checked={form.visible} onCheckedChange={(v) => setForm({ ...form, visible: v })} /></div>
        <div className="flex items-center justify-between"><FieldLabel htmlFor="menu-keep-alive">缓存</FieldLabel><Switch id="menu-keep-alive" checked={form.keep_alive} onCheckedChange={(v) => setForm({ ...form, keep_alive: v })} /></div>
        <div className="flex items-center justify-between"><FieldLabel htmlFor="menu-always-show">常显</FieldLabel><Switch id="menu-always-show" checked={form.always_show} onCheckedChange={(v) => setForm({ ...form, always_show: v })} /></div>
      </FormDialog>
    </div>
  )
}
