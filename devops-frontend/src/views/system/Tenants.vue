<script setup lang="ts">
import { reactive, ref } from 'vue'
import { tenantApi, type SysTenant, type TenantSave } from '@/api/system'
import { useListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column } from '@/components/ui/types'
import { Field, TextInput, Tag, StatusTag } from '@/components/ui'
import { ListPage, FormDialog, PermissionButton } from '@/components/business'

type MutList = ListPageState<SysTenant> & { afterMutate: (removedCount?: number) => Promise<void> }

const list = useListPage<SysTenant>({
  query: { name: '', code: '' },
  fetch: (p) => tenantApi.list(p),
  rowKey: 'id',
}) as MutList

const query = list.query as { name: string; code: string }

const columns: Column[] = [
  { key: 'name', title: '租户名称', width: 160 },
  { key: 'code', title: '租户编码', width: 130 },
  { key: 'contactName', title: '联系人', width: 110 },
  { key: 'contactMobile', title: '联系电话', width: 130 },
  { key: 'expireTime', title: '到期时间', width: 130 },
  { key: 'status', title: '状态', width: 90 },
  { key: 'actions', title: '操作', width: 190, className: 'col-actions' },
]

const dialogOpen = ref(false)
const busy = ref(false)
const editingId = ref<string | null>(null)

function emptyForm() {
  return {
    name: '',
    code: '',
    contact_name: '',
    contact_mobile: '',
    expire_time: '',
    remark: '',
    admin_username: '',
    admin_password: '',
  }
}

const form = reactive(emptyForm())

function openCreate(): void {
  editingId.value = null
  Object.assign(form, emptyForm())
  dialogOpen.value = true
}

async function openEdit(row: SysTenant): Promise<void> {
  busy.value = true
  try {
    const t = await tenantApi.get(row.id)
    editingId.value = row.id
    Object.assign(form, {
      name: t.name,
      code: t.code,
      contact_name: t.contactName ?? '',
      contact_mobile: t.contactMobile ?? '',
      expire_time: t.expireTime ?? '',
      remark: t.remark ?? '',
      admin_username: '',
      admin_password: '',
    })
    dialogOpen.value = true
  } catch (e) {
    toastError(e, '加载租户失败')
  } finally {
    busy.value = false
  }
}

async function submit(): Promise<void> {
  if (!form.name.trim()) {
    toastError(new Error('请填写租户名称'))
    return
  }
  if (!editingId.value) {
    if (!form.code.trim()) {
      toastError(new Error('请填写租户编码'))
      return
    }
    if (!form.admin_username.trim() || !form.admin_password) {
      toastError(new Error('请填写管理员账号与密码'))
      return
    }
  }
  busy.value = true
  try {
    const body: TenantSave = { name: form.name.trim() }
    if (editingId.value) {
      if (form.contact_name) body.contact_name = form.contact_name
      if (form.contact_mobile) body.contact_mobile = form.contact_mobile
      if (form.expire_time) body.expire_time = form.expire_time
      body.remark = form.remark
      await tenantApi.update(editingId.value, body)
    } else {
      body.code = form.code.trim()
      if (form.contact_name) body.contact_name = form.contact_name
      if (form.contact_mobile) body.contact_mobile = form.contact_mobile
      if (form.expire_time) body.expire_time = form.expire_time
      body.remark = form.remark
      body.admin_username = form.admin_username.trim()
      body.admin_password = form.admin_password
      await tenantApi.create(body)
    }
    toastOk(editingId.value ? '租户已更新' : '租户已创建')
    dialogOpen.value = false
    await list.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

async function toggleStatus(row: SysTenant): Promise<void> {
  const next = row.status === 1 ? 0 : 1
  const ok = await confirmAction({
    title: next === 0 ? '确认停用租户' : '确认启用租户',
    text: `将${next === 0 ? '停用' : '启用'}「${row.name}」，确认继续？`,
    danger: next === 0,
  })
  if (!ok) return
  busy.value = true
  try {
    await tenantApi.updateStatus(row.id, next)
    toastOk(next === 1 ? '租户已启用' : '租户已停用')
    await list.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

async function remove(row: SysTenant): Promise<void> {
  const ok = await confirmAction({ title: '确认删除', text: '删除后不可恢复，确认继续？', danger: true })
  if (!ok) return
  busy.value = true
  try {
    await tenantApi.remove(row.id)
    toastOk('已删除')
    await list.afterMutate(1)
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <ListPage title="租户管理" desc="平台租户、联系人与有效期" :list="list" :columns="columns" table-title="租户列表" row-key="id" min-width="960">
    <template #actions>
      <PermissionButton code="platform:tenant:create" variant="primary" icon="ep:plus" label="新增租户" @click="openCreate" />
    </template>

    <template #search>
      <Field label="租户名称">
        <TextInput v-model="query.name" placeholder="租户名称" />
      </Field>
      <Field label="租户编码">
        <TextInput v-model="query.code" placeholder="租户编码" />
      </Field>
    </template>

    <template #cell-code="{ value }">
      <span class="mono">{{ value }}</span>
    </template>
    <template #cell-contactMobile="{ value }">
      <span class="mono">{{ value || '-' }}</span>
    </template>
    <template #cell-expireTime="{ value }">
      <Tag v-if="!value" kind="ok">永久</Tag>
      <span v-else class="mono">{{ value }}</span>
    </template>
    <template #cell-status="{ row }">
      <StatusTag :kind="row.status === 1 ? 'ok' : 'off'" :text="row.status === 1 ? '启用' : '停用'" />
    </template>
    <template #cell-actions="{ row }">
      <PermissionButton code="platform:tenant:update" variant="link" label="编辑" @click="openEdit(row)" />
      <PermissionButton
        code="platform:tenant:update"
        variant="link"
        :label="row.status === 1 ? '停用' : '启用'"
        @click="toggleStatus(row)"
      />
      <PermissionButton code="platform:tenant:delete" variant="link" danger label="删除" @click="remove(row)" />
    </template>
  </ListPage>

  <FormDialog :open="dialogOpen" :title="editingId ? '编辑租户' : '新增租户'" :busy="busy" @close="dialogOpen = false" @submit="submit">
    <Field label="租户名称" required>
      <TextInput v-model="form.name" placeholder="租户名称" />
    </Field>
    <Field label="租户编码" required :help="editingId ? '编码创建后不可改' : undefined">
      <TextInput v-model="form.code" placeholder="唯一编码" :disabled="Boolean(editingId)" />
    </Field>
    <Field label="联系人">
      <TextInput v-model="form.contact_name" placeholder="联系人" />
    </Field>
    <Field label="联系电话">
      <TextInput v-model="form.contact_mobile" placeholder="联系电话" />
    </Field>
    <Field label="到期时间" help="格式 YYYY-MM-DD，留空为永久">
      <TextInput v-model="form.expire_time" placeholder="YYYY-MM-DD 可空" />
    </Field>
    <template v-if="!editingId">
      <Field label="管理员账号" required>
        <TextInput v-model="form.admin_username" placeholder="初始管理员登录名" />
      </Field>
      <Field label="管理员密码" required>
        <TextInput v-model="form.admin_password" type="password" placeholder="初始管理员密码" />
      </Field>
    </template>
    <Field label="备注">
      <TextInput v-model="form.remark" placeholder="备注" />
    </Field>
  </FormDialog>
</template>
