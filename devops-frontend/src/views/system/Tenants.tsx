import { useEffect, useState } from 'react'
import { adminSystem, type TenantUpdateWrite, type TenantWrite } from '@/api/admin/system'
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

/** Backend *time.Time requires RFC3339; a bare YYYY-MM-DD is rejected by Huma. */
function toRfc3339(value: string): string | null {
  const v = value.trim()
  if (!v) return null
  const m = v.match(/^(\d{4}-\d{2}-\d{2})(?:[ T](\d{2}:\d{2}:\d{2}))?(Z|[+-]\d{2}:\d{2})?$/)
  if (!m) return null
  const date = m[1]!
  const time = m[2] ?? '00:00:00'
  const zone = m[3]
  const d = new Date(`${date}T${time}${zone ?? ''}`)
  if (Number.isNaN(d.getTime())) return null
  if (zone) return `${date}T${time}${zone}`
  const p = (n: number) => String(n).padStart(2, '0')
  const offset = -d.getTimezoneOffset()
  const sign = offset >= 0 ? '+' : '-'
  const abs = Math.abs(offset)
  return `${date}T${time}${sign}${p(Math.floor(abs / 60))}:${p(abs % 60)}`
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
    const expire = toRfc3339(form.expire_time)
    if (form.expire_time.trim() && !expire) {
      setFormError('过期时间格式不正确')
      return
    }
    setBusy(true)
    try {
      if (editing) {
        const body: TenantUpdateWrite = {
          name: form.name,
          contact_name: form.contact_name,
          contact_mobile: form.contact_mobile,
          expire_time: expire,
          remark: form.remark,
        }
        await adminSystem.updateTenant(editing.id, body)
      } else {
        const body: TenantWrite = {
          name: form.name, code: form.code,
          contact_name: form.contact_name, contact_mobile: form.contact_mobile,
          admin_username: form.admin_username, admin_password: form.admin_password,
        }
        if (expire) body.expire_time = expire
        if (form.remark) body.remark = form.remark
        await adminSystem.createTenant(body)
      }
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
                expire_time: t.expireTime?.slice(0, 10) ?? '', remark: t.remark ?? '', admin_username: '', admin_password: '',
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