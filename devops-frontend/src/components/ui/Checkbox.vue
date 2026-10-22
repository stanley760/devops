<script setup lang="ts">
withDefaults(
  defineProps<{
    modelValue?: boolean
    label?: string
    disabled?: boolean
  }>(),
  {
    modelValue: false,
    label: '',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

function onChange(e: Event): void {
  emit('update:modelValue', (e.target as HTMLInputElement).checked)
}
</script>

<template>
  <label class="checkbox">
    <input type="checkbox" :checked="modelValue" :disabled="disabled" @change="onChange" />
    <span v-if="label">{{ label }}</span>
    <slot />
  </label>
</template>
