<script setup lang="ts">
import { computed, ref } from 'vue'
import { loginLogApi, type LoginLog } from '@/api/system'
import { useListPage, type ListPageState } from '@/composables/list'
import { confirmAction } from '@/composables/confirm'
import { toastOk, toastError } from '@/composables/toast'
import type { Column } from '@/components/ui/types'
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
} from '@/components/ui'
import { PermissionButton } from '@/components/business'

type MutList = ListPageState<LoginLog> & { afterMutate: (removedCount?: number) => Promise<void> }

const list = useListPage<LoginLog>({
  query: { username: '', status: '' },
  fetch: (p) => loginLogApi.list(p),
  rowKey: 'id',
}) as MutList

const query = list.query as { username: string; status: string | number }
const rows = list.rows
const total = list.total
const page = list.page
const pageSize = list.pageSize
const loading = list.loading
const selected = list.selected

const okCount = computed(() => rows.value.filter((r) => r.status === 1).length)
const failCount = computed(() => rows.value.filter((r) => (r.status ?? 0) === 0).length)

const columns: Column[] = [
  { key: 'username', title: '账号', width: 130 },
  { key: 'ip', title: 'IP', width: 130 },
  { key: 'location', title: '归属地', width: 130 },
  { key: 'browser', title: '浏览器', width: 120 },
  { key: 'os', title: '操作系统', width: 120 },
  { key: 'status', title: '状态', width: 100 },
  { key: 'msg', title: '提示' },
  { key: 'loginTime', title: '登录时间', width: 160 },
]

const statusOptions = [
  { value: '', label: '全部' },
  { value: 1, label: '成功' },
  { value: 0, label: '失败' },
]

const busy = ref(false)

async function batchRemove(): Promise<void> {
  const ids = selected.value.map(String)
  if (!ids.length) return
  const ok = await confirmAction({ title: '确认删除', text: `删除后不可恢复，确认删除 ${ids.length} 条记录？`, danger: true })
  if (!ok) return
  busy.value = true
  try {
    await loginLogApi.remove(ids)
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
    text: '确认清空全部登录日志？此操作不可撤销',
    danger: true,
    confirmLabel: '清空',
  })
  if (!ok) return
  busy.value = true
  try {
    await loginLogApi.clean()
    toastOk('登录日志已清空')
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
    <PageHeader title="登录日志" desc="登录鉴权审计" />
    <div style="display: flex; gap: 16px; margin-bottom: 16px">
      <Metric label="总记录" :value="total" sub="全部登录日志" />
      <Metric label="今日成功" :value="okCount" sub="按当前页估算" />
      <Metric label="今日失败" :value="failCount" sub="按当前页估算" />
    </div>

    <Panel title="登录记录">
      <template #tools>
        <IconButton icon="ep:refresh" label="刷新" :disabled="busy || loading" @click="list.load()" />
        <PermissionButton code="system:loginlog:clean" variant="danger" icon="ep:delete" label="清空" @click="cleanAll" />
      </template>

      <div style="display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-end; margin-bottom: 12px">
        <Field label="账号">
          <TextInput v-model="query.username" placeholder="登录账号" icon="ep:user" />
        </Field>
        <Field label="状态">
          <Select v-model="query.status" :options="statusOptions" placeholder="全部" />
        </Field>
        <Button variant="primary" @click="list.search()">查询</Button>
        <Button variant="ghost" @click="list.reset()">重置</Button>
      </div>

      <div v-if="selected.length" style="margin-bottom: 10px; display: flex; gap: 8px; align-items: center">
        <span style="opacity: 0.7">已选 {{ selected.length }} 项</span>
        <PermissionButton code="system:loginlog:remove" variant="danger" label="批量删除" @click="batchRemove" />
      </div>

      <DataTable
        v-model:selected="selected"
        :columns="columns"
        :rows="rows"
        :loading="loading"
        selectable
        row-key="id"
        min-width="980"
      >
        <template #cell-username="{ value }">
          <span class="mono">{{ value }}</span>
        </template>
        <template #cell-ip="{ value }">
          <span class="mono">{{ value || '-' }}</span>
        </template>
        <template #cell-status="{ row }">
          <StatusTag :kind="row.status === 1 ? 'ok' : 'crit'" :text="row.status === 1 ? '登录成功' : '失败'" />
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
  </div>
</template>
