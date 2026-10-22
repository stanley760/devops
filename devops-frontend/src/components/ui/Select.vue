<script setup lang="ts">
import type { SelectOption } from './types'

const props = withDefaults(
  defineProps<{
    modelValue?: string | number
    options: SelectOption[]
    placeholder?: string
    disabled?: boolean
  }>(),
  {
    modelValue: '',
    placeholder: '请选择',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string | number]
}>()

function onChange(e: Event): void {
  const raw = (e.target as HTMLSelectElement).value
  const opt = props.options.find((o) => String(o.value) === raw)
  emit('update:modelValue', opt ? opt.value : raw)
}
</script>

<template>
  <select class="select" :value="modelValue" :disabled="disabled" @change="onChange">
    <option v-if="placeholder" value="" disabled>{{ placeholder }}</option>
    <option v-for="opt in options" :key="opt.value" :value="opt.value" :disabled="opt.disabled">
      {{ opt.label }}
    </option>
  </select>
</template>
