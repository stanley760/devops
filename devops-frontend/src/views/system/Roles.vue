<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { roleApi, menuApi, type SysRole, type RoleSave, type SysMenu } from '@/api/system'
import { useListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column } from '@/components/ui/types'
import { Field, TextInput, Select, Switch, Tag, StatusTag, Modal } from '@/components/ui'
import { ListPage, FormDialog, PermissionButton } from '@/components/business'
import MenuCheckTree from './parts/MenuCheckTree.vue'

type MutList = ListPageState<SysRole> & { afterMutate: (removedCount?: number) => Promise<void> }

const list = useListPage<SysRole>({
  query: { name: '', status: '' },
  fetch: (p) => roleApi.list(p),
  rowKey: 'id',
}) as MutList

const query = list.query as { name: string; status: string | number }

const columns: Column[] = [
  { key: 'code', title: '编码', width: 150 },
  { key: 'name', title: '名称', width: 140 },
  { key: 'sort', title: '排序', width: 80, align: 'right' },
  { key: 'dataScope', title: '数据范围', width: 130 },
  { key: 'status', title: '状态', width: 90 },
  { key: 'remark', title: '备注' },
  { key: 'actions', title: '操作', width: 210, className: 'col-actions' },
]

const statusOptions = [
  { value: '', label: '全部' },
  { value: 1, label: '启用' },
  { value: 0, label: '停用' },
]

const scopeOptions = [
  { value: 1, label: '全部' },
  { value: 2, label: '自定义' },
  { value: 3, label: '本部门' },
  { value: 4, label: '本部门及以下' },
  { value: 5, label: '仅本人' },
]

const scopeLabel: Record<number, string> = {
  1: '全部',
  2: '自定义',
  3: '本部门',
  4: '本部门及以下',
  5: '仅本人',
}

const dialogOpen = ref(false)
const busy = ref(false)
const editingId = ref<string | null>(null)

function emptyForm() {
  return { name: '', code: '', sort: 0, data_scope: 1, remark: '', status: true }
}

const form = reactive(emptyForm())

function openCreate(): void {
  editingId.value = null
  Object.assign(form, emptyForm())
  dialogOpen.value = true
}

async function openEdit(row: SysRole): Promise<void> {
  busy.value = true
  try {
    const r = await roleApi.get(row.id)
    editingId.value = row.id
    Object.assign(form, {
      name: r.name,
      code: r.code,
      sort: r.sort ?? 0,
      data_scope: r.dataScope ?? 1,
      remark: r.remark ?? '',
      status: r.status !== 0,
    })
    dialogOpen.value = true
  } catch (e) {
    toastError(e, '加载角色失败')
  } finally {
    busy.value = false
  }
}

