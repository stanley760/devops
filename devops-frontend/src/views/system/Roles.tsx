import { useEffect, useState } from 'react'
import { adminSystem, encodeDeptIDsJson, parseDeptIDsJson, type RoleWrite } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { usePermissions } from '@/lib/permission'
import { ApiError } from '@/lib/http'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import NoAccess from '@/views/NoAccess'
import type { SystemDept, SystemMenu, SystemRole } from '@/lib/types'

interface RoleForm {
  name: string
  code: string
  sort: number
  data_scope: number
  data_scope_dept_ids: string[]
  status: number
  remark: string
}

const emptyForm: RoleForm = {
  name: '', code: '', sort: 1, data_scope: 1, data_scope_dept_ids: [], status: 1, remark: '',
}

const dataScopeOptions = [
  { value: 1, label: '全部数据' },
  { value: 2, label: '自定义部门' },
  { value: 3, label: '本部门' },
  { value: 4, label: '本部门及以下' },
  { value: 5, label: '仅本人' },
]

function isForbidden(err: unknown): boolean {
  return err instanceof ApiError && (err.code === 403 || err.status === 403)
}

function isBuiltin(role: SystemRole): boolean {
  return role.type === 1
}

/** Flatten the dept tree (backend returns DeptTreeResp) for the custom-scope checkboxes. */
function flattenDepts(nodes: SystemDept[], depth = 0, out: SystemDept[] = []): SystemDept[] {
  for (const n of nodes) {
    out.push({ ...n, name: `${'　'.repeat(depth)}${n.name}` })
    const children = (n as SystemDept & { children?: SystemDept[] }).children
    if (children?.length) flattenDepts(children, depth + 1, out)
  }
  return out
}

function MenuTreeCheckbox({
  nodes, selected, onToggle,
}: {
  nodes: SystemMenu[]
  selected: Set<string>
  onToggle: (id: string) => void
}) {
  return (
    <div className="space-y-1">
      {nodes.map((n) => (
        <div key={n.id}>
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={selected.has(n.id)}
              onChange={() => onToggle(n.id)}
              aria-label={`选菜单 ${n.name}`}
            />
            {n.name}
          </label>
          {n.children?.length ? (
            <div className="ml-4">
              <MenuTreeCheckbox nodes={n.children} selected={selected} onToggle={onToggle} />
            </div>
          ) : null}
        </div>
      ))}
    </div>
  )
}

