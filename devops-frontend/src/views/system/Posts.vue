<script setup lang="ts">
import { reactive, ref } from 'vue'
import { postApi, type SysPost, type PostSave } from '@/api/system'
import { useListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column } from '@/components/ui/types'
import { Field, TextInput, Select, Switch, StatusTag } from '@/components/ui'
import { ListPage, FormDialog, PermissionButton } from '@/components/business'

type MutList = ListPageState<SysPost> & { afterMutate: (removedCount?: number) => Promise<void> }

const list = useListPage<SysPost>({
  query: { code: '', name: '', status: '' },
  fetch: (p) => postApi.list(p),
  rowKey: 'id',
}) as MutList

const query = list.query as { code: string; name: string; status: string | number }

const columns: Column[] = [
  { key: 'code', title: '编码', width: 150 },
  { key: 'name', title: '名称', width: 160 },
  { key: 'sort', title: '排序', width: 80, align: 'right' },
  { key: 'status', title: '状态', width: 90 },
  { key: 'remark', title: '备注' },
  { key: 'actions', title: '操作', width: 140, className: 'col-actions' },
]

const statusOptions = [
  { value: '', label: '全部' },
  { value: 1, label: '启用' },
  { value: 0, label: '停用' },
]

const dialogOpen = ref(false)
const busy = ref(false)
const editingId = ref<string | null>(null)

function emptyForm() {
  return { code: '', name: '', sort: 0, status: true, remark: '' }
}

const form = reactive(emptyForm())

function openCreate(): void {
  editingId.value = null
  Object.assign(form, emptyForm())
  dialogOpen.value = true
}

function openEdit(row: SysPost): void {
  editingId.value = row.id
  Object.assign(form, {
    code: row.code,
    name: row.name,
    sort: row.sort ?? 0,
    status: row.status !== 0,
    remark: row.remark ?? '',
  })
  dialogOpen.value = true
}

async function submit(): Promise<void> {
  if (!form.code.trim() || !form.name.trim()) {
    toastError(new Error('请填写编码与名称'))
    return
  }
  busy.value = true
  try {
    const body: PostSave = {
      code: form.code.trim(),
      name: form.name.trim(),
      sort: Number(form.sort) || 0,
      status: form.status ? 1 : 0,
      remark: form.remark,
    }
    if (editingId.value) await postApi.update(editingId.value, body)
    else await postApi.create(body)
    toastOk(editingId.value ? '岗位已更新' : '岗位已创建')
    dialogOpen.value = false
    await list.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

async function remove(row: SysPost): Promise<void> {
  const ok = await confirmAction({ title: '确认删除', text: '删除后不可恢复，确认继续？', danger: true })
  if (!ok) return
  busy.value = true
  try {
    await postApi.remove(row.id)
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
  <ListPage title="岗位管理" desc="组织岗位编码与状态" :list="list" :columns="columns" table-title="岗位列表" row-key="id">
    <template #actions>
      <PermissionButton code="system:post:create" variant="primary" icon="ep:plus" label="新增岗位" @click="openCreate" />
    </template>

    <template #search>
      <Field label="编码">
        <TextInput v-model="query.code" placeholder="岗位编码" />
      </Field>
      <Field label="名称">
        <TextInput v-model="query.name" placeholder="岗位名称" />
      </Field>
      <Field label="状态">
        <Select v-model="query.status" :options="statusOptions" placeholder="全部" />
      </Field>
    </template>

    <template #cell-code="{ value }">
      <span class="mono">{{ value }}</span>
    </template>
    <template #cell-status="{ row }">
      <StatusTag :kind="row.status === 1 ? 'ok' : 'off'" :text="row.status === 1 ? '启用' : '停用'" />
    </template>
    <template #cell-actions="{ row }">
      <PermissionButton code="system:post:update" variant="link" label="编辑" @click="openEdit(row)" />
      <PermissionButton code="system:post:delete" variant="link" danger label="删除" @click="remove(row)" />
    </template>
  </ListPage>

  <FormDialog :open="dialogOpen" :title="editingId ? '编辑岗位' : '新增岗位'" :busy="busy" @close="dialogOpen = false" @submit="submit">
    <Field label="岗位编码" required :help="editingId ? '编码创建后不可改' : undefined">
      <TextInput v-model="form.code" placeholder="岗位编码" :disabled="Boolean(editingId)" />
    </Field>
    <Field label="岗位名称" required>
      <TextInput v-model="form.name" placeholder="岗位名称" />
    </Field>
    <Field label="排序">
      <TextInput v-model="form.sort" type="number" placeholder="排序" />
    </Field>
    <Field label="状态">
      <Switch v-model="form.status" />
    </Field>
    <Field label="备注">
      <TextInput v-model="form.remark" placeholder="备注" />
    </Field>
  </FormDialog>
</template>
