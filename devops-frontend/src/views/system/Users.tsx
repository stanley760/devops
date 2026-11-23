import { useEffect, useState } from 'react'
import { adminSystem, type UserWrite } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { usePermissions } from '@/lib/permission'
import { ApiError } from '@/lib/http'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import NoAccess from '@/views/NoAccess'
import type { SystemDept, SystemPost, SystemRole, SystemUser } from '@/lib/types'

interface UserForm {
  username: string
  nickname: string
  password: string
  deptId: string
  email: string
  mobile: string
  gender: number
  remark: string
  postIds: string[]
  roleIds: string[]
  status: number
}

const emptyForm: UserForm = {
  username: '', nickname: '', password: '', deptId: '', email: '', mobile: '', gender: 0,
  remark: '', postIds: [], roleIds: [], status: 1,
}

const genderOptions = [
  { value: 0, label: '未知' },
  { value: 1, label: '男' },
  { value: 2, label: '女' },
]

function isForbidden(err: unknown): boolean {
  return err instanceof ApiError && (err.code === 403 || err.status === 403)
}

/** Flatten a dept tree (backend returns DeptTreeResp) for the select options. */
function flattenDepts(nodes: SystemDept[], depth = 0, out: Array<{ id: string; name: string }> = []) {
  for (const n of nodes) {
    out.push({ id: n.id, name: `${'　'.repeat(depth)}${n.name}` })
    const children = (n as SystemDept & { children?: SystemDept[] }).children
    if (children?.length) flattenDepts(children, depth + 1, out)
  }
  return out
}

