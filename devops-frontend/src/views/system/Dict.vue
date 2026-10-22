<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { dictApi, type DictType, type DictTypeSave, type DictData, type DictDataSave } from '@/api/system'
import { useListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column } from '@/components/ui/types'
import { Field, TextInput, Switch, StatusTag, Panel, PageHeader, DataTable, Pagination, Tag } from '@/components/ui'
import { FormDialog, PermissionButton } from '@/components/business'

type MutList<Row> = ListPageState<Row> & { afterMutate: (removedCount?: number) => Promise<void> }

const typeList = useListPage<DictType>({
  query: {},
  fetch: (p) => dictApi.type.list(p),
  rowKey: 'id',
  pageSize: 10,
}) as MutList<DictType>

const dataList = useListPage<DictData>({
  query: { dict_type: '' },
  fetch: (p) => dictApi.data.list(p),
  rowKey: 'id',
  pageSize: 10,
  immediate: false,
}) as MutList<DictData>

const selectedId = ref('')
const selectedType = ref('')

const typeColumns: Column[] = [
  { key: 'type', title: '字典类型', width: 180 },
  { key: 'name', title: '名称', width: 160 },
  { key: 'status', title: '状态', width: 90 },
  { key: 'remark', title: '备注' },
  { key: 'actions', title: '操作', width: 140, className: 'col-actions' },
]

const dataColumns: Column[] = [
  { key: 'label', title: '标签', width: 140 },
  { key: 'value', title: '值', width: 120 },
  { key: 'sort', title: '排序', width: 80, align: 'right' },
  { key: 'status', title: '状态', width: 90 },
  { key: 'cssClass', title: '样式类', width: 120 },
  { key: 'actions', title: '操作', width: 140, className: 'col-actions' },
]

function selectType(row: DictType): void {
  selectedId.value = row.id
  selectedType.value = row.type
  dataList.query.dict_type = row.type
  void dataList.search()
}

watch(
  () => typeList.rows.value,
  (rows) => {
    if (!selectedType.value) return
    const still = rows.some((r) => r.type === selectedType.value)
    if (!still) {
      selectedId.value = ''
      selectedType.value = ''
      dataList.rows.value = []
      dataList.total.value = 0
    }
  },
)

/* ---------------- 类型表单 ---------------- */

const typeDialog = ref(false)
const typeBusy = ref(false)
const typeEditingId = ref<string | null>(null)
const typeForm = reactive({ name: '', type: '', status: true, remark: '' })

function openTypeCreate(): void {
  typeEditingId.value = null
  Object.assign(typeForm, { name: '', type: '', status: true, remark: '' })
  typeDialog.value = true
}

function openTypeEdit(row: DictType): void {
  typeEditingId.value = row.id
  Object.assign(typeForm, {
    name: row.name,
    type: row.type,
    status: row.status !== 0,
    remark: row.remark ?? '',
  })
  typeDialog.value = true
}

async function submitType(): Promise<void> {
  if (!typeForm.name.trim() || !typeForm.type.trim()) {
    toastError(new Error('请填写类型名称与类型编码'))
    return
  }
  typeBusy.value = true
  try {
    const body: DictTypeSave = {
      name: typeForm.name.trim(),
      status: typeForm.status ? 1 : 0,
      remark: typeForm.remark,
    }
    if (typeEditingId.value) {
      await dictApi.type.update(typeEditingId.value, body)
    } else {
      body.type = typeForm.type.trim()
      await dictApi.type.create(body)
    }
    toastOk(typeEditingId.value ? '字典类型已更新' : '字典类型已创建')
    typeDialog.value = false
    await typeList.load()
  } catch (e) {
    toastError(e)
  } finally {
    typeBusy.value = false
  }
}

async function removeType(row: DictType): Promise<void> {
  const ok = await confirmAction({ title: '确认删除', text: '删除后不可恢复，确认继续？', danger: true })
  if (!ok) return
  typeBusy.value = true
  try {
    await dictApi.type.remove(row.id)
    toastOk('已删除')
    await typeList.afterMutate(1)
  } catch (e) {
    toastError(e)
  } finally {
    typeBusy.value = false
  }
}

/* ---------------- 数据项表单 ---------------- */

const dataDialog = ref(false)
const dataBusy = ref(false)
const dataEditingId = ref<string | null>(null)
const dataForm = reactive({ label: '', value: '', sort: 0, status: true, css_class: '' })

function openDataCreate(): void {
  dataEditingId.value = null
  Object.assign(dataForm, { label: '', value: '', sort: 0, status: true, css_class: '' })
  dataDialog.value = true
}

function openDataEdit(row: DictData): void {
  dataEditingId.value = row.id
  Object.assign(dataForm, {
    label: row.label,
    value: row.value,
    sort: row.sort ?? 0,
    status: row.status !== 0,
    css_class: row.cssClass ?? '',
  })
  dataDialog.value = true
}

async function submitData(): Promise<void> {
  if (!dataForm.label.trim() || !dataForm.value.trim()) {
    toastError(new Error('请填写标签与值'))
    return
  }
  if (!selectedType.value) {
    toastError(new Error('请先选择字典类型'))
    return
  }
  dataBusy.value = true
  try {
    const body: DictDataSave = {
      dict_type: selectedType.value,
      label: dataForm.label.trim(),
      value: dataForm.value.trim(),
      sort: Number(dataForm.sort) || 0,
      status: dataForm.status ? 1 : 0,
    }
    if (dataForm.css_class) body.css_class = dataForm.css_class
    if (dataEditingId.value) await dictApi.data.update(dataEditingId.value, body)
    else await dictApi.data.create(body)
    toastOk(dataEditingId.value ? '数据项已更新' : '数据项已创建')
    dataDialog.value = false
    await dataList.load()
  } catch (e) {
    toastError(e)
  } finally {
    dataBusy.value = false
  }
}

