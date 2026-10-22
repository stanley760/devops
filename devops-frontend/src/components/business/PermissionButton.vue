<script setup lang="ts">
/**
 * 权限按钮：无对应权限码时整棵子树不渲染（不占位、不可聚焦）。
 */
import { computed } from 'vue'
import { Button } from '@/components/ui'
import type { ButtonVariant, ControlSize } from '@/components/ui/types'
import { hasPermission } from '@/composables/auth'
import Icon from '@/components/Icon.vue'

const props = withDefaults(
  defineProps<{
    /** 权限标识，如 system:user:create */
    code: string
    variant?: ButtonVariant
    size?: ControlSize
    icon?: string
    label?: string
    danger?: boolean
    disabled?: boolean
    loading?: boolean
  }>(),
  { variant: 'primary', size: 'md' },
)

const allowed = computed(() => hasPermission(props.code))
</script>

<template>
  <Button
    v-if="allowed"
    :variant="variant"
    :size="size"
    :danger="danger"
    :disabled="disabled"
    :loading="loading"
  >
    <template v-if="icon" #prefix><Icon :name="icon" :size="size === 'sm' ? 14 : 16" /></template>
    <slot>{{ label }}</slot>
  </Button>
</template>
