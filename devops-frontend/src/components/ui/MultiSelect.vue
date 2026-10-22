<script setup lang="ts">
/**
 * 多选控件：在选项集合上做多选，v-model 为选中项的 value 数组。
 *
 * 为什么不用原生 multiple select：原生多选框在窄栏里只能按住 Ctrl 点选，
 * 可用性很差。这里用一列可勾选项 + 全选/清空，交互与单选 Select 对齐。
 *
 * 约定：options 为空时渲染空态提示而不是空白（通常是调用方无列表权限），
 * 并通过 disabled 让整个控件失效，避免用户对着空框猜。
 */
import { computed } from 'vue'
import Checkbox from './Checkbox.vue'
import type { SelectOption } from './types'

const props = withDefaults(
  defineProps<{
    modelValue?: Array<string | number>
    options: SelectOption[]
    placeholder?: string
    disabled?: boolean
    /** 每列最多几项，超出换行 */
    columns?: number
  }>(),
  {
    modelValue: () => [],
    placeholder: '暂无可选项',
    disabled: false,
    columns: 2,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: Array<string | number>]
}>()

const selected = computed(() => new Set(props.modelValue.map(String)))

function toggle(value: string | number, checked: boolean): void {
  const next = new Map(props.modelValue.map((v) => [String(v), v]))
  if (checked) next.set(String(value), value)
  else next.delete(String(value))
  emit('update:modelValue', [...next.values()])
}

function toggleAll(checked: boolean): void {
  emit('update:modelValue', checked ? props.options.map((o) => o.value) : [])
}

const allChecked = computed(
  () => props.options.length > 0 && props.options.every((o) => selected.value.has(String(o.value))),
)
</script>

<template>
  <div class="multi-select" :style="{ '--multi-columns': String(columns) }">
    <div v-if="!options.length" class="multi-empty">{{ placeholder }}</div>
    <template v-else>
      <div class="multi-toolbar">
        <Checkbox
          :model-value="allChecked"
          :disabled="disabled"
          :label="allChecked ? '取消全选' : '全选'"
          @update:model-value="toggleAll"
        />
        <span class="multi-count">已选 {{ modelValue.length }} / {{ options.length }}</span>
      </div>
      <div class="multi-items">
        <label v-for="opt in options" :key="String(opt.value)" class="multi-item">
          <Checkbox
            :model-value="selected.has(String(opt.value))"
            :label="opt.label"
            :disabled="disabled || opt.disabled"
            @update:model-value="(v: boolean) => toggle(opt.value, v)"
          />
        </label>
      </div>
    </template>
  </div>
</template>

<style scoped>
.multi-select {
  border: 1px solid var(--glass-stroke-2);
  border-radius: var(--r-s);
  background: var(--glass-1-strong);
  box-shadow: var(--glass-inner-soft);
  padding: 8px 10px;
  max-height: 220px;
  overflow-y: auto;
}

.multi-empty {
  padding: 10px 2px;
  font-size: 12px;
  color: var(--ink-4);
}

.multi-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-bottom: 8px;
  margin-bottom: 8px;
  border-bottom: 1px dashed var(--glass-stroke-2);
}

.multi-count {
  font-size: 11px;
  color: var(--ink-4);
  font-variant-numeric: tabular-nums;
}

.multi-items {
  display: grid;
  grid-template-columns: repeat(var(--multi-columns, 2), minmax(0, 1fr));
  gap: 2px 12px;
}

.multi-item {
  min-width: 0;
}
</style>