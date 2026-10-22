<script setup lang="ts">
import { computed } from 'vue'
import type { ButtonVariant, ControlSize } from './types'

const props = withDefaults(
  defineProps<{
    variant?: ButtonVariant
    size?: ControlSize
    block?: boolean
    loading?: boolean
    disabled?: boolean
    danger?: boolean
  }>(),
  {
    variant: 'secondary',
    size: 'md',
    block: false,
    loading: false,
    disabled: false,
    danger: false,
  },
)

const cls = computed(() => [
  props.variant === 'link' ? 'btn-link' : `btn-${props.variant}`,
  props.size === 'sm' ? 'btn-sm' : '',
  props.block ? 'btn-block' : '',
  props.loading ? 'btn-loading' : '',
  props.danger && props.variant === 'link' ? 'is-danger' : '',
])
</script>

<template>
  <button type="button" class="btn" :class="cls" :disabled="disabled || loading">
    <slot name="prefix" />
    <slot />
  </button>
</template>
