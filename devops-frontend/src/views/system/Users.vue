<script setup lang="ts">
import { reactive, ref } from 'vue'
import { userApi, type SysUser, type UserSave } from '@/api/system'
import { useListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column } from '@/components/ui/types'
import { Field, TextInput, Select, Switch, Modal, StatusTag } from '@/components/ui'
import { ListPage, FormDialog, PermissionButton } from '@/components/business'

type MutList = ListPageState<SysUser> & { afterMutate: (removedCount?: number) => Promise<void> }

const list = useListPage<SysUser>({
  query: { username: '', status: '' },
  fetch: (p) => userApi.list(p),
  rowKey: 'id',
}) as MutList

const query = list.query as { username: string; status: string | number }

const columns: Column[] = [
  { key: 'username', title: '用户名', width: 140 },
  { key: 'nickname', title: '昵称', width: 120 },
  { key: 'deptName', title: '部门', width: 140 },
  { key: 'mobile', title: '手机号', width: 130 },
  { key: 'email', title: '邮箱', width: 180 },
  { key: 'status', title: '状态', width: 90 },
  { key: 'loginDate', title: '最近登录', width: 160 },
  { key: 'actions', title: '操作', width: 230, className: 'col-actions' },
]

const statusOptions = [
  { value: '', label: '全部' },
  { value: 1, label: '启用' },
  { value: 0, label: '停用' },
]

const genderOptions = [
  { value: 0, label: '未知' },
  { value: 1, label: '男' },
  { value: 2, label: '女' },
]

const dialogOpen = ref(false)
const busy = ref(false)
const editingId = ref<string | null>(null)

function emptyForm() {
  return {
    username: '',
    nickname: '',
    password: '',
    email: '',
    mobile: '',
    gender: 0,
    remark: '',
    status: true,
  }
}

const form = reactive(emptyForm())

function openCreate(): void {
  editingId.value = null
  Object.assign(form, emptyForm())
  dialogOpen.value = true
}

async function openEdit(row: SysUser): Promise<void> {
  busy.value = true
  try {
    const u = await userApi.get(row.id)
    editingId.value = row.id
    Object.assign(form, {
      username: u.username,
      nickname: u.nickname ?? '',
      password: '',
      email: u.email ?? '',
      mobile: u.mobile ?? '',
      gender: u.gender ?? 0,
      remark: u.remark ?? '',
      status: u.status !== 0,
    })
    dialogOpen.value = true
  } catch (e) {
    toastError(e, '加载用户失败')
  } finally {
    busy.value = false
  }
}

async function submit(): Promise<void> {
  if (!form.username.trim() || !form.nickname.trim()) {
    toastError(new Error('请填写用户名与昵称'))
    return
  }
  if (!editingId.value && !form.password) {
    toastError(new Error('请填写初始密码'))
    return
  }
  busy.value = true
  try {
    const body: UserSave = {
      username: form.username.trim(),
      nickname: form.nickname.trim(),
      deptId: '',
      email: form.email,
      mobile: form.mobile,
      gender: Number(form.gender) || 0,
      remark: form.remark,
      status: form.status ? 1 : 0,
    }
    if (form.password) body.password = form.password
    if (editingId.value) await userApi.update(editingId.value, body)
    else await userApi.create(body)
    toastOk(editingId.value ? '用户已更新' : '用户已创建')
    dialogOpen.value = false
    await list.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

async function remove(row: SysUser): Promise<void> {
  const ok = await confirmAction({ title: '确认删除', text: '删除后不可恢复，确认继续？', danger: true })
  if (!ok) return
  busy.value = true
  try {
    await userApi.remove(row.id)
    toastOk('已删除')
    await list.afterMutate(1)
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

const pwdOpen = ref(false)
const pwdBusy = ref(false)
const pwdValue = ref('')
const pwdTarget = ref<SysUser | null>(null)

function openReset(row: SysUser): void {
  pwdTarget.value = row
  pwdValue.value = ''
  pwdOpen.value = true
}

async function submitPassword(): Promise<void> {
  if (!pwdTarget.value || !pwdValue.value) {
    toastError(new Error('请输入新密码'))
    return
  }
  pwdBusy.value = true
  try {
    await userApi.resetPassword(pwdTarget.value.id, pwdValue.value)
    toastOk('密码已重置')
    pwdOpen.value = false
  } catch (e) {
    toastError(e)
  } finally {
    pwdBusy.value = false
  }
}
</script>

<template>
  <ListPage title="用户管理" desc="系统账号、状态与登录信息" :list="list" :columns="columns" table-title="用户列表" row-key="id">
    <template #actions>
      <PermissionButton code="system:user:create" variant="primary" icon="ep:plus" label="新增用户" @click="openCreate" />
    </template>

    <template #search>
      <Field label="用户名">
        <TextInput v-model="query.username" placeholder="用户名" icon="ep:user" />
      </Field>
      <Field label="状态">
        <Select v-model="query.status" :options="statusOptions" placeholder="全部" />
      </Field>
    </template>

    <template #cell-username="{ value }">
      <span class="mono">{{ value }}</span>
    </template>
    <template #cell-status="{ row }">
      <StatusTag :kind="row.status === 1 ? 'ok' : 'off'" :text="row.status === 1 ? '启用' : '停用'" />
    </template>
    <template #cell-actions="{ row }">
      <PermissionButton code="system:user:update" variant="link" label="编辑" @click="openEdit(row)" />
      <PermissionButton code="system:user:update-password" variant="link" label="重置密码" @click="openReset(row)" />
      <PermissionButton code="system:user:delete" variant="link" danger label="删除" @click="remove(row)" />
    </template>
  </ListPage>

  <FormDialog :open="dialogOpen" :title="editingId ? '编辑用户' : '新增用户'" :busy="busy" @close="dialogOpen = false" @submit="submit">
    <Field label="用户名" required>
      <TextInput v-model="form.username" placeholder="登录用户名" :disabled="Boolean(editingId)" />
    </Field>
    <Field label="昵称" required>
      <TextInput v-model="form.nickname" placeholder="显示名称" />
    </Field>
    <Field v-if="!editingId" label="初始密码" required>
      <TextInput v-model="form.password" type="password" placeholder="初始密码" />
    </Field>
    <Field label="邮箱">
      <TextInput v-model="form.email" placeholder="邮箱" />
    </Field>
    <Field label="手机号">
      <TextInput v-model="form.mobile" placeholder="手机号" />
    </Field>
    <Field label="性别">
      <Select v-model="form.gender" :options="genderOptions" />
    </Field>
    <Field label="备注">
      <TextInput v-model="form.remark" placeholder="备注" />
    </Field>
    <Field label="状态">
      <Switch v-model="form.status" />
    </Field>
  </FormDialog>

  <Modal :open="pwdOpen" title="重置密码" :busy="pwdBusy" confirm-text="重置" danger width="md" @close="pwdOpen = false" @confirm="submitPassword">
    <Field label="新密码" required>
      <TextInput v-model="pwdValue" type="password" placeholder="请输入新密码" />
    </Field>
  </Modal>
</template>
