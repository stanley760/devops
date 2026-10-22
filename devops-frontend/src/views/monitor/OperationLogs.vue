<script setup lang="ts">
import { computed, ref } from 'vue'
import { operationLogApi, type OperationLog } from '@/api/system'
import { useListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column, SpecRow } from '@/components/ui/types'
import {
  Button,
  IconButton,
  Field,
  TextInput,
  Select,
  StatusTag,
  Panel,
  PageHeader,
  DataTable,
  Pagination,
  Metric,
  Drawer,
  SpecPlate,
} from '@/components/ui'
import { PermissionButton } from '@/components/business'

type MutList = ListPageState<OperationLog> & { afterMutate: (removedCount?: number) => Promise<void> }

const list = useListPage<OperationLog>({
  query: { title: '', status: '' },
  fetch: (p) => operationLogApi.list(p),
  rowKey: 'id',
}) as MutList

const query = list.query as { title: string; status: string | number }
const rows = list.rows
const total = list.total
const page = list.page
const pageSize = list.pageSize
const loading = list.loading
const selected = list.selected

const errorCount = computed(() => rows.value.filter((r) => (r.status ?? 0) !== 0).length)
const avgCost = computed(() => {
  const data = rows.value
  if (!data.length) return 0
  return Math.round(data.reduce((s, r) => s + (r.costMs ?? 0), 0) / data.length)
})

const columns: Column[] = [
  { key: 'title', title: '操作', width: 140 },
  { key: 'action', title: '动作', width: 150 },
  { key: 'method', title: '方法', width: 90 },
  { key: 'operatorName', title: '操作人', width: 110 },
  { key: 'ip', title: 'IP', width: 130 },
  { key: 'status', title: '状态', width: 90 },
  { key: 'costMs', title: '耗时', width: 90, align: 'right' },
  { key: 'operTime', title: '操作时间', width: 160 },
  { key: 'actions', title: '操作', width: 90, className: 'col-actions' },
]

const statusOptions = [
  { value: '', label: '全部' },
  { value: 0, label: '成功' },
  { value: 1, label: '失败' },
]

const busy = ref(false)
const detailOpen = ref(false)
const detail = ref<OperationLog | null>(null)

const detailRows = computed<SpecRow[]>(() => {
  const d = detail.value
  if (!d) return []
  return [
    { key: '标题', val: d.title ?? '-' },
    { key: '动作', val: d.action ?? '-' },
    { key: '方法', val: d.method ?? '-' },
    { key: '操作人', val: d.operatorName ?? d.operator ?? '-' },
    { key: 'IP', val: d.ip ?? '-' },
    { key: '状态', val: (d.status ?? 0) === 0 ? '成功' : '失败' },
    { key: '耗时', val: `${d.costMs ?? 0} ms` },
    { key: '操作时间', val: d.operTime ?? '-' },
    { key: '错误信息', val: d.errorMsg ?? '-' },
  ]
})

function openDetail(row: OperationLog): void {
  detail.value = row
  detailOpen.value = true
}

async function batchRemove(): Promise<void> {
  const ids = selected.value.map(String)
  if (!ids.length) return
  const ok = await confirmAction({ title: '确认删除', text: `删除后不可恢复，确认删除 ${ids.length} 条记录？`, danger: true })
  if (!ok) return
  busy.value = true
  try {
    await operationLogApi.remove(ids)
    toastOk('已删除')
    await list.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}

async function cleanAll(): Promise<void> {
  const ok = await confirmAction({
    title: '确认清空',
    text: '确认清空全部操作日志？此操作不可撤销',
    danger: true,
    confirmLabel: '清空',
  })
  if (!ok) return
  busy.value = true
  try {
    await operationLogApi.clean()
    toastOk('操作日志已清空')
    await list.load()
  } catch (e) {
    toastError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div>
    <PageHeader title="操作日志" desc="后台操作审计与耗时分析" />
    <div style="display: flex; gap: 16px; margin-bottom: 16px">
      <Metric label="总记录" :value="total" sub="全部操作日志" />
      <Metric label="异常操作" :value="errorCount" sub="按当前页估算" />
      <Metric label="平均耗时" :value="avgCost" sub="ms · 按当前页估算" />
    </div>

    <Panel title="操作记录">
      <template #tools>
        <IconButton icon="ep:refresh" label="刷新" :disabled="busy || loading" @click="list.load()" />
        <PermissionButton code="system:operationlog:clean" variant="danger" icon="ep:delete" label="清空" @click="cleanAll" />
      </template>

      <div style="display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; margin-bottom: 12px">
        <Field label="操作标题">
          <TextInput v-model="query.title" placeholder="操作标题" />
        </Field>
        <Field label="状态">
          <Select v-model="query.status" :options="statusOptions" placeholder="全部" />
        </Field>
        <Button variant="primary" @click="list.search()">查询</Button>
        <Button variant="ghost" @click="list.reset()">重置</Button>
      </div>

      <div v-if="selected.length" style="margin-bottom: 10px; display: flex; gap: 8px; align-items: center">
        <span style="opacity: 0.7">已选 {{ selected.length }} 项</span>
        <PermissionButton code="system:operationlog:remove" variant="danger" label="批量删除" @click="batchRemove" />
      </div>

      <DataTable
        v-model:selected="selected"
        :columns="columns"
        :rows="rows"
        :loading="loading"
        selectable
        row-key="id"
        min-width="1020"
      >
        <template #cell-action="{ value }">
          <span class="mono" style="font-size: 12px">{{ value || '-' }}</span>
        </template>
        <template #cell-method="{ value }">
          <span class="mono">{{ value || '-' }}</span>
        </template>
        <template #cell-ip="{ value }">
          <span class="mono">{{ value || '-' }}</span>
        </template>
        <template #cell-status="{ row }">
          <StatusTag :kind="(row.status ?? 0) === 0 ? 'ok' : 'crit'" :text="(row.status ?? 0) === 0 ? '成功' : '失败'" />
        </template>
        <template #cell-costMs="{ value }">
          <span class="num">{{ value ?? 0 }}</span><span class="ro-unit">ms</span>
        </template>
        <template #cell-actions="{ row }">
          <Button variant="link" @click="openDetail(row)">详情</Button>
        </template>
      </DataTable>

      <Pagination
        :page="page"
        :page-size="pageSize"
        :total="total"
        @update:page="list.setPage"
        @update:page-size="list.setPageSize"
      />
    </Panel>

    <Drawer :open="detailOpen" title="操作详情" hide-footer width="wide" @close="detailOpen = false">
      <SpecPlate title="操作详情" :rows="detailRows" />
    </Drawer>
  </div>
</template>