async function removeData(row: DictData): Promise<void> {
  const ok = await confirmAction({ title: '确认删除', text: '删除后不可恢复，确认继续？', danger: true })
  if (!ok) return
  dataBusy.value = true
  try {
    await dictApi.data.remove(row.id)
    toastOk('已删除')
    await dataList.afterMutate(1)
  } catch (e) {
    toastError(e)
  } finally {
    dataBusy.value = false
  }
}
</script>

<template>
  <div>
    <PageHeader title="字典管理" desc="字典类型与数据项维护" />
    <div style="display: flex; flex-direction: column; gap: 16px">
      <Panel title="字典类型">
        <template #tools>
          <PermissionButton code="system:dict:create" variant="primary" icon="ep:plus" label="新增类型" @click="openTypeCreate" />
        </template>
        <DataTable
          :columns="typeColumns"
          :rows="typeList.rows.value"
          :loading="typeList.loading.value"
          :selected="selectedId ? [selectedId] : []"
          row-key="id"
          min-width="720"
          @row-click="selectType"
        >
          <template #cell-type="{ row }">
            <span class="mono" style="cursor: pointer" @click="selectType(row)">{{ row.type }}</span>
          </template>
          <template #cell-name="{ row }">
            <span style="cursor: pointer" @click="selectType(row)">{{ row.name }}</span>
          </template>
          <template #cell-status="{ row }">
            <StatusTag :kind="row.status === 1 ? 'ok' : 'off'" :text="row.status === 1 ? '启用' : '停用'" />
          </template>
          <template #cell-actions="{ row }">
            <PermissionButton code="system:dict:update" variant="link" label="编辑" @click="openTypeEdit(row)" />
            <PermissionButton code="system:dict:delete" variant="link" danger label="删除" @click="removeType(row)" />
          </template>
        </DataTable>
        <Pagination
          :page="typeList.page.value"
          :page-size="typeList.pageSize.value"
          :total="typeList.total.value"
          @update:page="typeList.setPage"
          @update:page-size="typeList.setPageSize"
        />
      </Panel>

      <Panel :title="selectedType ? `数据项 · ${selectedType}` : '数据项'">
        <template #tools>
          <PermissionButton
            code="system:dict:create"
            variant="primary"
            icon="ep:plus"
            label="新增数据"
            :disabled="!selectedType"
            @click="openDataCreate"
          />
        </template>
        <div v-if="!selectedType" style="padding: 24px; opacity: 0.65; text-align: center">
          请选择上方的字典类型后查看数据项
        </div>
        <template v-else>
          <DataTable
            :columns="dataColumns"
            :rows="dataList.rows.value"
            :loading="dataList.loading.value"
            row-key="id"
            min-width="720"
          >
            <template #cell-value="{ value }">
              <span class="mono">{{ value }}</span>
            </template>
            <template #cell-status="{ row }">
              <StatusTag :kind="row.status === 1 ? 'ok' : 'off'" :text="row.status === 1 ? '启用' : '停用'" />
            </template>
            <template #cell-cssClass="{ value }">
              <Tag v-if="value" kind="violet">{{ value }}</Tag>
              <span v-else>-</span>
            </template>
            <template #cell-actions="{ row }">
              <PermissionButton code="system:dict:update" variant="link" label="编辑" @click="openDataEdit(row)" />
              <PermissionButton code="system:dict:delete" variant="link" danger label="删除" @click="removeData(row)" />
            </template>
          </DataTable>
          <Pagination
            :page="dataList.page.value"
            :page-size="dataList.pageSize.value"
            :total="dataList.total.value"
            @update:page="dataList.setPage"
            @update:page-size="dataList.setPageSize"
          />
        </template>
      </Panel>
    </div>

    <FormDialog :open="typeDialog" :title="typeEditingId ? '编辑字典类型' : '新增字典类型'" :busy="typeBusy" @close="typeDialog = false" @submit="submitType">
      <Field label="类型名称" required>
        <TextInput v-model="typeForm.name" placeholder="类型名称" />
      </Field>
      <Field label="类型编码" required :help="typeEditingId ? '创建后不可改' : undefined">
        <TextInput v-model="typeForm.type" placeholder="如 sys_user_gender" :disabled="Boolean(typeEditingId)" />
      </Field>
      <Field label="状态">
        <Switch v-model="typeForm.status" />
      </Field>
      <Field label="备注">
        <TextInput v-model="typeForm.remark" placeholder="备注" />
      </Field>
    </FormDialog>

    <FormDialog :open="dataDialog" :title="dataEditingId ? '编辑数据项' : '新增数据项'" :busy="dataBusy" @close="dataDialog = false" @submit="submitData">
      <Field label="标签" required>
        <TextInput v-model="dataForm.label" placeholder="显示标签" />
      </Field>
      <Field label="值" required>
        <TextInput v-model="dataForm.value" placeholder="字典值" />
      </Field>
      <Field label="排序">
        <TextInput v-model="dataForm.sort" type="number" placeholder="排序" />
      </Field>
      <Field label="样式类">
        <TextInput v-model="dataForm.css_class" placeholder="css_class 可空" />
      </Field>
      <Field label="状态">
        <Switch v-model="dataForm.status" />
      </Field>
    </FormDialog>
  </div>
</template>