async function submit(): Promise<void> {
  if (!form.name.trim()) {
    toastError(new Error('请填写角色名称'))
    return
  }
  if (!editingId.value && !form.code.trim()) {
    toastError(new Error('请填写角色编码'))
    return
  }
  busy.value = true
  try {
    const body: RoleSave = {
      name: form.name.trim(),
      sort: Number(form.sort) || 0,
      data_scope: Number(form.data_scope) || 1,
      status: form.status ? 1 : 0,
      remark: form.remark,
    }
    if (editingId.value) {
      await roleApi.update(editingId.value, body)
    } else {
      body.code = form.code.trim()
      await roleApi.create(body)
    }
    toastOk(editingId.value ? '角色已更新' : '角色已创建')
    dialogOpen.value = false
    await list.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

async function remove(row: SysRole): Promise<void> {
  const ok = await confirmAction({ title: '确认删除', text: '删除后不可恢复，确认继续？', danger: true })
  if (!ok) return
  busy.value = true
  try {
    await roleApi.remove(row.id)
    toastOk('已删除')
    await list.afterMutate(1)
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

/* ---------------- 分配菜单 ---------------- */

const menuOpen = ref(false)
const menuBusy = ref(false)
const menuTrees = ref<SysMenu[]>([])
const checkedMap = reactive<Record<string, boolean>>({})
const menuTarget = ref<SysRole | null>(null)

async function openAssign(row: SysRole): Promise<void> {
  menuTarget.value = row
  menuBusy.value = true
  menuOpen.value = true
  for (const k of Object.keys(checkedMap)) delete checkedMap[k]
  try {
    const [tree, detail] = await Promise.all([menuApi.tree(), roleApi.get(row.id)])
    menuTrees.value = tree
    for (const id of detail.menuIds ?? []) checkedMap[id] = true
  } catch (e) {
    toastError(e, '加载菜单失败')
    menuOpen.value = false
  } finally {
    menuBusy.value = false
  }
}

watch(menuOpen, (open) => {
  if (!open) {
    menuTarget.value = null
    menuTrees.value = []
    for (const k of Object.keys(checkedMap)) delete checkedMap[k]
  }
})

async function submitAssign(): Promise<void> {
  if (!menuTarget.value) return
  const ids = Object.keys(checkedMap).filter((k) => checkedMap[k])
  menuBusy.value = true
  try {
    await roleApi.assignMenus(menuTarget.value.id, ids)
    toastOk('菜单权限已保存')
    menuOpen.value = false
  } catch (e) {
    toastError(e)
  } finally {
    menuBusy.value = false
  }
}
</script>

<template>
  <ListPage title="角色管理" desc="角色、数据范围与菜单权限" :list="list" :columns="columns" table-title="角色列表" row-key="id">
    <template #actions>
      <PermissionButton code="system:role:create" variant="primary" icon="ep:plus" label="新增角色" @click="openCreate" />
    </template>

    <template #search>
      <Field label="角色名称">
        <TextInput v-model="query.name" placeholder="角色名称" />
      </Field>
      <Field label="状态">
        <Select v-model="query.status" :options="statusOptions" placeholder="全部" />
      </Field>
    </template>

    <template #cell-code="{ value }">
      <span class="mono">{{ value }}</span>
    </template>
    <template #cell-dataScope="{ value }">
      <Tag kind="blue">{{ scopeLabel[Number(value)] ?? '-' }}</Tag>
    </template>
    <template #cell-status="{ row }">
      <StatusTag :kind="row.status === 1 ? 'ok' : 'off'" :text="row.status === 1 ? '启用' : '停用'" />
    </template>
    <template #cell-actions="{ row }">
      <PermissionButton code="system:role:update" variant="link" label="编辑" @click="openEdit(row)" />
      <PermissionButton code="system:role:assign" variant="link" label="分配菜单" @click="openAssign(row)" />
      <PermissionButton
        v-if="row.type !== 1"
        code="system:role:delete"
        variant="link"
        danger
        label="删除"
        @click="remove(row)"
      />
      <span v-else class="btn-link" style="opacity: 0.45; cursor: not-allowed" title="内置角色">删除</span>
    </template>
  </ListPage>

  <FormDialog :open="dialogOpen" :title="editingId ? '编辑角色' : '新增角色'" :busy="busy" @close="dialogOpen = false" @submit="submit">
    <Field label="角色名称" required>
      <TextInput v-model="form.name" placeholder="角色名称" />
    </Field>
    <Field label="角色编码" required :help="editingId ? '编码创建后不可改' : undefined">
      <TextInput v-model="form.code" placeholder="角色编码" :disabled="Boolean(editingId)" />
    </Field>
    <Field label="排序">
      <TextInput v-model="form.sort" type="number" placeholder="排序" />
    </Field>
    <Field label="数据范围">
      <Select v-model="form.data_scope" :options="scopeOptions" />
    </Field>
    <Field label="备注">
      <TextInput v-model="form.remark" placeholder="备注" />
    </Field>
    <Field label="状态">
      <Switch v-model="form.status" />
    </Field>
  </FormDialog>

  <Modal
    :open="menuOpen"
    title="分配菜单权限"
    :busy="menuBusy"
    confirm-text="保存"
    width="wide"
    @close="menuOpen = false"
    @confirm="submitAssign"
  >
    <div v-if="menuTrees.length">
      <MenuCheckTree v-for="node in menuTrees" :key="node.id" :node="node" :checked="checkedMap" />
    </div>
  </Modal>
</template>
