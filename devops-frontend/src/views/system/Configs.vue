<script setup lang="ts">
import { reactive, ref } from 'vue'
import { configApi, type SysConfig, type ConfigSave } from '@/api/system'
import { useListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column } from '@/components/ui/types'
import { Field, TextInput, Select, Switch, Tag } from '@/components/ui'
import { ListPage, FormDialog, PermissionButton } from '@/components/business'

type MutList = ListPageState<SysConfig> & { afterMutate: (removedCount?: number) => Promise<void> }

const list = useListPage<SysConfig>({
  query: { config_name: '' },
  fetch: (p) => configApi.list(p),
  rowKey: 'id',
}) as MutList

const query = list.query as { config_name: string }

const columns: Column[] = [
  { key: 'configKey', title: '参数键', width: 200 },
  { key: 'configName', title: '参数名称', width: 160 },
  { key: 'configValue', title: '参数值', width: 220 },
  { key: 'type', title: '类型', width: 90 },
  { key: 'visible', title: '可见', width: 80 },
  { key: 'remark', title: '备注' },
  { key: 'actions', title: '操作', width: 140, className: 'col-actions' },
]

const typeOptions = [
  { value: 1, label: '内置' },
  { value: 2, label: '自定义' },
]

const dialogOpen = ref(false)
const busy = ref(false)
const editingId = ref<string | null>(null)

function emptyForm() {
  return { config_key: '', config_name: '', config_value: '', type: 2, visible: true, remark: '' }
}

const form = reactive(emptyForm())

function openCreate(): void {
  editingId.value = null
  Object.assign(form, emptyForm())
  dialogOpen.value = true
}

function openEdit(row: SysConfig): void {
  editingId.value = row.id
  Object.assign(form, {
    config_key: row.configKey,
    config_name: row.configName,
    config_value: row.configValue ?? '',
    type: row.type ?? 2,
    visible: row.visible !== false,
    remark: row.remark ?? '',
  })
  dialogOpen.value = true
}

async function submit(): Promise<void> {
  if (!form.config_name.trim()) {
    toastError(new Error('请填写参数名称'))
    return
  }
  if (!editingId.value && !form.config_key.trim()) {
    toastError(new Error('请填写参数键'))
    return
  }
  busy.value = true
  try {
    const body: ConfigSave = {
      config_name: form.config_name.trim(),
      config_value: form.config_value,
      type: Number(form.type) || 2,
      visible: form.visible,
      remark: form.remark,
    }
    if (editingId.value) {
      await configApi.update(editingId.value, body)
    } else {
      body.config_key = form.config_key.trim()
      await configApi.create(body)
    }
    toastOk(editingId.value ? '参数已更新' : '参数已创建')
    dialogOpen.value = false
    await list.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

async function remove(row: SysConfig): Promise<void> {
  const ok = await confirmAction({ title: '确认删除', text: '删除后不可恢复，确认继续？', danger: true })
  if (!ok) return
  busy.value = true
  try {
    await configApi.remove(row.id)
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
  <ListPage title="参数配置" desc="运行时系统参数与开关" :list="list" :columns="columns" table-title="参数列表" row-key="id" min-width="960">
    <template #actions>
      <PermissionButton code="system:config:create" variant="primary" icon="ep:plus" label="新增参数" @click="openCreate" />
    </template>

    <template #search>
      <Field label="参数名称">
        <TextInput v-model="query.config_name" placeholder="参数名称" />
      </Field>
    </template>

    <template #cell-configKey="{ value }">
      <span class="mono">{{ value }}</span>
    </template>
    <template #cell-configValue="{ value }">
      <span class="mono" :title="String(value)" style="display: inline-block; max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap">{{ value }}</span>
    </template>
    <template #cell-type="{ value }">
      <Tag :kind="value === 1 ? 'violet' : 'blue'">{{ value === 1 ? '内置' : '自定义' }}</Tag>
    </template>
    <template #cell-visible="{ row }">
      <span>{{ row.visible === false ? '否' : '是' }}</span>
    </template>
    <template #cell-actions="{ row }">
      <PermissionButton code="system:config:update" variant="link" label="编辑" @click="openEdit(row)" />
      <PermissionButton code="system:config:delete" variant="link" danger label="删除" @click="remove(row)" />
    </template>
  </ListPage>

  <FormDialog :open="dialogOpen" :title="editingId ? '编辑参数' : '新增参数'" :busy="busy" @close="dialogOpen = false" @submit="submit">
    <Field label="参数键" required :help="editingId ? '创建后不可改' : undefined">
      <TextInput v-model="form.config_key" placeholder="如 sys.user.initPassword" :disabled="Boolean(editingId)" />
    </Field>
    <Field label="参数名称" required>
      <TextInput v-model="form.config_name" placeholder="参数名称" />
    </Field>
    <Field label="参数值">
      <TextInput v-model="form.config_value" placeholder="参数值" />
    </Field>
    <Field label="类型">
      <Select v-model="form.type" :options="typeOptions" />
    </Field>
    <Field label="可见">
      <Switch v-model="form.visible" />
    </Field>
    <Field label="备注">
      <TextInput v-model="form.remark" placeholder="备注" />
    </Field>
  </FormDialog>
</template>
