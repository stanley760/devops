import { useEffect, useState } from 'react'
import { adminSystem, type TenantWrite } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import type { SystemTenant } from '@/lib/types'

interface TenantForm {
  name: string; code: string; contact_name: string; contact_mobile: string
  expire_time: string; remark: string; admin_username: string; admin_password: string
}
const emptyForm: TenantForm = {
  name: '', code: '', contact_name: '', contact_mobile: '',
  expire_time: '', remark: '', admin_username: '', admin_password: '',
}

export default function Tenants() {
  const confirm = useConfirm()
  const list = useListPage<SystemTenant>({ fetcher: (p) => adminSystem.listTenants(p) })
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemTenant | null>(null)
  const [form, setForm] = useState<TenantForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void list.load().catch((err: unknown) => {
      toastError(err instanceof Error ? err.message : '加载租户失败')
    })
  }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.name || !form.code || !form.contact_name) {
      setFormError('名称/编码/联系人必填')
      return
    }
    if (!editing && (!form.admin_username || !form.admin_password)) {
      setFormError('管理员账号与密码必填')
      return
    }
    setBusy(true)
    try {
      const base: TenantWrite = {
        name: form.name, code: form.code,
        contact_name: form.contact_name, contact_mobile: form.contact_mobile,
      }
      if (form.expire_time) base.expire_time = form.expire_time
      if (form.remark) base.remark = form.remark
      if (editing) await adminSystem.updateTenant(editing.id, base)
      else await adminSystem.createTenant({
        ...base, admin_username: form.admin_username, admin_password: form.admin_password,
      })
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await list.afterMutate()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const doDelete = async (t: SystemTenant) => {
    const ok = await confirm({ title: `确认删除租户「${t.name}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    try {
      await adminSystem.deleteTenant(t.id)
      toastSuccess('删除成功')
      await list.afterMutate()
    } catch (err) {
      toastError(err instanceof Error ? err.message : '删除失败')
    }
  }

  const columns: ListColumn<SystemTenant>[] = [
    { key: 'name', title: '名称' },
    { key: 'code', title: '编码' },
    { key: 'contactName', title: '联系人' },
    { key: 'contactMobile', title: '联系电话' },
    {
      key: 'op', title: '操作',
      render: (t) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="platform:tenant:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(t)
              setForm({
                name: t.name, code: t.code, contact_name: t.contactName, contact_mobile: t.contactMobile,
                expire_time: t.expireTime ?? '', remark: t.remark ?? '', admin_username: '', admin_password: '',
              })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton permission="platform:tenant:delete" variant="ghost" size="sm" onClick={() => void doDelete(t)}>删除</PermissionButton>
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
        toolbar={
          <PermissionButton
            permission="platform:tenant:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormError(null); setFormOpen(true) }}
          >新增租户</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑租户' : '新增租户'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field><FieldLabel htmlFor="tenant-name">租户名称</FieldLabel><Input id="tenant-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required /></Field>
        <Field><FieldLabel htmlFor="tenant-code">租户编码</FieldLabel><Input id="tenant-code" value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} disabled={!!editing} required /></Field>
        <Field><FieldLabel htmlFor="tenant-contact-name">联系人</FieldLabel><Input id="tenant-contact-name" value={form.contact_name} onChange={(e) => setForm({ ...form, contact_name: e.target.value })} required /></Field>
        <Field><FieldLabel htmlFor="tenant-contact-mobile">联系电话</FieldLabel><Input id="tenant-contact-mobile" value={form.contact_mobile} onChange={(e) => setForm({ ...form, contact_mobile: e.target.value })} /></Field>
        <Field><FieldLabel htmlFor="tenant-expire-time">过期时间（YYYY-MM-DD）</FieldLabel><Input id="tenant-expire-time" value={form.expire_time} onChange={(e) => setForm({ ...form, expire_time: e.target.value })} placeholder="2027-12-31" /></Field>
        {!editing && (
          <>
            <Field><FieldLabel htmlFor="tenant-admin-username">管理员账号</FieldLabel><Input id="tenant-admin-username" value={form.admin_username} onChange={(e) => setForm({ ...form, admin_username: e.target.value })} /></Field>
            <Field><FieldLabel htmlFor="tenant-admin-password">管理员密码</FieldLabel><Input id="tenant-admin-password" type="password" value={form.admin_password} onChange={(e) => setForm({ ...form, admin_password: e.target.value })} /></Field>
          </>
        )}
        <Field><FieldLabel htmlFor="tenant-remark">备注</FieldLabel><Textarea id="tenant-remark" value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} /></Field>
      </FormDialog>
    </div>
  )
}