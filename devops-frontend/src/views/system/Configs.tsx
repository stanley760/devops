import { useEffect, useState } from 'react'
import { adminSystem, type ConfigWrite } from '@/api/admin/system'
import { useListPage } from '@/lib/list'
import { useOptions } from '@/lib/options'
import { useConfirm } from '@/lib/confirm'
import { toastError, toastSuccess } from '@/lib/toast'
import { ListPage, type ListColumn } from '@/components/business/ListPage'
import { SearchForm } from '@/components/business/SearchForm'
import { FormDialog } from '@/components/business/FormDialog'
import { PermissionButton } from '@/components/business/PermissionButton'
import { Badge } from '@/components/ui/badge'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import type { DictItem, SystemConfig } from '@/lib/types'

interface ConfigForm {
  config_key: string; config_name: string; config_value: string
  type: number; visible: boolean; remark: string
}
const emptyForm: ConfigForm = { config_key: '', config_name: '', config_value: '', type: 2, visible: true, remark: '' }

// 内置键 → 控件映射
function controlFor(key: string): 'switch' | 'captcha-type-select' | 'input' {
  if (key === 'sys.account.registerUser' || key === 'sys.captcha.enabled') return 'switch'
  if (key === 'sys.captcha.type') return 'captcha-type-select'
  return 'input'
}

export default function Configs() {
  const confirm = useConfirm()
  const list = useListPage<SystemConfig>({ fetcher: (p) => adminSystem.listConfigs(p) })
  const captchaTypeOptions = useOptions<DictItem>(
    'dict:sys_captcha_type',
    () => adminSystem.listDictItems('sys_captcha_type'),
  )
  const [formOpen, setFormOpen] = useState(false)
  const [editing, setEditing] = useState<SystemConfig | null>(null)
  const [form, setForm] = useState<ConfigForm>({ ...emptyForm })
  const [formError, setFormError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    void list.load().catch((err: unknown) => {
      toastError(err instanceof Error ? err.message : '加载参数失败')
    })
  }, [list.page, list.pageSize, list.query]) // eslint-disable-line react-hooks/exhaustive-deps

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setFormError(null)
    if (!form.config_key || !form.config_name) {
      setFormError('键与名称必填')
      return
    }
    setBusy(true)
    try {
      const payload: ConfigWrite = {
        config_key: form.config_key, config_name: form.config_name, config_value: form.config_value,
        type: form.type, visible: form.visible,
      }
      if (form.remark) payload.remark = form.remark
      if (editing) await adminSystem.updateConfig(editing.id, payload)
      else await adminSystem.createConfig(payload)
      toastSuccess(editing ? '更新成功' : '创建成功')
      setFormOpen(false)
      await list.afterMutate()
    } catch (err) {
      setFormError(err instanceof Error ? err.message : '保存失败')
    } finally {
      setBusy(false)
    }
  }

  const doDelete = async (c: SystemConfig) => {
    if (c.type === 1) return // 内置不可删除（双保险）
    const ok = await confirm({ title: `确认删除参数「${c.configName}」？`, danger: true, confirmText: '删除' })
    if (!ok) return
    try {
      await adminSystem.deleteConfig(c.id)
      toastSuccess('删除成功')
      await list.afterMutate()
    } catch (err) {
      toastError(err instanceof Error ? err.message : '删除失败')
    }
  }

  const columns: ListColumn<SystemConfig>[] = [
    { key: 'configKey', title: '键' },
    { key: 'configName', title: '名称' },
    { key: 'configValue', title: '值' },
    {
      key: 'type', title: '类型',
      render: (c) => <Badge variant={c.type === 1 ? 'secondary' : 'outline'}>{c.type === 1 ? '内置' : '自定义'}</Badge>,
    },
    {
      key: 'op', title: '操作',
      render: (c) => (
        <div className="flex gap-1">
          <PermissionButton
            permission="system:config:update" variant="ghost" size="sm"
            onClick={() => {
              setEditing(c)
              setForm({
                config_key: c.configKey, config_name: c.configName, config_value: c.configValue,
                type: c.type, visible: c.visible, remark: c.remark ?? '',
              })
              setFormError(null)
              setFormOpen(true)
            }}
          >编辑</PermissionButton>
          {c.type !== 1 && (
            <PermissionButton permission="system:config:delete" variant="ghost" size="sm" onClick={() => void doDelete(c)}>删除</PermissionButton>
          )}
        </div>
      ),
    },
  ]

  const control = editing ? controlFor(editing.configKey) : 'input'

  return (
    <div className="space-y-3">
      <SearchForm
        fields={[{ key: 'config_key', label: '键' }, { key: 'config_name', label: '名称' }]}
        value={list.query} onSearch={() => void list.search()} onReset={() => void list.reset()}
      />
      <ListPage
        state={list} columns={columns}
        toolbar={
          <PermissionButton
            permission="system:config:create"
            onClick={() => { setEditing(null); setForm({ ...emptyForm }); setFormError(null); setFormOpen(true) }}
          >新增参数</PermissionButton>
        }
      />
      <FormDialog open={formOpen} onOpenChange={setFormOpen} title={editing ? '编辑参数' : '新增参数'} onSubmit={submit} busy={busy}>
        {formError && <p className="text-destructive text-sm">{formError}</p>}
        <Field>
          <FieldLabel htmlFor="config-key">参数键</FieldLabel>
          <Input id="config-key" value={form.config_key} onChange={(e) => setForm({ ...form, config_key: e.target.value })} disabled={!!editing} required />
        </Field>
        <Field>
          <FieldLabel htmlFor="config-name">参数名称</FieldLabel>
          <Input id="config-name" value={form.config_name} onChange={(e) => setForm({ ...form, config_name: e.target.value })} required />
        </Field>
        <Field>
          <FieldLabel htmlFor="config-value">参数值</FieldLabel>
          {control === 'switch' && (
            <Switch
              id="config-value" aria-label="参数值"
              checked={form.config_value === 'true'}
              onCheckedChange={(v) => setForm({ ...form, config_value: v ? 'true' : 'false' })}
            />
          )}
          {control === 'captcha-type-select' && (
            <select
              id="config-value" aria-label="参数值"
              className="border-input bg-background h-9 w-full rounded-md border px-3 text-sm"
              value={form.config_value}
              onChange={(e) => setForm({ ...form, config_value: e.target.value })}
            >
              {captchaTypeOptions.data.map((o) => (
                <option key={o.id} value={o.value}>{o.label}</option>
              ))}
            </select>
          )}
          {control === 'input' && (
            <Input id="config-value" aria-label="参数值" value={form.config_value} onChange={(e) => setForm({ ...form, config_value: e.target.value })} />
          )}
        </Field>
        <div className="flex items-center justify-between">
          <FieldLabel htmlFor="config-visible">可见</FieldLabel>
          <Switch id="config-visible" checked={form.visible} onCheckedChange={(v) => setForm({ ...form, visible: v })} />
        </div>
        <Field><FieldLabel htmlFor="config-remark">备注</FieldLabel><Input id="config-remark" value={form.remark} onChange={(e) => setForm({ ...form, remark: e.target.value })} /></Field>
      </FormDialog>
    </div>
  )
}