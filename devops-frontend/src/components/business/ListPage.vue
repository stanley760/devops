<script setup lang="ts">
/**
 * 列表页壳：页头 / 搜索栏 / 数据面板 / 分页 / 批量浮条。
 * list 是「普通对象包 Ref」而非 reactive 代理，模板层一律经 computed 取 .value。
 */
import { computed, useSlots } from 'vue'
import type { ListPageState } from '@/composables/list'
import type { Column } from '@/components/ui/types'
import { Button, IconButton, PageHeader, Panel, DataTable, Pagination } from '@/components/ui'
import { floatBarEnter, floatBarLeave } from '@/composables/motion'
import SearchForm from './SearchForm.vue'

const props = withDefaults(
  defineProps<{
    title: string
    desc?: string
    list: ListPageState<unknown>
    columns: Column[]
    tableTitle?: string
    selectable?: boolean
    rowKey?: string
    minWidth?: string | number
  }>(),
  {
    tableTitle: '数据列表',
    selectable: false,
    rowKey: 'id',
  },
)

const slots = useSlots()

/* list 内部全是 Ref，模板不会自动解包嵌套 ref，逐项建 computed */
const rows = computed(() => props.list.rows.value)
const total = computed(() => props.list.total.value)
const page = computed(() => props.list.page.value)
const pageSize = computed(() => props.list.pageSize.value)
const loading = computed(() => props.list.loading.value)
const exporting = computed(() => props.list.exporting.value)
const selected = computed(() => props.list.selected.value)
const canExport = computed(() => props.list.canExport)

const metaText = computed(() => `共 ${total.value} 条`)

/** 透传 DataTable 的 cell-<key> 具名插槽 */
function cellSlotNames(): string[] {
  return Object.keys(slots).filter((n) => n.startsWith('cell-'))
}

function onSearch(): void {
  void props.list.search()
}
function onReset(): void {
  void props.list.reset()
}
function onLoad(): void {
  void props.list.load()
}
function onExport(): void {
  void props.list.exportData()
}
function onSelected(v: Array<string | number>): void {
  props.list.selected.value = v
}
function clearSelection(): void {
  props.list.selected.value = []
}
function onSetPage(p: number): void {
  void props.list.setPage(p)
}
function onSetPageSize(n: number): void {
  void props.list.setPageSize(n)
}
</script>

<template>
  <div>
    <PageHeader :title="title" :desc="desc">
      <slot name="spec" />
      <template #actions>
        <slot name="actions" />
      </template>
    </PageHeader>

    <SearchForm v-if="slots.search" @search="onSearch" @reset="onReset">
      <slot name="search" :query="list.query" />
    </SearchForm>

    <slot name="before" />

    <div :class="slots.side ? 'split' : undefined">
      <div :class="slots.side ? 'split-main' : undefined">
        <Panel :title="tableTitle" :meta="metaText" flush>
          <template #tools>
            <slot name="tools" />
            <IconButton icon="ep:refresh" label="刷新" size="sm" @click="onLoad" />
            <IconButton
              v-if="canExport"
              icon="ep:download"
              label="导出"
              size="sm"
              :disabled="exporting"
              @click="onExport"
            />
          </template>
          <slot />
          <DataTable
            :columns="columns"
            :rows="rows"
            :loading="loading"
            :selected="selected"
            :selectable="selectable"
            :row-key="rowKey"
            :min-width="minWidth"
            @update:selected="onSelected"
          >
            <template v-for="name in cellSlotNames()" #[name]="scope" :key="name">
              <slot :name="name" v-bind="scope ?? {}" />
            </template>
          </DataTable>
          <Pagination
            :page="page"
            :page-size="pageSize"
            :total="total"
            @update:page="onSetPage"
            @update:page-size="onSetPageSize"
          />
        </Panel>
      </div>
      <aside v-if="slots.side" class="split-side m-glass">
        <slot name="side" />
      </aside>
    </div>

    <!-- 批量操作浮条：深色铭牌质感，选中即浮出 -->
    <Teleport to="body">
      <Transition :css="false" @enter="floatBarEnter" @leave="floatBarLeave">
        <div v-if="selected.length > 0" class="float-bar m-plate">
          <span class="fb-count">已选 {{ selected.length }} 项</span>
          <slot name="batch" :ids="selected" />
          <Button size="sm" variant="ghost" style="color: var(--plate-ink)" @click="clearSelection">
            取消
          </Button>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
