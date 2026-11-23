import { useEffect, useState } from 'react'
import { adminSystem, type PostWrite } from '@/api/admin/system'
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
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import NoAccess from '@/views/NoAccess'
import type { SystemPost } from '@/lib/types'

interface PostForm {
  code: string
  name: string
  sort: number
  status: number
  remark: string
}
const emptyForm: PostForm = { code: '', name: '', sort: 1, status: 1, remark: '' }

function isForbidden(err: unknown): boolean {
  return err instanceof ApiError && (err.code === 403 || err.status === 403)
}

export default function Posts() {
  const confirm = useConfirm()
  const { has } = usePermissions()
  const canList = has('system:post:list')
  const list = useListPage<SystemPost>({ fetcher: (p) => adminSystem.listPosts(p) })
  const [forbidden, setForbidden] = useState(false)
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemPost | null>(null)
  const [form, setForm] = useState<PostForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!canList) return
    void list.load().catch((err: unknown) => {
      if (isForbidden(err)) setForbidden(true)
      else toastError(err instanceof Error ? err.message : '加载岗位失败')
    })
  }, [canList, list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  if (!canList || forbidden) return <NoAccess />

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.code.trim() || !form.name.trim()) {
      setFormError('编码与名称必填')
      return
    }
    setBusy(true)
    try {
      const payload: PostWrite = {
        code: form.code.trim(),
        name: form.name.trim(),
        sort: form.sort,
        status: form.status,
      }
      if (form.remark.trim()) payload.remark = form.remark.trim()
      if (editing) await adminSystem.updatePost(editing.id, payload)
      else await adminSystem.createPost(payload)
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

  const doDelete = async (p: SystemPost) => {
    const ok = await confirm({ title: `确认删除岗位「${p.name}」？`, danger: true, confirmText: '确定' })
    if (!ok) return
    try {
      await adminSystem.deletePost(p.id)
      toastSuccess('删除成功')
      await list.afterMutate()
    } catch (err) {
      if (isForbidden(err)) setForbidden(true)
      else toastError(err instanceof Error ? err.message : '删除失败')
    }
  }

  const columns: ListColumn<SystemPost>[] = [
    { key: 'code', title: '编码' },
    { key: 'name', title: '名称' },
    { key: 'sort', title: '排序' },
    {
      key: 'status', title: '状态',
      render: (r) => <Badge variant={r.status === 1 ? 'default' : 'secondary'}>{r.status === 1 ? '正常' : '停用'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (r) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:post:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(r)
              setForm({ code: r.code, name: r.name, sort: r.sort, status: r.status, remark: r.remark ?? '' })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          <PermissionButton permission="system:post:delete" variant="ghost" size="sm" onClick={() => void doDelete(r)}>删除</PermissionButton>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'code', label: '编码' }, { key: 'name', label: '名称' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns}
        toolbar={
          <PermissionButton
            permission="system:post:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormError(null); setFormOpen(true) }}
          >新增岗位</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑岗位' : '新增岗位'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field>
          <FieldLabel htmlFor="post-code">岗位编码</FieldLabel>
          <Input id="post-code" value={form.code} onChange={(e) => setForm({ ...form, code: e.target.value })} disabled={!!editing} required />
        </Field>
        <Field>
          <FieldLabel htmlFor="post-name">岗位名称</FieldLabel>
          <Input id="post-name" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} required />
        </Field>
        <Field>
          <FieldLabel htmlFor="post-sort">排序</FieldLabel>
          <Input id="post-sort" type="number" value={form.sort} onChange={(e) => setForm({ ...form, sort: Number(e.target.value) })} />
        </Field>
        <div className="flex items-center justify-between">
          <FieldLabel htmlFor="post-status">状态</FieldLabel>
          <Switch id="post-status" checked={form.status === 1} onCheckedChange={(v) => setForm({ ...form, status: v ? 1 : 0 })} />
        </div>
        <Field>
          <FieldLabel htmlFor="post-remark">备注</FieldLabel>
          <Input id="post-remark" value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} />
        </Field>
      </FormDialog>
    </div>
  )
}
