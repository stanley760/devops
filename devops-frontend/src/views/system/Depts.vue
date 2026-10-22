<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { deptApi, userApi, type SysDept, type DeptSave } from '@/api/system'
import { useTreeListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column, SelectOption } from '@/components/ui/types'
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

const dialogOpen = ref(false)
const busy = ref(false)
const editingId = ref<string | null>(null)

/** 负责人候选：远程搜索用户（用户量大，不整表拉取）。 */
const leaderOptions = ref<SelectOption[]>([])
const leaderLoading = ref(false)

async function searchLeader(kw: string): Promise<void> {
  leaderLoading.value = true
  try {
    const res = await userApi.list({ page_num: 1, page_size: 20, username: kw.trim() })
    leaderOptions.value = (res?.list ?? []).map((u) => ({
      value: u.id,
      label: `${u.nickname || u.username}`,
    }))
  } catch (e) {
    // 无 user:list 权限或网络失败：留空并提示，不阻断表单其余字段
    leaderOptions.value = []
    toastError(e, '加载负责人候选失败')
  } finally {
    leaderLoading.value = false
  }
}

function emptyForm() {
  return {
    name: '',
    parentId: '0',
    sort: 0,
    leader_user_id: '',
    phone: '',
    email: '',
    status: true,
  }
}

const form = reactive(emptyForm())

const emailError = computed(() => {
  const v = form.email.trim()
  if (!v) return ''
  return /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(v) ? '' : '邮箱格式不正确'
})

function openCreate(parent?: DeptRow): void {
  editingId.value = null
  Object.assign(form, emptyForm())
  form.parentId = parent?.id ?? '0'
  void searchLeader('')
  dialogOpen.value = true
}

function openEdit(row: DeptRow): void {
  editingId.value = row.id
  Object.assign(form, {
    name: row.name,
    // 后端 ID 解析：顶级是 0，不是空串（空串会 400）
    parentId: row.parentId && row.parentId !== '0' ? row.parentId : '0',
    sort: row.sort ?? 0,
    leader_user_id: row.leaderUserId && row.leaderUserId !== '0' ? row.leaderUserId : '',
    phone: row.phone ?? '',
    email: row.email ?? '',
    status: row.status !== 0,
  })
  // 已有负责人时补一个候选项，否则 Select 显示成空白
  leaderOptions.value = row.leaderUserId && row.leaderUserId !== '0'
    ? [{ value: row.leaderUserId, label: row.leaderName || row.leaderUserId }]
    : []
  void searchLeader('')
  dialogOpen.value = true
}

/**
 * 父部门选项排除自身及其子树，否则可以把自己挂到自己下面形成环
 * （后端不校验环，树会递归到栈溢出）。
 */
const parentOptions = computed<SelectOption[]>(() => {
  const excluded = new Set<string>()
  const editing = editingId.value
  if (editing) {
    excluded.add(editing)
    // 自身及其子树：从原始树里找（扁平行里 children 已被剥掉）
    const markSubtree = (nodes: SysDept[]): boolean => {
      for (const n of nodes) {
        if (n.id === editing) {
          const walk = (subs: SysDept[]): void => {
            for (const s of subs) {
              excluded.add(s.id)
              if (s.children?.length) walk(s.children)
            }
          }
          walk(n.children ?? [])
          return true
        }
        if (n.children?.length && markSubtree(n.children)) return true
      }
      return false
    }
    markSubtree(treeList.rows.value)
  }
  const opts: SelectOption[] = [{ value: '0', label: '顶级部门' }]
  for (const r of flatRows.value) {
    if (excluded.has(r.id)) continue
    opts.push({ value: r.id, label: `${'　'.repeat(r.depth)}${r.name}` })
  }
  return opts
})

async function submit(): Promise<void> {
  if (!form.name.trim()) {
    toastError(new Error('请填写部门名称'))
    return
  }
  if (form.parentId && form.parentId === editingId.value) {
    toastError(new Error('上级部门不能选择自身'))
    return
  }
  if (emailError.value) {
    toastError(new Error(emailError.value))
    return
  }
  busy.value = true
  try {
    const body: DeptSave = {
      name: form.name.trim(),
      // 顶级传 '0'；空串会被后端 ID 解析拒绝
      parentId: form.parentId || '0',
      sort: Number(form.sort) || 0,
      status: form.status ? 1 : 0,
    }
    if (form.leader_user_id) body.leader_user_id = form.leader_user_id
    if (form.phone.trim()) body.phone = form.phone.trim()
    // 邮箱必须省略而非传空串：后端 format 校验器对 "" 也会执行（400）
    if (form.email.trim()) body.email = form.email.trim()
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
    <template #cell-leaderName="{ value }">
      <span>{{ value || '-' }}</span>
    </template>
    <template #cell-phone="{ value }">
      <span class="mono">{{ value || '-' }}</span>
    </template>
    <template #cell-email="{ value }">
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
    <Field label="上级部门" help="顶级部门请选「顶级部门」，不要留空">
      <Select v-model="form.parentId" :options="parentOptions" placeholder="顶级部门" />
    </Field>
    <Field label="负责人" :help="leaderLoading ? '加载中…' : '输入关键字可搜索用户，选填'">
      <Select v-model="form.leader_user_id" :options="leaderOptions" placeholder="不设负责人" />
    </Field>
    <Field label="联系电话">
      <TextInput v-model="form.phone" placeholder="联系电话" />
    </Field>
    <Field label="邮箱" :error="emailError">
      <TextInput v-model="form.email" placeholder="选填，格式 name@example.com" />
    </Field>
    <Field label="排序">
      <TextInput v-model="form.sort" type="number" placeholder="排序" />
    </Field>
    <Field label="状态">
      <Switch v-model="form.status" />
    </Field>
  </FormDialog>
</template>
