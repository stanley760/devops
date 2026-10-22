<script setup lang="ts">
import { onBeforeUnmount, watch } from 'vue'
import { overlayEnter, overlayLeave } from '@/composables/motion'
import Button from './Button.vue'
import IconButton from './IconButton.vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    title?: string
    busy?: boolean
    confirmText?: string
    cancelText?: string
    danger?: boolean
    width?: 'md' | 'wide'
    hideFooter?: boolean
  }>(),
  {
    title: '',
    busy: false,
    confirmText: '保存',
    cancelText: '取消',
    danger: false,
    width: 'md',
    hideFooter: false,
  },
)

const emit = defineEmits<{
  close: []
  confirm: []
}>()

function onKey(e: KeyboardEvent): void {
  if (e.key === 'Escape') emit('close')
}

watch(
  () => props.open,
  (open) => {
    if (open) window.addEventListener('keydown', onKey)
    else window.removeEventListener('keydown', onKey)
  },
  { immediate: true },
)

onBeforeUnmount(() => window.removeEventListener('keydown', onKey))

const enter = (el: Element, done: () => void) => overlayEnter(el, done, 'modal')
const leave = (el: Element, done: () => void) => overlayLeave(el, done, 'modal')
</script>

<template>
  <Teleport to="body">
    <Transition :css="false" @enter="enter" @leave="leave">
      <div v-if="open" class="overlay-backdrop" @click="emit('close')" />
    </Transition>
    <Transition :css="false" @enter="enter" @leave="leave">
      <div v-if="open" class="modal m-float" :class="width === 'wide' && 'is-wide'">
        <div class="modal-head">
          <div class="modal-title">{{ title }}</div>
          <IconButton icon="ep:close" label="关闭" size="sm" @click="emit('close')" />
        </div>
        <div class="modal-body">
          <slot />
        </div>
        <div v-if="!hideFooter" class="modal-foot">
          <Button variant="ghost" @click="emit('close')">{{ cancelText }}</Button>
          <Button :variant="danger ? 'danger' : 'primary'" :loading="busy" @click="emit('confirm')">
            {{ confirmText }}
          </Button>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
