<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { menuApi, type SysMenu, type MenuSave, type MenuType } from '@/api/system'
import { useTreeListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column } from '@/components/ui/types'
import { Field, TextInput, Select, Switch, Tag, StatusTag } from '@/components/ui'
import { ListPage, FormDialog, PermissionButton } from '@/components/business'

type MenuRow = SysMenu & { depth: number }
type MutList = ListPageState<MenuRow> & { afterMutate: (removedCount?: number) => Promise<void> }

function flatten(rows: SysMenu[], depth = 0, out: MenuRow[] = []): MenuRow[] {
  for (const r of rows) {
    out.push({ ...r, depth, children: undefined })
    if (r.children?.length) flatten(r.children, depth + 1, out)
  }
  return out
}

const treeList = useTreeListPage<SysMenu>({
  query: {},
  fetch: async () => {
    const tree = await menuApi.tree()
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
  { key: 'name', title: '名称', width: 220 },
  { key: 'permission', title: '权限标识', width: 180 },
  { key: 'type', title: '类型', width: 90 },
  { key: 'sort', title: '排序', width: 80, align: 'right' },
  { key: 'path', title: '路由路径', width: 180 },
  { key: 'visible', title: '可见', width: 90 },
  { key: 'actions', title: '操作', width: 210, className: 'col-actions' },
]

const typeOptions = [
  { value: 1, label: '目录' },
  { value: 2, label: '菜单' },
  { value: 3, label: '按钮' },
]

const parentOptions = computed(() => {
  const opts: Array<{ value: string; label: string }> = [{ value: '', label: '顶级菜单' }]
  for (const r of flatRows.value) {
    if (Number(r.type) === 3) continue
    opts.push({ value: r.id, label: `${'　'.repeat(r.depth)}${r.name}` })
  }
  return opts
})

const dialogOpen = ref(false)
const busy = ref(false)
const editingId = ref<string | null>(null)

function emptyForm() {
  return {
    parentId: '',
    name: '',
    type: 1,
    path: '',
    icon: '',
    component: '',
    permission: '',
    sort: 0,
    status: true,
    visible: true,
    keep_alive: true,
    always_show: true,
  }
}

const form = reactive(emptyForm())

function openCreate(parent?: SysMenu): void {
  editingId.value = null
  Object.assign(form, emptyForm())
  form.parentId = parent?.id ?? ''
  dialogOpen.value = true
}

function openEdit(row: MenuRow): void {
  editingId.value = row.id
  Object.assign(form, {
    parentId: row.parentId ?? '',
    name: row.name,
    type: row.type,
    path: row.path ?? '',
    icon: row.icon ?? '',
    component: row.component ?? '',
    permission: row.permission ?? '',
    sort: row.sort ?? 0,
    status: row.status !== 0,
    visible: row.visible !== false,
    keep_alive: row.keepAlive !== false,
    always_show: row.alwaysShow !== false,
  })
  dialogOpen.value = true
}

async function submit(): Promise<void> {
  if (!form.name.trim()) {
    toastError(new Error('请填写菜单名称'))
    return
  }
  busy.value = true
  try {
    const body: MenuSave = {
      name: form.name.trim(),
      type: Number(form.type) as MenuType,
      sort: Number(form.sort) || 0,
      parentId: form.parentId || '',
      path: form.path,
      icon: form.icon,
      component: form.component,
      permission: form.permission,
      status: form.status ? 1 : 0,
      visible: form.visible,
      keep_alive: form.keep_alive,
      always_show: form.always_show,
    }
    if (editingId.value) await menuApi.update(editingId.value, body)
    else await menuApi.create(body)
    toastOk(editingId.value ? '菜单已更新' : '菜单已创建')
    dialogOpen.value = false
    await treeList.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

async function remove(row: MenuRow): Promise<void> {
  const ok = await confirmAction({ title: '确认删除', text: '删除后不可恢复，确认继续？', danger: true })
  if (!ok) return
  busy.value = true
  try {
    await menuApi.remove(row.id)
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
  <ListPage title="菜单管理" desc="目录 / 菜单 / 按钮与路由配置" :list="list" :columns="columns" table-title="菜单树" row-key="id" min-width="980">
    <template #actions>
      <PermissionButton code="system:menu:create" variant="primary" icon="ep:plus" label="新增菜单" @click="openCreate()" />
    </template>

    <template #cell-name="{ row }">
      <span :style="{ paddingLeft: `${(row as MenuRow).depth * 16}px` }">{{ row.name }}</span>
    </template>
    <template #cell-permission="{ value }">
      <span class="mono" style="font-size: 12px">{{ value || '-' }}</span>
    </template>
    <template #cell-type="{ value }">
      <Tag :kind="value === 1 ? 'violet' : value === 2 ? 'blue' : 'default'">
        {{ value === 1 ? '目录' : value === 2 ? '菜单' : '按钮' }}
      </Tag>
    </template>
    <template #cell-path="{ value }">
      <span class="mono" style="font-size: 12px">{{ value || '-' }}</span>
    </template>
    <template #cell-visible="{ row }">
      <StatusTag :kind="row.visible === false ? 'off' : 'ok'" :text="row.visible === false ? '隐藏' : '启用'" />
    </template>
    <template #cell-actions="{ row }">
      <PermissionButton code="system:menu:create" variant="link" label="新增子项" @click="openCreate(row as SysMenu)" />
      <PermissionButton code="system:menu:update" variant="link" label="编辑" @click="openEdit(row as MenuRow)" />
      <PermissionButton code="system:menu:delete" variant="link" danger label="删除" @click="remove(row as MenuRow)" />
    </template>
  </ListPage>

  <FormDialog :open="dialogOpen" :title="editingId ? '编辑菜单' : '新增菜单'" :busy="busy" width="wide" @close="dialogOpen = false" @submit="submit">
    <Field label="上级菜单">
      <Select v-model="form.parentId" :options="parentOptions" placeholder="顶级菜单" />
    </Field>
    <Field label="菜单名称" required>
      <TextInput v-model="form.name" placeholder="菜单名称" />
    </Field>
    <Field label="类型">
      <Select v-model="form.type" :options="typeOptions" />
    </Field>
    <Field label="路由路径">
      <TextInput v-model="form.path" placeholder="如 /system/user" />
    </Field>
    <Field label="图标">
      <TextInput v-model="form.icon" placeholder="如 ep:setting" />
    </Field>
    <Field label="组件路径">
      <TextInput v-model="form.component" placeholder="如 system/user/index" />
    </Field>
    <Field label="权限标识">
      <TextInput v-model="form.permission" placeholder="如 system:user:list" />
    </Field>
    <Field label="排序">
      <TextInput v-model="form.sort" type="number" placeholder="排序" />
    </Field>
    <Field label="状态">
      <Switch v-model="form.status" />
    </Field>
    <Field label="可见">
      <Switch v-model="form.visible" />
    </Field>
    <Field label="缓存 (keep_alive)">
      <Switch v-model="form.keep_alive" />
    </Field>
    <Field label="总是显示 (always_show)">
      <Switch v-model="form.always_show" />
    </Field>
  </FormDialog>
</template>
