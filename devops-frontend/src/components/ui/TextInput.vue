<script setup lang="ts">
import Icon from '../Icon.vue'
import type { ControlSize } from './types'

withDefaults(
  defineProps<{
    modelValue?: string | number
    placeholder?: string
    type?: string
    disabled?: boolean
    invalid?: boolean
    icon?: string
    size?: ControlSize
  }>(),
  {
    modelValue: '',
    placeholder: '',
    type: 'text',
    disabled: false,
    invalid: false,
    icon: '',
    size: 'md',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
  focus: [event: FocusEvent]
  blur: [event: FocusEvent]
}>()

function onInput(e: Event): void {
  emit('update:modelValue', (e.target as HTMLInputElement).value)
}

function onFocus(e: FocusEvent): void {
  emit('focus', e)
}

function onBlur(e: FocusEvent): void {
  emit('blur', e)
}
</script>

<template>
  <div v-if="icon" class="input-affix">
    <span class="affix-icon"><Icon :name="icon" :size="16" /></span>
    <input
      class="input"
      :class="{ 'is-invalid': invalid }"
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      @input="onInput"
      @focus="onFocus"
      @blur="onBlur"
    />
  </div>
  <input
    v-else
    class="input"
    :class="{ 'is-invalid': invalid }"
    :type="type"
    :value="modelValue"
    :placeholder="placeholder"
    :disabled="disabled"
    @input="onInput"
    @focus="onFocus"
    @blur="onBlur"
  />
</template>
