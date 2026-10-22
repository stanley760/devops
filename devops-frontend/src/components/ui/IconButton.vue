<script setup lang="ts">
import Icon from '../Icon.vue'
import type { ControlSize } from './types'

withDefaults(
  defineProps<{
    icon: string
    label: string
    size?: ControlSize
    disabled?: boolean
  }>(),
  {
    size: 'md',
    disabled: false,
  },
)
</script>

<template>
  <button
    type="button"
    class="icon-btn"
    :class="{ 'icon-btn-sm': size === 'sm' }"
    :aria-label="label"
    :title="label"
    :disabled="disabled"
  >
    <Icon :name="icon" />
  </button>
</template>
