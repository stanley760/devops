<script setup lang="ts">
/**
 * 表单弹窗薄封装：统一「保存」确认语义与忙碌态，
 * 表单体（Field + 输入控件）由调用方以默认插槽注入。
 */
import { Modal } from '@/components/ui'

withDefaults(
  defineProps<{
    open: boolean
    title: string
    busy?: boolean
    width?: 'md' | 'wide'
  }>(),
  { busy: false, width: 'md' },
)

const emit = defineEmits<{ close: []; submit: [] }>()
</script>

<template>
  <Modal
    :open="open"
    :title="title"
    :busy="busy"
    :width="width"
    confirm-text="保存"
    @confirm="emit('submit')"
    @close="emit('close')"
  >
    <slot />
  </Modal>
</template>
