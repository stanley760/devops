<script lang="ts">
export type { Column } from './types'
</script>

<script setup lang="ts">
import { useSlots } from 'vue'
import Checkbox from './Checkbox.vue'
import type { Column } from './types'

const props = withDefaults(
  defineProps<{
    columns: Column[]
    rows: any[]
    rowKey?: string
    loading?: boolean
    selected?: Array<string | number>
    selectable?: boolean
    minWidth?: string | number
    emptyText?: string
  }>(),
  {
    rowKey: 'id',
    loading: false,
    selected: () => [],
    selectable: false,
    minWidth: undefined,
    emptyText: '暂无数据',
  },
)

const emit = defineEmits<{
  'update:selected': [selected: Array<string | number>]
  rowClick: [row: any]
}>()

const slots = useSlots()

function cellSlot(key: string) {
  return slots[`cell-${key}`]
}

function rowKeyOf(row: any): string | number {
  return row[props.rowKey]
}

function isSelected(row: any): boolean {
  return props.selected.includes(rowKeyOf(row))
}

function toggleRow(row: any, on: boolean): void {
  const key = rowKeyOf(row)
  const next = on
    ? props.selected.includes(key)
      ? props.selected
      : [...props.selected, key]
    : props.selected.filter((k) => k !== key)
  emit('update:selected', next)
}

function thStyle(col: Column) {
  return {
    width: col.width == null ? undefined : typeof col.width === 'number' ? `${col.width}px` : col.width,
    textAlign: col.align ?? 'left',
  }
}

function tableStyle() {
  return {
    minWidth:
      props.minWidth == null
        ? undefined
        : typeof props.minWidth === 'number'
          ? `${props.minWidth}px`
          : props.minWidth,
  }
}
</script>

<template>
  <div :class="loading && 'table-loading'">
    <div class="table-wrap">
      <table class="table" :style="tableStyle()">
        <thead>
          <tr>
            <th v-if="selectable" />
            <th v-for="col in columns" :key="col.key" :class="col.className" :style="thStyle(col)">
              {{ col.title }}
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="!rows.length">
            <td :colspan="columns.length + (selectable ? 1 : 0)">
              <div class="table-empty">{{ emptyText }}</div>
            </td>
          </tr>
          <tr
            v-for="row in rows"
            :key="rowKeyOf(row)"
            :class="{ 'is-selected': isSelected(row) }"
            @click="emit('rowClick', row)"
          >
            <td v-if="selectable" @click.stop>
              <Checkbox :model-value="isSelected(row)" @update:model-value="(v: boolean) => toggleRow(row, v)" />
            </td>
            <td v-for="col in columns" :key="col.key" :class="col.className">
              <template v-if="cellSlot(col.key)">
                <component :is="cellSlot(col.key)" :row="row" :value="row[col.key]" />
              </template>
              <template v-else>{{ row[col.key] }}</template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