export default function Roles() {
  const confirm = useConfirm()
  const { has } = usePermissions()
  const canList = has('system:role:list')
  const list = useListPage<SystemRole>({ fetcher: (p) => adminSystem.listRoles(p) })
  const [forbidden, setForbidden] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemRole | null>(null)
  const [form, setForm] = useState<RoleForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [assignOpen, setAssignOpen] = useState(false)
  const [assignRole, setAssignRole] = useState<SystemRole | null>(null)
  const [menus, setMenus] = useState<SystemMenu[]>([])
  const [selectedMenus, setSelectedMenus] = useState<Set<string>>(new Set())
  const [assignLoading, setAssignLoading] = useState(false)
  const [assignFailed, setAssignFailed] = useState(false)
  const [depts, setDepts] = useState<SystemDept[]>([])

  useEffect(() => {
    if (!canList) return
    void list.load().catch((err: unknown) => {
      if (isForbidden(err)) setForbidden(true)
      else toastError(err instanceof Error ? err.message : '加载角色失败')
    })
  }, [canList, list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    void adminSystem.listDepts().then((rows) => setDepts(flattenDepts(rows))).catch(() => setDepts([]))
  }, [])

  if (!canList || forbidden) return <NoAccess />

  const openCreate = () => {
    setEditing(null)
    setForm({ ...emptyForm })
    setFormError(null)
    setFormOpen(true)
  }

  const openEdit = (r: SystemRole) => {
    setEditing(r)
    setForm({
      name: r.name,
      code: r.code,
      sort: r.sort,
      data_scope: r.dataScope,
      data_scope_dept_ids: parseDeptIDsJson(r.dataScopeDeptIds),
      status: r.status,
      remark: r.remark ?? '',
    })
    setFormError(null)
    setFormOpen(true)
  }

  const openAssign = async (r: SystemRole) => {
    setAssignRole(r)
    setSelectedMenus(new Set())
    setMenus([])
    setAssignFailed(false)
    setAssignLoading(true)
    setAssignOpen(true)
    try {
      const [tree, detail] = await Promise.all([
        adminSystem.listMenuTree(),
        adminSystem.getRole(r.id),
      ])
      setMenus(tree)
      setSelectedMenus(new Set(detail.menuIds ?? []))
    } catch (err) {
      toastError(err instanceof Error ? err.message : '加载菜单失败')
      setAssignFailed(true)
      setAssignOpen(false)
    } finally {
      setAssignLoading(false)
    }
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.name.trim() || !form.code.trim()) {
      setFormError('名称与编码必填')
      return
    }
    if (form.data_scope === 2 && form.data_scope_dept_ids.length === 0) {
      setFormError('自定义数据权限必须选择部门')
      return
    }
    setBusy(true)
    try {
      const payload: RoleWrite = {
        name: form.name.trim(),
        code: form.code.trim(),
        sort: form.sort,
        data_scope: form.data_scope,
        data_scope_dept_ids: encodeDeptIDsJson(form.data_scope_dept_ids),
        status: form.status,
      }
      if (form.remark.trim()) payload.remark = form.remark.trim()
      if (editing) await adminSystem.updateRole(editing.id, payload)
      else await adminSystem.createRole(payload)
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await list.afterMutate()
    } catch (err) {
      if (isForbidden(err)) setForbidden(true)
      else setFormError(err instanceof Error ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const doAssign = async () => {
    if (!assignRole || assignLoading || assignFailed) return
    try {
      await adminSystem.assignRoleMenus(assignRole.id, [...selectedMenus])
      toastSuccess('菜单分配成功')
      setAssignOpen(false)
    } catch (err) {
      if (isForbidden(err)) setForbidden(true)
      else toastError(err instanceof Error ? err.message : '菜单分配失败')
    }
  }

  const doDelete = async (r: SystemRole) => {
    if (isBuiltin(r)) return // 内置角色不可删除（UI 已隐藏，函数内双保险）
    const ok = await confirm({ title: `确认删除角色「${r.name}」？`, danger: true, confirmText: '确定' })
    if (!ok) return
    try {
      await adminSystem.deleteRole(r.id)
      toastSuccess('删除成功')
      await list.afterMutate()
    } catch (err) {
      if (isForbidden(err)) setForbidden(true)
      else toastError(err instanceof Error ? err.message : '删除失败')
    }
  }

  const columns: ListColumn<SystemRole>[] = [
    { key: 'name', title: '名称' },
    { key: 'code', title: '编码' },
    { key: 'sort', title: '排序' },
    {
      key: 'status', title: '状态',
      render: (r) => <Badge variant={r.status === 1 ? 'default' : 'secondary'}>{r.status === 1 ? '正常' : '停用'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (r) => (
        <div className="flex gap-1">
          <PermissionButton permission="system:role:update" variant="ghost" size="sm" onClick={() => openEdit(r)}>编辑</PermissionButton>
          <PermissionButton permission="system:role:assign" variant="ghost" size="sm" onClick={() => void openAssign(r)}>分配菜单</PermissionButton>
          {!isBuiltin(r) && (
            <PermissionButton permission="system:role:delete" variant="ghost" size="sm" onClick={() => void doDelete(r)}>删除</PermissionButton>
          )}
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'name', label: '名称' }, { key: 'code', label: '编码' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns}
        toolbar={<PermissionButton permission="system:role:create" onClick={openCreate}>新增角色</PermissionButton>}
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑角色' : '新增角色'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field>
          <FieldLabel htmlFor="role-name">角色名称</FieldLabel>
          <Input id="role-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
        </Field>
        <Field>
          <FieldLabel htmlFor="role-code">角色编码</FieldLabel>
          <Input id="role-code" value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} disabled={!!editing} required />
        </Field>
        <Field>
          <FieldLabel htmlFor="role-sort">排序</FieldLabel>
          <Input id="role-sort" type="number" value={form.sort} onChange={(e) => setForm({ ...form, sort: Number(e.target.value) })} />
        </Field>
        <Field>
          <FieldLabel htmlFor="role-data-scope">数据权限范围</FieldLabel>
          <select
            id="role-data-scope"
            className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
            value={form.data_scope}
            onChange={(e) => setForm({ ...form, data_scope: Number(e.target.value) })}
            aria-label="数据权限范围"
          >
            {dataScopeOptions.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
          </select>
        </Field>
        {form.data_scope === 2 && (
          <Field>
            <FieldLabel>自定义部门</FieldLabel>
            <div className="flex flex-wrap gap-2">
              {depts.map((d) => (
                <label key={d.id} className="flex items-center gap-1 text-sm">
                  <input
                    type="checkbox"
                    aria-label={d.name}
                    checked={form.data_scope_dept_ids.includes(d.id)}
                    onChange={(e) =>
                      setForm({
                        ...form,
                        data_scope_dept_ids: e.target.checked
                          ? [...form.data_scope_dept_ids, d.id]
                          : form.data_scope_dept_ids.filter((x) => x !== d.id),
                      })
                    }
                  />
                  {d.name}
                </label>
              ))}
            </div>
          </Field>
        )}
        <div className="flex items-center justify-between">
          <FieldLabel htmlFor="role-status">状态</FieldLabel>
          <Switch id="role-status" checked={form.status === 1} onCheckedChange={(v) => setForm({ ...form, status: v ? 1 : 0 })} />
        </div>
        <Field>
          <FieldLabel htmlFor="role-remark">备注</FieldLabel>
          <Input id="role-remark" value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} />
        </Field>
      </FormDialog>
      <Dialog open={assignOpen} onOpenChange={setAssignOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader><DialogTitle>分配菜单（{assignRole?.name}）</DialogTitle></DialogHeader>
          <div className="max-h-80 overflow-auto">
            <MenuTreeCheckbox
              nodes={menus}
              selected={selectedMenus}
              onToggle={(id) => {
                setSelectedMenus((prev) => {
                  const next = new Set(prev)
                  if (next.has(id)) next.delete(id)
                  else next.add(id)
                  return next
                })
              }}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setAssignOpen(false)}>取消</Button>
            <Button disabled={assignLoading || assignFailed} onClick={() => void doAssign()}>确定分配</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