export default function Users() {
  const confirm = useConfirm()
  const { has } = usePermissions()
  const canList = has('system:user:list')
  const list = useListPage<SystemUser>({ fetcher: (p) => adminSystem.listUsers(p) })
  const [forbidden, setForbidden] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemUser | null>(null)
  const [form, setForm] = useState<UserForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [pwdUser, setPwdUser] = useState<SystemUser | null>(null)
  const [newPassword, setNewPassword] = useState('')
  const [pwdError, setPwdError] = useState<string | null>(null)
  const [depts, setDepts] = useState<Array<{ id: string; name: string }>>([])
  const [posts, setPosts] = useState<SystemPost[]>([])
  const [roles, setRoles] = useState<SystemRole[]>([])

  useEffect(() => {
    if (!canList) return
    void list.load().catch((err: unknown) => {
      if (isForbidden(err)) setForbidden(true)
      else toastError(err instanceof Error ? err.message : '加载用户失败')
    })
  }, [canList, list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    // Option loaders degrade to empty lists: current user may lack list permissions.
    void adminSystem.listDepts().then((rows) => setDepts(flattenDepts(rows))).catch(() => setDepts([]))
    void adminSystem.listAllPosts().then(setPosts).catch(() => setPosts([]))
    void adminSystem.listAllRoles().then(setRoles).catch(() => setRoles([]))
  }, [])

  if (!canList || forbidden) return <NoAccess />

  const openCreate = () => {
    setEditing(null)
    setForm({ ...emptyForm })
    setFormError(null)
    setFormOpen(true)
  }

  const openEdit = async (u: SystemUser) => {
    setBusy(true)
    try {
      // List rows omit roleIds/postIds; the detail endpoint returns the assigned IDs.
      const detail = await adminSystem.getUser(u.id)
      setEditing(detail)
      setForm({
        username: detail.username,
        nickname: detail.nickname,
        password: '',
        deptId: detail.deptId,
        email: detail.email ?? '',
        mobile: detail.mobile ?? '',
        gender: typeof detail.gender === 'number' ? detail.gender : 0,
        remark: detail.remark ?? '',
        postIds: detail.postIds ?? [],
        roleIds: detail.roleIds ?? [],
        status: detail.status,
      })
      setFormError(null)
      setFormOpen(true)
    } catch (err) {
      toastError(err instanceof Error ? err.message : '加载用户详情失败')
    } finally {
      setBusy(false)
    }
  }

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.username.trim() || !form.nickname.trim()) {
      setFormError('用户名与昵称必填')
      return
    }
    if (!editing && form.password.length < 6) {
      setFormError('初始密码至少 6 位')
      return
    }
    // deptId is required by the backend; empty string yields 400, so validate first.
    if (!form.deptId) {
      setFormError('部门必填')
      return
    }
    setBusy(true)
    try {
      const payload: UserWrite = {
        username: form.username.trim(),
        nickname: form.nickname.trim(),
        deptId: form.deptId,
        postIds: form.postIds,
        roleIds: form.roleIds,
        gender: form.gender,
        status: form.status,
      }
      if (form.email.trim()) payload.email = form.email.trim()
      if (form.mobile.trim()) payload.mobile = form.mobile.trim()
      if (form.remark.trim()) payload.remark = form.remark.trim()
      if (!editing) payload.password = form.password
      if (editing) await adminSystem.updateUser(editing.id, payload)
      else await adminSystem.createUser(payload)
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

  const doResetPassword = async () => {
    if (!pwdUser) return
    if (newPassword.length < 6) {
      setPwdError('密码至少 6 位')
      return
    }
    setPwdError(null)
    try {
      await adminSystem.updateUserPassword(pwdUser.id, { password: newPassword })
      toastSuccess('密码已重置')
      setPwdUser(null)
      setNewPassword('')
      setPwdError(null)
    } catch (err) {
      if (isForbidden(err)) setForbidden(true)
      else toastError(err instanceof Error ? err.message : '重置密码失败')
    }
  }

  const doDelete = async (u: SystemUser) => {
    const ok = await confirm({ title: `确认删除用户「${u.username}」？`, danger: true, confirmText: '确定' })
    if (!ok) return
    try {
      await adminSystem.deleteUser(u.id)
      toastSuccess('删除成功')
      await list.afterMutate()
    } catch (err) {
      if (isForbidden(err)) setForbidden(true)
      else toastError(err instanceof Error ? err.message : '删除失败')
    }
  }

  const columns: ListColumn<SystemUser>[] = [
    { key: 'username', title: '用户名' },
    { key: 'nickname', title: '昵称' },
    {
      key: 'status', title: '状态',
      render: (r) => <Badge variant={r.status === 1 ? 'default' : 'secondary'}>{r.status === 1 ? '正常' : '停用'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (r) => (
        <div className="flex gap-1">
          <PermissionButton permission="system:user:update" variant="ghost" size="sm" onClick={() => openEdit(r)}>编辑</PermissionButton>
          <PermissionButton permission="system:user:update-password" variant="ghost" size="sm" onClick={() => { setPwdError(null); setPwdUser(r) }}>重置密码</PermissionButton>
          <PermissionButton permission="system:user:delete" variant="ghost" size="sm" onClick={() => void doDelete(r)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'username', label: '用户名' }, { key: 'mobile', label: '手机号' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list}
        columns={columns}
        toolbar={<PermissionButton permission="system:user:create" onClick={openCreate}>新增用户</PermissionButton>}
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑用户' : '新增用户'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field>
          <FieldLabel htmlFor="user-username">用户名</FieldLabel>
          <Input id="user-username" value={form.username} onChange={(e) => setForm({ ...form, username: e.target.value })} disabled={!!editing} required />
        </Field>
        <Field>
          <FieldLabel htmlFor="user-nickname">昵称</FieldLabel>
          <Input id="user-nickname" value={form.nickname} onChange={(e) => setForm({ ...form, nickname: e.target.value })} required />
        </Field>
        {!editing && (
          <Field>
            <FieldLabel htmlFor="user-password">初始密码</FieldLabel>
            <Input id="user-password" type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} required />
          </Field>
        )}
        <Field>
          <FieldLabel htmlFor="user-dept">部门</FieldLabel>
          <select
            id="user-dept"
            className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
            value={form.deptId}
            onChange={(e) => setForm({ ...form, deptId: e.target.value })}
            aria-label="部门"
          >
            <option value="">请选择部门</option>
            {depts.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
          </select>
        </Field>
        <Field>
          <FieldLabel htmlFor="user-email">邮箱</FieldLabel>
          <Input id="user-email" value={form.email} onChange={(e) => setForm({ ...form, email: e.target.value })} />
        </Field>
        <Field>
          <FieldLabel htmlFor="user-mobile">手机号</FieldLabel>
          <Input id="user-mobile" value={form.mobile} onChange={(e) => setForm({ ...form, mobile: e.target.value })} />
        </Field>
        <Field>
          <FieldLabel htmlFor="user-gender">性别</FieldLabel>
          <select
            id="user-gender"
            className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
            value={form.gender}
            onChange={(e) => setForm({ ...form, gender: Number(e.target.value) })}
            aria-label="性别"
          >
            {genderOptions.map((g) => <option key={g.value} value={g.value}>{g.label}</option>)}
          </select>
        </Field>
        <Field>
          <FieldLabel>岗位（多选）</FieldLabel>
          <div className="flex flex-wrap gap-2">
            {posts.map((p) => (
              <label key={p.id} className="flex items-center gap-1 text-sm">
                <input
                  type="checkbox"
                  checked={form.postIds.includes(p.id)}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      postIds: e.target.checked ? [...form.postIds, p.id] : form.postIds.filter((x) => x !== p.id),
                    })
                  }
                />
                {p.name}
              </label>
            ))}
          </div>
        </Field>
        <Field>
          <FieldLabel>角色（多选）</FieldLabel>
          <div className="flex flex-wrap gap-2">
            {roles.map((r) => (
              <label key={r.id} className="flex items-center gap-1 text-sm">
                <input
                  type="checkbox"
                  checked={form.roleIds.includes(r.id)}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      roleIds: e.target.checked ? [...form.roleIds, r.id] : form.roleIds.filter((x) => x !== r.id),
                    })
                  }
                />
                {r.name}
              </label>
            ))}
          </div>
        </Field>
      </FormDialog>
      <Dialog open={pwdUser != null} onOpenChange={(o) => { if (!o) setPwdUser(null) }}>
        <DialogContent className="max-w-sm">
          <DialogHeader><DialogTitle>重置密码（{pwdUser?.username}）</DialogTitle></DialogHeader>
          {pwdError && <p className="text-destructive text-sm">{pwdError}</p>}
          <Field>
            <FieldLabel htmlFor="new-password">新密码</FieldLabel>
            <Input id="new-password" type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} />
          </Field>
          <DialogFooter>
            <Button variant="outline" onClick={() => setPwdUser(null)}>取消</Button>
            <Button onClick={() => void doResetPassword()}>确定重置</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
