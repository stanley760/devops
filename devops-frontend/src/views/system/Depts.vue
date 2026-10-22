<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { deptApi, type SysDept, type DeptSave } from '@/api/system'
import { useTreeListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column } from '@/components/ui/types'
import { Field, TextInput, Select, Switch, StatusTag } from '@/components/ui'
import { ListPage, FormDialog, PermissionButton } from '@/components/business'

type DeptRow = SysDept & { depth: number }
type MutList = ListPageState<DeptRow> & { afterMutate: (removedCount?: number) => Promise<void> }

function flatten(rows: SysDept[], depth = 0, out: DeptRow[] = []): DeptRow[] {
  for (const r of rows) {
    out.push({ ...r, depth, children: undefined })
    if (r.children?.length) flatten(r.children, depth + 1, out)
  }
  return out
}

const treeList = useTreeListPage<SysDept>({
  query: {},
  fetch: async () => {
    const tree = await deptApi.tree()
    return { list: tree, total: tree.length, page_num: 1, page_size: 1, pages: 1 }
  },
})

const flatRows = computed(() => flatten(treeList.rows.value))
const list: MutList = {
  ...(treeList as unknown as MutList),
  rows: flatRows,
  total: computed(() => flatRows.value.length),
} as MutList

const columns: Column[] = [
  { key: 'name', title: '部门名称', width: 220 },
  { key: 'leaderName', title: '负责人', width: 120 },
  { key: 'phone', title: '联系电话', width: 130 },
  { key: 'email', title: '邮箱', width: 180 },
  { key: 'sort', title: '排序', width: 80, align: 'right' },
  { key: 'status', title: '状态', width: 90 },
  { key: 'actions', title: '操作', width: 180, className: 'col-actions' },
]

const parentOptions = computed(() => {
  const opts: Array<{ value: string; label: string }> = [{ value: '', label: '顶级部门' }]
  for (const r of flatRows.value) opts.push({ value: r.id, label: `${'　'.repeat(r.depth)}${r.name}` })
  return opts
})

const dialogOpen = ref(false)
const busy = ref(false)
const editingId = ref<string | null>(null)

function emptyForm() {
  return {
    name: '',
    parentId: '',
    sort: 0,
    leader_user_id: '',
    phone: '',
    email: '',
    status: true,
  }
}

const form = reactive(emptyForm())

function openCreate(parent?: DeptRow): void {
  editingId.value = null
  Object.assign(form, emptyForm())
  form.parentId = parent?.id ?? ''
  dialogOpen.value = true
}

function openEdit(row: DeptRow): void {
  editingId.value = row.id
  Object.assign(form, {
    name: row.name,
    parentId: row.parentId ?? '',
    sort: row.sort ?? 0,
    leader_user_id: row.leaderUserId ?? '',
    phone: row.phone ?? '',
    email: row.email ?? '',
    status: row.status !== 0,
  })
  dialogOpen.value = true
}

async function submit(): Promise<void> {
  if (!form.name.trim()) {
    toastError(new Error('请填写部门名称'))
    return
  }
  busy.value = true
  try {
    const body: DeptSave = {
      name: form.name.trim(),
      parentId: form.parentId || '',
      sort: Number(form.sort) || 0,
      status: form.status ? 1 : 0,
    }
    if (form.leader_user_id) body.leader_user_id = form.leader_user_id
    if (form.phone) body.phone = form.phone
    if (form.email) body.email = form.email
    if (editingId.value) await deptApi.update(editingId.value, body)
    else await deptApi.create(body)
    toastOk(editingId.value ? '部门已更新' : '部门已创建')
    dialogOpen.value = false
    await treeList.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

async function remove(row: DeptRow): Promise<void> {
  const ok = await confirmAction({ title: '确认删除', text: '删除后不可恢复，确认继续？', danger: true })
  if (!ok) return
  busy.value = true
  try {
    await deptApi.remove(row.id)
    toastOk('已删除')
    await treeList.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <ListPage title="部门管理" desc="组织架构与部门负责人" :list="list" :columns="columns" table-title="部门树" row-key="id" min-width="900">
    <template #actions>
      <PermissionButton code="system:dept:create" variant="primary" icon="ep:plus" label="新增部门" @click="openCreate()" />
    </template>

    <template #cell-name="{ row }">
      <span :style="{ paddingLeft: `${((row as DeptRow).depth ?? 0) * 16}px` }">{{ row.name }}</span>
    </template>
    <template #cell-phone="{ value }">
      <span class="mono">{{ value || '-' }}</span>
    </template>
    <template #cell-status="{ row }">
      <StatusTag :kind="row.status === 1 ? 'ok' : 'off'" :text="row.status === 1 ? '启用' : '停用'" />
    </template>
    <template #cell-actions="{ row }">
      <PermissionButton code="system:dept:create" variant="link" label="新增子项" @click="openCreate(row as DeptRow)" />
      <PermissionButton code="system:dept:update" variant="link" label="编辑" @click="openEdit(row as DeptRow)" />
      <PermissionButton code="system:dept:delete" variant="link" danger label="删除" @click="remove(row as DeptRow)" />
    </template>
  </ListPage>

  <FormDialog :open="dialogOpen" :title="editingId ? '编辑部门' : '新增部门'" :busy="busy" @close="dialogOpen = false" @submit="submit">
    <Field label="部门名称" required>
      <TextInput v-model="form.name" placeholder="部门名称" />
    </Field>
    <Field label="上级部门">
      <Select v-model="form.parentId" :options="parentOptions" placeholder="顶级部门" />
    </Field>
    <Field label="负责人 ID">
      <TextInput v-model="form.leader_user_id" placeholder="负责人用户 ID" />
    </Field>
    <Field label="联系电话">
      <TextInput v-model="form.phone" placeholder="联系电话" />
    </Field>
    <Field label="邮箱">
      <TextInput v-model="form.email" placeholder="邮箱" />
    </Field>
    <Field label="排序">
      <TextInput v-model="form.sort" type="number" placeholder="排序" />
    </Field>
    <Field label="状态">
      <Switch v-model="form.status" />
    </Field>
  </FormDialog>
</template>
